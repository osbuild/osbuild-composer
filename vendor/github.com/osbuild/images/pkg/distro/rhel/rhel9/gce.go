package rhel9

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkGCEImageType(rd *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"gce",
		"image.tar.gz",
		"application/gzip",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "archive"},
		[]string{"archive"},
	)

	it.NameAliases = []string{"gce-rhui"}
	// The configuration for non-RHUI images does not touch the RHSM configuration at all.
	// https://issues.redhat.com/browse/COMPOSER-2157
	it.DefaultImageConfig = imageConfig(rd, a.String(), "gce")
	it.DefaultSize = 20 * datasizes.GibiByte
	it.Bootable = true
	// TODO: the base partition table still contains the BIOS boot partition, but the image is UEFI-only
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}
