package store

import (
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
)

type StateTransitionError struct {
	message string
}

func (ste *StateTransitionError) Error() string {
	return ste.message
}

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

// A Compose represent the task of building a set of images from a single blueprint.
// It contains all the information necessary to generate the inputs for the job, as
// well as the job's state.
type Compose struct {
	Blueprint  *blueprint.Blueprint
	ImageBuild ImageBuild
	Packages   []rpmmd.PackageSpec
}

// DeepCopy creates a copy of the Compose structure
func (c *Compose) DeepCopy() Compose {
	var newBpPtr *blueprint.Blueprint = nil
	if c.Blueprint != nil {
		bpCopy := *c.Blueprint
		newBpPtr = &bpCopy
	}
	pkgs := make([]rpmmd.PackageSpec, len(c.Packages))
	copy(pkgs, c.Packages)

	return Compose{
		Blueprint:  newBpPtr,
		ImageBuild: c.ImageBuild.DeepCopy(),
		Packages:   pkgs,
	}
}
