//go:build integration

package main

import (
	"context"
	"encoding/json"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/jobqueuetest"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/pkg/jobqueue/dbjobqueue"
)

func TestDBJobQueueMaintenance(t *testing.T) {
	dbMaintenance, err := newDB(jobqueuetest.TestDbURL())
	require.NoError(t, err)
	defer dbMaintenance.Close()
	q, err := dbjobqueue.New(jobqueuetest.TestDbURL())
	require.NoError(t, err)
	defer q.Close()

	_, err = dbMaintenance.Conn.Exec(context.Background(), "DELETE FROM jobs")
	require.NoError(t, err)

	t.Run("testDeleteJob", func(t *testing.T) {
		testDeleteJob(t, dbMaintenance, q)
	})
	t.Run("testVacuum", func(t *testing.T) {
		testVacuum(t, dbMaintenance, q)
	})

}

func setExpired(t *testing.T, d db, id uuid.UUID) {
	_, err := d.Conn.Exec(context.Background(), "UPDATE jobs SET expires_at = NOW() - INTERVAL '1 SECOND' WHERE id = $1", id)
	require.NoError(t, err)
}

func testDeleteJob(t *testing.T, d db, q *dbjobqueue.DBJobQueue) {
	id, err := q.Enqueue("octopus", nil, nil, "")
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, id)
	_, _, _, _, _, err = q.Dequeue(context.Background(), uuid.Nil, []string{"octopus"}, []string{""})
	require.NoError(t, err)

	type Result struct {
		Result string `json:"result"`
	}
	result := Result{
		"deleteme",
	}

	res, err := json.Marshal(result)
	require.NoError(t, err)
	err = q.RequeueOrFinishJob(id, 0, res)
	require.NoError(t, err)

	_, _, r, _, _, _, _, _, _, err := q.JobStatus(id)
	require.NoError(t, err)

	var r1 Result
	require.NoError(t, json.Unmarshal(r, &r1))
	require.Equal(t, result, r1)

	rows, err := d.DeleteJobs()
	require.NoError(t, err)
	require.Equal(t, int64(0), rows)

	setExpired(t, d, id)
	rows, err = d.ExpiredJobCount()
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	rows, err = d.DeleteJobs()
	require.NoError(t, err)
	require.Equal(t, int64(1), rows)

	_, _, _, _, _, _, _, _, _, err = q.JobStatus(id)
	require.Error(t, err)
}

func testVacuum(t *testing.T, d db, q *dbjobqueue.DBJobQueue) {
	require.NoError(t, d.VacuumAnalyze())
	require.NoError(t, d.LogVacuumStats())
}
