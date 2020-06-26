package worker

import (
	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/target"
)

//
// JSON-serializable types for the jobqueue
//

type OSBuildJob struct {
	Manifest distro.Manifest  `json:"manifest"`
	Targets  []*target.Target `json:"targets,omitempty"`
}

type OSBuildJobResult struct {
	OSBuildOutput *common.ComposeResult `json:"osbuild_output,omitempty"`
}

//
// JSON-serializable types for the HTTP API
//

type statusResponse struct {
	Status string `json:"status"`
}

type errorResponse struct {
	Message string `json:"message"`
}

type addJobRequest struct {
}

type addJobResponse struct {
	Id       uuid.UUID        `json:"id"`
	Manifest distro.Manifest  `json:"manifest"`
	Targets  []*target.Target `json:"targets,omitempty"`
}

type jobResponse struct {
	Id       uuid.UUID `json:"id"`
	Canceled bool      `json:"canceled"`
}

type updateJobRequest struct {
	Status common.ImageBuildState `json:"status"`
	Result *OSBuildJobResult      `json:"result"`
}

type updateJobResponse struct {
}
