package osbuild1

// A FixBLSStageOptions struct is empty, as the stage takes no options.
//
// The FixBLSStage fixes the paths in the Boot Loader Specification
// snippets installed into /boot. grub2's kernel install script will
// try to guess the correct path to the kernel and bootloader, and adjust
// the boot loader scripts accordingly. When run under OSBuild this does
// not work correctly, so this stage essentially reverts the "fixup".
type FixBLSStageOptions struct {
}

func (FixBLSStageOptions) isStageOptions() {}

// NewFixBLSStage creates a new FixBLSStage.
func NewFixBLSStage() *Stage {
	return &Stage{
		Name:    "org.osbuild.fix-bls",
		Options: &FixBLSStageOptions{},
	}
}
