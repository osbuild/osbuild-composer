package target

import "github.com/google/uuid"

// Deprecated: TargetNameLocal should not be used by any new code.
const TargetNameLocal TargetName = "org.osbuild.local"

// Deprecated: LocalTargetOptions should not be used by any new code.
// The data structure is kept for backward compatibility and to ensure
// that old osbuild-composer instances which were upgraded will be able
// to read the target details from the local store.
type LocalTargetOptions struct {
	ComposeId       uuid.UUID `json:"compose_id"`
	ImageBuildId    int       `json:"image_build_id"`
	Filename        string    `json:"filename"`
	StreamOptimized bool      `json:"stream_optimized"` // return image as stream optimized
}

func (LocalTargetOptions) isTargetOptions() {}

// Deprecated: NewLocalTarget should not be used by any new code.
func NewLocalTarget(options *LocalTargetOptions) *Target {
	return newTarget(TargetNameLocal, options)
}
