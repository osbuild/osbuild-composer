package osbuildexecutor

import (
	"github.com/osbuild/images/pkg/osbuild"
)

type hostExecutor struct{}

func (he *hostExecutor) RunOSBuild(manifest []byte, opts *osbuild.OSBuildOptions) (*osbuild.Result, error) {
	return osbuild.RunOSBuild(manifest, opts)
}

func NewHostExecutor() Executor {
	return &hostExecutor{}
}
