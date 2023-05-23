// Package jobqueue provides a generic interface to a simple job queue.
//
// Jobs are pushed to the queue with Enqueue(). Workers call Dequeue() to
// receive a job and FinishJob() to report one as finished.
//
// Each job has a type and arguments corresponding to this type. These are
// opaque to the job queue, but it mandates that the arguments must be
// serializable to JSON. Similarly, a job's result has opaque result arguments
// that are determined by its type.
//
// A job can have dependencies. It is not run until all its dependencies have
// finished.
package jobqueue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// JobQueue is an interface to a simple job queue. It is safe for concurrent use.
type JobQueue interface {
	// Enqueues a job.
	//
	// `args` must be JSON-serializable and fit the given `jobType`, i.e., a worker
	// that is running that job must know the format of `args`.
	//
	// All dependencies must already exist, but the job isn't run until all of them
	// have finished.
	//
	// Returns the id of the new job, or an error.
	Enqueue(jobType string, args interface{}, dependencies []uuid.UUID, channel string) (uuid.UUID, error)

	// Dequeues a job, blocking until one is available.
	//
	// Waits until a job with a type of any of `jobTypes` and any of `channels`
	// is available, or `ctx` is canceled.
	//
	// Returns the job's id, token, dependencies, type, and arguments, or an error. Arguments
	// can be unmarshaled to the type given in Enqueue().
	Dequeue(ctx context.Context, jobTypes []string, channels []string) (uuid.UUID, uuid.UUID, []uuid.UUID, string, json.RawMessage, error)

	// Dequeues a pending job by its ID in a non-blocking way.
	//
	// Returns the job's token, dependencies, type, and arguments, or an error. Arguments
	// can be unmarshaled to the type given in Enqueue().
	DequeueByID(ctx context.Context, id uuid.UUID) (uuid.UUID, []uuid.UUID, string, json.RawMessage, error)

	// Tries to requeue a running job by its ID
	//
	// Returns the given job to the pending state. If the job has reached
	// the maxRetries number of retries already, finish the job instead.
	// `result` must fit the associated job type and must be serializable to JSON.
	RequeueOrFinishJob(id uuid.UUID, maxRetries uint64, result interface{}) error

	// Cancel a job. Does nothing if the job has already finished.
	CancelJob(id uuid.UUID) error

	// If the job has finished, returns the result as raw JSON.
	//
	// Returns the current status of the job, in the form of three times:
	// queued, started, and finished. `started` and `finished` might be the
	// zero time (check with t.IsZero()), when the job is not running or
	// finished, respectively.
	//
	// Lastly, the IDs of the jobs dependencies are returned.
	JobStatus(id uuid.UUID) (jobType string, channel string, result json.RawMessage, queued, started, finished time.Time, canceled bool, deps []uuid.UUID, dependents []uuid.UUID, err error)
	// Does the same as JobStatus but without fetching the result blob
	// To be used instead of JobStatus if possible
	JobStatusWoResult(id uuid.UUID) (jobType string, channel string, queued, started, finished time.Time, canceled bool, deps []uuid.UUID, dependents []uuid.UUID, err error)

	// Query multiple fields under the result column at once and stores them in to the response object.
	// paths: a list of dot separated fields composing a searchable path in the result column
	//        each names in the path must correspond to a subsequent json key.
	// response: an object that has for characteristic to contains public fields associated with json tags matching the paths
	//
	// for example:
	// type Object struct {
	//     Name string `json:"name"`
	// }
	// type ResponseStruct struct {
	//     Name  string `json:"name"`
	//     Obj1  Object `json:"obj"`
	//     Obj2 *Object `json:"obj2"`
	// }
	// var response ResponseStruct
	// err = QueryResultFields(id, []string{"name", "obj.name", "obj2.name"), &response)
	//
	// After calling the QueryResultFields, response will contain its Name , Obj1 and Obj2 fields field up.
	// Obj2 will be instantiated on the fly.
	//
	// Pointer instantiation only works for one level of indirection, for pointers on pointers the user needs to provide
	// a valid chain to dereference.
	//
	// For instance:
	// type Object struct {
	//     Name string `json:"name"`
	// }
	// type ResponseStruct struct {
	//     Obj **Object `json:"obj"`
	// }
	// obj := &Object{}
	// var response ResponseStruct
	// response.Obj = &obj
	// err = QueryResultFields(id, []string{"name", "obj.name", "obj2.name"), &response)
	QueryResultFields(id uuid.UUID, paths []string, response any) error

	// Check for presence of a specific path and is content in a JSON field.
	// path -> a dot separated path within the result, for example osbuild_output.error
	// Returns a boolean set to True if the requested path contains data
	TestResultFieldExists(id uuid.UUID, path string) (bool, error)

	// Job returns all the parameters that define a job (everything provided during Enqueue).
	Job(id uuid.UUID) (jobType string, args json.RawMessage, dependencies []uuid.UUID, channel string, err error)

	// Find job by token, this will return an error if the job hasn't been dequeued
	IdFromToken(token uuid.UUID) (id uuid.UUID, err error)

	// Get a list of tokens which haven't been updated in the specified time frame
	Heartbeats(olderThan time.Duration) (tokens []uuid.UUID)

	// Reset the last heartbeat time to time.Now()
	RefreshHeartbeat(token uuid.UUID)
}

