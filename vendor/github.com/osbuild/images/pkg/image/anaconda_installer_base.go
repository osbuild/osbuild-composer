package image

import (
	"math/rand"

	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/manifest"
)

// common struct that all anaconda installers share
type AnacondaInstallerBase struct {
	InstallerCustomizations manifest.InstallerCustomizations
	RootfsCompression       string
	Kickstart               *kickstart.Options
}

func initIsoTreePipeline(isoTreePipeline *manifest.AnacondaInstallerISOTree, img *AnacondaInstallerBase, rng *rand.Rand) {
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.InstallerCustomizations.Release
	isoTreePipeline.Kickstart = img.Kickstart

	isoTreePipeline.RootfsCompression = img.RootfsCompression
	isoTreePipeline.RootfsType = img.InstallerCustomizations.ISORootfsType

	isoTreePipeline.KernelOpts = img.InstallerCustomizations.KernelOptionsAppend
	if img.InstallerCustomizations.FIPS {
		isoTreePipeline.KernelOpts = append(isoTreePipeline.KernelOpts, "fips=1")
	}
	isoTreePipeline.ISOBoot = img.InstallerCustomizations.ISOBoot
}
