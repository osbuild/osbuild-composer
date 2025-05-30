package rhel10

import (
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkTarImgType() *rhel.ImageType {
	return rhel.NewImageType(
		"tar",
		"root.tar.xz",
		"application/x-tar",
		packageSetLoader,
		rhel.TarImage,
		[]string{"build"},
		[]string{"os", "archive"},
		[]string{"archive"},
	)
}

func mkImageInstallerImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"image-installer",
		"installer.iso",
		"application/x-iso9660-image",
		packageSetLoader,
		rhel.ImageInstallerImage,
		[]string{"build"},
		[]string{"anaconda-tree", "efiboot-tree", "os", "bootiso-tree", "bootiso"},
		[]string{"bootiso"},
	)

	it.BootISO = true
	it.Bootable = true
	it.ISOLabelFn = distroISOLabelFunc

	it.DefaultInstallerConfig = &distro.InstallerConfig{
		AdditionalDracutModules: []string{
			"nvdimm", // non-volatile DIMM firmware (provides nfit, cuse, and nd_e820)
			"net-lib",
		},
		AdditionalDrivers: []string{
			"ipmi_devintf",
			"ipmi_msghandler",
		},
	}

	return it
}
