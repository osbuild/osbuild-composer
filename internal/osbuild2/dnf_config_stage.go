package osbuild2

// DNFConfigStageOptions represents persistent DNF configuration.
type DNFConfigStageOptions struct {
	// List of DNF variables.
	Variables []DNFVariable `json:"variables,omitempty"`
}

func (DNFConfigStageOptions) isStageOptions() {}

// NewDNFConfigStageOptions creates a new DNFConfig Stage options object.
func NewDNFConfigStageOptions(variables []DNFVariable) *DNFConfigStageOptions {
	return &DNFConfigStageOptions{
		Variables: variables,
	}
}

// NewDNFConfigStage creates a new DNFConfig Stage object.
func NewDNFConfigStage(options *DNFConfigStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.dnf.config",
		Options: options,
	}
}

// DNFVariable represents a single DNF variable.
type DNFVariable struct {
	// Name of the variable.
	Name string `json:"name"`
	// Value of the variable.
	Value string `json:"value"`
}
