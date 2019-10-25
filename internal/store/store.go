// Package store contains primitives for representing and changing the
// osbuild-composer state.
package store

import (
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

	"github.com/google/uuid"
)

// A Store contains all the persistent state of osbuild-composer, and is serialized
// on every change, and deserialized on start.
type Store struct {
	Blueprints map[string]blueprint.Blueprint `json:"blueprints"`
	Workspace  map[string]blueprint.Blueprint `json:"workspace"`
	Composes   map[uuid.UUID]Compose          `json:"composes"`

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
		case "FAILED", "FINISHED":
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
		bp.Groups = []blueprint.Package{}
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
		bp.Groups = []blueprint.Package{}
	}
	if bp.Version == "" {
		bp.Version = "0.0.0"
	}

	return true
}

func (s *Store) PushBlueprint(bp blueprint.Blueprint) {
	s.change(func() error {
		delete(s.Workspace, bp.Name)
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
	pipeline, err := bp.ToPipeline(composeType)
	if err != nil {
		return &InvalidRequestError{"invalid output type: " + composeType}
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
		name, mime, err := blueprint.FilenameFromType(compose.OutputType)
		if err != nil {
			panic("invalid output type")
		}
		for _, t := range compose.Targets {
			switch options := t.Options.(type) {
			case *target.LocalTargetOptions:
				file, err := os.Open(options.Location + "/" + name)
				if err == nil {
					return &Image{
						File: file,
						Name: name,
						Mime: mime,
					}, nil
				}
			}
		}
		return nil, &NotFoundError{"image could not be found"}
	}

	return nil, &NotFoundError{"compose could not be found"}
}
