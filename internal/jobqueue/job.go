package jobqueue

import (
	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/target"
)

type Job struct {
	ComposeID    uuid.UUID
	ImageBuildID int
	Manifest     *osbuild.Manifest
	Targets      []*target.Target
}

func NewJob(id uuid.UUID, imageBuildID int, manifest *osbuild.Manifest, targets []*target.Target) *Job {
	return &Job{id, imageBuildID, manifest, targets}
}
