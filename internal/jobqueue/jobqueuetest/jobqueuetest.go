// Package jobqueuetest provides test functions to verify a JobQueue
// implementation satisfies the interface in package jobqueue.

package jobqueuetest

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/pkg/jobqueue"
	"github.com/stretchr/testify/require"
)

type MakeJobQueue func() (q jobqueue.JobQueue, stop func(), err error)

type testResult struct {
}

func TestJobQueue(t *testing.T, makeJobQueue MakeJobQueue) {
	wrap := func(f func(t *testing.T, q jobqueue.JobQueue)) func(*testing.T) {
		q, stop, err := makeJobQueue()
		require.NoError(t, err)
		return func(t *testing.T) {
			defer stop() // use defer because f() might call testing.T.FailNow()
			f(t, q)
		}
	}

	t.Run("errors", wrap(testErrors))
	t.Run("args", wrap(testArgs))
	t.Run("cancel", wrap(testCancel))
	t.Run("requeue", wrap(testRequeue))
	t.Run("requeue-limit", wrap(testRequeueLimit))
	t.Run("job-types", wrap(testJobTypes))
	t.Run("dependencies", wrap(testDependencies))
	t.Run("multiple-workers", wrap(testMultipleWorkers))
	t.Run("multiple-workers-single-job-type", wrap(testMultipleWorkersSingleJobType))
	t.Run("heartbeats", wrap(testHeartbeats))
	t.Run("timeout", wrap(testDequeueTimeout))
	t.Run("dequeue-by-id", wrap(testDequeueByID))
	t.Run("multiple-channels", wrap(testMultipleChannels))
	t.Run("100-dequeuers", wrap(test100dequeuers))
	t.Run("workers", wrap(testWorkers))
}

func pushTestJob(t *testing.T, q jobqueue.JobQueue, jobType string, args interface{}, dependencies []uuid.UUID, channel string) uuid.UUID {
	t.Helper()
	id, err := q.Enqueue(jobType, args, dependencies, channel)
	require.NoError(t, err)
	require.NotEmpty(t, id)
	return id
}

func finishNextTestJob(t *testing.T, q jobqueue.JobQueue, jobType string, result interface{}, deps []uuid.UUID) uuid.UUID {
	id, tok, d, typ, args, err := q.Dequeue(context.Background(), uuid.Nil, []string{jobType}, []string{""})
	require.NoError(t, err)
	require.NotEmpty(t, id)
	require.NotEmpty(t, tok)
	require.ElementsMatch(t, deps, d)
	require.Equal(t, jobType, typ)
	require.NotNil(t, args)

	requeued, err := q.RequeueOrFinishJob(id, 0, result)
	require.NoError(t, err)
	require.False(t, requeued)

	return id
}

func testErrors(t *testing.T, q jobqueue.JobQueue) {
	// not serializable to JSON
	id, err := q.Enqueue("test", make(chan string), nil, "")
	require.Error(t, err)
	require.Equal(t, uuid.Nil, id)

	// invalid dependency
	id, err = q.Enqueue("test", "arg0", []uuid.UUID{uuid.New()}, "")
	require.Error(t, err)
	require.Equal(t, uuid.Nil, id)

	// token gets removed
	pushTestJob(t, q, "octopus", nil, nil, "")
	id, tok, _, _, _, err := q.Dequeue(context.Background(), uuid.Nil, []string{"octopus"}, []string{""})
	require.NoError(t, err)
	require.NotEmpty(t, tok)

	idFromT, err := q.IdFromToken(tok)
	require.NoError(t, err)
	require.Equal(t, id, idFromT)

	requeued, err := q.RequeueOrFinishJob(id, 0, nil)
	require.NoError(t, err)
	require.False(t, requeued)

	// Make sure the token gets removed
	id, err = q.IdFromToken(tok)
	require.Equal(t, uuid.Nil, id)
	require.Equal(t, jobqueue.ErrNotExist, err)
}

