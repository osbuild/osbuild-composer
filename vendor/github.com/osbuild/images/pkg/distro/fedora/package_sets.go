package fedora

import (
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/rpmmd"
)

func packageSetLoader(t *imageType) (rpmmd.PackageSet, error) {
	return defs.PackageSet(t, "", VersionReplacements())
}
