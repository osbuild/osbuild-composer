// Package store contains primitives for representing and changing the
// osbuild-composer state.
package store

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"github.com/osbuild/osbuild-composer/internal/target"

	"github.com/osbuild/osbuild-composer/internal/distro"
	_ "github.com/osbuild/osbuild-composer/internal/distro/fedora30"

	"github.com/google/uuid"
)

// A Store contains all the persistent state of osbuild-composer, and is serialized
// on every change, and deserialized on start.
type Store struct {
	Blueprints        map[string]blueprint.Blueprint         `json:"blueprints"`
	Workspace         map[string]blueprint.Blueprint         `json:"workspace"`
	Composes          map[uuid.UUID]Compose                  `json:"composes"`
	Sources           map[string]SourceConfig                `json:"sources"`
	BlueprintsChanges map[string]map[string]blueprint.Change `json:"changes"`

	mu           sync.RWMutex // protects all fields
	pendingJobs  chan Job
	stateChannel chan []byte
}

// A Compose represent the task of building one image. It contains all the information
// necessary to generate the inputs for the job, as well as the job's state.
type Compose struct {
	QueueStatus string               `json:"queue_status"`
	Blueprint   *blueprint.Blueprint `json:"blueprint"`
	OutputType  string               `json:"output-type"`
	Targets     []*target.Target     `json:"targets"`
	JobCreated  time.Time            `json:"job_created"`
	JobStarted  time.Time            `json:"job_started"`
	JobFinished time.Time            `json:"job_finished"`
}

// A Job contains the information about a compose a worker needs to process it.
type Job struct {
	ComposeID uuid.UUID
	Pipeline  *pipeline.Pipeline
	Targets   []*target.Target
}

// An Image represents the image resulting from a compose.
type Image struct {
	File *os.File
	Name string
	Mime string
	Size int64
}

type SourceConfig struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	URL      string `json:"url"`
	CheckGPG bool   `json:"check_gpg"`
	CheckSSL bool   `json:"check_ssl"`
	System   bool   `json:"system"`
}

type NotFoundError struct {
	message string
}

func (e *NotFoundError) Error() string {
	return e.message
}

type NotPendingError struct {
	message string
}

func (e *NotPendingError) Error() string {
	return e.message
}

type NotRunningError struct {
	message string
}

func (e *NotRunningError) Error() string {
	return e.message
}

type InvalidRequestError struct {
	message string
}

func (e *InvalidRequestError) Error() string {
	return e.message
}

func New(stateFile *string) *Store {
	var s Store

	if stateFile != nil {
		state, err := ioutil.ReadFile(*stateFile)
		if state != nil {
			err := json.Unmarshal(state, &s)
			if err != nil {
				log.Fatalf("invalid initial state: %v", err)
			}
		} else if !os.IsNotExist(err) {
			log.Fatalf("cannot read state: %v", err)
		}

		s.stateChannel = make(chan []byte, 128)

		go func() {
			for {
				err := writeFileAtomically(*stateFile, <-s.stateChannel, 0755)
				if err != nil {
					log.Fatalf("cannot write state: %v", err)
				}
			}
		}()
	}

	if s.Blueprints == nil {
		s.Blueprints = make(map[string]blueprint.Blueprint)
	}
	if s.Workspace == nil {
		s.Workspace = make(map[string]blueprint.Blueprint)
	}
	if s.Composes == nil {
		// TODO: push waiting/running composes to workers again
		s.Composes = make(map[uuid.UUID]Compose)
	}
	if s.Sources == nil {
		s.Sources = make(map[string]SourceConfig)
	}
	if s.BlueprintsChanges == nil {
		s.BlueprintsChanges = make(map[string]map[string]blueprint.Change)
	}
	s.pendingJobs = make(chan Job, 200)

	return &s
}

func writeFileAtomically(filename string, data []byte, mode os.FileMode) error {
	dir, name := filepath.Dir(filename), filepath.Base(filename)

	tmpfile, err := ioutil.TempFile(dir, name+"-*.tmp")
	if err != nil {
		return err
	}

	_, err = tmpfile.Write(data)
	if err != nil {
		os.Remove(tmpfile.Name())
		return err
	}

	err = tmpfile.Chmod(mode)
	if err != nil {
		return err
	}

	err = tmpfile.Close()
	if err != nil {
		os.Remove(tmpfile.Name())
		return err
	}

	err = os.Rename(tmpfile.Name(), filename)
	if err != nil {
		os.Remove(tmpfile.Name())
		return err
	}

	return nil
}

func (s *Store) change(f func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := f()

	if s.stateChannel != nil {
		serialized, err := json.Marshal(s)
		if err != nil {
			// we ought to know all types that go into the store
			panic(err)
		}

		s.stateChannel <- serialized
	}
	return result
}

