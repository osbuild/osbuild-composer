package osbuildexecutor

import (
	"io"

	"github.com/osbuild/images/pkg/osbuild"
)

type OsbuildOpts struct {
	StoreDir    string
	OutputDir   string
	Exports     []string
	ExportPaths []string
	Checkpoints []string
	ExtraEnv    []string
	Result      bool
}

type Executor interface {
	RunOSBuild(manifest []byte, opts *OsbuildOpts, errorWriter io.Writer) (*osbuild.Result, error)
}
