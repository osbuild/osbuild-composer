package osbuild2

import (
	"encoding/json"
	"fmt"
)

type CloudInitStageOptions struct {
	ConfigFiles map[string]CloudInitConfigFile `json:"configuration_files,omitempty"`
}

func (CloudInitStageOptions) isStageOptions() {}

func NewCloudInitStage(options *CloudInitStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.cloud-init",
		Options: options,
	}
}

// Represents a cloud-init configuration file
type CloudInitConfigFile struct {
	SystemInfo *CloudInitConfigSystemInfo `json:"system_info,omitempty"`
}

// Unexported alias for use in CloudInitConfigFile's MarshalJSON() to prevent recursion
type cloudInitConfigFile CloudInitConfigFile

func (c CloudInitConfigFile) MarshalJSON() ([]byte, error) {
	if c.SystemInfo == nil {
		return nil, fmt.Errorf("at least one cloud-init configuration option must be specified")
	}
	configFile := cloudInitConfigFile(c)
	return json.Marshal(configFile)
}

// Represents the 'system_info' configuration section
type CloudInitConfigSystemInfo struct {
	DefaultUser *CloudInitConfigDefaultUser `json:"default_user,omitempty"`
}

// Unexported alias for use in CloudInitConfigSystemInfo's MarshalJSON() to prevent recursion
type cloudInitConfigSystemInfo CloudInitConfigSystemInfo

func (si CloudInitConfigSystemInfo) MarshalJSON() ([]byte, error) {
	if si.DefaultUser == nil {
		return nil, fmt.Errorf("at least one configuration option must be specified for 'system_info' section")
	}
	systemInfo := cloudInitConfigSystemInfo(si)
	return json.Marshal(systemInfo)
}

// Configuration of the 'default' user created by cloud-init.
type CloudInitConfigDefaultUser struct {
	Name string `json:"name,omitempty"`
}

// Unexported alias for use in CloudInitConfigDefaultUser's MarshalJSON() to prevent recursion
type cloudInitConfigDefaultUser CloudInitConfigDefaultUser

func (du CloudInitConfigDefaultUser) MarshalJSON() ([]byte, error) {
	if du.Name == "" {
		return nil, fmt.Errorf("at least one configuration option must be specified for 'default_user' section")
	}
	defaultUser := cloudInitConfigDefaultUser(du)
	return json.Marshal(defaultUser)
}
