package fsjobqueue_test

import (
	"os"
	"path"
	"testing"

	"github.com/osbuild/osbuild-composer/pkg/jobqueue"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/jobqueuetest"
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
