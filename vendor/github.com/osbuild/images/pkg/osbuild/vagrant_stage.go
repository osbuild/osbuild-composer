package osbuild

import (
	"fmt"
)

type VagrantProvider string

const (
	VagrantProviderLibvirt    VagrantProvider = "libvirt"
	VagrantProviderVirtualBox VagrantProvider = "virtualbox"
)

type VagrantSyncedFolderType string

const (
	VagrantSyncedFolderTypeRsync = "rsync"
)

type VagrantVirtualBoxStageOptions struct {
	MacAddress string `json:"mac_address"`
}

type VagrantSyncedFolderStageOptions struct {
	Type VagrantSyncedFolderType `json:"type"`
}

type VagrantStageOptions struct {
	Provider      VagrantProvider                             `json:"provider"`
	VirtualBox    *VagrantVirtualBoxStageOptions              `json:"virtualbox,omitempty"`
	SyncedFolders map[string]*VagrantSyncedFolderStageOptions `json:"synced_folders,omitempty"`
}

func (VagrantStageOptions) isStageOptions() {}

type VagrantStageInputs struct {
	Image *FilesInput `json:"image"`
}

func (VagrantStageInputs) isStageInputs() {}

func NewVagrantStage(options *VagrantStageOptions, inputs *VagrantStageInputs) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.vagrant",
		Options: options,
		Inputs:  inputs,
	}
}

func NewVagrantStageOptions(provider VagrantProvider) *VagrantStageOptions {
	return &VagrantStageOptions{
		Provider: provider,
	}
}

func (o *VagrantStageOptions) validate() error {
	if o.Provider != VagrantProviderLibvirt && o.Provider != VagrantProviderVirtualBox {
		return fmt.Errorf("unknown provider in vagrant stage options %s", o.Provider)
	}

	if o.Provider != VagrantProviderVirtualBox && len(o.SyncedFolders) > 0 {
		return fmt.Errorf("syncedfolders are only available for the virtualbox provider not for %q", o.Provider)
	}

	return nil
}

func NewVagrantStagePipelineFilesInputs(pipeline, file string) *VagrantStageInputs {
	input := NewFilesInput(NewFilesInputPipelineObjectRef(pipeline, file, nil))
	return &VagrantStageInputs{Image: input}
}
