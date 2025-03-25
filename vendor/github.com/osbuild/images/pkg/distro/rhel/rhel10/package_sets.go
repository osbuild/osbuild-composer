package rhel10

// This file defines package sets that are used by more than one image type.

import (
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/rpmmd"
)

func packageSetLoader(t *rhel.ImageType) (rpmmd.PackageSet, error) {
	return defs.PackageSet(t, "", nil)
}
