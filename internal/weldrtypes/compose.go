package weldrtypes

import (
	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/rpmmd"
)

// A Compose represent the task of building a set of images from a single blueprint.
// It contains all the information necessary to generate the inputs for the job, as
// well as the job's state.
type Compose struct {
	Blueprint  *blueprint.Blueprint
	ImageBuild ImageBuild
	Packages   []rpmmd.PackageSpec
}

// DeepCopy creates a copy of the Compose structure
func (c *Compose) DeepCopy() Compose {
	var newBpPtr *blueprint.Blueprint = nil
	if c.Blueprint != nil {
		bpCopy := *c.Blueprint
		newBpPtr = &bpCopy
	}
	pkgs := make([]rpmmd.PackageSpec, len(c.Packages))
	copy(pkgs, c.Packages)

	return Compose{
		Blueprint:  newBpPtr,
		ImageBuild: c.ImageBuild.DeepCopy(),
		Packages:   pkgs,
	}
}