func testArgs(t *testing.T, q jobqueue.JobQueue) {
	type argument struct {
		I int
		S string
	}

	oneargs := argument{7, "🐠"}
	var parsedArgs argument

	one := pushTestJob(t, q, "fish", oneargs, nil, "toucan")

	// Read job params before Dequeue
	jtype, jargs, jdeps, jchan, err := q.Job(one)
	require.NoError(t, err)
	err = json.Unmarshal(jargs, &parsedArgs)
	require.NoError(t, err)
	require.Equal(t, oneargs, parsedArgs)
	require.Empty(t, jdeps)
	require.Equal(t, "toucan", jchan)
	require.Equal(t, "fish", jtype)

	twoargs := argument{42, "🐙"}
	two := pushTestJob(t, q, "octopus", twoargs, nil, "kingfisher")

	// Read job params before Dequeue
	jtype, jargs, jdeps, jchan, err = q.Job(two)
	require.NoError(t, err)
	err = json.Unmarshal(jargs, &parsedArgs)
	require.NoError(t, err)
	require.Equal(t, twoargs, parsedArgs)
	require.Empty(t, jdeps)
	require.Equal(t, "kingfisher", jchan)
	require.Equal(t, "octopus", jtype)

	id, tok, deps, typ, args, err := q.Dequeue(context.Background(), uuid.Nil, []string{"octopus"}, []string{"kingfisher"})
	require.NoError(t, err)
	require.Equal(t, two, id)
	require.NotEmpty(t, tok)
	require.Empty(t, deps)
	require.Equal(t, "octopus", typ)
	err = json.Unmarshal(args, &parsedArgs)
	require.NoError(t, err)
	require.Equal(t, twoargs, parsedArgs)

	// Read job params after Dequeue
	jtype, jargs, jdeps, jchan, err = q.Job(id)
	require.NoError(t, err)
	require.Equal(t, args, jargs)
	require.Equal(t, deps, jdeps)
	require.Equal(t, "kingfisher", jchan)
	require.Equal(t, typ, jtype)

	id, tok, deps, typ, args, err = q.Dequeue(context.Background(), uuid.Nil, []string{"fish"}, []string{"toucan"})
	require.NoError(t, err)
	require.Equal(t, one, id)
	require.NotEmpty(t, tok)
	require.Empty(t, deps)
	require.Equal(t, "fish", typ)
	err = json.Unmarshal(args, &parsedArgs)
	require.NoError(t, err)
	require.Equal(t, oneargs, parsedArgs)

	jtype, jargs, jdeps, jchan, err = q.Job(id)
	require.NoError(t, err)
	require.Equal(t, args, jargs)
	require.Equal(t, deps, jdeps)
	require.Equal(t, "toucan", jchan)
	require.Equal(t, typ, jtype)

	_, _, _, _, err = q.Job(uuid.New())
	require.Error(t, err)
}

func testJobTypes(t *testing.T, q jobqueue.JobQueue) {
	one := pushTestJob(t, q, "octopus", nil, nil, "")
	two := pushTestJob(t, q, "clownfish", nil, nil, "")

	require.Equal(t, two, finishNextTestJob(t, q, "clownfish", testResult{}, nil))
	require.Equal(t, one, finishNextTestJob(t, q, "octopus", testResult{}, nil))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	id, tok, deps, typ, args, err := q.Dequeue(ctx, uuid.Nil, []string{"zebra"}, []string{""})
	require.Equal(t, err, jobqueue.ErrDequeueTimeout)
	require.Equal(t, uuid.Nil, id)
	require.Equal(t, uuid.Nil, tok)
	require.Empty(t, deps)
	require.Equal(t, "", typ)
	require.Nil(t, args)
}

func testDequeueTimeout(t *testing.T, q jobqueue.JobQueue) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*20)
	defer cancel()
	_, _, _, _, _, err := q.Dequeue(ctx, uuid.Nil, []string{"octopus"}, []string{""})
	require.Equal(t, jobqueue.ErrDequeueTimeout, err)

	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	_, _, _, _, _, err = q.Dequeue(ctx2, uuid.Nil, []string{"octopus"}, []string{""})
	require.Equal(t, jobqueue.ErrDequeueTimeout, err)
}

