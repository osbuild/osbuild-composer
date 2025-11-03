package osbuildexecutor

import (
	"github.com/osbuild/images/pkg/osbuild"
)

type Executor interface {
	RunOSBuild(manifest []byte, opts *osbuild.OSBuildOptions) (*osbuild.Result, error)
}
