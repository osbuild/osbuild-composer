package rhel9

import (
	"fmt"
	"os"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
)

// selinuxStageOptions returns the options for the org.osbuild.selinux stage.
// Setting the argument to 'true' relabels the '/usr/bin/cp' and '/usr/bin/tar'
// binaries with 'install_exec_t'. This should be set in the build root.
func selinuxStageOptions(labelcp bool) *osbuild.SELinuxStageOptions {
	options := &osbuild.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
	if labelcp {
		options.Labels = map[string]string{
			"/usr/bin/cp":  "system_u:object_r:install_exec_t:s0",
			"/usr/bin/tar": "system_u:object_r:install_exec_t:s0",
		}
	}
	return options
}

func buildStampStageOptions(arch, product, osVersion, variant string) *osbuild.BuildstampStageOptions {
	return &osbuild.BuildstampStageOptions{
		Arch:    arch,
		Product: product,
		Version: osVersion,
		Variant: variant,
		Final:   true,
	}
}

func dracutStageOptions(kernelVer, arch string, additionalModules []string) *osbuild.DracutStageOptions {
	kernel := []string{kernelVer}
	modules := []string{
		"bash",
		"systemd",
		"fips",
		"systemd-initrd",
		"modsign",
		"nss-softokn",
		"i18n",
		"convertfs",
		"network-manager",
		"network",
		"ifcfg",
		"url-lib",
		"drm",
		"plymouth",
		"prefixdevname",
		"prefixdevname-tools",
		"crypt",
		"dm",
		"dmsquash-live",
		"kernel-modules",
		"kernel-modules-extra",
		"kernel-network-modules",
		"livenet",
		"lvm",
		"mdraid",
		"qemu",
		"qemu-net",
		"resume",
		"rootfs-block",
		"terminfo",
		"udev-rules",
		"dracut-systemd",
		"pollcdrom",
		"usrmount",
		"base",
		"fs-lib",
		"img-lib",
		"shutdown",
		"uefi-lib",
	}

	if arch == distro.X86_64ArchName {
		modules = append(modules, "biosdevname")
	}

	modules = append(modules, additionalModules...)
	return &osbuild.DracutStageOptions{
		Kernel:  kernel,
		Modules: modules,
		Install: []string{"/.buildstamp"},
	}
}

func grubISOStageOptions(installDevice, kernelVer, arch, vendor, product, osVersion, isolabel string, fdo *blueprint.FDOCustomization) *osbuild.GrubISOStageOptions {
	var architectures []string

	if arch == distro.X86_64ArchName {
		architectures = []string{"X64"}
	} else if arch == distro.Aarch64ArchName {
		architectures = []string{"AA64"}
	} else {
		panic("unsupported architecture")
	}

	grubISOStageOptions := &osbuild.GrubISOStageOptions{
		Product: osbuild.Product{
			Name:    product,
			Version: osVersion,
		},
		ISOLabel: isolabel,
		Kernel: osbuild.ISOKernel{
			Dir: "/images/pxeboot",
			Opts: []string{"rd.neednet=1",
				"coreos.inst.crypt_root=1",
				"coreos.inst.isoroot=" + isolabel,
				"coreos.inst.install_dev=" + installDevice,
				"coreos.inst.image_file=/run/media/iso/disk.img.xz",
				"coreos.inst.insecure"},
		},
		Architectures: architectures,
		Vendor:        vendor,
	}
	if fdo.HasFDO() {
		grubISOStageOptions.Kernel.Opts = append(grubISOStageOptions.Kernel.Opts, "fdo.manufacturing_server_url="+fdo.ManufacturingServerURL)
		if fdo.DiunPubKeyInsecure != "" {
			grubISOStageOptions.Kernel.Opts = append(grubISOStageOptions.Kernel.Opts, "fdo.diun_pub_key_insecure="+fdo.DiunPubKeyInsecure)
		}
		if fdo.DiunPubKeyHash != "" {
			grubISOStageOptions.Kernel.Opts = append(grubISOStageOptions.Kernel.Opts, "fdo.diun_pub_key_hash="+fdo.DiunPubKeyHash)
		}
		if fdo.DiunPubKeyRootCerts != "" {
			grubISOStageOptions.Kernel.Opts = append(grubISOStageOptions.Kernel.Opts, "fdo.diun_pub_key_root_certs=/fdo_diun_pub_key_root_certs.pem")
		}
	}

	return grubISOStageOptions
}

func xorrisofsStageOptions(filename, isolabel, arch string, isolinux bool) *osbuild.XorrisofsStageOptions {
	options := &osbuild.XorrisofsStageOptions{
		Filename: filename,
		VolID:    fmt.Sprintf(isolabel, arch),
		SysID:    "LINUX",
		EFI:      "images/efiboot.img",
		ISOLevel: 3,
	}

	if isolinux {
		options.Boot = &osbuild.XorrisofsBoot{
			Image:   "isolinux/isolinux.bin",
			Catalog: "isolinux/boot.cat",
		}

		options.IsohybridMBR = "/usr/share/syslinux/isohdpfx.bin"
	}

	return options
}

func ostreeConfigStageOptions(repo string, readOnly bool) *osbuild.OSTreeConfigStageOptions {
	return &osbuild.OSTreeConfigStageOptions{
		Repo: repo,
		Config: &osbuild.OSTreeConfig{
			Sysroot: &osbuild.SysrootOptions{
				ReadOnly:   common.BoolToPtr(readOnly),
				Bootloader: "none",
			},
		},
	}
}

func efiMkdirStageOptions() *osbuild.MkdirStageOptions {
	return &osbuild.MkdirStageOptions{
		Paths: []osbuild.Path{
			{
				Path: "/boot/efi",
				Mode: os.FileMode(0700),
			},
		},
	}
}
