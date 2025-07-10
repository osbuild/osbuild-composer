package generic

import (
	"fmt"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/distro/defs"
)

func newImageTypeFrom(d *distribution, ar *architecture, imgYAML defs.ImageTypeYAML) imageType {
	it := imageType{
		ImageTypeYAML: imgYAML,
		isoLabel:      d.getISOLabelFunc(imgYAML.ISOLabel),
	}
	it.defaultImageConfig = common.Must(it.ImageConfig(d.Name(), ar.name))
	it.defaultInstallerConfig = common.Must(it.InstallerConfig(d.Name(), ar.name))

	switch imgYAML.Image {
	case "disk":
		it.image = diskImage
	case "container":
		it.image = containerImage
	case "image_installer":
		it.image = imageInstallerImage
	case "live_installer":
		it.image = liveInstallerImage
	case "bootable_container":
		it.image = bootableContainerImage
	case "iot":
		it.image = iotImage
	case "iot_commit":
		it.image = iotCommitImage
	case "iot_container":
		it.image = iotContainerImage
	case "iot_installer":
		it.image = iotInstallerImage
	case "iot_simplified_installer":
		it.image = iotSimplifiedInstallerImage
	case "tar":
		it.image = tarImage
	default:
		err := fmt.Errorf("unknown image func: %v for %v", imgYAML.Image, imgYAML.Name())
		panic(err)
	}

	return it
}
