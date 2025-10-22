package osbuildexecutor

import (
	"io"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/osbuild"
)

type Executor interface {
	RunOSBuild(manifest []byte, exports, checkpoints []string, errorWriter io.Writer, logger *logrus.Entry, opts *osbuild.OSBuildOptions) (*osbuild.Result, error)
}
