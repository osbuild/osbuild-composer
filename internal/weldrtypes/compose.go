package weldrtypes

import (
	"github.com/osbuild/blueprint/pkg/blueprint"
)

// A Compose represent the task of building a set of images from a single blueprint.
// It contains all the information necessary to generate the inputs for the job, as
// well as the job's state.
type Compose struct {
	Blueprint  *blueprint.Blueprint
	ImageBuild ImageBuild
	Packages   []DepsolvedPackageInfo
}

// DeepCopy creates a copy of the Compose structure
func (c *Compose) DeepCopy() Compose {
	var newBpPtr *blueprint.Blueprint = nil
	if c.Blueprint != nil {
		bpCopy := *c.Blueprint
		newBpPtr = &bpCopy
	}
	pkgs := make([]DepsolvedPackageInfo, len(c.Packages))
	copy(pkgs, c.Packages)

	return Compose{
		Blueprint:  newBpPtr,
		ImageBuild: c.ImageBuild.DeepCopy(),
		Packages:   pkgs,
	}
}
