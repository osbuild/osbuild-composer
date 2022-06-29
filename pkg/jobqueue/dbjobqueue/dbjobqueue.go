// Package dbjobqueue implements the interfaces in package jobqueue backed by a
// PostreSQL database.
//
// Data is stored non-reduntantly. Any data structure necessary for efficient
// access (e.g., dependants) are kept in memory.
package dbjobqueue

import (
	"container/list"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/osbuild/osbuild-composer/internal/common/slogger"
	"github.com/osbuild/osbuild-composer/pkg/jobqueue"
	"github.com/sirupsen/logrus"
)

const (
	sqlNotify   = `NOTIFY jobs`
	sqlListen   = `LISTEN jobs`
	sqlUnlisten = `UNLISTEN jobs`

	sqlEnqueue = `INSERT INTO jobs(id, type, args, queued_at, channel) VALUES ($1, $2, $3, NOW(), $4)`
	sqlDequeue = `
		UPDATE jobs
		SET token = $1, started_at = now()
		WHERE id = (
		  SELECT id
		  FROM ready_jobs
			  -- use ANY here, because "type in ()" doesn't work with bound parameters
			  -- literal syntax for this is '{"a", "b"}': https://www.postgresql.org/docs/13/arrays.html
		  WHERE type = ANY($2) AND channel = ANY($3)
		  LIMIT 1
		  FOR UPDATE SKIP LOCKED
		)
		RETURNING id, type, args`

	sqlDequeueByID = `
		UPDATE jobs
		SET token = $1, started_at = now()
		WHERE id = (
		  SELECT id
		  FROM ready_jobs
		  WHERE id = $2
		  LIMIT 1
		  FOR UPDATE SKIP LOCKED
		)
		RETURNING token, type, args, queued_at, started_at`

	sqlInsertDependency  = `INSERT INTO job_dependencies VALUES ($1, $2)`
	sqlQueryDependencies = `
		SELECT dependency_id
		FROM job_dependencies
		WHERE job_id = $1`
	sqlQueryDependents = `
		SELECT job_id
		FROM job_dependencies
		WHERE dependency_id = $1`

	sqlQueryJob = `
		SELECT type, args, channel, started_at, finished_at, canceled
		FROM jobs
		WHERE id = $1`
	sqlQueryJobStatus = `
		SELECT type, channel, result, queued_at, started_at, finished_at, canceled
		FROM jobs
		WHERE id = $1`
	sqlQueryRunningId = `
                SELECT id
                FROM jobs
                WHERE token = $1 AND finished_at IS NULL AND canceled = FALSE`
	sqlFinishJob = `
		UPDATE jobs
		SET finished_at = now(), result = $1
		WHERE id = $2 AND finished_at IS NULL
		RETURNING finished_at`
	sqlCancelJob = `
		UPDATE jobs
		SET canceled = TRUE
		WHERE id = $1 AND finished_at IS NULL
		RETURNING type, started_at`

	sqlInsertHeartbeat = `
                INSERT INTO heartbeats(token, id, heartbeat)
                VALUES ($1, $2, now())`
	sqlQueryHeartbeats = `
                SELECT token
                FROM heartbeats
                WHERE age(now(), heartbeat) > $1`
	sqlRefreshHeartbeat = `
                UPDATE heartbeats
                SET heartbeat = now()
                WHERE token = $1`
	sqlDeleteHeartbeat = `
                DELETE FROM heartbeats
                WHERE id = $1`
)

type DBJobQueue struct {
	logger       jobqueue.SimpleLogger
	pool         *pgxpool.Pool
	dequeuers    *dequeuers
	stopListener func()
}

// thread-safe list of dequeuers
type dequeuers struct {
	list  *list.List
	mutex sync.Mutex
}

func newDequeuers() *dequeuers {
	return &dequeuers{
		list: list.New(),
	}
}

func (d *dequeuers) pushBack(c chan struct{}) *list.Element {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.list.PushBack(c)
}

func (d *dequeuers) remove(e *list.Element) {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.list.Remove(e)
}

