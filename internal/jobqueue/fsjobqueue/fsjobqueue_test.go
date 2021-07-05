package fsjobqueue_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
)

type testResult struct {
}

func cleanupTempDir(t *testing.T, dir string) {
	err := os.RemoveAll(dir)
	require.NoError(t, err)
}

func newTemporaryQueue(t *testing.T) (jobqueue.JobQueue, string) {
	dir, err := ioutil.TempDir("", "jobqueue-test-")
	require.NoError(t, err)

	q, err := fsjobqueue.New(dir)
	require.NoError(t, err)
	require.NotNil(t, q)

	return q, dir
}

func pushTestJob(t *testing.T, q jobqueue.JobQueue, jobType string, args interface{}, dependencies []uuid.UUID) uuid.UUID {
	t.Helper()
	id, err := q.Enqueue(jobType, args, dependencies)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	return id
}

func finishNextTestJob(t *testing.T, q jobqueue.JobQueue, jobType string, result interface{}, deps []uuid.UUID) uuid.UUID {
	id, tok, d, typ, args, err := q.Dequeue(context.Background(), []string{jobType})
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.NotEmpty(t, tok)
	require.ElementsMatch(t, deps, d)
	require.Equal(t, jobType, typ)
	require.NotNil(t, args)

	err = q.FinishJob(id, result)
	require.NoError(t, err)

	return id
}

func TestNonExistant(t *testing.T) {
	q, err := fsjobqueue.New("/non-existant-directory")
	require.Error(t, err)
	require.Nil(t, q)
}

func TestErrors(t *testing.T) {
	q, dir := newTemporaryQueue(t)
	defer cleanupTempDir(t, dir)

	// not serializable to JSON
	id, err := q.Enqueue("test", make(chan string), nil)
	require.Error(t, err)
	require.Equal(t, uuid.Nil, id)

	// invalid dependency
	id, err = q.Enqueue("test", "arg0", []uuid.UUID{uuid.New()})
	require.Error(t, err)
	require.Equal(t, uuid.Nil, id)

	// token gets removed
	pushTestJob(t, q, "octopus", nil, nil)
	id, tok, _, _, _, err := q.Dequeue(context.Background(), []string{"octopus"})
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	idFromT, err := q.IdFromToken(tok)
	require.NoError(t, err)
	require.Equal(t, id, idFromT)

	err = q.FinishJob(id, nil)
	require.NoError(t, err)

	// Make sure the token gets removed
	id, err = q.IdFromToken(tok)
	require.Equal(t, uuid.Nil, id)
	require.Equal(t, jobqueue.ErrNotExist, err)
}

func TestArgs(t *testing.T) {
	type argument struct {
		I int
		S string
	}

	q, dir := newTemporaryQueue(t)
	defer cleanupTempDir(t, dir)

	oneargs := argument{7, "🐠"}
	one := pushTestJob(t, q, "fish", oneargs, nil)

	twoargs := argument{42, "🐙"}
	two := pushTestJob(t, q, "octopus", twoargs, nil)

	var parsedArgs argument

	id, tok, deps, typ, args, err := q.Dequeue(context.Background(), []string{"octopus"})
	require.NoError(t, err)
	require.Equal(t, two, id)
	require.NotEmpty(t, tok)
	require.Empty(t, deps)
	require.Equal(t, "octopus", typ)
	err = json.Unmarshal(args, &parsedArgs)
	require.NoError(t, err)
	require.Equal(t, twoargs, parsedArgs)

	// Read job params after Dequeue
	jtype, jargs, jdeps, err := q.Job(id)
	require.NoError(t, err)
	require.Equal(t, args, jargs)
	require.Equal(t, deps, jdeps)
	require.Equal(t, typ, jtype)

	id, tok, deps, typ, args, err = q.Dequeue(context.Background(), []string{"fish"})
	require.NoError(t, err)
	require.Equal(t, one, id)
	require.NotEmpty(t, tok)
	require.Empty(t, deps)
	require.Equal(t, "fish", typ)
	err = json.Unmarshal(args, &parsedArgs)
	require.NoError(t, err)
	require.Equal(t, oneargs, parsedArgs)

	jtype, jargs, jdeps, err = q.Job(id)
	require.NoError(t, err)
	require.Equal(t, args, jargs)
	require.Equal(t, deps, jdeps)
	require.Equal(t, typ, jtype)

	_, _, _, err = q.Job(uuid.New())
	require.Error(t, err)
}

