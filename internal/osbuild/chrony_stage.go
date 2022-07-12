package osbuild

import (
	"encoding/json"
	"fmt"
)

// Exactly one of 'Timeservers' or 'Servers' must be specified
type ChronyStageOptions struct {
	Timeservers []string             `json:"timeservers,omitempty"`
	Servers     []ChronyConfigServer `json:"servers,omitempty"`
	LeapsecTz   *string              `json:"leapsectz,omitempty"`
}

func (ChronyStageOptions) isStageOptions() {}

// Use '*ToPtr()' functions from the internal/common package to set the pointer values in literals
type ChronyConfigServer struct {
	Hostname string `json:"hostname"`
	Minpoll  *int   `json:"minpoll,omitempty"`
	Maxpoll  *int   `json:"maxpoll,omitempty"`
	Iburst   *bool  `json:"iburst,omitempty"`
	Prefer   *bool  `json:"prefer,omitempty"`
}

// Unexported alias for use in ChronyStageOptions's MarshalJSON() to prevent recursion
type chronyStageOptions ChronyStageOptions

func (o ChronyStageOptions) MarshalJSON() ([]byte, error) {
	if (len(o.Timeservers) != 0 && len(o.Servers) != 0) || (len(o.Timeservers) == 0 && len(o.Servers) == 0) {
		return nil, fmt.Errorf("exactly one of 'Timeservers' or 'Servers' must be specified")
	}
	stageOptions := chronyStageOptions(o)
	return json.Marshal(stageOptions)
}

func NewChronyStage(options *ChronyStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.chrony",
		Options: options,
	}
}
