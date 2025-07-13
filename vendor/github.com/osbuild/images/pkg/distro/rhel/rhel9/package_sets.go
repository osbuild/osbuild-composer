package rhel9

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/rpmmd"
)

func packageSetLoader(t *rhel.ImageType) (map[string]rpmmd.PackageSet, error) {
	return defs.PackageSets(t)
}

func imageConfig(d *rhel.Distribution, archName, imageType string) *distro.ImageConfig {
	return common.Must(defs.ImageConfig(d.Name(), archName, imageType))
}
