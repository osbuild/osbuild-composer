package job

import (
	"osbuild-composer/internal/pipeline"
	"osbuild-composer/internal/target"

	"github.com/google/uuid"
)

type Job struct {
	ComposeID uuid.UUID
	Pipeline  pipeline.Pipeline
	Targets   []target.Target
}
