package image

import (
	"math/rand"

	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
)

// common struct that all anaconda installers share
type AnacondaInstallerBase struct {
	InstallerCustomizations manifest.InstallerCustomizations
	ISOCustomizations       manifest.ISOCustomizations
	RootfsCompression       string

	Kickstart                    *kickstart.Options
	InteractiveDefaultsKickstart *kickstart.Options
}

func (img *AnacondaInstallerBase) Bootloaders(buildPipeline manifest.Build, platform platform.Platform, kernelOpts []string) []manifest.ISOBootloader {
	// Setup the bootloaders
	bootloaders := make([]manifest.ISOBootloader, 0)

	switch img.ISOCustomizations.BootType {
	case manifest.SyslinuxISOBoot:
		syslinux := manifest.NewISOLinuxBootloader(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
		syslinux.Platform = platform
		syslinux.KernelOpts = kernelOpts
		bootloaders = append(bootloaders, syslinux)

	case manifest.Grub2ISOBoot:
		grub2 := manifest.NewGrub2X86Bootloader(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
		grub2.Platform = platform
		grub2.ISOLabel = img.ISOCustomizations.Label
		grub2.KernelOpts = kernelOpts
		grub2.DefaultMenu = img.InstallerCustomizations.DefaultMenu
		bootloaders = append(bootloaders, grub2)

	case manifest.Grub2PPCISOBoot:
		grub2ppc64 := manifest.NewGrub2PPC64Bootloader(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
		grub2ppc64.Platform = platform
		grub2ppc64.ISOLabel = img.ISOCustomizations.Label
		grub2ppc64.KernelOpts = kernelOpts
		grub2ppc64.DefaultMenu = img.InstallerCustomizations.DefaultMenu
		bootloaders = append(bootloaders, grub2ppc64)
	}

	// Skip using UEFI on PPC64LE
	if img.ISOCustomizations.BootType != manifest.Grub2PPCISOBoot {
		// EFI bootloader adds a pipeline and adds stages
		bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
		bootTreePipeline.Platform = platform
		bootTreePipeline.UEFIVendor = platform.GetUEFIVendor()
		bootTreePipeline.ISOLabel = img.ISOCustomizations.Label
		bootTreePipeline.DefaultMenu = img.InstallerCustomizations.DefaultMenu
		bootTreePipeline.KernelOpts = kernelOpts
		bootloaders = append(bootloaders, bootTreePipeline)
	}
	return bootloaders
}

func initIsoTreePipeline(isoTreePipeline *manifest.AnacondaInstallerISOTree, img *AnacondaInstallerBase, rng *rand.Rand) {
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.Release = img.InstallerCustomizations.Release
	isoTreePipeline.Kickstart = img.Kickstart

	isoTreePipeline.RootfsCompression = img.RootfsCompression
	isoTreePipeline.RootfsType = img.ISOCustomizations.RootfsType

	isoTreePipeline.ISOBoot = img.ISOCustomizations.BootType
}
