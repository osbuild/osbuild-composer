package weldr

import (
	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/store"
	"log"
	"sort"
)

type ComposeEntry struct {
	ID          uuid.UUID        `json:"id"`
	Blueprint   string           `json:"blueprint"`
	Version     string           `json:"version"`
	ComposeType string           `json:"compose_type"`
	ImageSize   uint64           `json:"image_size"`
	QueueStatus string           `json:"queue_status"`
	JobCreated  float64          `json:"job_created"`
	JobStarted  float64          `json:"job_started,omitempty"`
	JobFinished float64          `json:"job_finished,omitempty"`
	Uploads     []UploadResponse `json:"uploads,omitempty"`
}

func composeToComposeEntry(id uuid.UUID, compose store.Compose, includeUploads bool) *ComposeEntry {
	var composeEntry ComposeEntry

	composeEntry.ID = id
	composeEntry.Blueprint = compose.Blueprint.Name
	composeEntry.Version = compose.Blueprint.Version
	composeEntry.ComposeType = compose.OutputType
	composeEntry.QueueStatus = compose.QueueStatus

	if includeUploads {
		composeEntry.Uploads = TargetsToUploadResponses(compose.Targets)
	}

	switch compose.QueueStatus {
	case "WAITING":
		composeEntry.JobCreated = float64(compose.JobCreated.UnixNano()) / 1000000000

	case "RUNNING":
		composeEntry.JobCreated = float64(compose.JobCreated.UnixNano()) / 1000000000
		composeEntry.JobStarted = float64(compose.JobStarted.UnixNano()) / 1000000000

	case "FINISHED":
		if compose.Image != nil {
			composeEntry.ImageSize = compose.Size
		} else {
			log.Printf("finished compose with id %s has nil image\n", id.String())
			composeEntry.ImageSize = 0
		}

		composeEntry.JobCreated = float64(compose.JobCreated.UnixNano()) / 1000000000
		composeEntry.JobStarted = float64(compose.JobStarted.UnixNano()) / 1000000000
		composeEntry.JobFinished = float64(compose.JobFinished.UnixNano()) / 1000000000

	case "FAILED":
		composeEntry.JobCreated = float64(compose.JobCreated.UnixNano()) / 1000000000
		composeEntry.JobStarted = float64(compose.JobStarted.UnixNano()) / 1000000000
		composeEntry.JobFinished = float64(compose.JobFinished.UnixNano()) / 1000000000
	default:
		panic("invalid compose state")
	}

	return &composeEntry
}

func composesToComposeEntries(composes map[uuid.UUID]store.Compose, uuids []uuid.UUID, includeUploads bool) []*ComposeEntry {
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
