package rhel9

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/wsl"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/osbuild"
)

func mkWSLImgType() *rhel.ImageType {
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

	it.Compression = "xz"
	it.DefaultImageConfig = &distro.ImageConfig{
		CloudInit: []*osbuild.CloudInitStageOptions{
			{
				Filename: "99_wsl.cfg",
				Config: osbuild.CloudInitConfigFile{
					DatasourceList: []string{
						"WSL",
						"None",
					},
					Network: &osbuild.CloudInitConfigNetwork{
						Config: "disabled",
					},
				},
			},
		},
		Locale:    common.ToPtr("en_US.UTF-8"),
		NoSElinux: common.ToPtr(true),
		WSL: &wsl.WSL{
			Config: &wsl.WSLConfig{
				BootSystemd: true,
			},
		},
	}

	return it
}
