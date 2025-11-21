package osbuildexecutor

import (
	"time"

	"github.com/sirupsen/logrus"

	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

const (
	MinTimeBetweenUpdates = time.Second * 30
)

type Executor interface {
	RunOSBuild(manifest []byte, logger logrus.FieldLogger, job worker.Job, opts *osbuild.OSBuildOptions) (*osbuild.Result, error)
}