func (d *dequeuers) notifyAll() {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	cur := d.list.Front()
	for cur != nil {
		listenerChan := cur.Value.(chan struct{})

		// notify in a non-blocking way
		select {
		case listenerChan <- struct{}{}:
		default:
		}
		cur = cur.Next()
	}
}

// Config allows more detailed customization of queue behavior
type Config struct {
	// Logger is used for all logging of the queue, when not provided, the stanard
	// global logger (logrus) is used.
	Logger jobqueue.SimpleLogger
}

// New creates a new DBJobQueue object for `url` with default configuration.
func New(url string) (*DBJobQueue, error) {
	stdLogger := slogger.NewLogrusLogger(logrus.StandardLogger())
	config := Config{
		Logger: stdLogger,
	}
	return NewWithConfig(url, config)
}

// NewWithLogger creates a new DBJobQueue object for `url` with specific configuration.
func NewWithConfig(url string, config Config) (*DBJobQueue, error) {
	pool, err := pgxpool.Connect(context.Background(), url)
	if err != nil {
		return nil, fmt.Errorf("error establishing connection: %v", err)
	}

	listenContext, cancel := context.WithCancel(context.Background())
	q := &DBJobQueue{
		logger:       config.Logger,
		pool:         pool,
		dequeuers:    newDequeuers(),
		stopListener: cancel,
	}

	listenerReady := make(chan struct{})
	go q.listen(listenContext, listenerReady)

	// wait for the listener to become ready
	<-listenerReady

	return q, nil
}

func (q *DBJobQueue) listen(ctx context.Context, ready chan<- struct{}) {
	conn, err := q.pool.Acquire(ctx)
	if err != nil {
		panic(fmt.Errorf("error connecting to database: %v", err))
	}
	defer func() {
		// use the empty context as the listening context is already cancelled at this point
		_, err := conn.Exec(context.Background(), sqlUnlisten)
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			q.logger.Error(err, "Error unlistening for jobs in dequeue")
		}
		conn.Release()
	}()

	_, err = conn.Exec(ctx, sqlListen)
	if err != nil {
		panic(fmt.Errorf("error listening on jobs channel: %v", err))
	}

	ready <- struct{}{}

	for {
		_, err = conn.Conn().WaitForNotification(ctx)
		if err != nil {
			// shutdown the listener if the context is canceled
			if errors.Is(err, context.Canceled) {
				q.logger.Info("Shutting down the listener")
				return
			}

			// otherwise, just log the error and continue, there might just
			// be a temporary networking issue
			q.logger.Error(err, "Error waiting for notification on jobs channel")
			continue
		}

		// something happened in the database, notify all dequeuers
		q.dequeuers.notifyAll()
	}
}

func (q *DBJobQueue) Close() {
	q.stopListener()
	q.pool.Close()
}

