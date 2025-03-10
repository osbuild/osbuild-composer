package osbuild

import (
	"github.com/osbuild/images/pkg/rpmmd"
)

type DNFModuleConfig struct {
	Name     string   `json:"name,omitempty"`
	Stream   string   `json:"stream,omitempty"`
	State    string   `json:"state,omitempty"`
	Profiles []string `json:"profiles"`
}

type DNFModuleConfigStageOptions struct {
	Config *DNFModuleConfig `json:"conf,omitempty"`
}

func (DNFModuleConfigStageOptions) isStageOptions() {}

// NewDNFModuleConfigStageOptions creates a new DNFConfig Stage options object.
func NewDNFModuleConfigStageOptions(config *DNFModuleConfig) *DNFModuleConfigStageOptions {
	return &DNFModuleConfigStageOptions{
		Config: config,
	}
}

func (o DNFModuleConfigStageOptions) validate() error {
	return nil
}

// NewDNFModuleConfigStage creates a new DNFModuleConfig Stage object.
func NewDNFModuleConfigStage(options *DNFModuleConfigStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.dnf.module-config",
		Options: options,
	}
}

func GenDNFModuleConfigStages(modules []rpmmd.ModuleSpec) []*Stage {
	stages := make([]*Stage, len(modules))

	for _, module := range modules {
		data := module.ModuleConfigFile.Data

		stage := NewDNFModuleConfigStage(&DNFModuleConfigStageOptions{
			Config: &DNFModuleConfig{
				Name:     data.Name,
				Stream:   data.Stream,
				State:    data.State,
				Profiles: data.Profiles,
			},
		})

		stages = append(stages, stage)
	}

	return stages
}
