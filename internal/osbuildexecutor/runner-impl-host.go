package osbuildexecutor

import (
	"io"

	"github.com/osbuild/images/pkg/osbuild"
)

type hostExecutor struct{}

func (he *hostExecutor) RunOSBuild(manifest []byte, opts *OsbuildOpts, errorWriter io.Writer) (*osbuild.Result, error) {
	if opts == nil {
		opts = &OsbuildOpts{}
	}

	return osbuild.RunOSBuild(manifest, opts.StoreDir, opts.OutputDir, opts.Exports, opts.Checkpoints, opts.ExtraEnv, opts.Result, errorWriter)
}

func NewHostExecutor() Executor {
	return &hostExecutor{}
}
