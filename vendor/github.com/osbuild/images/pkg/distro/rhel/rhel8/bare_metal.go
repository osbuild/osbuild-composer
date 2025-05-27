package rhel8

import (
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkImageInstaller() *rhel.ImageType {
	it := rhel.NewImageType(
		"image-installer",
		"installer.iso",
		"application/x-iso9660-image",
		packageSetLoader,
		rhel.ImageInstallerImage,
		[]string{"build"},
		[]string{"anaconda-tree", "rootfs-image", "efiboot-tree", "os", "bootiso-tree", "bootiso"},
		[]string{"bootiso"},
	)

	it.BootISO = true
	it.Bootable = true
	it.ISOLabelFn = distroISOLabelFunc

	it.DefaultInstallerConfig = &distro.InstallerConfig{
		AdditionalDracutModules: []string{
			"ifcfg",
		},
	}

	return it
}

func mkTarImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"tar",
		"root.tar.xz",
		"application/x-tar",
		packageSetLoader,
		rhel.TarImage,
		[]string{"build"},
		[]string{"os", "archive"},
		[]string{"archive"},
	)

	return it
}