func TestJobTypes(t *testing.T) {
	q, dir := newTemporaryQueue(t)
	defer cleanupTempDir(t, dir)

	one := pushTestJob(t, q, "octopus", nil, nil)
	two := pushTestJob(t, q, "clownfish", nil, nil)

	require.Equal(t, two, finishNextTestJob(t, q, "clownfish", testResult{}, nil))
	require.Equal(t, one, finishNextTestJob(t, q, "octopus", testResult{}, nil))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	id, tok, deps, typ, args, err := q.Dequeue(ctx, []string{"zebra"})
	require.Equal(t, err, context.Canceled)
	require.Equal(t, uuid.Nil, id)
	require.Equal(t, uuid.Nil, tok)
	require.Empty(t, deps)
	require.Equal(t, "", typ)
	require.Nil(t, args)
}

func TestDependencies(t *testing.T) {
	q, dir := newTemporaryQueue(t)
	defer cleanupTempDir(t, dir)

	t.Run("done-before-pushing-dependant", func(t *testing.T) {
		one := pushTestJob(t, q, "test", nil, nil)
		two := pushTestJob(t, q, "test", nil, nil)

		r := []uuid.UUID{}
		r = append(r, finishNextTestJob(t, q, "test", testResult{}, nil))
		r = append(r, finishNextTestJob(t, q, "test", testResult{}, nil))
		require.ElementsMatch(t, []uuid.UUID{one, two}, r)

		j := pushTestJob(t, q, "test", nil, []uuid.UUID{one, two})
		_, queued, started, finished, canceled, deps, err := q.JobStatus(j)
		require.NoError(t, err)
		require.True(t, !queued.IsZero())
		require.True(t, started.IsZero())
		require.True(t, finished.IsZero())
		require.False(t, canceled)
		require.ElementsMatch(t, deps, []uuid.UUID{one, two})

		require.Equal(t, j, finishNextTestJob(t, q, "test", testResult{}, []uuid.UUID{one, two}))

		result, queued, started, finished, canceled, deps, err := q.JobStatus(j)
		require.NoError(t, err)
		require.True(t, !queued.IsZero())
		require.True(t, !started.IsZero())
		require.True(t, !finished.IsZero())
		require.False(t, canceled)
		require.ElementsMatch(t, deps, []uuid.UUID{one, two})

		err = json.Unmarshal(result, &testResult{})
		require.NoError(t, err)
	})

	t.Run("done-after-pushing-dependant", func(t *testing.T) {
		one := pushTestJob(t, q, "test", nil, nil)
		two := pushTestJob(t, q, "test", nil, nil)

		j := pushTestJob(t, q, "test", nil, []uuid.UUID{one, two})
		_, queued, started, finished, canceled, deps, err := q.JobStatus(j)
		require.NoError(t, err)
		require.True(t, !queued.IsZero())
		require.True(t, started.IsZero())
		require.True(t, finished.IsZero())
		require.False(t, canceled)
		require.ElementsMatch(t, deps, []uuid.UUID{one, two})

		r := []uuid.UUID{}
		r = append(r, finishNextTestJob(t, q, "test", testResult{}, nil))
		r = append(r, finishNextTestJob(t, q, "test", testResult{}, nil))
		require.ElementsMatch(t, []uuid.UUID{one, two}, r)

		require.Equal(t, j, finishNextTestJob(t, q, "test", testResult{}, []uuid.UUID{one, two}))

		result, queued, started, finished, canceled, deps, err := q.JobStatus(j)
		require.NoError(t, err)
		require.True(t, !queued.IsZero())
		require.True(t, !started.IsZero())
		require.True(t, !finished.IsZero())
		require.False(t, canceled)
		require.ElementsMatch(t, deps, []uuid.UUID{one, two})

		err = json.Unmarshal(result, &testResult{})
		require.NoError(t, err)
	})
}

