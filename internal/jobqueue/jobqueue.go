// Package jobqueue provides a generic interface to a simple job queue.
//
// Jobs are pushed to the queue with Enqueue(). Workers call Dequeue() to
// receive a job and FinishJob() to report one as finished.
//
// Each job has a type and arguments corresponding to this type. These are
// opaque to the job queue, but it mandates that the arguments must be
// serializable to JSON. Similarly, a job's result has opaque result arguments
// that are determined by its type.
//
// A job can have dependencies. It is not run until all its dependencies have
// finished.
package jobqueue

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
)

// JobQueue is an interface to a simple job queue. It is safe for concurrent use.
type JobQueue interface {
	// Enqueues a job.
	//
	// `args` must be JSON-serializable and fit the given `jobType`, i.e., a worker
	// that is running that job must know the format of `args`.
	//
	// All dependencies must already exist, but the job isn't run until all of them
	// have finished.
	//
	// Returns the id of the new job, or an error.
	Enqueue(jobType string, args interface{}, dependencies []uuid.UUID) (uuid.UUID, error)

	// Dequeues a job, blocking until one is available.
	//
	// Waits until a job with a type of any of `jobTypes` is available, or `ctx` is
	// canceled.
	//
	// All jobs in `jobTypes` must take the same type of `args`, corresponding to
	// the one that was passed to Enqueue().
	//
	// Returns the job's id or an error.
	Dequeue(ctx context.Context, jobTypes []string, args interface{}) (uuid.UUID, error)

	// Mark the job with `id` as finished. `result` must fit the associated
	// job type and must be serializable to JSON.
	FinishJob(id uuid.UUID, result interface{}) error

	// Returns the current status of the job. If the job has already
	// finished, its result will be returned in `result`. Also returns the
	// time the job was
	//    queued   - always valid
	//    started  - valid when the job is running or has finished
	//    finished - valid when the job has finished
	JobStatus(id uuid.UUID, result interface{}) (status JobStatus, queued, started, finished time.Time, err error)
}

type JobStatus int

const (
	JobPending JobStatus = iota
	JobRunning
	JobFinished
)

func (s JobStatus) String() string {
	switch s {
	case JobPending:
		return "pending"
	case JobRunning:
		return "running"
	case JobFinished:
		return "finished"
	default:
		return "<invalid>"
	}
}

func (s JobStatus) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

func (s *JobStatus) UnmarshalJSON(data []byte) error {
	var str string
	if err := json.Unmarshal(data, &str); err != nil {
		return err
	}
	switch str {
	case "pending":
		*s = JobPending
	case "running":
		*s = JobRunning
	case "finished":
		*s = JobFinished
	}
	return nil
}

var (
	ErrNotExist   = errors.New("job does not exist")
	ErrNotRunning = errors.New("job is not running")
)