func (s *Store) ListBlueprints() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.Blueprints))
	for name := range s.Blueprints {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

type ComposeEntry struct {
	ID          uuid.UUID `json:"id"`
	Blueprint   string    `json:"blueprint"`
	Version     string    `json:"version"`
	ComposeType string    `json:"compose_type"`
	ImageSize   int64     `json:"image_size"`
	QueueStatus string    `json:"queue_status"`
	JobCreated  float64   `json:"job_created"`
	JobStarted  float64   `json:"job_started,omitempty"`
	JobFinished float64   `json:"job_finished,omitempty"`
}

func (s *Store) ListQueue(uuids []uuid.UUID) []*ComposeEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	newCompose := func(id uuid.UUID, compose Compose) *ComposeEntry {
		switch compose.QueueStatus {
		case "WAITING":
			return &ComposeEntry{
				ID:          id,
				Blueprint:   compose.Blueprint.Name,
				Version:     compose.Blueprint.Version,
				ComposeType: compose.OutputType,
				QueueStatus: compose.QueueStatus,
				JobCreated:  float64(compose.JobCreated.UnixNano()) / 1000000000,
			}
		case "RUNNING":
			return &ComposeEntry{
				ID:          id,
				Blueprint:   compose.Blueprint.Name,
				Version:     compose.Blueprint.Version,
				ComposeType: compose.OutputType,
				QueueStatus: compose.QueueStatus,
				JobCreated:  float64(compose.JobCreated.UnixNano()) / 1000000000,
				JobStarted:  float64(compose.JobStarted.UnixNano()) / 1000000000,
			}
		case "FINISHED":
			image, err := s.GetImage(id)
			imageSize := int64(0)
			if err == nil {
				imageSize = image.Size
			}
			return &ComposeEntry{
				ID:          id,
				Blueprint:   compose.Blueprint.Name,
				Version:     compose.Blueprint.Version,
				ComposeType: compose.OutputType,
				ImageSize:   imageSize,
				QueueStatus: compose.QueueStatus,
				JobCreated:  float64(compose.JobCreated.UnixNano()) / 1000000000,
				JobStarted:  float64(compose.JobStarted.UnixNano()) / 1000000000,
				JobFinished: float64(compose.JobFinished.UnixNano()) / 1000000000,
			}
		case "FAILED":
			return &ComposeEntry{
				ID:          id,
				Blueprint:   compose.Blueprint.Name,
				Version:     compose.Blueprint.Version,
				ComposeType: compose.OutputType,
				QueueStatus: compose.QueueStatus,
				JobCreated:  float64(compose.JobCreated.UnixNano()) / 1000000000,
				JobStarted:  float64(compose.JobStarted.UnixNano()) / 1000000000,
				JobFinished: float64(compose.JobFinished.UnixNano()) / 1000000000,
			}
		default:
			panic("invalid compose state")
		}
	}

	var composes []*ComposeEntry
	if uuids == nil {
		composes = make([]*ComposeEntry, 0, len(s.Composes))
		for id, compose := range s.Composes {
			composes = append(composes, newCompose(id, compose))
		}
	} else {
		composes = make([]*ComposeEntry, 0, len(uuids))
		for _, id := range uuids {
			if compose, exists := s.Composes[id]; exists {
				composes = append(composes, newCompose(id, compose))
			}
		}
	}

	return composes
}

func (s *Store) GetBlueprint(name string, bp *blueprint.Blueprint, changed *bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var inWorkspace bool
	*bp, inWorkspace = s.Workspace[name]
	if !inWorkspace {
		var ok bool
		*bp, ok = s.Blueprints[name]
		if !ok {
			return false
		}
	}

	// cockpit-composer cannot deal with missing "packages" or "modules"
	if bp.Packages == nil {
		bp.Packages = []blueprint.Package{}
	}
	if bp.Modules == nil {
		bp.Modules = []blueprint.Package{}
	}
	if bp.Groups == nil {
		bp.Groups = []blueprint.Group{}
	}
	if bp.Version == "" {
		bp.Version = "0.0.0"
	}

	if changed != nil {
		*changed = inWorkspace
	}

	return true
}

func (s *Store) GetBlueprintCommitted(name string, bp *blueprint.Blueprint) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ok bool
	*bp, ok = s.Blueprints[name]
	if !ok {
		return false
	}

	// cockpit-composer cannot deal with missing "packages" or "modules"
	if bp.Packages == nil {
		bp.Packages = []blueprint.Package{}
	}
	if bp.Modules == nil {
		bp.Modules = []blueprint.Package{}
	}
	if bp.Groups == nil {
		bp.Groups = []blueprint.Group{}
	}
	if bp.Version == "" {
		bp.Version = "0.0.0"
	}

	return true
}

func (s *Store) GetBlueprintChanges(name string) []blueprint.Change {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var changes []blueprint.Change

	for _, change := range s.BlueprintsChanges[name] {
		changes = append(changes, change)
	}

	return changes
}

