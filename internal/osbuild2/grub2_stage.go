package osbuild2

import "github.com/google/uuid"

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
}

type GRUB2UEFI struct {
	Vendor string `json:"vendor"`
}

func (GRUB2StageOptions) isStageOptions() {}

// NewGRUB2Stage creates a new GRUB2 stage object.
func NewGRUB2Stage(options *GRUB2StageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.grub2",
		Options: options,
	}
}
