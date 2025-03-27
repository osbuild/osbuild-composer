package rhel9

import (
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/rpmmd"
)

func packageSetLoader(t *rhel.ImageType) (rpmmd.PackageSet, error) {
	return defs.PackageSet(t, "", nil)
}
