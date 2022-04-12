// +build integration

package main

import (
	"context"
	"fmt"
	"log"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/dbjobqueue"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

const url = "postgres://postgres:foobar@localhost:5432/osbuildcomposer"

func initQ() (*worker.Server, error) {
	// clear db before each run
	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		return nil, err
	}
	defer conn.Close(context.Background())
	for _, table := range []string{"job_dependencies", "heartbeats", "jobs"} {
		_, err = conn.Exec(context.Background(), fmt.Sprintf("DELETE FROM %s", table))
		if err != nil {
			return nil, err
		}
	}
	err = conn.Close(context.Background())
	if err != nil {
		return nil, err
	}

	q, err := dbjobqueue.New(url)
	if err != nil {
		return nil, err
	}

	return worker.NewServer(log.Default(), q, worker.Config{}), nil
}

// enqueues a depsolve -> manifest -> osbuild job
func enqueueJob(ws *worker.Server) (uuid.UUID, error) {
	dId, err := ws.EnqueueDepsolve(&worker.DepsolveJob{}, "")
	if err != nil {
		return uuid.Nil, err
	}
	mId, err := ws.EnqueueManifestJobByID(&worker.ManifestJobByID{}, dId, "")
	if err != nil {
		return uuid.Nil, err
	}

	oId, err := ws.EnqueueOSBuildAsDependency("x86_64", &worker.OSBuildJob{}, mId, "")
	return oId, err
}

// enqueues a koji-init -> manifest -> osbuild-koji -> koji-finalize job
//            depsolve  ->          -> osbuild-koji
func enqueueKojiJob(ws *worker.Server) (uuid.UUID, error) {
	dId, err := ws.EnqueueDepsolve(&worker.DepsolveJob{}, "")
	if err != nil {
		return uuid.Nil, err
	}
	mId, err := ws.EnqueueManifestJobByID(&worker.ManifestJobByID{}, dId, "")
	if err != nil {
		return uuid.Nil, err
	}
	initId, err := ws.EnqueueKojiInit(&worker.KojiInitJob{}, "")
	if err != nil {
		return uuid.Nil, err
	}
	oId1, err := ws.EnqueueOSBuildKojiAsDependency("x86_64", &worker.OSBuildKojiJob{}, mId, initId, "")
	if err != nil {
		return uuid.Nil, err
	}
	oId2, err := ws.EnqueueOSBuildKojiAsDependency("x86_64", &worker.OSBuildKojiJob{}, mId, initId, "")
	if err != nil {
		return uuid.Nil, err
	}

	fId, err := ws.EnqueueKojiFinalize(&worker.KojiFinalizeJob{}, initId, []uuid.UUID{oId1, oId2}, "")
	return fId, err
}

func TestDBCleanup(t *testing.T) {
	ws, err := initQ()
	require.NoError(t, err)

	exists := func(id uuid.UUID) bool {
		res := &worker.OSBuildJobResult{}
		_, _, err := ws.OSBuildJobStatus(id, res)
		if err == jobqueue.ErrNotExist {
			return false
		}
		require.NoError(t, err)
		return true
	}

	existsKoji := func(id uuid.UUID) bool {
		res := &worker.KojiFinalizeJobResult{}
		_, _, err := ws.KojiFinalizeJobStatus(id, res)
		if err == jobqueue.ErrNotExist {
			return false
		}
		require.NoError(t, err)
		return true
	}

	osbuildPre := make([]uuid.UUID, 10)
	for i := range osbuildPre {
		id, err := enqueueJob(ws)
		require.NoError(t, err)
		osbuildPre[i] = id
	}

	kojiPre := make([]uuid.UUID, 10)
	for i := range kojiPre {
		id, err := enqueueKojiJob(ws)
		require.NoError(t, err)
		kojiPre[i] = id
	}

	res := &worker.KojiFinalizeJobResult{}
	status, _, err := ws.KojiFinalizeJobStatus(kojiPre[9], res)
	require.NoError(t, err)
	cutoff := status.Queued

	osbuildPost := make([]uuid.UUID, 10)
	for i := range osbuildPost {
		id, err := enqueueJob(ws)
		require.NoError(t, err)
		osbuildPost[i] = id
	}

	kojiPost := make([]uuid.UUID, 10)
	for i := range kojiPost {
		id, err := enqueueKojiJob(ws)
		require.NoError(t, err)
		kojiPost[i] = id
	}

	// All jobs still exist after a dry run
	require.NoError(t, DBCleanup(url, true, cutoff))
	for i := 0; i < 10; i += 1 {
		require.True(t, exists(osbuildPre[i]))
		require.True(t, existsKoji(kojiPre[i]))
		require.True(t, exists(osbuildPost[i]))
		require.True(t, existsKoji(kojiPost[i]))
	}

	// All jobs which had their depsolve and/or koji-init jobs queued before the cutoff are gone
	require.NoError(t, DBCleanup(url, false, cutoff))
	for i := 0; i < 10; i += 1 {
		require.False(t, exists(osbuildPre[i]))
		require.False(t, existsKoji(kojiPre[i]))
		require.True(t, exists(osbuildPost[i]))
		require.True(t, existsKoji(kojiPost[i]))
	}
}
