package weldr

import (
	"sort"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/compose"
)

type ComposeEntry struct {
	ID          uuid.UUID              `json:"id"`
	Blueprint   string                 `json:"blueprint"`
	Version     string                 `json:"version"`
	ComposeType common.ImageType       `json:"compose_type"`
	ImageSize   uint64                 `json:"image_size"` // This is user-provided image size, not actual file size
	QueueStatus common.ImageBuildState `json:"queue_status"`
	JobCreated  float64                `json:"job_created"`
	JobStarted  float64                `json:"job_started,omitempty"`
	JobFinished float64                `json:"job_finished,omitempty"`
	Uploads     []uploadResponse       `json:"uploads,omitempty"`
}

func composeToComposeEntry(id uuid.UUID, compose compose.Compose, includeUploads bool) *ComposeEntry {
	var composeEntry ComposeEntry

	composeEntry.ID = id
	composeEntry.Blueprint = compose.Blueprint.Name
	composeEntry.Version = compose.Blueprint.Version
	composeEntry.ComposeType = compose.ImageBuilds[0].ImageType
	composeEntry.QueueStatus = compose.ImageBuilds[0].QueueStatus

	if includeUploads {
		composeEntry.Uploads = targetsToUploadResponses(compose.ImageBuilds[0].Targets)
	}

	switch compose.ImageBuilds[0].QueueStatus {
	case common.IBWaiting:
		composeEntry.JobCreated = float64(compose.ImageBuilds[0].JobCreated.UnixNano()) / 1000000000

	case common.IBRunning:
		composeEntry.JobCreated = float64(compose.ImageBuilds[0].JobCreated.UnixNano()) / 1000000000
		composeEntry.JobStarted = float64(compose.ImageBuilds[0].JobStarted.UnixNano()) / 1000000000

	case common.IBFinished:
		composeEntry.ImageSize = compose.ImageBuilds[0].Size
		composeEntry.JobCreated = float64(compose.ImageBuilds[0].JobCreated.UnixNano()) / 1000000000
		composeEntry.JobStarted = float64(compose.ImageBuilds[0].JobStarted.UnixNano()) / 1000000000
		composeEntry.JobFinished = float64(compose.ImageBuilds[0].JobFinished.UnixNano()) / 1000000000

	case common.IBFailed:
		composeEntry.JobCreated = float64(compose.ImageBuilds[0].JobCreated.UnixNano()) / 1000000000
		composeEntry.JobStarted = float64(compose.ImageBuilds[0].JobStarted.UnixNano()) / 1000000000
		composeEntry.JobFinished = float64(compose.ImageBuilds[0].JobFinished.UnixNano()) / 1000000000
	default:
		panic("invalid compose state")
	}

	return &composeEntry
}

func composesToComposeEntries(composes map[uuid.UUID]compose.Compose, uuids []uuid.UUID, includeUploads bool) []*ComposeEntry {
	var composeEntries []*ComposeEntry
	if uuids == nil {
		composeEntries = make([]*ComposeEntry, 0, len(composes))
		for id, compose := range composes {
			composeEntries = append(composeEntries, composeToComposeEntry(id, compose, includeUploads))
		}
	} else {
		composeEntries = make([]*ComposeEntry, 0, len(uuids))
		for _, id := range uuids {
			if compose, exists := composes[id]; exists {
				composeEntries = append(composeEntries, composeToComposeEntry(id, compose, includeUploads))
			}
		}
	}

	// make this function output more predictable
	sort.Slice(composeEntries, func(i, j int) bool {
		return composeEntries[i].ID.String() < composeEntries[j].ID.String()
	})

	return composeEntries
}
