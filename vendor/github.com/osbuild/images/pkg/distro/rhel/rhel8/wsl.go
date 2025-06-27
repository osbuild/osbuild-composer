package rhel8

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkWslImgType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"wsl",
		"image.wsl",
		"application/x-tar",
		packageSetLoader,
		rhel.TarImage,
		[]string{"build"},
		[]string{"os", "archive"},
		[]string{"archive"},
	)
	it.DefaultImageConfig = imageConfig(rd, a.String(), "wsl")
	it.Compression = "xz"

	return it
}
