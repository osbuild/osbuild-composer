package osbuild

import (
	"github.com/osbuild/images/pkg/customizations/wsl"
)

type WSLConfStageOptions struct {
	Boot WSLConfBootOptions `json:"boot"`
}

type WSLConfBootOptions struct {
	Systemd bool `json:"systemd"`
}

func (WSLConfStageOptions) isStageOptions() {}

func NewWSLConfStage(options *WSLConfStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.wsl.conf",
		Options: options,
	}
}

func NewWSLConfStageOptions(config *wsl.WSLConfig) *WSLConfStageOptions {
	if config == nil {
		return nil
	}

	return &WSLConfStageOptions{
		Boot: WSLConfBootOptions{
			Systemd: config.BootSystemd,
		},
	}
}
