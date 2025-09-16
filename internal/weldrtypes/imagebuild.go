package weldrtypes

import (
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"
)

// ImageBuild represents a single image build inside a compose
type ImageBuild struct {
	ID          int
	ImageType   distro.ImageType
	Manifest    manifest.OSBuildManifest
	Targets     []*target.Target
	JobCreated  time.Time
	JobStarted  time.Time
	JobFinished time.Time
	Size        uint64
	JobID       uuid.UUID
	// Kept for backwards compatibility. Image builds which were done
	// before the move to the job queue use this to store whether they
	// finished successfully.
	QueueStatus common.ImageBuildState
}

// DeepCopy creates a copy of the ImageBuild structure
func (ib *ImageBuild) DeepCopy() ImageBuild {
	var newTargets []*target.Target
	for _, t := range ib.Targets {
		newTarget := *t
		newTargets = append(newTargets, &newTarget)
	}
	// Create new image build struct
	return ImageBuild{
		ID:          ib.ID,
		QueueStatus: ib.QueueStatus,
		ImageType:   ib.ImageType,
		Manifest:    ib.Manifest,
		Targets:     newTargets,
		JobCreated:  ib.JobCreated,
		JobStarted:  ib.JobStarted,
		JobFinished: ib.JobFinished,
		Size:        ib.Size,
		JobID:       ib.JobID,
	}
}
