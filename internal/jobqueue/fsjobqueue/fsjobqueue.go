// Package fsjobqueue implements a filesystem-backed job queue. It implements
// the interfaces in package jobqueue.
//
// Jobs are stored in the file system, using the `jsondb` package. However,
// this package does not use the file system as a database, but keeps some
// state in memory. This means that access to a given directory must be
// exclusive to only one `fsJobQueue` object at a time. A single `fsJobQueue`
// can be safely accessed from multiple goroutines, though.
//
// Data is stored non-reduntantly. Any data structure necessary for efficient
// access (e.g., dependants) are kept in memory.
package fsjobqueue

import (
	"container/list"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/pkg/jobqueue"

	"github.com/osbuild/osbuild-composer/internal/jsondb"
)

type fsJobQueue struct {
	// Protects all fields of this struct. In particular, it ensures
	// transactions on `db` are atomic. All public functions except
	// JobStatus hold it while they're running. Dequeue() releases it
	// briefly while waiting on pending channels.
	mu sync.Mutex

	db *jsondb.JSONDatabase

	// List of pending job
	pending *list.List

	// Set of goroutines waiting for new pending jobs
	listeners map[chan struct{}]struct{}

	// Maps job ids to the jobs that depend on it, if any of those
	// dependants have not yet finished.
	dependants map[uuid.UUID][]uuid.UUID

	// Currently running jobs. Workers are not handed job ids, but
	// independent tokens which serve as an indirection. This enables
	// race-free uploading of artifacts and makes restarting composer more
	// robust (workers from an old run cannot report results for jobs
	// composer thinks are not running).
	// This map maps these tokens to job ids. Artifacts are stored in
	// `$STATE_DIRECTORY/artifacts/tmp/$TOKEN` while the worker is running,
	// and renamed to `$STATE_DIRECTORY/artifacts/$JOB_ID` once the job is
	// reported as done.
	jobIdByToken map[uuid.UUID]uuid.UUID
	heartbeats   map[uuid.UUID]time.Time // token -> heartbeat

	workerIDByToken map[uuid.UUID]uuid.UUID // token -> workerID
	workers         map[uuid.UUID]worker
}

type worker struct {
	Channel   string    `json:"channel"`
	Arch      string    `json:"arch"`
	Heartbeat time.Time `json:"heartbeat"`
	Tokens    map[uuid.UUID]struct{}
}

// On-disk job struct. Contains all necessary (but non-redundant) information
// about a job. These are not held in memory by the job queue, but
// (de)serialized on each access.
type job struct {
	Id           uuid.UUID       `json:"id"`
	Token        uuid.UUID       `json:"token"`
	Type         string          `json:"type"`
	Args         json.RawMessage `json:"args,omitempty"`
	Dependencies []uuid.UUID     `json:"dependencies"`
	Dependents   []uuid.UUID     `json:"dependents"`
	Result       json.RawMessage `json:"result,omitempty"`
	Channel      string          `json:"channel"`

	QueuedAt   time.Time `json:"queued_at,omitempty"`
	StartedAt  time.Time `json:"started_at,omitempty"`
	FinishedAt time.Time `json:"finished_at,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`

	Retries  uint64 `json:"retries"`
	Canceled bool   `json:"canceled,omitempty"`
}

// Create a new fsJobQueue object for `dir`. This object must have exclusive
// access to `dir`. If `dir` contains jobs created from previous runs, they are
// loaded and rescheduled to run if necessary.
func New(dir string) (*fsJobQueue, error) {
	q := &fsJobQueue{
		db:              jsondb.New(dir, 0600),
		pending:         list.New(),
		dependants:      make(map[uuid.UUID][]uuid.UUID),
		jobIdByToken:    make(map[uuid.UUID]uuid.UUID),
		heartbeats:      make(map[uuid.UUID]time.Time),
		listeners:       make(map[chan struct{}]struct{}),
		workers:         make(map[uuid.UUID]worker),
		workerIDByToken: make(map[uuid.UUID]uuid.UUID),
	}

	// Look for jobs that are still pending and build the dependant map.
	ids, err := q.db.List()
	if err != nil {
		return nil, fmt.Errorf("error listing jobs: %v", err)
	}

	for _, id := range ids {
		jobId, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("invalid job '%s' in db: %v", id, err)
		}
		j, err := q.readJob(jobId)
		if err != nil {
			// Skip invalid jobs, leaving them in place for later examination
			continue
		}

		// If a job is running, and not cancelled, track the token
		if !j.StartedAt.IsZero() && j.FinishedAt.IsZero() && !j.Canceled {
			// Fail older running jobs which don't have a token stored
			if j.Token == uuid.Nil {
				_, err = q.RequeueOrFinishJob(j.Id, 0, nil)
				if err != nil {
					return nil, fmt.Errorf("Error finishing job '%s' without a token: %v", j.Id, err)
				}
			} else {
				q.jobIdByToken[j.Token] = j.Id
				q.heartbeats[j.Token] = time.Now()
			}
		}

		err = q.maybeEnqueue(j, true)
		if err != nil {
			return nil, err
		}
	}

	return q, nil
}

