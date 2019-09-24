package job

import (
	"osbuild-composer/internal/pipeline"
	"osbuild-composer/internal/target"
)

type Job struct {
	ComposeID string
	Pipeline  pipeline.Pipeline
	Target    target.Target
}