func testDependencies(t *testing.T, q jobqueue.JobQueue) {
	t.Run("done-before-pushing-dependant", func(t *testing.T) {
		one := pushTestJob(t, q, "test", nil, nil, "")
		two := pushTestJob(t, q, "test", nil, nil, "")

		r := []uuid.UUID{}
		r = append(r, finishNextTestJob(t, q, "test", testResult{}, nil))
		r = append(r, finishNextTestJob(t, q, "test", testResult{}, nil))
		require.ElementsMatch(t, []uuid.UUID{one, two}, r)

		j := pushTestJob(t, q, "test", nil, []uuid.UUID{one, two}, "")
		_, _, _, _, _, _, _, _, dependents, err := q.JobStatus(one)
		require.NoError(t, err)
		require.ElementsMatch(t, dependents, []uuid.UUID{j})

		jobType, _, _, queued, started, finished, canceled, deps, dependents, err := q.JobStatus(j)
		require.NoError(t, err)
		require.Equal(t, jobType, "test")
		require.True(t, !queued.IsZero())
		require.True(t, started.IsZero())
		require.True(t, finished.IsZero())
		require.False(t, canceled)
		require.ElementsMatch(t, deps, []uuid.UUID{one, two})
		require.Empty(t, dependents)

		require.Equal(t, j, finishNextTestJob(t, q, "test", testResult{}, []uuid.UUID{one, two}))

		jobType, _, result, queued, started, finished, canceled, deps, dependents, err := q.JobStatus(j)
		require.NoError(t, err)
		require.Equal(t, jobType, "test")
		require.True(t, !queued.IsZero())
		require.True(t, !started.IsZero())
		require.True(t, !finished.IsZero())
		require.False(t, canceled)
		require.ElementsMatch(t, deps, []uuid.UUID{one, two})
		require.Empty(t, dependents)

		err = json.Unmarshal(result, &testResult{})
		require.NoError(t, err)
	})

	t.Run("done-after-pushing-dependant", func(t *testing.T) {
		one := pushTestJob(t, q, "test", nil, nil, "")
		two := pushTestJob(t, q, "test", nil, nil, "")

		j := pushTestJob(t, q, "test", nil, []uuid.UUID{one, two}, "")
		jobType, _, _, queued, started, finished, canceled, deps, _, err := q.JobStatus(j)
		require.NoError(t, err)
		require.Equal(t, jobType, "test")
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

		jobType, _, result, queued, started, finished, canceled, deps, _, err := q.JobStatus(j)
		require.NoError(t, err)
		require.Equal(t, jobType, "test")
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
func testMultipleWorkers(t *testing.T, q jobqueue.JobQueue) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		id, tok, deps, typ, args, err := q.Dequeue(ctx, uuid.Nil, []string{"octopus"}, []string{""})
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
	id := pushTestJob(t, q, "clownfish", nil, nil, "")
	r, tok, deps, typ, args, err := q.Dequeue(context.Background(), uuid.Nil, []string{"clownfish"}, []string{""})
	require.NoError(t, err)
	require.Equal(t, id, r)
	require.NotEmpty(t, tok)
	require.Empty(t, deps)
	require.Equal(t, "clownfish", typ)
	require.Equal(t, json.RawMessage("null"), args)

	// Now wake up the Dequeue() in the goroutine and wait for it to finish.
	_ = pushTestJob(t, q, "octopus", nil, nil, "")
	<-done
}

func testMultipleWorkersSingleJobType(t *testing.T, q jobqueue.JobQueue) {
	var wg sync.WaitGroup
	wg.Add(2)

	// Start two listeners
	for i := 0; i < 2; i += 1 {
		go func() {
			defer wg.Add(-1)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			id, tok, deps, typ, args, err := q.Dequeue(ctx, uuid.Nil, []string{"clownfish"}, []string{""})
			require.NoError(t, err)
			require.NotEmpty(t, id)
			require.NotEmpty(t, tok)
			require.Empty(t, deps)
			require.Equal(t, "clownfish", typ)
			require.Equal(t, json.RawMessage("null"), args)
		}()
	}

	// Increase the likelihood that the above goroutines were scheduled and
	// is waiting in Dequeue().
	time.Sleep(10 * time.Millisecond)

	// Satisfy the first listener
	_ = pushTestJob(t, q, "clownfish", nil, nil, "")

	// Wait a bit for the listener to process the job
	time.Sleep(10 * time.Millisecond)

	// Satisfy the second listener
	_ = pushTestJob(t, q, "clownfish", nil, nil, "")

	wg.Wait()
}

func testCancel(t *testing.T, q jobqueue.JobQueue) {
	// Cancel a non-existing job
	err := q.CancelJob(uuid.New())
	require.Error(t, err)

	// Cancel a pending job
	id := pushTestJob(t, q, "clownfish", nil, nil, "")
	require.NotEmpty(t, id)
	err = q.CancelJob(id)
	require.NoError(t, err)
	jobType, _, result, _, _, _, canceled, _, _, err := q.JobStatus(id)
	require.NoError(t, err)
	require.Equal(t, jobType, "clownfish")
	require.True(t, canceled)
	require.Nil(t, result)
	_, err = q.RequeueOrFinishJob(id, 0, &testResult{})
	require.Error(t, err)

	// Cancel a running job, which should not dequeue the canceled job from above
	id = pushTestJob(t, q, "clownfish", nil, nil, "")
	require.NotEmpty(t, id)
	r, tok, deps, typ, args, err := q.Dequeue(context.Background(), uuid.Nil, []string{"clownfish"}, []string{""})
	require.NoError(t, err)
	require.Equal(t, id, r)
	require.NotEmpty(t, tok)
	require.Empty(t, deps)
	require.Equal(t, "clownfish", typ)
	require.Equal(t, json.RawMessage("null"), args)
	err = q.CancelJob(id)
	require.NoError(t, err)
	jobType, _, result, _, _, _, canceled, _, _, err = q.JobStatus(id)
	require.NoError(t, err)
	require.Equal(t, jobType, "clownfish")
	require.True(t, canceled)
	require.Nil(t, result)
	_, err = q.RequeueOrFinishJob(id, 0, &testResult{})
	require.Error(t, err)

	// Cancel a finished job, which is a no-op
	id = pushTestJob(t, q, "clownfish", nil, nil, "")
	require.NotEmpty(t, id)
	r, tok, deps, typ, args, err = q.Dequeue(context.Background(), uuid.Nil, []string{"clownfish"}, []string{""})
	require.NoError(t, err)
	require.Equal(t, id, r)
	require.NotEmpty(t, tok)
	require.Empty(t, deps)
	require.Equal(t, "clownfish", typ)
	require.Equal(t, json.RawMessage("null"), args)
	requeued, err := q.RequeueOrFinishJob(id, 0, &testResult{})
	require.NoError(t, err)
	require.False(t, requeued)
	err = q.CancelJob(id)
	require.Error(t, err)
	require.Equal(t, jobqueue.ErrNotRunning, err)
	jobType, _, result, _, _, _, canceled, _, _, err = q.JobStatus(id)
	require.NoError(t, err)
	require.Equal(t, jobType, "clownfish")
	require.False(t, canceled)
	err = json.Unmarshal(result, &testResult{})
	require.NoError(t, err)
}

func testRequeue(t *testing.T, q jobqueue.JobQueue) {
	// Requeue a non-existing job
	_, err := q.RequeueOrFinishJob(uuid.New(), 1, nil)
	require.Error(t, err)

	// Requeue a pending job
	id := pushTestJob(t, q, "clownfish", nil, nil, "")
	require.NotEmpty(t, id)
	_, err = q.RequeueOrFinishJob(id, 1, nil)
	require.Error(t, err)

	// Requeue a running job
	r, tok1, deps, typ, args, err := q.Dequeue(context.Background(), uuid.Nil, []string{"clownfish"}, []string{""})
	require.NoError(t, err)
	require.Equal(t, id, r)
	require.NotEmpty(t, tok1)
	require.Empty(t, deps)
	require.Equal(t, "clownfish", typ)
	require.Equal(t, json.RawMessage("null"), args)
	requeued, err := q.RequeueOrFinishJob(id, 1, nil)
	require.NoError(t, err)
	require.True(t, requeued)
	r, tok2, deps, typ, args, err := q.Dequeue(context.Background(), uuid.Nil, []string{"clownfish"}, []string{""})
	require.NoError(t, err)
	require.Equal(t, id, r)
	require.NotEmpty(t, tok2)
	require.NotEqual(t, tok1, tok2)
	require.Empty(t, deps)
	require.Equal(t, "clownfish", typ)
	require.Equal(t, json.RawMessage("null"), args)
	jobType, _, result, _, _, _, canceled, _, _, err := q.JobStatus(id)
	require.NoError(t, err)
	require.Equal(t, jobType, "clownfish")
	require.False(t, canceled)
	require.Nil(t, result)
	requeued, err = q.RequeueOrFinishJob(id, 0, &testResult{})
	require.NoError(t, err)
	require.False(t, requeued)

	// Requeue a finished job
	_, err = q.RequeueOrFinishJob(id, 1, nil)
	require.Error(t, err)
}

func testRequeueLimit(t *testing.T, q jobqueue.JobQueue) {
	// Start a job
	id := pushTestJob(t, q, "clownfish", nil, nil, "")
	require.NotEmpty(t, id)
	_, _, _, _, _, err := q.Dequeue(context.Background(), uuid.Nil, []string{"clownfish"}, []string{""})
	require.NoError(t, err)
	// Requeue once
	requeued, err := q.RequeueOrFinishJob(id, 1, nil)
	require.NoError(t, err)
	require.True(t, requeued)
	// Start again
	_, _, _, _, _, err = q.Dequeue(context.Background(), uuid.Nil, []string{"clownfish"}, []string{""})
	require.NoError(t, err)
	_, _, result, _, _, finished, _, _, _, err := q.JobStatus(id)
	require.NoError(t, err)
	require.True(t, finished.IsZero())
	require.Nil(t, result)
	// Requeue a second time, this time finishing it
	requeued, err = q.RequeueOrFinishJob(id, 1, &testResult{})
	require.NoError(t, err)
	require.False(t, requeued)
	_, _, result, _, _, finished, _, _, _, err = q.JobStatus(id)
	require.NoError(t, err)
	require.False(t, finished.IsZero())
	require.NotNil(t, result)
}

func testHeartbeats(t *testing.T, q jobqueue.JobQueue) {
	id := pushTestJob(t, q, "octopus", nil, nil, "")
	// No heartbeats for queued job
	require.Empty(t, q.Heartbeats(time.Second*0))

	r, tok, _, _, _, err := q.Dequeue(context.Background(), uuid.Nil, []string{"octopus"}, []string{""})
	require.NoError(t, err)
	require.Equal(t, id, r)
	require.NotEmpty(t, tok)

	tokens := q.Heartbeats(time.Second * 0)
	require.NoError(t, err)
	require.Contains(t, tokens, tok)

	time.Sleep(50 * time.Millisecond)
	tokens = q.Heartbeats(time.Millisecond * 50)
	require.NoError(t, err)
	require.Contains(t, tokens, tok)

	require.Empty(t, q.Heartbeats(time.Hour*24))

	id2, err := q.IdFromToken(tok)
	require.NoError(t, err)
	require.Equal(t, id2, id)

	requeued, err := q.RequeueOrFinishJob(id, 0, &testResult{})
	require.NoError(t, err)
	require.False(t, requeued)

	// No heartbeats for finished job
	require.Empty(t, q.Heartbeats(time.Second*0))
	require.NotContains(t, q.Heartbeats(time.Second*0), tok)
	_, err = q.IdFromToken(tok)
	require.Equal(t, err, jobqueue.ErrNotExist)
}

func testDequeueByID(t *testing.T, q jobqueue.JobQueue) {
	t.Run("basic", func(t *testing.T) {
		one := pushTestJob(t, q, "octopus", nil, nil, "")
		two := pushTestJob(t, q, "octopus", nil, nil, "")

		tok, d, typ, args, err := q.DequeueByID(context.Background(), one, uuid.Nil)
		require.NoError(t, err)
		require.NotEmpty(t, tok)
		require.Empty(t, d)
		require.Equal(t, "octopus", typ)
		require.NotNil(t, args)

		requeued, err := q.RequeueOrFinishJob(one, 0, nil)
		require.NoError(t, err)
		require.False(t, requeued)

		require.Equal(t, two, finishNextTestJob(t, q, "octopus", testResult{}, nil))
	})

	t.Run("cannot dequeue a job without finished deps", func(t *testing.T) {
		one := pushTestJob(t, q, "octopus", nil, nil, "")
		two := pushTestJob(t, q, "octopus", nil, []uuid.UUID{one}, "")

		_, _, _, _, err := q.DequeueByID(context.Background(), two, uuid.Nil)
		require.Equal(t, jobqueue.ErrNotPending, err)

		require.Equal(t, one, finishNextTestJob(t, q, "octopus", testResult{}, nil))
		require.Equal(t, two, finishNextTestJob(t, q, "octopus", testResult{}, []uuid.UUID{one}))
	})

	t.Run("cannot dequeue a non-pending job", func(t *testing.T) {
		one := pushTestJob(t, q, "octopus", nil, nil, "")

		_, _, _, _, _, err := q.Dequeue(context.Background(), uuid.Nil, []string{"octopus"}, []string{""})
		require.NoError(t, err)

		_, _, _, _, err = q.DequeueByID(context.Background(), one, uuid.Nil)
		require.Equal(t, jobqueue.ErrNotPending, err)

		requeued, err := q.RequeueOrFinishJob(one, 0, nil)
		require.NoError(t, err)
		require.False(t, requeued)

		_, _, _, _, err = q.DequeueByID(context.Background(), one, uuid.Nil)
		require.Equal(t, jobqueue.ErrNotPending, err)
	})
}

func testMultipleChannels(t *testing.T, q jobqueue.JobQueue) {
	t.Run("two single channel dequeuers", func(t *testing.T) {
		var wg sync.WaitGroup

		oneChan := make(chan uuid.UUID, 1)
		twoChan := make(chan uuid.UUID, 1)

		// dequeue kingfisher channel
		wg.Add(1)
		go func() {
			defer wg.Done()

			id, _, _, _, _, err := q.Dequeue(context.Background(), uuid.Nil, []string{"octopus"}, []string{"kingfisher"})
			require.NoError(t, err)

			expectedID := <-twoChan
			require.Equal(t, expectedID, id)
		}()

		// dequeue toucan channel
		wg.Add(1)
		go func() {
			defer wg.Done()

			id, _, _, _, _, err := q.Dequeue(context.Background(), uuid.Nil, []string{"octopus"}, []string{"toucan"})
			require.NoError(t, err)

			expectedID := <-oneChan
			require.Equal(t, expectedID, id)
		}()

		// enqueue into toucan channel
		one := pushTestJob(t, q, "octopus", nil, nil, "toucan")
		oneChan <- one
		// enqueue into kingfisher channel
		two := pushTestJob(t, q, "octopus", nil, nil, "kingfisher")
		twoChan <- two
		wg.Wait()
	})

	t.Run("one double channel dequeuers", func(t *testing.T) {
		var wg sync.WaitGroup

		oneChan := make(chan uuid.UUID, 1)
		twoChan := make(chan uuid.UUID, 1)

		// dequeue kingfisher channel
		wg.Add(1)
		go func() {
			defer wg.Done()

			id, _, _, _, _, err := q.Dequeue(context.Background(), uuid.Nil, []string{"octopus"}, []string{"kingfisher", "toucan"})
			require.NoError(t, err)

			expectedID := <-oneChan
			require.Equal(t, expectedID, id)

			id, _, _, _, _, err = q.Dequeue(context.Background(), uuid.Nil, []string{"octopus"}, []string{"kingfisher", "toucan"})
			require.NoError(t, err)

			expectedID = <-twoChan
			require.Equal(t, expectedID, id)
		}()

		// enqueue into toucan channel
		one := pushTestJob(t, q, "octopus", nil, nil, "toucan")
		oneChan <- one
		// enqueue into kingfisher channel
		two := pushTestJob(t, q, "octopus", nil, nil, "kingfisher")
		twoChan <- two
		wg.Wait()
	})

	t.Run("dequeing no-existing channel", func(t *testing.T) {
		// enqueue into toucan channel
		pushTestJob(t, q, "octopus", nil, nil, "toucan")

		// dequeue from an empty channel
		ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(time.Millisecond*100))
		defer cancel()
		_, _, _, _, _, err := q.Dequeue(ctx, uuid.Nil, []string{"octopus"}, []string{""})
		require.ErrorIs(t, err, jobqueue.ErrDequeueTimeout)

		// dequeue from toucan channel
		_, _, _, _, _, err = q.Dequeue(context.Background(), uuid.Nil, []string{"octopus"}, []string{"toucan"})
		require.NoError(t, err)

		// enqueue into an empty channel
		pushTestJob(t, q, "octopus", nil, nil, "")

		// dequeue from toucan channel
		ctx2, cancel2 := context.WithDeadline(context.Background(), time.Now().Add(time.Millisecond*100))
		defer cancel2()
		_, _, _, _, _, err = q.Dequeue(ctx2, uuid.Nil, []string{"octopus"}, []string{"toucan"})
		require.ErrorIs(t, err, jobqueue.ErrDequeueTimeout)

		// dequeue from an empty channel
		_, _, _, _, _, err = q.Dequeue(context.Background(), uuid.Nil, []string{"octopus"}, []string{""})
		require.NoError(t, err)
	})
}

// Tests that jobqueue implementations can have "unlimited" number of
// dequeuers.
// This was an issue in dbjobqueue in past: It used one DB connection per
// a dequeuer and there was a limit of DB connection count.
func test100dequeuers(t *testing.T, q jobqueue.JobQueue) {
	var wg sync.WaitGroup

	// Create 100 dequeuers
	const count = 100
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer func() {
				wg.Done()
			}()

			finishNextTestJob(t, q, "octopus", testResult{}, nil)
		}()
	}

	// wait a bit for all goroutines to initialize
	time.Sleep(100 * time.Millisecond)

	// try to do some other operations on the jobqueue
	id := pushTestJob(t, q, "clownfish", nil, nil, "")

	_, _, _, _, _, _, _, _, _, err := q.JobStatus(id)
	require.NoError(t, err)

	finishNextTestJob(t, q, "clownfish", testResult{}, nil)

	// fulfill the needs of all dequeuers
	for i := 0; i < count; i++ {
		pushTestJob(t, q, "octopus", nil, nil, "")
	}

	wg.Wait()

}