func (q *fsJobQueue) Enqueue(jobType string, args interface{}, dependencies []uuid.UUID, channel string) (uuid.UUID, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	var j = job{
		Id:           uuid.New(),
		Token:        uuid.Nil,
		Type:         jobType,
		Dependencies: dependencies,
		QueuedAt:     time.Now(),
		Channel:      channel,
	}

	var err error
	j.Args, err = json.Marshal(args)
	if err != nil {
		return uuid.Nil, fmt.Errorf("error marshaling job arguments: %v", err)
	}

	// Verify dependendencies early, so that the job doesn't get written
	// when one of them doesn't exist.
	for _, d := range j.Dependencies {
		var dep job
		exists, err := q.db.Read(d.String(), &dep)
		if err != nil {
			return uuid.Nil, err
		}
		if !exists {
			return uuid.Nil, jobqueue.ErrNotExist
		}

		dep.Dependents = append(dep.Dependents, j.Id)
		err = q.db.Write(d.String(), dep)
		if err != nil {
			return uuid.Nil, err
		}
	}

	// Write the job before updating in-memory state, so that the latter
	// doesn't become corrupt when writing fails.
	err = q.db.Write(j.Id.String(), j)
	if err != nil {
		return uuid.Nil, fmt.Errorf("cannot write job: %v:", err)
	}

	err = q.maybeEnqueue(&j, true)
	if err != nil {
		return uuid.Nil, err
	}

	return j.Id, nil
}

func (q *fsJobQueue) Dequeue(ctx context.Context, wID uuid.UUID, jobTypes, channels []string) (uuid.UUID, uuid.UUID, []uuid.UUID, string, json.RawMessage, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Return early if the context is already canceled.
	if err := ctx.Err(); err != nil {
		return uuid.Nil, uuid.Nil, nil, "", nil, jobqueue.ErrDequeueTimeout
	}

	// Add a new listener
	c := make(chan struct{}, 1)
	q.listeners[c] = struct{}{}
	defer delete(q.listeners, c)

	// Loop until finding a suitable job
	var j *job
	for {
		var found bool
		var err error
		j, found, err = q.dequeueSuitableJob(jobTypes, channels)
		if err != nil {
			return uuid.Nil, uuid.Nil, nil, "", nil, err
		}
		if found {
			break
		}

		// Unlock the mutex while polling channels, so that multiple goroutines
		// can wait at the same time.
		q.mu.Unlock()
		select {
		case <-c:
		case <-ctx.Done():
			// there's defer q.mu.Unlock(), so let's lock
			q.mu.Lock()
			return uuid.Nil, uuid.Nil, nil, "", nil, jobqueue.ErrDequeueTimeout
		}
		q.mu.Lock()
	}

	j.StartedAt = time.Now()

	j.Token = uuid.New()
	q.jobIdByToken[j.Token] = j.Id
	q.heartbeats[j.Token] = time.Now()
	if _, ok := q.workers[wID]; ok {
		q.workers[wID].Tokens[j.Token] = struct{}{}
		q.workerIDByToken[j.Token] = wID
	}

	err := q.db.Write(j.Id.String(), j)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, "", nil, fmt.Errorf("error writing job %s: %v", j.Id, err)
	}

	return j.Id, j.Token, j.Dependencies, j.Type, j.Args, nil
}

func (q *fsJobQueue) DequeueByID(ctx context.Context, id, wID uuid.UUID) (uuid.UUID, []uuid.UUID, string, json.RawMessage, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	j, err := q.readJob(id)
	if err != nil {
		return uuid.Nil, nil, "", nil, err
	}

	if !j.StartedAt.IsZero() {
		return uuid.Nil, nil, "", nil, jobqueue.ErrNotPending
	}

	depsFinished, err := q.hasAllFinishedDependencies(j)
	if err != nil {
		return uuid.Nil, nil, "", nil, err
	}
	if !depsFinished {
		return uuid.Nil, nil, "", nil, jobqueue.ErrNotPending
	}

	q.removePendingJob(id)

	j.StartedAt = time.Now()

	j.Token = uuid.New()
	q.jobIdByToken[j.Token] = j.Id
	q.heartbeats[j.Token] = time.Now()
	if _, ok := q.workers[wID]; ok {
		q.workers[wID].Tokens[j.Token] = struct{}{}
		q.workerIDByToken[j.Token] = wID
	}

	err = q.db.Write(j.Id.String(), j)
	if err != nil {
		return uuid.Nil, nil, "", nil, fmt.Errorf("error writing job %s: %v", j.Id, err)
	}

	return j.Token, j.Dependencies, j.Type, j.Args, nil
}

