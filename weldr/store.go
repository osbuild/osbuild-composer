package weldr

import (
	"sort"
	"sync"
)

type store struct {
	Blueprints map[string]blueprint `json:"blueprints"`

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

func (s *store) getBlueprint(name string) (blueprint, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	bp, ok := s.Blueprints[name]
	return bp, ok
}

func (s *store) pushBlueprint(bp blueprint) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.Blueprints[bp.Name] = bp
}
