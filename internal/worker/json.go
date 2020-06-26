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

// must be serializable!
type TargetError struct {
	Message string `json:"err"`
}

func (e *TargetError) Error() string {
	return e.Message
}

type TargetResult struct {
	Target target.Target `json:"target"`
	Error  *TargetError  `json:"error,omitempty"`
}

type OSBuildJobResult struct {
	OSBuildOutput *common.ComposeResult `json:"osbuild_output,omitempty"`
	Targets       []TargetResult        `json:"targets,omitempty"`
	GenericError  string                `json:"generic_error,omitempty"`
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
	Result *OSBuildJobResult `json:"result"`
}

type updateJobResponse struct {
}
