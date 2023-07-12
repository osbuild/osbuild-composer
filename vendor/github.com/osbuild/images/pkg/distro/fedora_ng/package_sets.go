package fedora_ng

// This file defines package sets that are used by more than one image type.

import (
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

func corePackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{};
}

func diskRawPackageSet(t *imageType) rpmmd.PackageSet {
	return corePackageSet(t)
}

func isoLivePackageSet(t *imageType) rpmmd.PackageSet {
	ps := corePackageSet(t)

	ps = ps.Append(rpmmd.PackageSet{
		Include: []string{"livesys-scripts", "fedora-logos", "dracut-live"},
	})

	// XXX I wonder if this has to be here or in the build? We only copy it to
	// XXX the correct place.
	if t.arch.Name() == platform.ARCH_X86_64.String() {
		ps = ps.Append(rpmmd.PackageSet{
			Include: []string{"syslinux-nonlinux"},
		})
	}

	return ps
}
