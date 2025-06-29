package rhel10

import (
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro/rhel"
)

func mkVagrantLibvirtImgType(d *rhel.Distribution, a arch.Arch) *rhel.ImageType {
	it := rhel.NewImageType(
		"vagrant-libvirt",
		"vagrant-libvirt.box",
		"application/x-tar",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "vagrant", "archive"},
		[]string{"archive"},
	)

	it.DefaultImageConfig = imageConfig(d, a.String(), "vagrant-libvirt")
	it.DefaultSize = 10 * datasizes.GibiByte
	it.Bootable = true
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}
