package osbuild

type SystemdBootConfig struct {
	RandomSeed         string `json:"random-seed,omitempty" yaml:"random-seed,omitempty"`
	MakeEntryDirectory string `json:"make-entry-directory,omitempty" yaml:"make-entry-directory,omitempty"`
	EntryToken         string `json:"entry-token,omitempty" yaml:"entry-token,omitempty"`
}

type BootctlInstallRootStageOptions struct {
	Root               string `json:"root"`
	ESPPath            string `json:"esp-path,omitempty"`
	BootPath           string `json:"boot-path,omitempty"`
	RelaxESPChecks     bool   `json:"relax-esp-checks,omitempty"`
	RandomSeed         string `json:"random-seed,omitempty"`
	MakeEntryDirectory string `json:"make-entry-directory,omitempty"`
	EntryToken         string `json:"entry-token,omitempty"`
}

func (BootctlInstallRootStageOptions) isStageOptions() {}

func NewBootctlInstallRootStage(opts *BootctlInstallRootStageOptions, devices map[string]Device, mounts []Mount) *Stage {
	return &Stage{
		Type:    "org.osbuild.bootctl.install.root",
		Options: opts,
		Devices: devices,
		Mounts:  mounts,
	}
}
