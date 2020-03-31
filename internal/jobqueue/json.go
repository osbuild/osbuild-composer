package jobqueue

import (
	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/target"
)

type addJobResponse struct {
	ID           uuid.UUID         `json:"id"`
	ImageBuildID int               `json:"image_build_id"`
	Manifest     *osbuild.Manifest `json:"manifest"`
	Targets      []*target.Target  `json:"targets"`
}

type updateJobRequest struct {
	Status common.ImageBuildState `json:"status"`
	Result *common.ComposeResult  `json:"result"`
}
