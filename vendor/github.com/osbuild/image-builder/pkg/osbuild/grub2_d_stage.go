package osbuild

// Grub2DStageOptions represents options for the
// org.osbuild.grub2.d stage.
//
// This stage writes a GRUB2 drop-in configuration file at a
// configurable path relative to the filesystem root.
type Grub2DStageOptions struct {
	// Path relative to the filesystem root
	Path string `json:"path"`

	// GRUB2 configuration to write
	Config *Grub2DConfig `json:"config"`
}

// Grub2DConfig contains GRUB2 settings for a drop-in config file.
type Grub2DConfig struct {
	TerminalInput  []string `json:"terminal_input,omitempty"`
	TerminalOutput []string `json:"terminal_output,omitempty"`
	Serial         string   `json:"serial,omitempty"`
}

func (Grub2DStageOptions) isStageOptions() {}

func NewGrub2DStage(options *Grub2DStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.grub2.d",
		Options: options,
	}
}

// NewGrub2DConfigFromGrub2Config creates a Grub2DConfig from a
// GRUB2Config, extracting only the console-related fields.
// Returns nil if no console settings are present.
func NewGrub2DConfigFromGrub2Config(cfg *GRUB2Config) *Grub2DConfig {
	if cfg == nil {
		return nil
	}
	c := &Grub2DConfig{
		TerminalInput:  cfg.TerminalInput,
		TerminalOutput: cfg.TerminalOutput,
		Serial:         cfg.Serial,
	}
	if len(c.TerminalInput) == 0 && len(c.TerminalOutput) == 0 && c.Serial == "" {
		return nil
	}
	return c
}
