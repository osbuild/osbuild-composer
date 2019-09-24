package queue

import "sync"

// Build is a request waiting for a worker
type Build struct {
	Pipeline string `json:"pipeline"`
	Manifest string `json:"manifest"`
}

// Manifest contains additional metadata attached do a pipeline that are necessary for workers
type Manifest struct {
	destination string
}

// Job is an image build already in progress
type Job struct {
	UUID  string `json:"uuid"`
	Build Build  `json:"build"`
}

// JobQueue contains already running jobs waiting for
type JobQueue struct {
	sync.Mutex
	incomingBuilds chan Build     // Channel of incoming builds form Weldr API, we never want to block on this
	waitingBuilds  []Build        // Unbounded FIFO queue of waiting builds
	runningJobs    map[string]Job // Already running jobs, key is UUID
}

// NewJobQueue creates object of type JobQueue
func NewJobQueue(timeout int, builds chan Build) *JobQueue {
	jobs := &JobQueue{
		incomingBuilds: builds,
		waitingBuilds:  make([]Build, 0),
		runningJobs:    make(map[string]Job),
	}
	go func() {
		for {
			// This call will block, do not put it inside the locked zone
			newBuild := <-jobs.incomingBuilds
			// Locking the whole job queue => as short as possible
			jobs.Lock()
			jobs.waitingBuilds = append(jobs.waitingBuilds, newBuild)
			jobs.Unlock()
		}
	}()
	return jobs
}

// StartNewJob starts a new job
func (j *JobQueue) StartNewJob(id string, worker string) Job {
	j.Lock()
	newBuild := j.waitingBuilds[0]        // Take the first element
	j.waitingBuilds = j.waitingBuilds[1:] // Discart 1st element
	j.Unlock()
	job := Job{
		UUID:  id,
		Build: newBuild,
	}
	j.runningJobs[id] = job
	return job
}
