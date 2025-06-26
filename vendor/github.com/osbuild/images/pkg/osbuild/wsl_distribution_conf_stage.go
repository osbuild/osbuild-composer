package osbuild

import (
	"github.com/osbuild/images/pkg/customizations/wsl"
)

type WSLDistributionConfStageOptions struct {
	OOBE     WSLDistributionConfOOBEOptions     `json:"oobe,omitempty"`
	Shortcut WSLDistributionConfShortcutOptions `json:"shortcut,omitempty"`
}

type WSLDistributionConfOOBEOptions struct {
	DefaultUID  *int   `json:"default_uid,omitempty"`
	DefaultName string `json:"default_name,omitempty"`
}

type WSLDistributionConfShortcutOptions struct {
	Enabled bool   `json:"enabled,omitempty"`
	Icon    string `json:"icon,omitempty"`
}

func (WSLDistributionConfStageOptions) isStageOptions() {}

func NewWSLDistributionConfStage(options *WSLDistributionConfStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.wsl-distribution.conf",
		Options: options,
	}
}

func NewWSLDistributionConfStageOptions(config *wsl.WSLDistributionConfig) *WSLDistributionConfStageOptions {
	if config == nil {
		return nil
	}

	options := &WSLDistributionConfStageOptions{}

	if config.OOBE != nil {
		options.OOBE = WSLDistributionConfOOBEOptions{
			DefaultUID:  config.OOBE.DefaultUID,
			DefaultName: config.OOBE.DefaultName,
		}
	}

	if config.Shortcut != nil {
		options.Shortcut = WSLDistributionConfShortcutOptions{
			Enabled: config.Shortcut.Enabled,
			Icon:    config.Shortcut.Icon,
		}
	}

	return options
}
