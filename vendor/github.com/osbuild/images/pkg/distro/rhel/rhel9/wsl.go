package rhel9

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkWSLImgType(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"wsl",
		"image.wsl",
		"application/x-tar",
		packageSetLoader,
		rhel.TarImage,
		[]string{"build"},
		[]string{"os", "archive", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.DefaultImageConfig = imageConfig(d, a.String(), "wsl")

	return it
}
