package rhel9

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkEc2ImgTypeX86_64(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"ec2",
		"image.raw.xz",
		"application/xz",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.DefaultImageConfig = imageConfig(d, a.String(), "ec2")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkAMIImgTypeX86_64(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"ami",
		"image.raw",
		"application/octet-stream",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image"},
		[]string{"image"},
	)

	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.DefaultImageConfig = imageConfig(d, a.String(), "ami")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}
func mkEC2SapImgTypeX86_64(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"ec2-sap",
		"image.raw.xz",
		"application/xz",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.DefaultImageConfig = imageConfig(d, a.String(), "ec2-sap")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkEc2HaImgTypeX86_64(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"ec2-ha",
		"image.raw.xz",
		"application/xz",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.DefaultImageConfig = imageConfig(d, a.String(), "ec2-ha")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkAMIImgTypeAarch64(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"ami",
		"image.raw",
		"application/octet-stream",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image"},
		[]string{"image"},
	)

	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.DefaultImageConfig = imageConfig(d, a.String(), "ami")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkEC2ImgTypeAarch64(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"ec2",
		"image.raw.xz",
		"application/xz",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "xz"},
		[]string{"xz"},
	)

	it.Compression = "xz"
	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.DefaultImageConfig = imageConfig(d, a.String(), "ec2")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}
