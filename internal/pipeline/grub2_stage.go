package pipeline

import "github.com/google/uuid"

type GRUB2StageOptions struct {
	RootFilesystemUUID uuid.UUID `json:"root_fs_uuid"`
	BootFilesystemUUID uuid.UUID `json:"boot_fs_uuid,omitempty"`
	KernelOptions      string    `json:"kernel_opts,omitempty"`
}

func (GRUB2StageOptions) isStageOptions() {}

func NewGRUB2StageOptions(rootFilesystemUUID uuid.UUID) *GRUB2StageOptions {
	return &GRUB2StageOptions{
		RootFilesystemUUID: rootFilesystemUUID,
	}

}

func NewGRUB2Stage(options *GRUB2StageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.grub2",
		Options: options,
	}
}

func (options *GRUB2StageOptions) SetBootFilesystemUUID(u uuid.UUID) {
	options.BootFilesystemUUID = u
}

func (options *GRUB2StageOptions) SetKernelOptions(kernelOptions string) {
	options.KernelOptions = kernelOptions
}
