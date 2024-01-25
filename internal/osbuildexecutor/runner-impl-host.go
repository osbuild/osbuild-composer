package osbuildexecutor

import (
	"io"

	"github.com/osbuild/images/pkg/osbuild"
)

type hostExecutor struct{}

func (he *hostExecutor) RunOSBuild(manifest []byte, store, outputDirectory string, exports, checkpoints,
	extraEnv []string, result bool, errorWriter io.Writer) (*osbuild.Result, error) {
	return osbuild.RunOSBuild(manifest, store, outputDirectory, exports, checkpoints, extraEnv, result, errorWriter)
}

func NewHostExecutor() Executor {
	return &hostExecutor{}
}
