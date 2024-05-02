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
	Enqueue(jobType string, args interface{}, dependencies []uuid.UUID, channel string) (uuid.UUID, error)

	// Dequeues a job, blocking until one is available.
	//
	// Waits until a job with a type of any of `jobTypes` and any of `channels`
	// is available, or `ctx` is canceled.
	//
	// Returns the job's id, token, dependencies, type, and arguments, or an error. Arguments
	// can be unmarshaled to the type given in Enqueue().
	Dequeue(ctx context.Context, workerID uuid.UUID, jobTypes []string, channels []string) (uuid.UUID, uuid.UUID, []uuid.UUID, string, json.RawMessage, error)

	// Dequeues a pending job by its ID in a non-blocking way.
	//
	// Returns the job's token, dependencies, type, and arguments, or an error. Arguments
	// can be unmarshaled to the type given in Enqueue().
	DequeueByID(ctx context.Context, id, workerID uuid.UUID) (uuid.UUID, []uuid.UUID, string, json.RawMessage, error)

	// Tries to requeue a running job by its ID
	//
	// If the job has reached the maxRetries number of retries already, finish the job instead.
	// `result` must fit the associated job type and must be serializable to JSON.
	// Fills in result, and returns if the job was requeued, or an error.
	RequeueOrFinishJob(id uuid.UUID, maxRetries uint64, result interface{}) (bool, error)

	// Cancel a job. Does nothing if the job has already finished.
	CancelJob(id uuid.UUID) error

	// If the job has finished, returns the result as raw JSON.
	//
	// Returns the current status of the job, in the form of three times:
	// queued, started, and finished. `started` and `finished` might be the
	// zero time (check with t.IsZero()), when the job is not running or
	// finished, respectively.
	//
	// Lastly, the IDs of the jobs dependencies are returned.
	JobStatus(id uuid.UUID) (jobType string, channel string, result json.RawMessage, queued, started, finished time.Time, canceled bool, deps []uuid.UUID, dependents []uuid.UUID, err error)

	// Job returns all the parameters that define a job (everything provided during Enqueue).
	Job(id uuid.UUID) (jobType string, args json.RawMessage, dependencies []uuid.UUID, channel string, err error)

	// Find job by token, this will return an error if the job hasn't been dequeued
	IdFromToken(token uuid.UUID) (id uuid.UUID, err error)

	// Get a list of tokens which haven't been updated in the specified time frame
	Heartbeats(olderThan time.Duration) (tokens []uuid.UUID)

	// Reset the last job heartbeat time to time.Now()
	RefreshHeartbeat(token uuid.UUID)

	// Inserts the worker and creates a UUID for it
	InsertWorker(channel, arch string) (uuid.UUID, error)

	// Reset the last worker's heartbeat time to time.Now()
	UpdateWorkerStatus(workerID uuid.UUID) error

	// Get a list of workers which haven't been updated in the specified time frame
	Workers(olderThan time.Duration) ([]Worker, error)

	// Deletes the worker
	DeleteWorker(workerID uuid.UUID) error

	// AllJobIDs returns a list of all job UUIDs that the worker knows about
	AllJobIDs() ([]uuid.UUID, error)

	// AllRootJobIDs returns a list of top level job UUIDs that the worker knows about
	AllRootJobIDs() ([]uuid.UUID, error)
}

// SimpleLogger provides a structured logging methods for the jobqueue library.
type SimpleLogger interface {
	// Info creates an info-level message and arbitrary amount of key-value string pairs which
	// can be optionally mapped to fields by underlying implementations.
	Info(msg string, args ...string)

	// Error creates an error-level message and arbitrary amount of key-value string pairs which
	// can be optionally mapped to fields by underlying implementations. The first error argument
	// can be set to nil when no context error is available.
	Error(err error, msg string, args ...string)
}

var (
	ErrNotExist       = errors.New("job does not exist")
	ErrNotPending     = errors.New("job is not pending")
	ErrNotRunning     = errors.New("job is not running")
	ErrCanceled       = errors.New("job was canceled")
	ErrDequeueTimeout = errors.New("dequeue context timed out or was canceled")
	ErrActiveJobs     = errors.New("worker has active jobs associated with it")
	ErrWorkerNotExist = errors.New("worker does not exist")
)

type Worker struct {
	ID      uuid.UUID
	Channel string
	Arch    string
}
