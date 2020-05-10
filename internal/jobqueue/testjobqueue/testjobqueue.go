// Package testjobqueue implements jobqueue interface. It is meant for testing,
// and as such doesn't implement two invariants of jobqueue: it is not safe for
// concurrent access and `Dequeue()` doesn't wait for new jobs to appear.
package testjobqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
)

type testJobQueue struct {
	jobs map[uuid.UUID]*job

	pending map[string][]uuid.UUID

	// Maps job ids to the jobs that depend on it
	dependants map[uuid.UUID][]uuid.UUID
}

type job struct {
	Id           uuid.UUID
	Type         string
	Args         json.RawMessage
	Dependencies []uuid.UUID
	Result       json.RawMessage
	QueuedAt     time.Time
	StartedAt    time.Time
	FinishedAt   time.Time
}

func New() *testJobQueue {
	return &testJobQueue{
		jobs:    make(map[uuid.UUID]*job),
		pending: make(map[string][]uuid.UUID),
	}
}

func (q *testJobQueue) Enqueue(jobType string, args interface{}, dependencies []uuid.UUID) (uuid.UUID, error) {
	var j = job{
		Id:           uuid.New(),
		Type:         jobType,
		Dependencies: uniqueUUIDList(dependencies),
		QueuedAt:     time.Now(),
	}

	var err error
	j.Args, err = json.Marshal(args)
	if err != nil {
		return uuid.Nil, err
	}

	q.jobs[j.Id] = &j

	// Verify dependencies and check how many of them are already finished.
	finished, err := q.countFinishedJobs(j.Dependencies)
	if err != nil {
		return uuid.Nil, err
	}

	// If all dependencies have finished, or there are none, queue the job.
	// Otherwise, update dependants so that this check is done again when
	// FinishJob() is called for a dependency.
	if finished == len(j.Dependencies) {
		q.pending[j.Type] = append(q.pending[j.Type], j.Id)
	} else {
		for _, id := range j.Dependencies {
			q.dependants[id] = append(q.dependants[id], j.Id)
		}
	}

	return j.Id, nil
}

func (q *testJobQueue) Dequeue(ctx context.Context, jobTypes []string, args interface{}) (uuid.UUID, error) {
	for _, t := range jobTypes {
		if len(q.pending[t]) == 0 {
			continue
		}

		id := q.pending[t][0]
		q.pending[t] = q.pending[t][1:]

		j := q.jobs[id]

		err := json.Unmarshal(j.Args, args)
		if err != nil {
			return uuid.Nil, err
		}

		j.StartedAt = time.Now()
		return j.Id, nil
	}

	return uuid.Nil, errors.New("no job available")
}

func (q *testJobQueue) FinishJob(id uuid.UUID, result interface{}) error {
	j, exists := q.jobs[id]
	if !exists {
		return jobqueue.ErrNotExist
	}

	if j.StartedAt.IsZero() || !j.FinishedAt.IsZero() {
		return jobqueue.ErrNotRunning
	}

	var err error
	j.Result, err = json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling result: %v", err)
	}

	j.FinishedAt = time.Now()

	for _, depid := range q.dependants[id] {
		dep := q.jobs[depid]
		n, err := q.countFinishedJobs(dep.Dependencies)
		if err != nil {
			return err
		}
		if n == len(dep.Dependencies) {
			q.pending[dep.Type] = append(q.pending[dep.Type], dep.Id)
		}
	}
	delete(q.dependants, id)

	return nil
}

func (q *testJobQueue) JobStatus(id uuid.UUID, result interface{}) (queued, started, finished time.Time, err error) {
	var j *job

	j, exists := q.jobs[id]
	if !exists {
		err = jobqueue.ErrNotExist
		return
	}

	if !j.FinishedAt.IsZero() {
		err = json.Unmarshal(j.Result, result)
		if err != nil {
			return
		}
	}

	queued = j.QueuedAt
	started = j.StartedAt
	finished = j.FinishedAt

	return
}

// Returns the number of finished jobs in `ids`.
func (q *testJobQueue) countFinishedJobs(ids []uuid.UUID) (int, error) {
	n := 0
	for _, id := range ids {
		j, exists := q.jobs[id]
		if !exists {
			return 0, jobqueue.ErrNotExist
		}
		if j.Result != nil {
			n += 1
		}
	}

	return n, nil
}

// Sorts and removes duplicates from `ids`.
// Copied from fsjobqueue, which also contains a test.
func uniqueUUIDList(ids []uuid.UUID) []uuid.UUID {
	s := map[uuid.UUID]bool{}
	for _, id := range ids {
		s[id] = true
	}

	l := []uuid.UUID{}
	for id := range s {
		l = append(l, id)
	}

	sort.Slice(l, func(i, j int) bool {
		for b := 0; b < 16; b++ {
			if l[i][b] < l[j][b] {
				return true
			}
		}
		return false
	})

	return l
}
