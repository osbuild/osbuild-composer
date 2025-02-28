package osbuild

type MachineIdFirstBoot string

const (
	MachineIdFirstBootYes       MachineIdFirstBoot = "yes"
	MachineIdFirstBootNo        MachineIdFirstBoot = "no"
	MachineIdFirstBootPreserver MachineIdFirstBoot = "preserve"
)

type MachineIdStageOptions struct {
	// Determines the state of `/etc/machine-id`, valid values are
	// `yes` (reset to `uninitialized`), `no` (empty), `preserve` (keep).
	FirstBoot MachineIdFirstBoot `json:"first-boot"`
}

func (MachineIdStageOptions) isStageOptions() {}

func NewMachineIdStageOptions(firstboot MachineIdFirstBoot) *MachineIdStageOptions {
	return &MachineIdStageOptions{
		FirstBoot: firstboot,
	}
}

func NewMachineIdStage(options *MachineIdStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.machine-id",
		Options: options,
	}
}
