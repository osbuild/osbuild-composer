package osbuild

import (
	"fmt"

	"github.com/osbuild/images/pkg/platform"
)

type BootcInstallToFilesystemOptions struct {
	// options for --root-ssh-authorized-keys
	RootSSHAuthorizedKeys []string `json:"root-ssh-authorized-keys,omitempty"`
	// options for --karg
	Kargs []string `json:"kernel-args,omitempty"`

	// option for --target-imgref
	TargetImgref string `json:"target-imgref"`
}

func (BootcInstallToFilesystemOptions) isStageOptions() {}

// NewBootcInstallToFilesystem creates a new stage for the
// org.osbuild.bootc.install-to-filesystem stage.
//
// It requires a mount setup so that bootupd can be run by bootc. I.e
// "/", "/boot" and "/boot/efi" need to be set up so that
// bootc/bootupd find and install all required bootloader bits.
//
// The mounts input should be generated with GenBootupdDevicesMounts.
func NewBootcInstallToFilesystemStage(options *BootcInstallToFilesystemOptions, inputs ContainerDeployInputs, devices map[string]Device, mounts []Mount, pltf platform.Platform) (*Stage, error) {
	if err := validateBootupdMounts(mounts, pltf); err != nil {
		return nil, err
	}

	if len(inputs.Images.References) != 1 {
		return nil, fmt.Errorf("expected exactly one container input but got: %v (%v)", len(inputs.Images.References), inputs.Images.References)
	}

	return &Stage{
		Type:    "org.osbuild.bootc.install-to-filesystem",
		Options: options,
		Inputs:  inputs,
		Devices: devices,
		Mounts:  mounts,
	}, nil
}
