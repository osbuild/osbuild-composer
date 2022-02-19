package osbuild2

import (
	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/disk"
)

// The GRUB2StageOptions describes the bootloader configuration.
//
// The stage is responsible for installing all bootloader files in
// /boot as well as config files in /etc necessary for regenerating
// the configuration in /boot.
//
// Note that it is the role of an assembler to install any necessary
// bootloaders that are stored in the image outside of any filesystem.
type GRUB2StageOptions struct {
	RootFilesystemUUID uuid.UUID  `json:"root_fs_uuid"`
	BootFilesystemUUID *uuid.UUID `json:"boot_fs_uuid,omitempty"`
	KernelOptions      string     `json:"kernel_opts,omitempty"`
	Legacy             string     `json:"legacy,omitempty"`
	UEFI               *GRUB2UEFI `json:"uefi,omitempty"`
	SavedEntry         string     `json:"saved_entry,omitempty"`
	Greenboot          bool       `json:"greenboot,omitempty"`
}

type GRUB2UEFI struct {
	Vendor  string `json:"vendor"`
	Install bool   `json:"install,omitempty"`
	Unified bool   `json:"unified,omitempty"`
}

func (GRUB2StageOptions) isStageOptions() {}

// NewGRUB2Stage creates a new GRUB2 stage object.
func NewGRUB2Stage(options *GRUB2StageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.grub2",
		Options: options,
	}
}

func NewGrub2StageOptions(pt *disk.PartitionTable,
	kernelOptions string,
	kernel *blueprint.KernelCustomization,
	kernelVer string,
	uefi bool,
	legacy string,
	vendor string,
	install bool) *GRUB2StageOptions {

	rootFs := pt.FindMountable("/")
	if rootFs == nil {
		panic("root filesystem must be defined for grub2 stage, this is a programming error")
	}

	stageOptions := GRUB2StageOptions{
		RootFilesystemUUID: uuid.MustParse(rootFs.GetFSSpec().UUID),
		KernelOptions:      kernelOptions,
		Legacy:             legacy,
	}

	bootFs := pt.FindMountable("/boot")
	if bootFs != nil {
		bootFsUUID := uuid.MustParse(bootFs.GetFSSpec().UUID)
		stageOptions.BootFilesystemUUID = &bootFsUUID
	}

	if uefi {
		stageOptions.UEFI = &GRUB2UEFI{
			Vendor:  vendor,
			Install: install,
			Unified: legacy == "", // force unified grub scheme for pure efi systems
		}
	}

	if !uefi {
		stageOptions.Legacy = legacy
	}

	if kernel != nil {
		if kernel.Append != "" {
			stageOptions.KernelOptions += " " + kernel.Append
		}
		stageOptions.SavedEntry = "ffffffffffffffffffffffffffffffff-" + kernelVer
	}

	return &stageOptions
}
