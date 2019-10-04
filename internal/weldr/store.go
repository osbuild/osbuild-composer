package weldr

import (
	"encoding/json"
	"log"
	"osbuild-composer/internal/blueprint"
	"osbuild-composer/internal/job"
	"osbuild-composer/internal/target"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type store struct {
	Blueprints map[string]blueprint.Blueprint `json:"blueprints"`
	Workspace  map[string]blueprint.Blueprint `json:"workspace"`
	Composes   map[uuid.UUID]compose          `json:"composes"`

	mu           sync.RWMutex // protects all fields
	pendingJobs  chan<- job.Job
	jobUpdates   <-chan job.Status
	stateChannel chan<- []byte
}

type compose struct {
	QueueStatus string               `json:"queue_status"`
	Blueprint   *blueprint.Blueprint `json:"blueprint"`
	OutputType  string               `json:"output-type"`
	Targets     []*target.Target     `json:"targets"`
	JobCreated  time.Time            `json:"job_created"`
	JobStarted  time.Time            `json:"job_started"`
	JobFinished time.Time            `json:"job_finished"`
}

func newStore(initialState []byte, stateChannel chan<- []byte, pendingJobs chan<- job.Job, jobUpdates <-chan job.Status) *store {
	var s store

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
		s.Composes = make(map[uuid.UUID]compose)
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

func (s *store) change(f func()) {
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

func (s *store) listBlueprints() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.Blueprints))
	for name := range s.Blueprints {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

type composeEntry struct {
	ID          uuid.UUID `json:"id"`
	Blueprint   string    `json:"blueprint"`
	QueueStatus string    `json:"queue_status"`
	JobCreated  float64   `json:"job_created"`
	JobStarted  float64   `json:"job_started,omitempty"`
	JobFinished float64   `json:"job_finished,omitempty"`
}

func (s *store) listQueue(uuids []uuid.UUID) []*composeEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	newCompose := func(id uuid.UUID, compose compose) *composeEntry {
		switch compose.QueueStatus {
		case "WAITING":
			return &composeEntry{
				ID:          id,
				Blueprint:   compose.Blueprint.Name,
				QueueStatus: compose.QueueStatus,
				JobCreated:  float64(compose.JobCreated.UnixNano()) / 1000000000,
			}
		case "RUNNING":
			return &composeEntry{
				ID:          id,
				Blueprint:   compose.Blueprint.Name,
				QueueStatus: compose.QueueStatus,
				JobCreated:  float64(compose.JobCreated.UnixNano()) / 1000000000,
				JobStarted:  float64(compose.JobStarted.UnixNano()) / 1000000000,
			}
		case "FAILED", "FINISHED":
			return &composeEntry{
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

	var composes []*composeEntry
	if uuids == nil {
		composes = make([]*composeEntry, 0, len(s.Composes))
		for id, compose := range s.Composes {
			composes = append(composes, newCompose(id, compose))
		}
	} else {
		composes = make([]*composeEntry, 0, len(uuids))
		for _, id := range uuids {
			if compose, exists := s.Composes[id]; exists {
				composes = append(composes, newCompose(id, compose))
			}
		}
	}

	return composes
}

func (s *store) getBlueprint(name string, bp *blueprint.Blueprint, changed *bool) bool {
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

func (s *store) getBlueprintCommitted(name string, bp *blueprint.Blueprint) bool {
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

func (s *store) pushBlueprint(bp blueprint.Blueprint) {
	s.change(func() {
		delete(s.Workspace, bp.Name)
		s.Blueprints[bp.Name] = bp
	})
}

func (s *store) pushBlueprintToWorkspace(bp blueprint.Blueprint) {
	s.change(func() {
		s.Workspace[bp.Name] = bp
	})
}

func (s *store) deleteBlueprint(name string) {
	s.change(func() {
		delete(s.Workspace, name)
		delete(s.Blueprints, name)
	})
}

func (s *store) deleteBlueprintFromWorkspace(name string) {
	s.change(func() {
		delete(s.Workspace, name)
	})
}

func (s *store) addCompose(composeID uuid.UUID, bp *blueprint.Blueprint, composeType string) {
	targets := []*target.Target{
		target.NewLocalTarget(target.NewLocalTargetOptions("/var/lib/osbuild-composer/outputs/" + composeID.String())),
	}
	s.change(func() {
		s.Composes[composeID] = compose{
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
