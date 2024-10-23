//go:build integration

package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v4"
	"github.com/osbuild/osbuild-composer/pkg/jobqueue"
	"github.com/osbuild/osbuild-composer/pkg/jobqueue/dbjobqueue"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/jobqueuetest"
)

func TestJobQueueInterface(t *testing.T) {
	makeJobQueue := func() (jobqueue.JobQueue, func(), error) {
		// clear db before each run
		conn, err := pgx.Connect(context.Background(), jobqueuetest.TestDbURL())
		if err != nil {
			return nil, nil, err
		}
		defer conn.Close(context.Background())
		for _, table := range []string{"job_dependencies", "heartbeats", "jobs"} {
			_, err = conn.Exec(context.Background(), fmt.Sprintf("DELETE FROM %s", table))
			if err != nil {
				return nil, nil, err
			}
		}
		err = conn.Close(context.Background())
		if err != nil {
			return nil, nil, err
		}

		q, err := dbjobqueue.New(jobqueuetest.TestDbURL())
		if err != nil {
			return nil, nil, err
		}
		stop := func() {
			q.Close()
		}
		return q, stop, nil
	}

	jobqueuetest.TestJobQueue(t, makeJobQueue)
}
