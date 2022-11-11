package ignition

import "github.com/osbuild/osbuild-composer/internal/blueprint"

type Options struct {
	ProvisioningURL string
}

func FromBP(bpIgnitionFirstboot blueprint.FirstBootIgnitionCustomization) *Options {
	ignition := Options(bpIgnitionFirstboot)
	return &ignition
}