func (q *DBJobQueue) Enqueue(jobType string, args interface{}, dependencies []uuid.UUID, channel string) (uuid.UUID, error) {
	conn, err := q.pool.Acquire(context.Background())
	if err != nil {
		return uuid.Nil, fmt.Errorf("error connecting to database: %v", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(context.Background())
	if err != nil {
		return uuid.Nil, fmt.Errorf("error starting database transaction: %v", err)
	}
	defer func() {
		err := tx.Rollback(context.Background())
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			q.logger.Error(err, "Error rolling back enqueue transaction")
		}
	}()

	id := uuid.New()
	_, err = tx.Exec(context.Background(), sqlEnqueue, id, jobType, args, channel)
	if err != nil {
		return uuid.Nil, fmt.Errorf("error enqueuing job: %v", err)
	}

	for _, d := range dependencies {
		_, err = tx.Exec(context.Background(), sqlInsertDependency, id, d)
		if err != nil {
			return uuid.Nil, fmt.Errorf("error inserting dependency: %v", err)
		}
	}

	_, err = tx.Exec(context.Background(), sqlNotify)
	if err != nil {
		return uuid.Nil, fmt.Errorf("error notifying jobs channel: %v", err)
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to commit database transaction: %v", err)
	}

	q.logger.Info("Enqueued job", "job_type", jobType, "job_id", id.String(), "job_dependencies", fmt.Sprintf("%+v", dependencies))

	return id, nil
}

func (q *DBJobQueue) Dequeue(ctx context.Context, jobTypes []string, channels []string) (uuid.UUID, uuid.UUID, []uuid.UUID, string, json.RawMessage, error) {
	// Return early if the context is already canceled.
	if err := ctx.Err(); err != nil {
		return uuid.Nil, uuid.Nil, nil, "", nil, jobqueue.ErrDequeueTimeout
	}

	// add ourselves as a dequeuer
	c := make(chan struct{}, 1)
	el := q.dequeuers.pushBack(c)
	defer q.dequeuers.remove(el)

	var id uuid.UUID
	var jobType string
	var args json.RawMessage
	token := uuid.New()
	for {
		var err error
		id, jobType, args, err = q.dequeueMaybe(ctx, token, jobTypes, channels)
		if err == nil {
			break
		}
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return uuid.Nil, uuid.Nil, nil, "", nil, fmt.Errorf("error dequeuing job: %v", err)
		}

		// no suitable job was found, wait for the next queue update
		select {
		case <-c:
		case <-ctx.Done():
			return uuid.Nil, uuid.Nil, nil, "", nil, jobqueue.ErrDequeueTimeout
		}
	}

	conn, err := q.pool.Acquire(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, "", nil, fmt.Errorf("error connecting to database: %v", err)
	}
	defer conn.Release()

	// insert heartbeat
	_, err = conn.Exec(ctx, sqlInsertHeartbeat, token, id)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, "", nil, fmt.Errorf("error inserting the job's heartbeat: %v", err)
	}

	dependencies, err := q.jobDependencies(ctx, conn, id)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, "", nil, fmt.Errorf("error querying the job's dependencies: %v", err)
	}

	q.logger.Info("Dequeued job", "job_type", jobType, "job_id", id.String(), "job_dependencies", fmt.Sprintf("%+v", dependencies))

	return id, token, dependencies, jobType, args, nil
}

// dequeueMaybe is just a smaller helper for acquiring a connection and
// running the sqlDequeue query
func (q *DBJobQueue) dequeueMaybe(ctx context.Context, token uuid.UUID, jobTypes []string, channels []string) (id uuid.UUID, jobType string, args json.RawMessage, err error) {
	var conn *pgxpool.Conn
	conn, err = q.pool.Acquire(ctx)
	if err != nil {
		return
	}
	defer conn.Release()

	err = conn.QueryRow(ctx, sqlDequeue, token, jobTypes, channels).Scan(&id, &jobType, &args)
	return
}

func (q *DBJobQueue) DequeueByID(ctx context.Context, id uuid.UUID) (uuid.UUID, []uuid.UUID, string, json.RawMessage, error) {
	// Return early if the context is already canceled.
	if err := ctx.Err(); err != nil {
		return uuid.Nil, nil, "", nil, jobqueue.ErrDequeueTimeout
	}

	conn, err := q.pool.Acquire(ctx)
	if err != nil {
		return uuid.Nil, nil, "", nil, fmt.Errorf("error connecting to database: %v", err)
	}
	defer conn.Release()

	var jobType string
	var args json.RawMessage
	var started, queued *time.Time
	token := uuid.New()

	err = conn.QueryRow(ctx, sqlDequeueByID, token, id).Scan(&token, &jobType, &args, &queued, &started)
	if err == pgx.ErrNoRows {
		return uuid.Nil, nil, "", nil, jobqueue.ErrNotPending
	} else if err != nil {
		return uuid.Nil, nil, "", nil, fmt.Errorf("error dequeuing job: %v", err)
	}

	// insert heartbeat
	_, err = conn.Exec(ctx, sqlInsertHeartbeat, token, id)
	if err != nil {
		return uuid.Nil, nil, "", nil, fmt.Errorf("error inserting the job's heartbeat: %v", err)
	}

	dependencies, err := q.jobDependencies(ctx, conn, id)
	if err != nil {
		return uuid.Nil, nil, "", nil, fmt.Errorf("error querying the job's dependencies: %v", err)
	}

	q.logger.Info("Dequeued job", "job_type", jobType, "job_id", id.String(), "job_dependencies", fmt.Sprintf("%+v", dependencies))

	return token, dependencies, jobType, args, nil
}