func (q *fsJobQueue) UpdateJobResult(id uuid.UUID, result interface{}) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	j, err := q.readJob(id)
	if err != nil {
		return err
	}

	if j.Canceled {
		return jobqueue.ErrCanceled
	}

	if j.StartedAt.IsZero() || !j.FinishedAt.IsZero() {
		return jobqueue.ErrNotRunning
	}

	j.Result, err = json.Marshal(result)
	if err != nil {
		return fmt.Errorf("error marshaling result: %w", err)
	}

	err = q.db.Write(id.String(), j)
	if err != nil {
		return fmt.Errorf("error writing job %s: %w", id, err)
	}

	return nil
}

func (q *fsJobQueue) RequeueOrFinishJob(id uuid.UUID, maxRetries uint64, result interface{}) (bool, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	j, err := q.readJob(id)
	if err != nil {
		return false, err
	}

	if j.Canceled {
		return false, jobqueue.ErrCanceled
	}

	if j.StartedAt.IsZero() || !j.FinishedAt.IsZero() {
		return false, jobqueue.ErrNotRunning
	}

	delete(q.jobIdByToken, j.Token)
	delete(q.heartbeats, j.Token)
	if wID, ok := q.workerIDByToken[j.Token]; ok {
		delete(q.workers[wID].Tokens, j.Token)
		delete(q.workerIDByToken, j.Token)
	}

	if j.Retries >= maxRetries {
		j.FinishedAt = time.Now()

		j.Result, err = json.Marshal(result)
		if err != nil {
			return false, fmt.Errorf("error marshaling result: %v", err)
		}

		// Write before notifying dependants, because it will be read again.
		err = q.db.Write(id.String(), j)
		if err != nil {
			return false, fmt.Errorf("error writing job %s: %v", id, err)
		}

		for _, depid := range q.dependants[id] {
			dep, err := q.readJob(depid)
			if err != nil {
				return false, err
			}
			err = q.maybeEnqueue(dep, false)
			if err != nil {
				return false, err
			}
		}
		delete(q.dependants, id)
		return false, nil
	} else {
		j.Token = uuid.Nil
		j.StartedAt = time.Time{}
		j.Retries += 1

		// Write the job before updating in-memory state, so that the latter
		// doesn't become corrupt when writing fails.
		err = q.db.Write(j.Id.String(), j)
		if err != nil {
			return false, fmt.Errorf("cannot write job: %v", err)
		}

		// add the job to the list of pending ones
		q.pending.PushBack(j.Id)

		// notify all listeners in a non-blocking way
		for c := range q.listeners {
			select {
			case c <- struct{}{}:
			default:
			}
		}
		return true, nil
	}
}

func (q *fsJobQueue) CancelJob(id uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	j, err := q.readJob(id)
	if err != nil {
		return err
	}

	if !j.FinishedAt.IsZero() {
		return jobqueue.ErrNotRunning
	}

	// if the cancelled job is pending, remove it from the list
	if j.StartedAt.IsZero() {
		q.removePendingJob(id)
	}

	j.Canceled = true

	delete(q.heartbeats, j.Token)

	err = q.db.Write(id.String(), j)
	if err != nil {
		return fmt.Errorf("error writing job %s: %v", id, err)
	}

	return nil
}

func (q *fsJobQueue) FailJob(id uuid.UUID, result interface{}) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	j, err := q.readJob(id)
	if err != nil {
		return err
	}

	if !j.FinishedAt.IsZero() {
		return jobqueue.ErrFinished
	}

	if !j.StartedAt.IsZero() {
		return jobqueue.ErrRunning
	}

	j.Result, err = json.Marshal(result)
	if err != nil {
		return err
	}

	j.StartedAt = time.Now()
	j.FinishedAt = time.Now()
	j.Token = uuid.New()

	err = q.db.Write(id.String(), j)
	if err != nil {
		return fmt.Errorf("error writing job %s: %v", id, err)
	}

	return nil
}

