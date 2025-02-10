package osbuild

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Status is a high level aggregation of the low-level osbuild monitor
// messages. It is more structured and meant to be used by UI frontends.
//
// this is intentionally minimal at the beginning until we figure
// out the best API, exposing the jsonseq direct feels too messy
// and based on what we learn here we may consider tweaking
// the osbuild progress
type Status struct {
	// Trace contains a single log line, usually very low-level or
	// stage output but useful for e.g. bug reporting. Should in
	// general not be displayed to the user but the concatenation
	// of all "trace" lines should give the same information as
	// running osbuild on a terminal
	Trace string

	// Message contains a high level user-visible message about
	// e.g. a startus change
	Message string

	// Progress contains the current progress.
	Progress *Progress

	// Timestamp contains the timestamp the message was recieved in
	Timestamp time.Time
}

// Progress provides progress information from an osbuild build.
// Each progress can have an arbitrary number of sub-progress information
//
// Note while those can be nested arbitrarly deep in practise
// we are at 2 levels currently:
//  1. overall pipeline progress
//  2. stages inside each pipeline
//
// we might get
//  3. stage progress (e.g. rpm install progress)
//
// in the future
type Progress struct {
	// A human readable message about what is going on
	Message string
	// The amount of work already done
	Done int
	// The total amount of work for this (sub)progress
	Total int

	SubProgress *Progress
}

// NewStatusScanner returns a StatusScanner that can parse osbuild
// jsonseq monitor status messages
func NewStatusScanner(r io.Reader) *StatusScanner {
	scanner := bufio.NewScanner(r)
	// osbuild can currently generate very long messages, the default
	// 64kb is too small for e.g. the dracut stage (see also
	// https://github.com/osbuild/osbuild/issues/1976). Increase for
	// but to unblock us.
	buf := make([]byte, 0, 512_000)
	scanner.Buffer(buf, 512_000)
	return &StatusScanner{
		scanner:         scanner,
		contextMap:      make(map[string]*contextJSON),
		stageContextMap: make(map[string]*stageContextJSON),
	}
}

// StatusScanner scan scan the osbuild jsonseq monitor output
type StatusScanner struct {
	scanner         *bufio.Scanner
	contextMap      map[string]*contextJSON
	stageContextMap map[string]*stageContextJSON
}

// Status returns a single status struct from the scanner or nil
// if the end of the status reporting is reached.
func (sr *StatusScanner) Status() (*Status, error) {
	if !sr.scanner.Scan() {
		return nil, sr.scanner.Err()
	}

	var status statusJSON
	line := sr.scanner.Bytes()
	line = bytes.Trim(line, "\x1e")
	if err := json.Unmarshal(line, &status); err != nil {
		return nil, fmt.Errorf("cannot scan line %q: %w", line, err)
	}
	// keep track of the context
	id := status.Context.ID
	context := sr.contextMap[id]
	if context == nil {
		sr.contextMap[id] = &status.Context
		context = &status.Context
	}
	ts := time.UnixMilli(int64(status.Timestamp * 1000))
	pipelineName := context.Pipeline.Name

	var trace, msg string
	// This is a convention, "osbuild.montior" sends the high level
	// status, the other messages contain low-level stdout/stderr
	// output from individual stages like "org.osbuild.rpm".
	if context.Origin == "osbuild.monitor" {
		msg = strings.TrimSpace(status.Message)
	} else {
		trace = strings.TrimSpace(status.Message)
	}

	st := &Status{
		Trace:   trace,
		Message: msg,
		Progress: &Progress{
			Done:    status.Progress.Done,
			Total:   status.Progress.Total,
			Message: fmt.Sprintf("Pipeline %s", pipelineName),
		},
		Timestamp: ts,
	}

	// add subprogress
	stageID := context.Pipeline.Stage.ID
	stageContext := sr.stageContextMap[stageID]
	if stageContext == nil {
		sr.stageContextMap[id] = &context.Pipeline.Stage
		stageContext = &context.Pipeline.Stage
	}
	stageName := fmt.Sprintf("Stage %s", stageContext.Name)
	prog := st.Progress
	for subProg := status.Progress.SubProgress; subProg != nil; subProg = subProg.SubProgress {
		prog.SubProgress = &Progress{
			Done:    subProg.Done,
			Total:   subProg.Total,
			Message: stageName,
		}
		prog = prog.SubProgress
	}

	return st, nil
}

// statusJSON is a single status entry from the osbuild monitor
type statusJSON struct {
	Context  contextJSON  `json:"context"`
	Progress progressJSON `json:"progress"`
	// Add "Result" here once
	// https://github.com/osbuild/osbuild/pull/1831 is merged

	Message   string  `json:"message"`
	Timestamp float64 `json:"timestamp"`
}

// contextJSON is the context for which a status is given. Once a context
// was sent to the user from then on it is only referenced by the ID
type contextJSON struct {
	Origin   string `json:"origin"`
	ID       string `json:"id"`
	Pipeline struct {
		ID    string           `json:"id"`
		Name  string           `json:"name"`
		Stage stageContextJSON `json:"stage"`
	} `json:"pipeline"`
}

type stageContextJSON struct {
	Name string `json:"name"`
	ID   string `json:"id"`
}

// progress is the progress information associcated with a given status.
// The details about nesting are the same as for "Progress" above.
type progressJSON struct {
	Name  string `json:"name"`
	Total int    `json:"total"`
	Done  int    `json:"done"`

	SubProgress *progressJSON `json:"progress"`
}
