// Package store contains primitives for representing and changing the
// osbuild-composer state.
package store

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"osbuild-composer/internal/blueprint"
	"osbuild-composer/internal/job"
	"osbuild-composer/internal/target"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// A Store contains all the persistent state of osbuild-composer, and is serialized
// on every change, and deserialized on start.
type Store struct {
	Blueprints map[string]blueprint.Blueprint `json:"blueprints"`
	Workspace  map[string]blueprint.Blueprint `json:"workspace"`
	Composes   map[uuid.UUID]Compose          `json:"composes"`

	mu           sync.RWMutex // protects all fields
	pendingJobs  chan<- job.Job
	jobUpdates   <-chan job.Status
	stateChannel chan<- []byte
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

// An Image represents the image resulting from a compose.
type Image struct {
	File *os.File
	Name string
	Mime string
}

func New(initialState []byte, stateChannel chan<- []byte, pendingJobs chan<- job.Job, jobUpdates <-chan job.Status) *Store {
	var s Store

	if initialState != nil {
		err := json.Unmarshal(initialState, &s)
		if err != nil {
			log.Fatalf("invalid initial state: %v", err)
		}
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
	s.stateChannel = stateChannel
	s.pendingJobs = pendingJobs
	s.jobUpdates = jobUpdates

	go func() {
		for {
			update := <-s.jobUpdates
			s.change(func() {
				compose, exists := s.Composes[update.ComposeID]
				if !exists {
					return
				}
				if compose.QueueStatus != update.Status {
					switch update.Status {
					case "RUNNING":
						compose.JobStarted = time.Now()
					case "FINISHED":
						fallthrough
					case "FAILED":
						compose.JobFinished = time.Now()
					}
					compose.QueueStatus = update.Status
					s.Composes[update.ComposeID] = compose
				}
			})
		}
	}()

	return &s
}

func (s *Store) change(f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	f()

	if s.stateChannel != nil {
		serialized, err := json.Marshal(s)
		if err != nil {
			// we ought to know all types that go into the store
			panic(err)
		}

		s.stateChannel <- serialized
	}
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
				QueueStatus: compose.QueueStatus,
				JobCreated:  float64(compose.JobCreated.UnixNano()) / 1000000000,
			}
		case "RUNNING":
			return &ComposeEntry{
				ID:          id,
				Blueprint:   compose.Blueprint.Name,
				QueueStatus: compose.QueueStatus,
				JobCreated:  float64(compose.JobCreated.UnixNano()) / 1000000000,
				JobStarted:  float64(compose.JobStarted.UnixNano()) / 1000000000,
			}
		case "FAILED", "FINISHED":
			return &ComposeEntry{
				ID:          id,
				Blueprint:   compose.Blueprint.Name,
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
	s.change(func() {
		delete(s.Workspace, bp.Name)
		s.Blueprints[bp.Name] = bp
	})
}

func (s *Store) PushBlueprintToWorkspace(bp blueprint.Blueprint) {
	s.change(func() {
		s.Workspace[bp.Name] = bp
	})
}

func (s *Store) DeleteBlueprint(name string) {
	s.change(func() {
		delete(s.Workspace, name)
		delete(s.Blueprints, name)
	})
}

func (s *Store) DeleteBlueprintFromWorkspace(name string) {
	s.change(func() {
		delete(s.Workspace, name)
	})
}

func (s *Store) AddCompose(composeID uuid.UUID, bp *blueprint.Blueprint, composeType string) {
	targets := []*target.Target{
		target.NewLocalTarget(target.NewLocalTargetOptions("/var/lib/osbuild-composer/outputs/" + composeID.String())),
	}
	s.change(func() {
		s.Composes[composeID] = Compose{
			QueueStatus: "WAITING",
			Blueprint:   bp,
			OutputType:  composeType,
			Targets:     targets,
			JobCreated:  time.Now(),
		}
	})
	s.pendingJobs <- job.Job{
		ComposeID: composeID,
		Pipeline:  bp.ToPipeline(composeType),
		Targets:   targets,
	}
}

func (s *Store) GetImage(composeID uuid.UUID) (*Image, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if compose, exists := s.Composes[composeID]; exists {
		if compose.QueueStatus != "FINISHED" {
			return nil, errors.New("compose not ready")
		}
		name, mime := blueprint.FilenameFromType(compose.OutputType)
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
	}

	return nil, errors.New("image not found")
}
