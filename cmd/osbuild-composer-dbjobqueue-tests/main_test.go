//go:build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/pkg/jobqueue"
	"github.com/osbuild/osbuild-composer/pkg/jobqueue/dbjobqueue"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/jobqueuetest"
)

func migrate(migration string) error {
	// migrate
	gopath, err := exec.Command("go", "env", "GOPATH").Output()
	if err != nil {
		return err
	}
	tern := fmt.Sprintf("%s/bin/tern", strings.TrimSpace(string(gopath)))
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	if out, err := exec.Command(
		tern,
		"migrate",
		"--conn-string",
		jobqueuetest.TestDbURL(),
		"-m",
		fmt.Sprintf("%s/../../pkg/jobqueue/dbjobqueue/schemas", wd),
		"-d",
		migration,
	).CombinedOutput(); err != nil {
		fmt.Println("tern output:", string(out))
		return err
	}
	return nil
}

func TestJobQueueInterface(t *testing.T) {
	makeJobQueue := func(migration string, clean bool) (jobqueue.JobQueue, func(), error) {
		err := migrate(migration)
		if err != nil {
			return nil, nil, err
		}

		if clean {
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

	// run first, as migrations aren't reversible
	testMigrationPath(t, makeJobQueue)

	jobqueuetest.TestJobQueue(t, func() (jobqueue.JobQueue, func(), error) {
		return makeJobQueue("last", true)
	})
}

func testMigrationPath(t *testing.T, makeJobQueue func(migration string, clean bool) (jobqueue.JobQueue, func(), error)) {
	q, stop, err := makeJobQueue("8", false)
	defer stop()
	require.NoError(t, err)

	id, err := q.Enqueue("test", "{\"arg\": \"impormtanmt\"}", nil, "")
	require.NoError(t, err)
	require.NotEmpty(t, id)
	id, tok, _, _, _, err := q.Dequeue(context.Background(), uuid.Nil, []string{"test"}, []string{""})
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.NotEmpty(t, tok)

	// make sure entering escaped nullbytes fails in 8
	_, err = q.RequeueOrFinishJob(id, 0, &jobqueuetest.TestResult{Logs: []byte("{\"blegh\\u0000\": \"\\u0000reallyimportant stuff!\"}")})
	require.Error(t, err)

	tr := jobqueuetest.TestResult{Logs: []byte("{\"blegh\": \"really important stuff!\"}")}
	_, err = q.RequeueOrFinishJob(id, 0, &tr)
	require.NoError(t, err)

	require.NoError(t, migrate("last"))

	// reopen the connection to the db, as the old connection will still treat the result column as jsonb
	stop()
	db_q, err := dbjobqueue.New(jobqueuetest.TestDbURL())
	require.NoError(t, err)
	defer db_q.Close()
	require.NoError(t, err)

	// make sure entering escaped nullbytes works in last
	id2, err := db_q.Enqueue("test", "{\"arg\": \"impormtanmt\"}", nil, "")
	require.NoError(t, err)
	require.NotEmpty(t, id2)
	id2, tok2, _, _, _, err := db_q.Dequeue(context.Background(), uuid.Nil, []string{"test"}, []string{""})
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.NotEmpty(t, tok2)
	_, err = db_q.RequeueOrFinishJob(id2, 0, &jobqueuetest.TestResult{Logs: []byte("{\"blegh\\u0000\": \"\\u0000\"}")})
	require.NoError(t, err)

	_, _, result, _, _, _, _, _, _, err := db_q.JobStatus(id)
	require.NoError(t, err)
	var tr2 jobqueuetest.TestResult
	require.NoError(t, json.Unmarshal(result, &tr2))
	require.Equal(t, tr, tr2)
}
