package osbuildexecutor

import (
	"io"

	"github.com/osbuild/images/pkg/osbuild"
)

type Executor interface {
	RunOSBuild(manifest []byte, exports, checkpoints []string, errorWriter io.Writer, opts *osbuild.OSBuildOptions) (*osbuild.Result, error)
}
