package pipeline

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
	RootFilesystemUUID uuid.UUID `json:"root_fs_uuid"`
	BootFilesystemUUID uuid.UUID `json:"boot_fs_uuid,omitempty"`
	KernelOptions      string    `json:"kernel_opts,omitempty"`
}

func (GRUB2StageOptions) isStageOptions() {}

// NewGRUB2StageOptions creates a new GRUB2StageOptions object. It sets the
// mandatory options.
func NewGRUB2StageOptions(rootFilesystemUUID uuid.UUID) *GRUB2StageOptions {
	return &GRUB2StageOptions{
		RootFilesystemUUID: rootFilesystemUUID,
	}

}

// NewGRUB2Stage creates a new GRUB2 stage object.
func NewGRUB2Stage(options *GRUB2StageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.grub2",
		Options: options,
	}
}

// SetRootFilesystemUUID sets the UUID of the filesystem containing /.
func (options *GRUB2StageOptions) SetRootFilesystemUUID(u uuid.UUID) {
	options.RootFilesystemUUID = u
}

// SetBootFilesystemUUID sets the UUID of the filesystem containing /boot.
func (options *GRUB2StageOptions) SetBootFilesystemUUID(u uuid.UUID) {
	options.BootFilesystemUUID = u
}

// SetKernelOptions sets the kernel options that should be passed at boot.
func (options *GRUB2StageOptions) SetKernelOptions(kernelOptions string) {
	options.KernelOptions = kernelOptions
}
