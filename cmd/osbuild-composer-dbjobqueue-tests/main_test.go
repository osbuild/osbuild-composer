// +build integration

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/dbjobqueue"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/jobqueuetest"
)

const url = "postgres://postgres:foobar@localhost:5432/osbuildcomposer"

func TestJobQueueInterface(t *testing.T) {
	makeJobQueue := func() (jobqueue.JobQueue, func(), error) {
		// clear db before each run
		conn, err := pgx.Connect(context.Background(), url)
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

		q, err := dbjobqueue.New(url)
		if err != nil {
			return nil, nil, err
		}
		stop := func() {
			q.Close()
		}
		return q, stop, nil
	}

	jobqueuetest.TestJobQueue(t, makeJobQueue)

	wrap := func(f func(t *testing.T, q *dbjobqueue.DBJobQueue)) func(*testing.T) {
		q, stop, err := makeJobQueue()
		require.NoError(t, err)
		return func(t *testing.T) {
			defer stop() // use defer because f() might call testing.T.FailNow()
			dbq, ok := q.(*dbjobqueue.DBJobQueue)
			require.True(t, ok)
			f(t, dbq)
		}
	}

	t.Run("maintenance-query-jobs-before", wrap(testJobsUptoByType))
	t.Run("maintenance-delete-job-results", wrap(testDeleteJobResult))
}

func setFinishedAt(t *testing.T, q *dbjobqueue.DBJobQueue, id uuid.UUID, finished time.Time) {
	conn, err := pgx.Connect(context.Background(), url)
	require.NoError(t, err)
	defer conn.Close(context.Background())

	started := finished.Add(-time.Second)
	queued := started.Add(-time.Second)

	_, err = conn.Exec(context.Background(), "UPDATE jobs SET queued_at = $1, started_at = $2, finished_at = $3, result = '{\"result\": \"success\" }' WHERE id = $4", queued, started, finished, id)
	require.NoError(t, err)
}

func testJobsUptoByType(t *testing.T, q *dbjobqueue.DBJobQueue) {
	date80 := time.Date(1980, time.January, 1, 0, 0, 0, 0, time.UTC)
	date85 := time.Date(1985, time.January, 1, 0, 0, 0, 0, time.UTC)
	date90 := time.Date(1990, time.January, 1, 0, 0, 0, 0, time.UTC)

	id80, err := q.Enqueue("octopus", nil, nil, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id80)
	_, _, _, _, _, err = q.Dequeue(context.Background(), []string{"octopus"}, []string{""})
	require.NoError(t, err)
	err = q.FinishJob(id80, nil)
	require.NoError(t, err)
	setFinishedAt(t, q, id80, date80)

	id85, err := q.Enqueue("octopus", nil, nil, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id85)
	_, _, _, _, _, err = q.Dequeue(context.Background(), []string{"octopus"}, []string{""})
	require.NoError(t, err)
	err = q.FinishJob(id85, nil)
	require.NoError(t, err)
	setFinishedAt(t, q, id85, date85)

	ids, err := q.JobsUptoByType([]string{"octopus"}, date85)
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{id80}, ids["octopus"])

	ids, err = q.JobsUptoByType([]string{"octopus"}, date90)
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{id80, id85}, ids["octopus"])
}

func testDeleteJobResult(t *testing.T, q *dbjobqueue.DBJobQueue) {
	id, err := q.Enqueue("octopus", nil, nil, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id)
	_, _, _, _, _, err = q.Dequeue(context.Background(), []string{"octopus"}, []string{""})
	require.NoError(t, err)

	type Result struct {
		Result string `json:"result"`
	}
	result := Result{
		"deleteme",
	}

	res, err := json.Marshal(result)
	require.NoError(t, err)
	err = q.FinishJob(id, res)
	require.NoError(t, err)

	_, _, r, _, _, _, _, _, err := q.JobStatus(id)
	require.NoError(t, err)

	var r1 Result
	require.NoError(t, json.Unmarshal(r, &r1))
	require.Equal(t, result, r1)

	rows, err := q.DeleteJobResult([]uuid.UUID{id})
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	_, _, r, _, _, _, _, _, err = q.JobStatus(id)
	require.NoError(t, err)
	require.Nil(t, r)
}
