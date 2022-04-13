// +build integration

package main

import (
	"context"
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
	t.Run("maintenance-delete-job-and-dependencies", wrap(testDeleteJobAndDependencies))
}

func setQueuedAt(t *testing.T, q *dbjobqueue.DBJobQueue, id uuid.UUID, queued time.Time) {
	conn, err := pgx.Connect(context.Background(), url)
	require.NoError(t, err)
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(), "UPDATE jobs SET queued_at = $1 WHERE id = $2", queued, id)
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
	setQueuedAt(t, q, id80, date80)

	id85, err := q.Enqueue("octopus", nil, nil, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id85)
	_, _, _, _, _, err = q.Dequeue(context.Background(), []string{"octopus"}, []string{""})
	require.NoError(t, err)
	err = q.FinishJob(id85, nil)
	require.NoError(t, err)
	setQueuedAt(t, q, id85, date85)

	ids, err := q.JobsUptoByType([]string{"octopus"}, date85)
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{id80}, ids["octopus"])

	ids, err = q.JobsUptoByType([]string{"octopus"}, date90)
	require.NoError(t, err)
	require.ElementsMatch(t, []uuid.UUID{id80, id85}, ids["octopus"])
}

func testDeleteJobAndDependencies(t *testing.T, q *dbjobqueue.DBJobQueue) {
	// id1 -> id2 -> id3
	id1, err := q.Enqueue("octopus", nil, nil, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id1)
	id2, err := q.Enqueue("octopus", nil, []uuid.UUID{id1}, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id2)
	id3, err := q.Enqueue("octopus", nil, []uuid.UUID{id2}, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id3)

	c1, err := q.Enqueue("octopus", nil, nil, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, c1)
	c2, err := q.Enqueue("octopus", nil, []uuid.UUID{c1}, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, c2)
	c3, err := q.Enqueue("octopus", nil, []uuid.UUID{c2}, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, c3)
	controls := []uuid.UUID{c1, c2, c3}

	_, _, _, _, err = q.Job(c1)
	require.NoError(t, err)

	require.NoError(t, q.DeleteJobIncludingDependants(id1))
	for _, id := range []uuid.UUID{id1, id2, id3} {
		_, _, _, _, err = q.Job(id)
		require.ErrorIs(t, err, jobqueue.ErrNotExist)
	}

	// controls should still exist
	for _, c := range controls {
		_, _, _, _, err = q.Job(c)
		require.NoError(t, err)
	}

	// id1 -> id2 -> id4 && id3 -> id4
	id1, err = q.Enqueue("octopus", nil, nil, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id1)
	id2, err = q.Enqueue("octopus", nil, []uuid.UUID{id1}, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id2)
	id3, err = q.Enqueue("octopus", nil, nil, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id3)
	id4, err := q.Enqueue("octopus", nil, []uuid.UUID{id2, id3}, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id4)

	require.NoError(t, q.DeleteJobIncludingDependants(id1))
	require.NoError(t, q.DeleteJobIncludingDependants(id3))
	for _, id := range []uuid.UUID{id1, id2, id3, id4} {
		_, _, _, _, err = q.Job(id)
		require.ErrorIs(t, err, jobqueue.ErrNotExist)
	}

	// controls should still exist
	for _, c := range controls {
		_, _, _, _, err = q.Job(c)
		require.NoError(t, err)
	}

	// multiple branches depend on id1
	// id1 -> id2a -> id3
	//     -> id2b
	id1, err = q.Enqueue("octopus", nil, nil, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id1)
	id2a, err := q.Enqueue("octopus", nil, []uuid.UUID{id1}, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id2a)
	id2b, err := q.Enqueue("octopus", nil, []uuid.UUID{id1}, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id2b)
	id3, err = q.Enqueue("octopus", nil, []uuid.UUID{id2a}, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id3)

	require.NoError(t, q.DeleteJobIncludingDependants(id1))
	for _, id := range []uuid.UUID{id1, id2a, id2b, id3} {
		_, _, _, _, err = q.Job(id)
		require.ErrorIs(t, err, jobqueue.ErrNotExist)
	}

	// controls should still exist
	for _, c := range controls {
		_, _, _, _, err = q.Job(c)
		require.NoError(t, err)
	}

	require.NoError(t, q.DeleteJobIncludingDependants(uuid.Nil))
	// controls should still exist
	for _, c := range controls {
		_, _, _, _, err = q.Job(c)
		require.NoError(t, err)
	}
}
