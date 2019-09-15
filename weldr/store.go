package weldr

import (
	"sort"
	"sync"
)

type store struct {
	Blueprints map[string]blueprint `json:"blueprints"`
	Workspace  map[string]blueprint `json:"workspace"`

	mu sync.RWMutex // protects all fields
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

func newStore() *store {
	return &store{
		Blueprints: make(map[string]blueprint),
		Workspace:  make(map[string]blueprint),
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

func (s *store) pushBlueprint(bp blueprint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Workspace, bp.Name)
	s.Blueprints[bp.Name] = bp
}

func (s *store) pushBlueprintToWorkspace(bp blueprint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Workspace[bp.Name] = bp
}

func (s *store) deleteBlueprint(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Workspace, name)
	delete(s.Blueprints, name)
}

func (s *store) deleteBlueprintFromWorkspace(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.Workspace, name)
}
