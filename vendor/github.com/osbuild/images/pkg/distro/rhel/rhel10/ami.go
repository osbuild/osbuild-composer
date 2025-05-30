package rhel10

import (
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkAMIImgTypeX86_64(d *rhel.Distribution) *rhel.ImageType {
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
	it.DefaultImageConfig = imageConfig(d, "x86_64", "ami")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkAMIImgTypeAarch64(rd *rhel.Distribution) *rhel.ImageType {
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
	it.DefaultImageConfig = imageConfig(rd, "aarch64", "ami")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// RHEL internal-only x86_64 EC2 image type
func mkEc2ImgTypeX86_64(rd *rhel.Distribution) *rhel.ImageType {
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
	it.DefaultImageConfig = imageConfig(rd, "x86_64", "ec2")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// RHEL internal-only aarch64 EC2 image type
func mkEC2ImgTypeAarch64(rd *rhel.Distribution) *rhel.ImageType {
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
	it.DefaultImageConfig = imageConfig(rd, "aarch64", "ec2")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

// RHEL internal-only x86_64 EC2 HA image type
func mkEc2HaImgTypeX86_64(rd *rhel.Distribution) *rhel.ImageType {
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
	it.DefaultImageConfig = imageConfig(rd, "x86_64", "ec2-ha")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

func mkEC2SapImgTypeX86_64(rd *rhel.Distribution) *rhel.ImageType {
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
	it.DefaultImageConfig = imageConfig(rd, "x86_64", "ec2-sap")
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}
