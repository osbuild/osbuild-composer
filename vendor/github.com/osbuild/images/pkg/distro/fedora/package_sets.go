package fedora

import (
	"github.com/osbuild/images/pkg/distro/packagesets"
	"github.com/osbuild/images/pkg/rpmmd"
)

func packageSetLoader(t *imageType) rpmmd.PackageSet {
	return packagesets.Load(t, VersionReplacements())
}