// Registers workers and runs jobs against them
func testWorkers(t *testing.T, q jobqueue.JobQueue) {
	one := pushTestJob(t, q, "octopus", nil, nil, "chan")

	w1, err := q.InsertWorker("chan", "x86_64")
	require.NoError(t, err)
	w2, err := q.InsertWorker("chan", "aarch64")
	require.NoError(t, err)

	workers, err := q.Workers(0)
	require.NoError(t, err)
	require.Len(t, workers, 2)
	require.Equal(t, "chan", workers[0].Channel)

	workers, err = q.Workers(time.Hour * 24)
	require.NoError(t, err)
	require.Len(t, workers, 0)

	_, _, _, _, _, err = q.Dequeue(context.Background(), w1, []string{"octopus"}, []string{"chan"})
	require.NoError(t, err)

	err = q.DeleteWorker(w1)
	require.Equal(t, err, jobqueue.ErrActiveJobs)

	err = q.UpdateWorkerStatus(w1)
	require.NoError(t, err)

	err = q.UpdateWorkerStatus(uuid.New())
	require.Equal(t, err, jobqueue.ErrWorkerNotExist)

	requeued, err := q.RequeueOrFinishJob(one, 0, &testResult{})
	require.NoError(t, err)
	require.False(t, requeued)

	err = q.DeleteWorker(w1)
	require.NoError(t, err)

	err = q.DeleteWorker(w2)
	require.NoError(t, err)
}
