package image

import (
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/platform"
)

// common struct that all anaconda installers share
type ISOBootloaders struct {
	InstallerCustomizations *manifest.InstallerCustomizations
	ISOCustomizations       *manifest.ISOCustomizations

	// Grub2 menu customization for x86, ppc64, uefi iso menus, not supported by syslinux
	// Setting these turns off all of the default menus
	Custom []manifest.ISOGrub2MenuEntry
}

func (img *ISOBootloaders) Bootloaders(buildPipeline manifest.Build, platform platform.Platform, kernelOpts []string) []manifest.ISOBootloader {
	// Setup the bootloaders
	bootloaders := make([]manifest.ISOBootloader, 0)

	var addUEFIBootTree bool
	switch img.ISOCustomizations.BootType {
	case manifest.Grub2UEFIOnlyISOBoot:
		addUEFIBootTree = true

	case manifest.SyslinuxISOBoot:
		syslinux := manifest.NewISOLinuxBootloader(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
		syslinux.Platform = platform
		syslinux.KernelOpts = kernelOpts
		bootloaders = append(bootloaders, syslinux)
		addUEFIBootTree = true

	case manifest.Grub2ISOBoot:
		grub2 := manifest.NewGrub2X86Bootloader(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
		grub2.Platform = platform
		grub2.ISOLabel = img.ISOCustomizations.Label
		grub2.KernelOpts = kernelOpts
		grub2.DefaultMenu = img.InstallerCustomizations.DefaultMenu
		grub2.MenuTimeout = img.InstallerCustomizations.MenuTimeout

		for _, entry := range img.Custom {
			grub2.Custom = append(grub2.Custom, manifest.ISOGrub2MenuEntry{
				Name:   entry.Name,
				Linux:  entry.Linux,
				Initrd: entry.Initrd,
			})
		}

		bootloaders = append(bootloaders, grub2)
		addUEFIBootTree = true

	case manifest.Grub2PPCISOBoot:
		grub2ppc64 := manifest.NewGrub2PPC64Bootloader(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
		grub2ppc64.Platform = platform
		grub2ppc64.ISOLabel = img.ISOCustomizations.Label
		grub2ppc64.KernelOpts = kernelOpts
		grub2ppc64.DefaultMenu = img.InstallerCustomizations.DefaultMenu
		grub2ppc64.MenuTimeout = img.InstallerCustomizations.MenuTimeout

		for _, entry := range img.Custom {
			grub2ppc64.Custom = append(grub2ppc64.Custom, manifest.ISOGrub2MenuEntry{
				Name:   entry.Name,
				Linux:  entry.Linux,
				Initrd: entry.Initrd,
			})
		}

		bootloaders = append(bootloaders, grub2ppc64)

	case manifest.S390ISOBoot:
		s390 := manifest.NewS390Bootloader(buildPipeline)
		s390.Platform = platform
		s390.KernelOpts = kernelOpts
		bootloaders = append(bootloaders, s390)
	}

	if addUEFIBootTree {
		// EFI bootloader adds a pipeline and adds stages
		efiPipeline := manifest.NewEFIBootTree(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
		efiPipeline.Platform = platform
		efiPipeline.UEFIVendor = platform.GetUEFIVendor()
		efiPipeline.ISOLabel = img.ISOCustomizations.Label
		efiPipeline.DefaultMenu = img.InstallerCustomizations.DefaultMenu
		efiPipeline.MenuTimeout = img.InstallerCustomizations.MenuTimeout
		efiPipeline.KernelOpts = kernelOpts

		for _, entry := range img.Custom {
			efiPipeline.MenuEntries = append(efiPipeline.MenuEntries, manifest.ISOGrub2MenuEntry{
				Name:   entry.Name,
				Linux:  entry.Linux,
				Initrd: entry.Initrd,
			})
		}

		bootloaders = append(bootloaders, efiPipeline)
	}
	return bootloaders
}
