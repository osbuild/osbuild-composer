package ignition

import "github.com/osbuild/osbuild-composer/internal/blueprint"

type FirstBootOptions struct {
	ProvisioningURL string
}

func FirstbootOptionsFromBP(bpIgnitionFirstboot blueprint.FirstBootIgnitionCustomization) *FirstBootOptions {
	ignition := FirstBootOptions(bpIgnitionFirstboot)
	return &ignition
}

type EmbeddedOptions struct {
	ProvisioningURL string
	Data64          string
}

func EmbeddedOptionsFromBP(bpIgnitionEmbedded blueprint.EmbeddedIgnitionCustomization) *EmbeddedOptions {
	ignition := EmbeddedOptions(bpIgnitionEmbedded)
	return &ignition
}
