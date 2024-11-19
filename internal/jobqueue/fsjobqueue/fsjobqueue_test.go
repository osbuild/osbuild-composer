package fsjobqueue_test

import (
	"os"
	"path"
	"sort"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/jobqueuetest"
	"github.com/osbuild/osbuild-composer/pkg/jobqueue"
)

func TestJobQueueInterface(t *testing.T) {
	jobqueuetest.TestJobQueue(t, func() (jobqueue.JobQueue, func(), error) {
		dir := t.TempDir()
		q, err := fsjobqueue.New(dir)
		if err != nil {
			return nil, nil, err
		}
		stop := func() {
		}
		return q, stop, nil
	})
}

func TestNonExistant(t *testing.T) {
	q, err := fsjobqueue.New("/non-existant-directory")
	require.Error(t, err)
	require.Nil(t, q)
}

func TestJobQueueBadJSON(t *testing.T) {
	dir := t.TempDir()

	// Write a purposfully invalid JSON file into the queue
	err := os.WriteFile(path.Join(dir, "/4f1cf5f8-525d-46b7-aef4-33c6a919c038.json"), []byte("{invalid json content"), 0600)
	require.Nil(t, err)

	q, err := fsjobqueue.New(dir)
	require.Nil(t, err)
	require.NotNil(t, q)
}

func sortUUIDs(entries []uuid.UUID) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].String() < entries[j].String()
	})
}

func TestAllRootJobIDs(t *testing.T) {
	dir := t.TempDir()
	q, err := fsjobqueue.New(dir)
	require.Nil(t, err)
	require.NotNil(t, q)

	var rootJobs []uuid.UUID

	// root with no dependencies
	jidRoot1, err := q.Enqueue("oneRoot", nil, nil, "OneRootJob")
	require.Nil(t, err)
	rootJobs = append(rootJobs, jidRoot1)

	// root with 2 dependencies
	jid1, err := q.Enqueue("twoDeps", nil, nil, "TwoDepJobs")
	require.Nil(t, err)
	jid2, err := q.Enqueue("twoDeps", nil, nil, "TwoDepJobs")
	require.Nil(t, err)
	jidRoot2, err := q.Enqueue("twoDeps", nil, []uuid.UUID{jid1, jid2}, "TwoDepJobs")
	require.Nil(t, err)
	rootJobs = append(rootJobs, jidRoot2)

	// root with 2 dependencies, one shared with the previous root
	jid3, err := q.Enqueue("sharedDeps", nil, nil, "SharedDepJobs")
	require.Nil(t, err)
	jidRoot3, err := q.Enqueue("sharedDeps", nil, []uuid.UUID{jid1, jid3}, "SharedDepJobs")
	require.Nil(t, err)
	rootJobs = append(rootJobs, jidRoot3)

	sortUUIDs(rootJobs)
	roots, err := q.AllRootJobIDs()
	require.Nil(t, err)
	require.Greater(t, len(roots), 0)
	sortUUIDs(roots)
	require.Equal(t, rootJobs, roots)
}

func TestAllJobIDs(t *testing.T) {
	dir := t.TempDir()
	q, err := fsjobqueue.New(dir)
	require.Nil(t, err)
	require.NotNil(t, q)

	var allJobs []uuid.UUID

	// root with no dependencies
	jidRoot1, err := q.Enqueue("oneRoot", nil, nil, "OneRootJob")
	require.Nil(t, err)
	allJobs = append(allJobs, jidRoot1)

	// root with 2 dependencies
	jid1, err := q.Enqueue("twoDeps", nil, nil, "TwoDepJobs")
	require.Nil(t, err)
	allJobs = append(allJobs, jid1)
	jid2, err := q.Enqueue("twoDeps", nil, nil, "TwoDepJobs")
	require.Nil(t, err)
	allJobs = append(allJobs, jid2)
	jidRoot2, err := q.Enqueue("twoDeps", nil, []uuid.UUID{jid1, jid2}, "TwoDepJobs")
	require.Nil(t, err)
	allJobs = append(allJobs, jidRoot2)

	sortUUIDs(allJobs)
	jobs, err := q.AllJobIDs()
	require.Nil(t, err)
	require.Greater(t, len(jobs), 0)
	sortUUIDs(jobs)
	require.Equal(t, allJobs, jobs)
}

func TestRemoveJob(t *testing.T) {
	dir := t.TempDir()
	q, err := fsjobqueue.New(dir)
	require.Nil(t, err)
	require.NotNil(t, q)

	// root with no dependencies
	jidRoot1, err := q.Enqueue("oneRoot", nil, nil, "OneRootJob")
	require.Nil(t, err)

	err = q.RemoveJob(jidRoot1)
	require.Nil(t, err)
	jobs, err := q.AllJobIDs()
	require.Nil(t, err)
	require.Equal(t, 0, len(jobs))

	// root with 2 dependencies
	jid1, err := q.Enqueue("twoDeps", nil, nil, "TwoDepJobs")
	require.Nil(t, err)
	jid2, err := q.Enqueue("twoDeps", nil, nil, "TwoDepJobs")
	require.Nil(t, err)
	jidRoot2, err := q.Enqueue("twoDeps", nil, []uuid.UUID{jid1, jid2}, "TwoDepJobs")
	require.Nil(t, err)

	// root with 2 dependencies, one shared with the previous root
	jid3, err := q.Enqueue("sharedDeps", nil, nil, "SharedDepJobs")
	require.Nil(t, err)
	jidRoot3, err := q.Enqueue("sharedDeps", nil, []uuid.UUID{jid1, jid3}, "SharedDepJobs")
	require.Nil(t, err)

	// This should only remove jidRoot2 and jid2, leaving jidRoot3, jid1, jid3
	err = q.RemoveJob(jidRoot2)
	require.Nil(t, err)
	jobs, err = q.AllJobIDs()
	require.Nil(t, err)
	require.Equal(t, 3, len(jobs))
	sortUUIDs(jobs)
	expected := []uuid.UUID{jidRoot3, jid1, jid3}
	sortUUIDs(expected)
	assert.Equal(t, expected, jobs)

	// This should remove the rest
	err = q.RemoveJob(jidRoot3)
	require.Nil(t, err)
	jobs, err = q.AllJobIDs()
	require.Nil(t, err)
	require.Equal(t, 0, len(jobs))
}
