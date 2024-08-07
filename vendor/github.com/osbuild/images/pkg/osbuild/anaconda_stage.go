package osbuild

import (
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"golang.org/x/exp/slices"
)

type AnacondaStageOptions struct {
	// Kickstart modules to enable
	KickstartModules []string `json:"kickstart-modules"`
}

func (AnacondaStageOptions) isStageOptions() {}

// Configure basic aspects of the Anaconda installer
func NewAnacondaStage(options *AnacondaStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.anaconda",
		Options: options,
	}
}

func defaultModuleStates() map[string]bool {
	return map[string]bool{
		anaconda.ModuleLocalization: false,
		anaconda.ModuleNetwork:      true,
		anaconda.ModulePayloads:     true,
		anaconda.ModuleRuntime:      false,
		anaconda.ModuleSecurity:     false,
		anaconda.ModuleServices:     false,
		anaconda.ModuleStorage:      true,
		anaconda.ModuleSubscription: false,
		anaconda.ModuleTimezone:     false,
		anaconda.ModuleUsers:        false,
	}
}

func setModuleStates(states map[string]bool, enable, disable []string) {
	for _, modname := range enable {
		states[modname] = true
	}
	for _, modname := range disable {
		states[modname] = false
	}
}

func filterEnabledModules(moduleStates map[string]bool) []string {
	enabled := make([]string, 0, len(moduleStates))
	for modname, state := range moduleStates {
		if state {
			enabled = append(enabled, modname)
		}
	}
	// sort the list to guarantee stable manifests
	slices.Sort(enabled)
	return enabled
}

func NewAnacondaStageOptions(enableModules, disableModules []string) *AnacondaStageOptions {
	states := defaultModuleStates()
	setModuleStates(states, enableModules, disableModules)

	return &AnacondaStageOptions{
		KickstartModules: filterEnabledModules(states),
	}
}
