package rhel8

import (
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/rpmmd"
)

func mkImageInstaller() *rhel.ImageType {
	it := rhel.NewImageType(
		"image-installer",
		"installer.iso",
		"application/x-iso9660-image",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: func(t *rhel.ImageType) (rpmmd.PackageSet, error) {
				return defs.PackageSet(t, "bare-metal", nil)
			},
			rhel.InstallerPkgsKey: packageSetLoader,
		},
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
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.TarImage,
		[]string{"build"},
		[]string{"os", "archive"},
		[]string{"archive"},
	)

	return it
}
