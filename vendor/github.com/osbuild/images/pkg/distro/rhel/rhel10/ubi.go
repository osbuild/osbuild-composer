package rhel10

import (
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkWSLImgType(rd *rhel.Distribution) *rhel.ImageType {
	it := rhel.NewImageType(
		"wsl",
		"disk.tar.gz",
		"application/x-tar",
		packageSetLoader,
		rhel.TarImage,
		[]string{"build"},
		[]string{"os", "archive"},
		[]string{"archive"},
	)

	it.DefaultImageConfig = imageConfig(rd, "", "wsl")
	return it
}