func (q *DBJobQueue) FinishJob(id uuid.UUID, result interface{}) error {
	conn, err := q.pool.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("error connecting to database: %v", err)
	}
	defer conn.Release()

	tx, err := conn.Begin(context.Background())
	if err != nil {
		return fmt.Errorf("error starting database transaction: %v", err)
	}
	defer func() {
		err = tx.Rollback(context.Background())
		if err != nil && !errors.Is(err, pgx.ErrTxClosed) {
			q.logger.Error(err, "Error rolling back finish job transaction", "job_id", id.String())
		}

	}()

	// Use double pointers for timestamps because they might be NULL, which would result in *time.Time == nil
	var started, finished *time.Time
	var jobType string
	canceled := false
	err = tx.QueryRow(context.Background(), sqlQueryJob, id).Scan(&jobType, nil, nil, &started, &finished, &canceled)
	if err == pgx.ErrNoRows {
		return jobqueue.ErrNotExist
	}
	if canceled {
		return jobqueue.ErrCanceled
	}
	if finished != nil {
		return jobqueue.ErrNotRunning
	}

	// Remove from heartbeats
	tag, err := tx.Exec(context.Background(), sqlDeleteHeartbeat, id)
	if err != nil {
		return fmt.Errorf("error finishing job %s: %v", id, err)
	}

	if tag.RowsAffected() != 1 {
		return jobqueue.ErrNotExist
	}

	err = tx.QueryRow(context.Background(), sqlFinishJob, result, id).Scan(&finished)

	if err == pgx.ErrNoRows {
		return jobqueue.ErrNotExist
	}
	if err != nil {
		return fmt.Errorf("error finishing job %s: %v", id, err)
	}

	_, err = conn.Exec(context.Background(), sqlNotify)
	if err != nil {
		return fmt.Errorf("error notifying jobs channel: %v", err)
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return fmt.Errorf("unable to commit database transaction: %v", err)
	}

	q.logger.Info("Finished job", "job_type", jobType, "job_id", id.String())

	return nil
}

func (q *DBJobQueue) CancelJob(id uuid.UUID) error {
	conn, err := q.pool.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("error connecting to database: %v", err)
	}
	defer conn.Release()

	var started *time.Time
	var jobType string
	err = conn.QueryRow(context.Background(), sqlCancelJob, id).Scan(&jobType, &started)
	if err == pgx.ErrNoRows {
		return jobqueue.ErrNotRunning
	}
	if err != nil {
		return fmt.Errorf("error canceling job %s: %v", id, err)
	}

	q.logger.Info("Cancelled job", "job_type", jobType, "job_id", id.String())

	return nil
}

func (q *DBJobQueue) JobStatus(id uuid.UUID) (jobType string, channel string, result json.RawMessage, queued, started, finished time.Time, canceled bool, deps []uuid.UUID, dependents []uuid.UUID, err error) {
	conn, err := q.pool.Acquire(context.Background())
	if err != nil {
		return
	}
	defer conn.Release()

	// Use double pointers for timestamps because they might be NULL, which would result in *time.Time == nil
	var sp, fp *time.Time
	var rp pgtype.JSON
	err = conn.QueryRow(context.Background(), sqlQueryJobStatus, id).Scan(&jobType, &channel, &rp, &queued, &sp, &fp, &canceled)
	if err != nil {
		return
	}
	if sp != nil {
		started = *sp
	}
	if fp != nil {
		finished = *fp
	}
	if rp.Status != pgtype.Null {
		result = rp.Bytes
	}

	deps, err = q.jobDependencies(context.Background(), conn, id)
	if err != nil {
		return
	}

	dependents, err = q.jobDependents(context.Background(), conn, id)
	if err != nil {
		return
	}
	return
}

