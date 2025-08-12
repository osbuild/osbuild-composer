package osbuild

import (
	"slices"
)

type AnacondaStageOptions struct {
	// Kickstart modules to enable
	//
	// Deprecated:
	//  RHEL 9:  Available but marked deprecated
	//  RHEL 10: Removed
	//  Fedora:  Removed
	//
	// https://bugzilla.redhat.com/show_bug.cgi?id=2023855#c10
	KickstartModules []string `json:"kickstart-modules,omitempty"`

	// Kickstart modules to activate
	//
	// Replaced kickstart-modules in newer versions.
	ActivatableModules []string `json:"activatable-modules,omitempty"`

	// Kickstart modules to forbid
	ForbiddenModules []string `json:"forbidden-modules,omitempty"`

	// Kickstart modules to activate but are allowed to fail
	OptionalModules []string `json:"optional-modules,omitempty"`
}

func (AnacondaStageOptions) isStageOptions() {}

// Configure basic aspects of the Anaconda installer
func NewAnacondaStage(options *AnacondaStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.anaconda",
		Options: options,
	}
}

func moduleStates(enable, disable []string) map[string]bool {
	states := map[string]bool{}

	for _, modname := range enable {
		states[modname] = true
	}
	for _, modname := range disable {
		states[modname] = false
	}

	return states
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

func NewAnacondaStageOptionsLegacy(enableModules, disableModules []string) *AnacondaStageOptions {
	states := moduleStates(enableModules, disableModules)

	return &AnacondaStageOptions{
		KickstartModules: filterEnabledModules(states),
	}
}

func NewAnacondaStageOptions(enableModules, disableModules []string) *AnacondaStageOptions {
	states := moduleStates(enableModules, disableModules)

	return &AnacondaStageOptions{
		ActivatableModules: filterEnabledModules(states),
	}
}