func getFieldByTag(tag string, rt reflect.Type, val reflect.Value) (interface{}, error) {
	// Only struct can be explored, error out if the recipient is not one.
	if rt.Kind() != reflect.Struct {
		return nil, errors.New("Recipient isn't a struct")
	}
	// Iterate through the fields to find out the one that has a json tag matching the requested name.
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		// ignore the arguments of the json tag
		v := strings.Split(f.Tag.Get("json"), ",")[0]
		if v == tag {
			// obtain the field from the recipient's instance and generate a pointer to it.
			v := val.FieldByName(f.Name)
			// In the special case where the field is a pointer, check that underlying data structure is instantiated
			// and if not create a New instance of it.
			if v.Kind() == reflect.Pointer {
				if v.IsNil() {
					v.Set(reflect.New(f.Type.Elem()))
				}
			}
			// Then return a pointer to the underlying field, or the field directly if it is impossible to have a
			// pointer to it.
			if v.CanAddr() {
				return v.Addr().Interface(), nil
			} else {
				return v.Interface(), nil
			}
		}
		// if the field is anonymous, it means embedding of another structure fields. The code needs to explore this
		// structure to find out the correct Tag. Jump into this structure, but keep the parent value as reference to
		// extract the field names.
		if f.Anonymous {
			return getFieldByTag(tag, f.Type, val)
		}
	}
	// Nothing was found, return a missing error
	return nil, errors.New("tag not found in struct")
}

// returns the struct field having the json tag within the recipient.
// recipient must be a struct, or a pointer to a struct.
//
// If the field pointed by the tag is a pointer to something that is not instantiated, the method will instantiate the
// pointed content for you before returning the pointer.
//
// With a caveat that double pointers aren't automatically supported, if you do need to use pointers on pointers, make
// sur the final data structure that'll receive information is not nil. If that's what you want.
func GetFieldByTag(tag string, recipient interface{}) (interface{}, error) {
	val := reflect.ValueOf(recipient)
	// Dereference the recipient if it's a pointer, dereferencing can happen many times in a row, for pointer on
	// pointers, or pointers on struct fields.
	for val.Kind() == reflect.Ptr {
		deref := val.Elem()
		if deref.Kind() == 0 {
			return nil, fmt.Errorf("Recipient '%s' is nil, please initialize it", tag)
		} else {
			val = deref
		}
	}
	return getFieldByTag(tag, val.Type(), val)
}

// Returns the right hand field of the path by going through each path element from the recipient.
// path is a dot separated list of fields from the root structure: for example
// path <- "a.b.c"
//
//	recipient <- {
//	  a:{
//	     b:{
//	         c:"bar"
//	     }
//	  }
//	}
//
// will return a pointer to c.
func FindRecipientByTagPath(recipient interface{}, path string) (field interface{}, err error) {
	field = recipient
	for _, tag := range strings.Split(path, ".") {
		field, err = GetFieldByTag(tag, field)
		if err != nil {
			return nil, err
		}
	}
	return field, nil
}

func FieldSanitazation(path string) error {
	// Check that path element contains only alphanumeric characters (dash and underscore are permitted too)
	rgx, err := regexp.Compile(`^[\w\d\-_]+$`)
	if err != nil {
		return err
	}
	for _, str := range strings.Split(path, ".") {
		if !rgx.MatchString(str) {
			return errors.New("path element doesn't match validation regex")
		}
	}
	return nil
}

// SimpleLogger provides a structured logging methods for the jobqueue library.
type SimpleLogger interface {
	// Info creates an info-level message and arbitrary amount of key-value string pairs which
	// can be optionally mapped to fields by underlying implementations.
	Info(msg string, args ...string)

	// Error creates an error-level message and arbitrary amount of key-value string pairs which
	// can be optionally mapped to fields by underlying implementations. The first error argument
	// can be set to nil when no context error is available.
	Error(err error, msg string, args ...string)
}

var (
	ErrNotExist       = errors.New("job does not exist")
	ErrNotPending     = errors.New("job is not pending")
	ErrNotRunning     = errors.New("job is not running")
	ErrCanceled       = errors.New("job was canceled")
	ErrDequeueTimeout = errors.New("dequeue context timed out or was canceled")
)
