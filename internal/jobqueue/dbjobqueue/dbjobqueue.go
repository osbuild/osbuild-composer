// Package dbjobqueue implements the interfaces in package jobqueue backed by a
// PostreSQL database.
//
// Data is stored non-reduntantly. Any data structure necessary for efficient
// access (e.g., dependants) are kept in memory.
package dbjobqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"

	logrus "github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/jobqueue"
)

const (
	sqlNotify   = `NOTIFY jobs`
	sqlListen   = `LISTEN jobs`
	sqlUnlisten = `UNLISTEN jobs`

	sqlEnqueue = `INSERT INTO jobs(id, type, args, queued_at) VALUES ($1, $2, $3, NOW())`
	sqlDequeue = `
		UPDATE jobs
		SET token = $1, started_at = now()
		WHERE id = (
		  SELECT id
		  FROM ready_jobs
			  -- use ANY here, because "type in ()" doesn't work with bound parameters
			  -- literal syntax for this is '{"a", "b"}': https://www.postgresql.org/docs/13/arrays.html
		  WHERE type = ANY($2)
		  LIMIT 1
		  FOR UPDATE SKIP LOCKED
		)
		RETURNING id, token, type, args`

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
		RETURNING token, type, args`

	sqlInsertDependency  = `INSERT INTO job_dependencies VALUES ($1, $2)`
	sqlQueryDependencies = `
		SELECT dependency_id
		FROM job_dependencies
		WHERE job_id = $1`

	sqlQueryJob = `
		SELECT type, args, finished_at, canceled
		FROM jobs
		WHERE id = $1`
	sqlQueryJobStatus = `
		SELECT result, queued_at, started_at, finished_at, canceled
		FROM jobs
		WHERE id = $1`
	sqlQueryRunningId = `
                SELECT id
                FROM jobs
                WHERE token = $1 AND finished_at IS NULL AND canceled = FALSE`
	sqlFinishJob = `
		UPDATE jobs
		SET finished_at = now(), result = $1
		WHERE id = $2 AND finished_at IS NULL`
	sqlCancelJob = `
		UPDATE jobs
		SET canceled = TRUE
		WHERE id = $1 AND finished_at IS NULL`

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

type dbJobQueue struct {
	pool *pgxpool.Pool
}

// Create a new dbJobQueue object for `url`.
func New(url string) (*dbJobQueue, error) {
	pool, err := pgxpool.Connect(context.Background(), url)
	if err != nil {
		return nil, fmt.Errorf("error establishing connection: %v", err)
	}

	return &dbJobQueue{pool}, nil
}

func (q *dbJobQueue) Close() {
	q.pool.Close()
}

func (q *dbJobQueue) Enqueue(jobType string, args interface{}, dependencies []uuid.UUID) (uuid.UUID, error) {
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
		if err != nil && !errors.As(err, &pgx.ErrTxClosed) {
			logrus.Error("error rolling back enqueue transaction: ", err)
		}
	}()

	id := uuid.New()
	_, err = conn.Exec(context.Background(), sqlEnqueue, id, jobType, args)
	if err != nil {
		return uuid.Nil, fmt.Errorf("error enqueuing job: %v", err)
	}

	for _, d := range dependencies {
		_, err = conn.Exec(context.Background(), sqlInsertDependency, id, d)
		if err != nil {
			return uuid.Nil, fmt.Errorf("error inserting dependency: %v", err)
		}
	}

	_, err = conn.Exec(context.Background(), sqlNotify)
	if err != nil {
		return uuid.Nil, fmt.Errorf("error notifying jobs channel: %v", err)
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return uuid.Nil, fmt.Errorf("unable to commit database transaction: %v", err)
	}

	logrus.Infof("Enqueued job of type %s with ID %s(dependencies %v)", jobType, id, dependencies)

	return id, nil
}

func (q *dbJobQueue) Dequeue(ctx context.Context, jobTypes []string) (uuid.UUID, uuid.UUID, []uuid.UUID, string, json.RawMessage, error) {
	// Return early if the context is already canceled.
	if err := ctx.Err(); err != nil {
		return uuid.Nil, uuid.Nil, nil, "", nil, jobqueue.ErrDequeueTimeout
	}

	conn, err := q.pool.Acquire(ctx)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, "", nil, fmt.Errorf("error connecting to database: %v", err)
	}
	defer func() {
		_, err := conn.Exec(ctx, sqlUnlisten)
		if err != nil && !errors.Is(err, context.DeadlineExceeded) {
			logrus.Error("Error unlistening for jobs in dequeue: ", err)
		}
		conn.Release()
	}()

	_, err = conn.Exec(ctx, sqlListen)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, "", nil, fmt.Errorf("error listening on jobs channel: %v", err)
	}

	var id uuid.UUID
	var jobType string
	var args json.RawMessage
	token := uuid.New()
	for {
		err = conn.QueryRow(ctx, sqlDequeue, token, jobTypes).Scan(&id, &token, &jobType, &args)
		if err == nil {
			break
		}
		if err != nil && !errors.As(err, &pgx.ErrNoRows) {
			return uuid.Nil, uuid.Nil, nil, "", nil, fmt.Errorf("error dequeuing job: %v", err)
		}
		_, err = conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if pgconn.Timeout(err) {
				return uuid.Nil, uuid.Nil, nil, "", nil, jobqueue.ErrDequeueTimeout
			}
			return uuid.Nil, uuid.Nil, nil, "", nil, fmt.Errorf("error waiting for notification on jobs channel: %v", err)
		}
	}

	// insert heartbeat
	_, err = conn.Exec(ctx, sqlInsertHeartbeat, token, id)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, "", nil, fmt.Errorf("error inserting the job's heartbeat: %v", err)
	}

	dependencies, err := q.jobDependencies(ctx, conn, id)
	if err != nil {
		return uuid.Nil, uuid.Nil, nil, "", nil, fmt.Errorf("error querying the job's dependencies: %v", err)
	}

	logrus.Infof("Dequeued job of type %v with ID %s", jobType, id)

	return id, token, dependencies, jobType, args, nil
}
func (q *dbJobQueue) DequeueByID(ctx context.Context, id uuid.UUID) (uuid.UUID, []uuid.UUID, string, json.RawMessage, error) {
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
	token := uuid.New()

	err = conn.QueryRow(ctx, sqlDequeueByID, token, id).Scan(&token, &jobType, &args)
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

	logrus.Infof("Dequeued job of type %v with ID %s", jobType, id)

	return token, dependencies, jobType, args, nil
}

func (q *dbJobQueue) FinishJob(id uuid.UUID, result interface{}) error {
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
		if err != nil && !errors.As(err, &pgx.ErrTxClosed) {
			logrus.Errorf("error rolling back finish job transaction for job %s: %v", id, err)
		}

	}()

	// Use double pointers for timestamps because they might be NULL, which would result in *time.Time == nil
	var finished *time.Time
	canceled := false
	err = conn.QueryRow(context.Background(), sqlQueryJob, id).Scan(nil, nil, nil, &finished, &canceled)
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
	tag, err := conn.Exec(context.Background(), sqlDeleteHeartbeat, id)
	if err != nil {
		return fmt.Errorf("error finishing job %s: %v", id, err)
	}

	if tag.RowsAffected() != 1 {
		return jobqueue.ErrNotExist
	}

	tag, err = conn.Exec(context.Background(), sqlFinishJob, result, id)
	if err != nil {
		return fmt.Errorf("error finishing job %s: %v", id, err)
	}

	if tag.RowsAffected() != 1 {
		return jobqueue.ErrNotExist
	}

	_, err = conn.Exec(context.Background(), sqlNotify)
	if err != nil {
		return fmt.Errorf("error notifying jobs channel: %v", err)
	}

	err = tx.Commit(context.Background())
	if err != nil {
		return fmt.Errorf("unable to commit database transaction: %v", err)
	}

	logrus.Infof("Finished job with ID %s", id)

	return nil
}

func (q *dbJobQueue) CancelJob(id uuid.UUID) error {
	conn, err := q.pool.Acquire(context.Background())
	if err != nil {
		return fmt.Errorf("error connecting to database: %v", err)
	}
	defer conn.Release()

	tag, err := conn.Exec(context.Background(), sqlCancelJob, id)
	if err != nil {
		return fmt.Errorf("error canceling job %s: %v", id, err)
	}

	if tag.RowsAffected() != 1 {
		return jobqueue.ErrNotRunning
	}

	logrus.Infof("Cancelled job with ID %s", id)

	return nil
}

func (q *dbJobQueue) JobStatus(id uuid.UUID) (result json.RawMessage, queued, started, finished time.Time, canceled bool, deps []uuid.UUID, err error) {
	conn, err := q.pool.Acquire(context.Background())
	if err != nil {
		return
	}
	defer conn.Release()

	// Use double pointers for timestamps because they might be NULL, which would result in *time.Time == nil
	var sp, fp *time.Time
	var rp pgtype.JSON
	err = conn.QueryRow(context.Background(), sqlQueryJobStatus, id).Scan(&rp, &queued, &sp, &fp, &canceled)
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
	return
}

// Job returns all the parameters that define a job (everything provided during Enqueue).
func (q *dbJobQueue) Job(id uuid.UUID) (jobType string, args json.RawMessage, dependencies []uuid.UUID, err error) {
	conn, err := q.pool.Acquire(context.Background())
	if err != nil {
		return
	}
	defer conn.Release()

	err = conn.QueryRow(context.Background(), sqlQueryJob, id).Scan(&jobType, &args, nil, nil)
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
func (q *dbJobQueue) IdFromToken(token uuid.UUID) (id uuid.UUID, err error) {
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
func (q *dbJobQueue) Heartbeats(olderThan time.Duration) (tokens []uuid.UUID) {
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
			logrus.Error("Unable to read token from heartbeats: ", err)
			continue
		}
		tokens = append(tokens, t)
	}
	if rows.Err() != nil {
		logrus.Error("Error reading tokens from heartbeats: ", rows.Err())
	}

	return
}

// Reset the last heartbeat time to time.Now()
func (q *dbJobQueue) RefreshHeartbeat(token uuid.UUID) {
	conn, err := q.pool.Acquire(context.Background())
	if err != nil {
		return
	}
	defer conn.Release()

	tag, err := conn.Exec(context.Background(), sqlRefreshHeartbeat, token)
	if err != nil {
		logrus.Error("Error refreshing heartbeat: ", err)
	}
	if tag.RowsAffected() != 1 {
		logrus.Error("No rows affected when refreshing heartbeat for ", token)
	}
}

func (q *dbJobQueue) jobDependencies(ctx context.Context, conn *pgxpool.Conn, id uuid.UUID) ([]uuid.UUID, error) {
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
