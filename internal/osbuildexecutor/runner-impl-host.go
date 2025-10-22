package osbuildexecutor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type hostExecutor struct{}

func (he *hostExecutor) RunOSBuild(manifest []byte, logger logrus.FieldLogger, job worker.Job, opts *osbuild.OSBuildOptions) (*osbuild.Result, error) {
	// MonitorFile needs an *os.File
	rPipe, wPipe, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("cannot create pipe for monitor file: %w", err)
	}
	defer rPipe.Close()
	defer wPipe.Close()
	opts.Monitor = osbuild.MonitorJSONSeq
	opts.MonitorFile = wPipe

	// Capture stdout to generate osbuild result
	var stdoutBuffer bytes.Buffer
	opts.Stdout = &stdoutBuffer

	cmd := osbuild.NewOSBuildCmd(manifest, opts)
	osbuildStatus := osbuild.NewStatusScanner(rPipe)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting osbuild: %v", err)
	}
	wPipe.Close()

	if err := handleProgress(osbuildStatus, logger, job); err != nil {
		return nil, fmt.Errorf("unable to construct osbuild result: %w", err)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("error running osbuild: %w", err)
	}

	// try to decode the output even though the job could have failed
	if stdoutBuffer.Len() == 0 {
		return nil, fmt.Errorf("osbuild did not return any output")
	}
	var result osbuild.Result
	err = json.Unmarshal(stdoutBuffer.Bytes(), &result)
	if err != nil {
		return nil, fmt.Errorf("error decoding osbuild output: %w\nraw output:\n%s", err, stdoutBuffer.String())
	}

	return &result, nil
}

func NewHostExecutor() Executor {
	return &hostExecutor{}
}
