package main

import (
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
)

type workerQueue struct {
	// manifest job channel
	jobQueue chan manifestJob

	// channel for sending messages from jobs to the printer
	msgQueue chan string

	// channel for sending errors from jobs to the collector
	errQueue chan error

	// global error list
	errors []error

	// total job count defined on workerQueue creation
	// sets the length of the job queue so that pushing to the queue doesn't block
	njobs uint32

	// total workers defined on workerQueue creation
	nworkers uint32

	// active worker count
	activeWorkers int32

	// wait group for all workers
	workerWG sync.WaitGroup

	// wait group for internal routines (printer and error collector)
	utilWG sync.WaitGroup
}

func newWorkerQueue(nworkers uint32, njobs uint32) *workerQueue {
	wq := workerQueue{
		jobQueue:      make(chan manifestJob, njobs),
		msgQueue:      make(chan string, nworkers),
		errQueue:      make(chan error, nworkers),
		errors:        make([]error, 0, nworkers),
		nworkers:      nworkers,
		activeWorkers: 0,
		njobs:         njobs,
	}

	return &wq
}

func (wq *workerQueue) start() {
	wq.startMessagePrinter()
	wq.startErrorCollector()
	for idx := uint32(0); idx < wq.nworkers; idx++ {
		wq.startWorker(idx)
	}
}

// close all queues and wait for waitgroups
func (wq *workerQueue) wait() []error {
	// close job channel and wait for workers to finish
	close(wq.jobQueue)
	wq.workerWG.Wait()

	// close message channels and wait for them to finish their work so we don't miss any messages or errors
	close(wq.msgQueue)
	close(wq.errQueue)
	wq.utilWG.Wait()
	return wq.errors
}

func (wq *workerQueue) startWorker(idx uint32) {
	wq.workerWG.Add(1)
	go func() {
		atomic.AddInt32(&(wq.activeWorkers), 1)
		defer atomic.AddInt32(&(wq.activeWorkers), -1)
		defer wq.workerWG.Done()
		for job := range wq.jobQueue {
			err := job(wq.msgQueue)
			if err != nil {
				wq.errQueue <- err
			}
		}
	}()
}

func (wq *workerQueue) startMessagePrinter() {
	wq.utilWG.Add(1)
	go func() {
		defer wq.utilWG.Done()
		var msglen int
		for msg := range wq.msgQueue {
			// clear previous line (avoids leftover trailing characters from progress)
			fmt.Print(strings.Repeat(" ", msglen) + "\r")
			fmt.Println(msg)
			msglen, _ = fmt.Printf(" == Jobs == Queue: %4d  Active: %4d  Total: %4d\r", len(wq.jobQueue), wq.activeWorkers, wq.njobs)
		}
		fmt.Println()
	}()
}

func (wq *workerQueue) startErrorCollector() {
	wq.utilWG.Add(1)
	go func() {
		defer wq.utilWG.Done()
		for err := range wq.errQueue {
			wq.errors = append(wq.errors, err)
		}
	}()
}

func (wq *workerQueue) submitJob(j manifestJob) {
	wq.jobQueue <- j
}
