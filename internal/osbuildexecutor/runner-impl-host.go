package osbuildexecutor

import (
	"io"

	"github.com/osbuild/images/pkg/osbuild"
)

type hostExecutor struct{}

func (he *hostExecutor) RunOSBuild(manifest []byte, exports, checkpoints []string, errorWriter io.Writer, opts *osbuild.OSBuildOptions) (*osbuild.Result, error) {
	return osbuild.RunOSBuild(manifest, exports, checkpoints, errorWriter, opts)
}

func NewHostExecutor() Executor {
	return &hostExecutor{}
}