// Job returns all the parameters that define a job (everything provided during Enqueue).
func (q *DBJobQueue) Job(id uuid.UUID) (jobType string, args json.RawMessage, dependencies []uuid.UUID, channel string, err error) {
	conn, err := q.pool.Acquire(context.Background())
	if err != nil {
		return
	}
	defer conn.Release()

	err = conn.QueryRow(context.Background(), sqlQueryJob, id).Scan(&jobType, &args, &channel, nil, nil, nil)
	if err == pgx.ErrNoRows {
		err = jobqueue.ErrNotExist
		return
	} else if err != nil {
		return
	}

	dependencies, err = q.jobDependencies(context.Background(), conn, id)
	return
}

// Find job by token, this will return an error if the job hasn't been dequeued
func (q *DBJobQueue) IdFromToken(token uuid.UUID) (id uuid.UUID, err error) {
	conn, err := q.pool.Acquire(context.Background())
	if err != nil {
		return uuid.Nil, fmt.Errorf("error establishing connection: %v", err)
	}
	defer conn.Release()

	err = conn.QueryRow(context.Background(), sqlQueryRunningId, token).Scan(&id)
	if err == pgx.ErrNoRows {
		return uuid.Nil, jobqueue.ErrNotExist
	} else if err != nil {
		return uuid.Nil, fmt.Errorf("Error retrieving id: %v", err)
	}

	return
}

// Get a list of tokens which haven't been updated in the specified time frame
func (q *DBJobQueue) Heartbeats(olderThan time.Duration) (tokens []uuid.UUID) {
	conn, err := q.pool.Acquire(context.Background())
	if err != nil {
		return
	}
	defer conn.Release()

	rows, err := conn.Query(context.Background(), sqlQueryHeartbeats, olderThan.String())
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var t uuid.UUID
		err = rows.Scan(&t)
		if err != nil {
			// Log the error and try to continue with the next row
			q.logger.Error(err, "Unable to read token from heartbeats")
			continue
		}
		tokens = append(tokens, t)
	}
	if rows.Err() != nil {
		q.logger.Error(rows.Err(), "Error reading tokens from heartbeats")
	}

	return
}

// Reset the last heartbeat time to time.Now()
func (q *DBJobQueue) RefreshHeartbeat(token uuid.UUID) {
	conn, err := q.pool.Acquire(context.Background())
	if err != nil {
		return
	}
	defer conn.Release()

	tag, err := conn.Exec(context.Background(), sqlRefreshHeartbeat, token)
	if err != nil {
		q.logger.Error(err, "Error refreshing heartbeat")
	}
	if tag.RowsAffected() != 1 {
		q.logger.Error(nil, "No rows affected when refreshing heartbeat", "job_token", token.String())
	}
}

func (q *DBJobQueue) jobDependencies(ctx context.Context, conn *pgxpool.Conn, id uuid.UUID) ([]uuid.UUID, error) {
	rows, err := conn.Query(ctx, sqlQueryDependencies, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dependencies := []uuid.UUID{}
	for rows.Next() {
		var d uuid.UUID
		err = rows.Scan(&d)
		if err != nil {
			return nil, err
		}

		dependencies = append(dependencies, d)
	}
	if rows.Err() != nil {
		return nil, err
	}

	return dependencies, nil
}

func (q *DBJobQueue) jobDependents(ctx context.Context, conn *pgxpool.Conn, id uuid.UUID) ([]uuid.UUID, error) {
	rows, err := conn.Query(ctx, sqlQueryDependents, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	dependents := []uuid.UUID{}
	for rows.Next() {
		var d uuid.UUID
		err = rows.Scan(&d)
		if err != nil {
			return nil, err
		}

		dependents = append(dependents, d)
	}
	if rows.Err() != nil {
		return nil, err
	}

	return dependents, nil
}