func (s *Store) PushBlueprint(bp blueprint.Blueprint) {
	s.change(func() error {
		hash := sha1.New()
		// Hash timestamp to create unique hash
		hash.Write([]byte(time.Now().String()))
		// Get hash as a byte slice
		commitBytes := hash.Sum(nil)
		// Get hash as a hex string
		commit := hex.EncodeToString(commitBytes)
		message := "Recipe " + bp.Name + ", version " + bp.Version + " saved."
		timestamp := time.Now().Format("2006-01-02T15:04:05Z")
		change := blueprint.Change{
			Commit:    commit,
			Message:   message,
			Timestamp: timestamp,
			Blueprint: bp,
		}

		delete(s.Workspace, bp.Name)
		if s.BlueprintsChanges[bp.Name] == nil {
			s.BlueprintsChanges[bp.Name] = make(map[string]blueprint.Change)
		}
		s.BlueprintsChanges[bp.Name][commit] = change
		s.Blueprints[bp.Name] = bp
		return nil
	})
}

func (s *Store) PushBlueprintToWorkspace(bp blueprint.Blueprint) {
	s.change(func() error {
		s.Workspace[bp.Name] = bp
		return nil
	})
}

func (s *Store) DeleteBlueprint(name string) {
	s.change(func() error {
		delete(s.Workspace, name)
		delete(s.Blueprints, name)
		return nil
	})
}

func (s *Store) DeleteBlueprintFromWorkspace(name string) {
	s.change(func() error {
		delete(s.Workspace, name)
		return nil
	})
}

func (s *Store) PushCompose(composeID uuid.UUID, bp *blueprint.Blueprint, composeType string) error {
	targets := []*target.Target{
		target.NewLocalTarget(target.NewLocalTargetOptions("/var/lib/osbuild-composer/outputs/" + composeID.String())),
	}
	d := distro.New("")
	pipeline, err := d.Pipeline(bp, composeType)
	if err != nil {
		return err
	}
	s.change(func() error {
		s.Composes[composeID] = Compose{
			QueueStatus: "WAITING",
			Blueprint:   bp,
			OutputType:  composeType,
			Targets:     targets,
			JobCreated:  time.Now(),
		}
		return nil
	})
	s.pendingJobs <- Job{
		ComposeID: composeID,
		Pipeline:  pipeline,
		Targets:   targets,
	}

	return nil
}

func (s *Store) PopCompose() Job {
	job := <-s.pendingJobs
	s.change(func() error {
		compose, exists := s.Composes[job.ComposeID]
		if !exists || compose.QueueStatus != "WAITING" {
			panic("Invalid job in queue.")
		}
		compose.JobStarted = time.Now()
		compose.QueueStatus = "RUNNING"
		s.Composes[job.ComposeID] = compose
		return nil
	})
	return job
}

func (s *Store) UpdateCompose(composeID uuid.UUID, status string) error {
	return s.change(func() error {
		compose, exists := s.Composes[composeID]
		if !exists {
			return &NotFoundError{"compose does not exist"}
		}
		if compose.QueueStatus == "WAITING" {
			return &NotPendingError{"compose has not been popped"}
		}
		switch status {
		case "RUNNING":
			switch compose.QueueStatus {
			case "RUNNING":
			default:
				return &NotRunningError{"compose was not running"}
			}
		case "FINISHED", "FAILED":
			switch compose.QueueStatus {
			case "RUNNING":
				compose.JobFinished = time.Now()
			default:
				return &NotRunningError{"compose was not running"}
			}
			compose.QueueStatus = status
			s.Composes[composeID] = compose
		default:
			return &InvalidRequestError{"invalid state transition"}
		}
		return nil
	})
}

func (s *Store) GetImage(composeID uuid.UUID) (*Image, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if compose, exists := s.Composes[composeID]; exists {
		if compose.QueueStatus != "FINISHED" {
			return nil, &InvalidRequestError{"compose was not finished"}
		}
		d := distro.New("")
		name, mime, err := d.FilenameFromType(compose.OutputType)
		if err != nil {
			panic("invalid output type")
		}
		for _, t := range compose.Targets {
			switch options := t.Options.(type) {
			case *target.LocalTargetOptions:
				file, err := os.Open(options.Location + "/" + name)
				if err == nil {
					fileStat, err := file.Stat()
					if err != nil {
						return nil, &NotFoundError{"image info could not be found"}
					}
					size := fileStat.Size()

					return &Image{
						File: file,
						Name: name,
						Mime: mime,
						Size: size,
					}, nil
				}
			}
		}
		return nil, &NotFoundError{"image could not be found"}
	}

	return nil, &NotFoundError{"compose could not be found"}
}

func (s *Store) PushSource(source SourceConfig) {
	s.change(func() error {
		s.Sources[source.Name] = source
		return nil
	})
}

func (s *Store) DeleteSource(name string) {
	s.change(func() error {
		delete(s.Sources, name)
		return nil
	})
}

func (s *Store) ListSources() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.Sources))
	for name := range s.Sources {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

func (s *Store) GetSource(name string) *SourceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	source, ok := s.Sources[name]
	if !ok {
		return nil
	}
	return &source
}

func (s *Store) GetAllSources() map[string]SourceConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	sources := make(map[string]SourceConfig)

	for k, v := range s.Sources {
		sources[k] = v
	}

	return sources
}