// Test that a job queue allows parallel access to multiple workers, mainly to
// verify the quirky unlocking in Dequeue().
func TestMultipleWorkers(t *testing.T) {
	q, dir := newTemporaryQueue(t)
	defer cleanupTempDir(t, dir)

	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		id, tok, deps, typ, args, err := q.Dequeue(ctx, []string{"octopus"})
		require.NoError(t, err)
		require.NotEmpty(t, id)
		require.NotEmpty(t, tok)
		require.Empty(t, deps)
		require.Equal(t, "octopus", typ)
		require.Equal(t, json.RawMessage("null"), args)
	}()

	// Increase the likelihood that the above goroutine was scheduled and
	// is waiting in Dequeue().
	time.Sleep(10 * time.Millisecond)

	// This call to Dequeue() should not block on the one in the goroutine.
	id := pushTestJob(t, q, "clownfish", nil, nil)
	r, tok, deps, typ, args, err := q.Dequeue(context.Background(), []string{"clownfish"})
	require.NoError(t, err)
	require.Equal(t, id, r)
	require.NotEmpty(t, tok)
	require.Empty(t, deps)
	require.Equal(t, "clownfish", typ)
	require.Equal(t, json.RawMessage("null"), args)

	// Now wake up the Dequeue() in the goroutine and wait for it to finish.
	_ = pushTestJob(t, q, "octopus", nil, nil)
	<-done
}

func TestCancel(t *testing.T) {
	q, dir := newTemporaryQueue(t)
	defer cleanupTempDir(t, dir)

	// Cancel a non-existing job
	err := q.CancelJob(uuid.New())
	require.Error(t, err)

	// Cancel a pending job
	id := pushTestJob(t, q, "clownfish", nil, nil)
	require.NotEmpty(t, id)
	err = q.CancelJob(id)
	require.NoError(t, err)
	result, _, _, _, canceled, _, err := q.JobStatus(id)
	require.NoError(t, err)
	require.True(t, canceled)
	require.Nil(t, result)
	err = q.FinishJob(id, &testResult{})
	require.Error(t, err)

	// Cancel a running job, which should not dequeue the canceled job from above
	id = pushTestJob(t, q, "clownfish", nil, nil)
	require.NotEmpty(t, id)
	r, tok, deps, typ, args, err := q.Dequeue(context.Background(), []string{"clownfish"})
	require.NoError(t, err)
	require.Equal(t, id, r)
	require.NotEmpty(t, tok)
	require.Empty(t, deps)
	require.Equal(t, "clownfish", typ)
	require.Equal(t, json.RawMessage("null"), args)
	err = q.CancelJob(id)
	require.NoError(t, err)
	result, _, _, _, canceled, _, err = q.JobStatus(id)
	require.NoError(t, err)
	require.True(t, canceled)
	require.Nil(t, result)
	err = q.FinishJob(id, &testResult{})
	require.Error(t, err)

	// Cancel a finished job, which is a no-op
	id = pushTestJob(t, q, "clownfish", nil, nil)
	require.NotEmpty(t, id)
	r, tok, deps, typ, args, err = q.Dequeue(context.Background(), []string{"clownfish"})
	require.NoError(t, err)
	require.Equal(t, id, r)
	require.NotEmpty(t, tok)
	require.Empty(t, deps)
	require.Equal(t, "clownfish", typ)
	require.Equal(t, json.RawMessage("null"), args)
	err = q.FinishJob(id, &testResult{})
	require.NoError(t, err)
	err = q.CancelJob(id)
	require.NoError(t, err)
	result, _, _, _, canceled, _, err = q.JobStatus(id)
	require.NoError(t, err)
	require.False(t, canceled)
	err = json.Unmarshal(result, &testResult{})
	require.NoError(t, err)
}

func TestHeartbeats(t *testing.T) {
	q, dir := newTemporaryQueue(t)
	defer cleanupTempDir(t, dir)

	id := pushTestJob(t, q, "octopus", nil, nil)
	// No heartbeats for queued job
	require.Empty(t, q.Heartbeats(time.Second*0))

	r, tok, _, _, _, err := q.Dequeue(context.Background(), []string{"octopus"})
	require.NoError(t, err)
	require.Equal(t, id, r)
	require.NotEmpty(t, tok)

	tokens := q.Heartbeats(time.Second * 0)
	require.Contains(t, tokens, tok)
	require.Empty(t, q.Heartbeats(time.Hour*24))

	id2, err := q.IdFromToken(tok)
	require.NoError(t, err)
	require.Equal(t, id, id2)

	err = q.FinishJob(id, &testResult{})
	require.NoError(t, err)

	// No heartbeats for finished job
	require.Empty(t, q.Heartbeats(time.Second*0))
	_, err = q.IdFromToken(tok)
	require.Equal(t, jobqueue.ErrNotExist, err)
}
