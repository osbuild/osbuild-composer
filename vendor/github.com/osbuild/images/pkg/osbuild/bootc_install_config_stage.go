package osbuild

type BootcInstallConfigStageOptions struct {
	Filename string             `json:"filename"`
	Config   BootcInstallConfig `json:"config"`
}

func (BootcInstallConfigStageOptions) isStageOptions() {}

type BootcInstallConfig struct {
	Install    *BootcInstallConfigInstall `json:"install,omitempty"`
	KernelArgs []string                   `json:"kargs,omitempty"`
	Block      []string                   `json:"block,omitempty"`
}

type BootcInstallConfigInstall struct {
	Filesystem BootcInstallConfigFilesystem `json:"filesystem"`
}

type BootcInstallConfigFilesystem struct {
	Root BootcInstallConfigFilesystemRoot `json:"root"`
}

type BootcInstallConfigFilesystemRoot struct {
	Type string `json:"type"`
}

// GenBootcInstallOptions is a helper function for creating stage options for
// org.osbuild.bootc.install.config with just the filename and root filesystem
// type set.
func GenBootcInstallOptions(filename, rootType string) *BootcInstallConfigStageOptions {
	return &BootcInstallConfigStageOptions{
		Filename: filename,
		Config: BootcInstallConfig{
			Install: &BootcInstallConfigInstall{
				Filesystem: BootcInstallConfigFilesystem{
					Root: BootcInstallConfigFilesystemRoot{
						Type: rootType,
					},
				},
			},
		},
	}
}

func NewBootcInstallConfigStage(options *BootcInstallConfigStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.bootc.install.config",
		Options: options,
	}
}
