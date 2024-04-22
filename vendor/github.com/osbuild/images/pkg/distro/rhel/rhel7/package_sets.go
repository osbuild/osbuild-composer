package rhel7

import (
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/rpmmd"
)

// packages that are only in some (sub)-distributions
func distroSpecificPackageSet(t *rhel.ImageType) rpmmd.PackageSet {
	if t.IsRHEL() {
		return rpmmd.PackageSet{
			Include: []string{"insights-client"},
		}
	}
	return rpmmd.PackageSet{}
}
