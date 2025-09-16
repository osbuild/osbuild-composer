package weldr

import (
	"sort"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/weldrtypes"
)

type ComposeEntry struct {
	ID          uuid.UUID              `json:"id"`
	Blueprint   string                 `json:"blueprint"`
	Version     string                 `json:"version"`
	ComposeType string                 `json:"compose_type"`
	ImageSize   uint64                 `json:"image_size"` // This is user-provided image size, not actual file size
	QueueStatus common.ImageBuildState `json:"queue_status"`
	JobCreated  float64                `json:"job_created"`
	JobStarted  float64                `json:"job_started,omitempty"`
	JobFinished float64                `json:"job_finished,omitempty"`
	Uploads     []uploadResponse       `json:"uploads,omitempty"`
}

func composeToComposeEntry(id uuid.UUID, compose weldrtypes.Compose, status *composeStatus, includeUploads bool) *ComposeEntry {
	var composeEntry ComposeEntry

	composeEntry.ID = id
	composeEntry.Blueprint = compose.Blueprint.Name
	composeEntry.Version = compose.Blueprint.Version
	composeEntry.ComposeType = compose.ImageBuild.ImageType.Name()

	if includeUploads {
		composeEntry.Uploads = targetsToUploadResponses(compose.ImageBuild.Targets, status.State)
	}

	switch status.State {
	case ComposeWaiting:
		composeEntry.QueueStatus = common.IBWaiting
		composeEntry.JobCreated = float64(status.Queued.UnixNano()) / 1000000000

	case ComposeRunning:
		composeEntry.QueueStatus = common.IBRunning
		composeEntry.JobCreated = float64(status.Queued.UnixNano()) / 1000000000
		composeEntry.JobStarted = float64(status.Started.UnixNano()) / 1000000000

	case ComposeFinished:
		composeEntry.QueueStatus = common.IBFinished
		composeEntry.ImageSize = compose.ImageBuild.Size
		composeEntry.JobCreated = float64(status.Queued.UnixNano()) / 1000000000
		composeEntry.JobStarted = float64(status.Started.UnixNano()) / 1000000000
		composeEntry.JobFinished = float64(status.Finished.UnixNano()) / 1000000000

	case ComposeFailed:
		composeEntry.QueueStatus = common.IBFailed
		composeEntry.JobCreated = float64(status.Queued.UnixNano()) / 1000000000
		composeEntry.JobStarted = float64(status.Started.UnixNano()) / 1000000000
		composeEntry.JobFinished = float64(status.Finished.UnixNano()) / 1000000000
	default:
		panic("invalid compose state")
	}

	return &composeEntry
}

func sortComposeEntries(entries []*ComposeEntry) {
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].ID.String() < entries[j].ID.String()
	})
}