func (q *fsJobQueue) JobStatus(id uuid.UUID) (jobType string, channel string, result json.RawMessage, queued, started, finished time.Time, canceled bool, deps []uuid.UUID, dependents []uuid.UUID, err error) {
	j, err := q.readJob(id)
	if err != nil {
		return
	}

	jobType = j.Type
	channel = j.Channel
	result = j.Result
	queued = j.QueuedAt
	started = j.StartedAt
	finished = j.FinishedAt
	canceled = j.Canceled
	deps = j.Dependencies
	dependents = j.Dependents

	return
}

func (q *fsJobQueue) Job(id uuid.UUID) (jobType string, args json.RawMessage, dependencies []uuid.UUID, channel string, err error) {
	j, err := q.readJob(id)
	if err != nil {
		return
	}

	jobType = j.Type
	args = j.Args
	dependencies = j.Dependencies
	channel = j.Channel

	return
}

func (q *fsJobQueue) IdFromToken(token uuid.UUID) (id uuid.UUID, err error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	id, ok := q.jobIdByToken[token]
	if !ok {
		return uuid.Nil, jobqueue.ErrNotExist
	}
	return id, nil
}

// Retrieve a list of tokens tied to jobs, which most recent action has been
// olderThan time ago
func (q *fsJobQueue) Heartbeats(olderThan time.Duration) (tokens []uuid.UUID) {
	q.mu.Lock()
	defer q.mu.Unlock()
	now := time.Now()
	for token, hb := range q.heartbeats {
		if now.Sub(hb) > olderThan {
			tokens = append(tokens, token)
		}
	}
	return
}

func (q *fsJobQueue) RefreshHeartbeat(token uuid.UUID) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if token != uuid.Nil {
		q.heartbeats[token] = time.Now()
	}
}

func (q *fsJobQueue) InsertWorker(channel, arch string) (uuid.UUID, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	wID := uuid.New()
	q.workers[wID] = worker{
		Channel:   channel,
		Arch:      arch,
		Heartbeat: time.Now(),
		Tokens:    make(map[uuid.UUID]struct{}),
	}
	return wID, nil
}

func (q *fsJobQueue) UpdateWorkerStatus(wID uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	worker, ok := q.workers[wID]
	if !ok {
		return jobqueue.ErrWorkerNotExist
	}

	worker.Heartbeat = time.Now()
	return nil
}

func (q *fsJobQueue) Workers(olderThan time.Duration) ([]jobqueue.Worker, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	workers := []jobqueue.Worker{}
	for wID, w := range q.workers {
		if now.Sub(w.Heartbeat) > olderThan {
			workers = append(workers, jobqueue.Worker{
				ID:      wID,
				Channel: w.Channel,
				Arch:    w.Arch,
			})
		}
	}
	return workers, nil
}

func (q *fsJobQueue) DeleteWorker(wID uuid.UUID) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	worker, ok := q.workers[wID]
	if !ok {
		return jobqueue.ErrWorkerNotExist
	}

	if len(worker.Tokens) != 0 {
		return jobqueue.ErrActiveJobs
	}
	delete(q.workers, wID)
	return nil
}

// Reads job with `id`. This is a thin wrapper around `q.db.Read`, which
// returns the job directly, or and error if a job with `id` does not exist.
func (q *fsJobQueue) readJob(id uuid.UUID) (*job, error) {
	var j job
	exists, err := q.db.Read(id.String(), &j)
	if err != nil {
		return nil, fmt.Errorf("error reading job '%s': %v", id, err)
	}
	if !exists {
		// return corrupt database?
		return nil, jobqueue.ErrNotExist
	}
	return &j, nil
}

// Enqueue `job` if it is pending and all its dependencies have finished.
// Update `q.dependants` if the job was not queued and updateDependants is true
// (i.e., when this is a new job).
// `q.mu` must be locked when this method is called. The only exception is
// `New()` because no concurrent calls are possible there.
func (q *fsJobQueue) maybeEnqueue(j *job, updateDependants bool) error {
	if !j.StartedAt.IsZero() {
		return nil
	}

	depsFinished, err := q.hasAllFinishedDependencies(j)
	if err != nil {
		return err
	}

	if depsFinished {
		// add the job to the list of pending ones
		q.pending.PushBack(j.Id)

		// notify all listeners in a non-blocking way
		for c := range q.listeners {
			select {
			case c <- struct{}{}:
			default:
			}
		}
	} else if updateDependants {
		for _, id := range j.Dependencies {
			q.dependants[id] = append(q.dependants[id], j.Id)
		}
	}

	return nil
}

