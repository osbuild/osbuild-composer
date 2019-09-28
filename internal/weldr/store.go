package weldr

import (
	"encoding/json"
	"log"
	"osbuild-composer/internal/job"
	"osbuild-composer/internal/pipeline"
	"osbuild-composer/internal/target"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

type store struct {
	Blueprints map[string]blueprint  `json:"blueprints"`
	Workspace  map[string]blueprint  `json:"workspace"`
	Composes   map[uuid.UUID]compose `json:"composes"`

	mu           sync.RWMutex // protects all fields
	pendingJobs  chan<- job.Job
	jobUpdates   <-chan job.Status
	stateChannel chan<- []byte
}

type blueprint struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Version     string             `json:"version,omitempty"`
	Packages    []blueprintPackage `json:"packages"`
	Modules     []blueprintPackage `json:"modules"`
}

type blueprintPackage struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type compose struct {
	QueueStatus string           `json:"queue_status"`
	Blueprint   *blueprint       `json:"blueprint"`
	OutputType  string           `json:"output-type"`
	Targets     []*target.Target `json:"targets"`
	JobCreated  time.Time        `json:"job_created"`
	JobStarted  time.Time        `json:"job_started"`
	JobFinished time.Time        `json:"job_finished"`
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
		s.Blueprints = make(map[string]blueprint)
	}
	if s.Workspace == nil {
		s.Workspace = make(map[string]blueprint)
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

func (s *store) getBlueprint(name string, bp *blueprint, changed *bool) bool {
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
		bp.Packages = []blueprintPackage{}
	}
	if bp.Modules == nil {
		bp.Modules = []blueprintPackage{}
	}

	if changed != nil {
		*changed = inWorkspace
	}

	return true
}

func (s *store) getBlueprintCommitted(name string, bp *blueprint) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var ok bool
	*bp, ok = s.Blueprints[name]
	if !ok {
		return false
	}

	// cockpit-composer cannot deal with missing "packages" or "modules"
	if bp.Packages == nil {
		bp.Packages = []blueprintPackage{}
	}
	if bp.Modules == nil {
		bp.Modules = []blueprintPackage{}
	}

	return true
}

func (s *store) pushBlueprint(bp blueprint) {
	s.change(func() {
		delete(s.Workspace, bp.Name)
		s.Blueprints[bp.Name] = bp
	})
}

func (s *store) pushBlueprintToWorkspace(bp blueprint) {
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

func (s *store) addCompose(composeID uuid.UUID, bp *blueprint, composeType string) {
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
		Pipeline:  bp.translateToPipeline(composeType),
		Targets:   targets,
	}
}

func (b *blueprint) translateToPipeline(outputFormat string) *pipeline.Pipeline {
	p := &pipeline.Pipeline{}
	p.SetAssembler(pipeline.NewTarAssembler(pipeline.NewTarAssemblerOptions("image.tar")))
	return p
}
