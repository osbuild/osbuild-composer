package osbuildexecutor

import (
	"fmt"
	"io"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/osbuild"
)

type hostExecutor struct{}

func (he *hostExecutor) RunOSBuild(manifest []byte, exports, checkpoints []string, errorWriter io.Writer, logger *logrus.Entry, opts *osbuild.OSBuildOptions) (*osbuild.Result, error) {
	opts.Monitor = osbuild.MonitorJSONSeq

	cmd := osbuild.NewOSBuildCmd(manifest, exports, checkpoints, opts)
	stdOut, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer stdOut.Close()
	cmd.Stderr = errorWriter

	osbuildStatus := osbuild.NewStatusScanner(stdOut)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("error starting osbuild: %v", err)
	}
	for {
		st, err := osbuildStatus.Status()
		if err != nil {
			return nil, fmt.Errorf(`error parsing osbuild status, please report a bug: %w`, err)
		}
		if st == nil {
			break
		}

		if st.Progress != nil {
			// TODO
			logger.Infof("%s (step %d out of %d)", st.Message, st.Progress.Done, st.Progress.Total)
		}
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("error running osbuild: %w", err)
	}

	result, err := osbuildStatus.Result()
	if err != nil {
		return nil, fmt.Errorf("unable to construct osbuild result: %w", err)
	}
	return result, nil
}

func NewHostExecutor() Executor {
	return &hostExecutor{}
}