// hasAllFinishedDependencies returns true if all dependencies of `j`
// are finished. Otherwise, false is returned. If one of the jobs cannot
// be read, an error is returned.
func (q *fsJobQueue) hasAllFinishedDependencies(j *job) (bool, error) {
	for _, id := range j.Dependencies {
		j, err := q.readJob(id)
		if err != nil {
			return false, err
		}
		if j.FinishedAt.IsZero() {
			return false, nil
		}
	}

	return true, nil
}

// dequeueSuitableJob finds a suitable job in the list of pending jobs, removes it from there and returns it
//
// The job must meet the following conditions:
// - must be pending
// - its dependencies must be finished
// - must be of one of the type from jobTypes
// - must be of one of the channel from channels
//
// If a suitable job is not found, false is returned.
// If an error occurs during the search, it's returned.
func (q *fsJobQueue) dequeueSuitableJob(jobTypes []string, channels []string) (*job, bool, error) {
	el := q.pending.Front()
	for el != nil {
		id := el.Value.(uuid.UUID)

		j, err := q.readJob(id)
		if err != nil {
			return nil, false, err
		}

		if !jobMatchesCriteria(j, jobTypes, channels) {
			el = el.Next()
			continue
		}

		ready, err := q.hasAllFinishedDependencies(j)
		if err != nil {
			return nil, false, err
		}
		if ready {
			q.pending.Remove(el)
			return j, true, nil
		}
		el = el.Next()
	}

	return nil, false, nil
}

// removePendingJob removes a job with given ID from the list of pending jobs
//
// If the job isn't in the list, this is no-op.
func (q *fsJobQueue) removePendingJob(id uuid.UUID) {
	el := q.pending.Front()
	for el != nil {
		if el.Value.(uuid.UUID) == id {
			q.pending.Remove(el)
			return
		}

		el = el.Next()
	}
}

// jobMatchesCriteria returns true if it matches criteria defined in parameters
//
// Criteria:
//   - the job's type is one of the acceptedJobTypes
//   - the job's channel is one of the acceptedChannels
func jobMatchesCriteria(j *job, acceptedJobTypes []string, acceptedChannels []string) bool {
	contains := func(slice []string, str string) bool {
		for _, item := range slice {
			if str == item {
				return true
			}
		}

		return false
	}

	return contains(acceptedJobTypes, j.Type) && contains(acceptedChannels, j.Channel)
}

// AllRootJobIDs Return a list of all the top level(root) job uuids
// This only includes jobs without any Dependents set
func (q *fsJobQueue) AllRootJobIDs(_ context.Context) ([]uuid.UUID, error) {
	ids, err := q.db.List()
	if err != nil {
		return nil, err
	}

	jobIDs := []uuid.UUID{}
	for _, id := range ids {
		var j job
		exists, err := q.db.Read(id, &j)
		if err != nil {
			return jobIDs, err
		}
		if !exists || len(j.Dependents) > 0 {
			continue
		}
		jobIDs = append(jobIDs, j.Id)
	}

	return jobIDs, nil
}

// DeleteJob will delete a job and all of its dependencies
// If a dependency has multiple depenents it will only delete the parent job from
// the dependants list and then re-save the job instead of removing it.
//
// This assumes that the jobs have been created correctly, and that they have
// no dependency loops. Shared Dependants are ok, but a job cannot have a dependancy
// on any of its parents (this should never happen).
func (q *fsJobQueue) DeleteJob(_ context.Context, id uuid.UUID) error {
	// Start it off with an empty parent
	return q.deleteJob(uuid.UUID{}, id)
}

// deleteJob will delete jobs as far down the list as possible
// missing dependencies are ignored, it deletes as much as it can.
// A missing parent (the first call) will be returned as an error
func (q *fsJobQueue) deleteJob(parent, id uuid.UUID) error {
	var j job
	_, err := q.db.Read(id.String(), &j)
	if err != nil {
		return err
	}

	// Delete the parent uuid from the Dependents list
	var deps []uuid.UUID
	for _, d := range j.Dependents {
		if d == parent {
			continue
		}
		deps = append(deps, d)
	}
	j.Dependents = deps

	// This job can only be deleted when the Dependents list is empty
	// Otherwise it needs to be saved with the new Dependents list
	if len(j.Dependents) > 0 {
		return q.db.Write(id.String(), j)
	}
	// Recursively delete the dependencies of this job
	for _, dj := range j.Dependencies {
		_ = q.deleteJob(id, dj)
	}

	return q.db.Delete(id.String())
}
