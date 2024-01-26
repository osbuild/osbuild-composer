package osbuildexecutor

import (
	"io"

	"github.com/osbuild/images/pkg/osbuild"
)

type Executor interface {
	RunOSBuild(manifest []byte, store, outputDirectory string, exports, exportPaths, checkpoints, extraEnv []string, result bool, errorWriter io.Writer) (*osbuild.Result, error)
}
