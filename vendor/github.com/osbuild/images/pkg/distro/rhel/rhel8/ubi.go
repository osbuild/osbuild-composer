package rhel8

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkWslImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"wsl",
		"disk.tar.gz",
		"application/x-tar",
		map[string]rhel.PackageSetFunc{
			rhel.OSPkgsKey: packageSetLoader,
		},
		rhel.TarImage,
		[]string{"build"},
		[]string{"os", "archive"},
		[]string{"archive"},
	)

	it.DefaultImageConfig = &distro.ImageConfig{
		Locale:    common.ToPtr("en_US.UTF-8"),
		NoSElinux: common.ToPtr(true),
		WSLConfig: &distro.WSLConfig{
			BootSystemd: true,
		},
	}

	return it
}
