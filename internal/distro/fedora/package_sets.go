package fedora

// This file defines package sets that are used by more than one image type.

import (
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func emptyPackageSet(t *imageType) rpmmd.PackageSet {
	return rpmmd.PackageSet{}
}
