package fedora

import (
	"fmt"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/rpmmd"
)

func packageSetLoader(t *imageType) (map[string]rpmmd.PackageSet, error) {
	return defs.PackageSets(t, VersionReplacements())
}

func imageConfig(d distribution, imageType string) *distro.ImageConfig {
	// arch is currently not used in fedora
	arch := ""
	return common.Must(defs.ImageConfig(d.name, arch, imageType, VersionReplacements()))
}

func installerConfig(d distribution, imageType string) *distro.InstallerConfig {
	// arch is currently not used in fedora
	arch := ""
	return common.Must(defs.InstallerConfig(d.name, arch, imageType, VersionReplacements()))
}

func newImageTypeFrom(d distribution, imgYAML defs.ImageTypeYAML) imageType {
	it := imageType{
		name:                   imgYAML.Name(),
		nameAliases:            imgYAML.NameAliases,
		filename:               imgYAML.Filename,
		compression:            imgYAML.Compression,
		mimeType:               imgYAML.MimeType,
		bootable:               imgYAML.Bootable,
		bootISO:                imgYAML.BootISO,
		rpmOstree:              imgYAML.RPMOSTree,
		isoLabel:               getISOLabelFunc(imgYAML.ISOLabel),
		defaultSize:            imgYAML.DefaultSize,
		buildPipelines:         imgYAML.BuildPipelines,
		payloadPipelines:       imgYAML.PayloadPipelines,
		exports:                imgYAML.Exports,
		requiredPartitionSizes: imgYAML.RequiredPartitionSizes,
		environment:            &imgYAML.Environment,
	}
	// XXX: make this a helper on imgYAML()
	it.defaultImageConfig = imageConfig(d, imgYAML.Name())
	it.defaultInstallerConfig = installerConfig(d, imgYAML.Name())
	it.packageSets = packageSetLoader

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
