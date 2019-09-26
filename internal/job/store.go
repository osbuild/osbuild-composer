package job

import (
	"sync"
)

type Store struct {
	jobs map[string]Job
	mu   sync.RWMutex
}

func NewStore() *Store {
	var s Store

	s.jobs = make(map[string]Job)

	return &s
}

func (s *Store) AddJob(id string, job Job) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.jobs[id]
	if exists {
		return false
	}

	s.jobs[id] = job

	return true
}

func (s *Store) UpdateJob(id string, job Job) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	req, _ := s.jobs[id]
	req.ComposeID = job.ComposeID
	req.Pipeline = job.Pipeline
	req.Targets = job.Targets

	return true
}

func (s *Store) DeleteJob(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.jobs, id)
}
