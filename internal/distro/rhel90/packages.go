package rhel90

import "github.com/osbuild/osbuild-composer/internal/rpmmd"

type archPackages struct {
	x8664   []string
	aarch64 []string
	ppc64le []string
	s390x   []string
}

var packages = struct {
	GenericBuild []string
	Bootloader   archPackages
	Build        archPackages
	Qcow2        rpmmd.PackageSet
}{
	GenericBuild: []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"glibc",
		"policycoreutils",
		"python39",
		"python3-iniparse", // dependency of org.osbuild.rhsm stage
		"qemu-img",
		"selinux-policy-targeted",
		"systemd",
		"tar",
		"xfsprogs",
		"xz",
	},
	Bootloader: archPackages{
		x8664: []string{
			"dracut-config-generic",
			"grub2-pc",
			"grub2-efi-x64",
			"shim-x64",
		},
		aarch64: []string{
			"dracut-config-generic",
			"efibootmgr",
			"grub2-efi-aa64",
			"grub2-tools",
			"shim-aa64",
		},
		ppc64le: []string{
			"dracut-config-generic",
			"powerpc-utils",
			"grub2-ppc64le",
			"grub2-ppc64le-modules",
		},
		s390x: []string{
			"dracut-config-generic",
			"s390utils-base",
		},
	},
	Build: archPackages{
		x8664: []string{
			"grub2-pc",
		},
		aarch64: nil,
		ppc64le: []string{
			"grub2-ppc64le",
			"grub2-ppc64le-modules",
		},
		s390x: nil,
	},
	Qcow2: rpmmd.PackageSet{
		Include: []string{
			"@Core",
			"authselect-compat",
			"chrony",
			"cloud-init",
			"cloud-utils-growpart",
			"cockpit-system",
			"cockpit-ws",
			"dnf",
			"dnf-utils",
			"dosfstools",
			"dracut-config-generic",
			"hostname",
			"NetworkManager",
			"nfs-utils",
			"oddjob",
			"oddjob-mkhomedir",
			"psmisc",
			"python3-jsonschema",
			"redhat-release",
			"redhat-release-eula",
			"rsync",
			"subscription-manager-cockpit",
			"tar",
			"tcpdump",
			"yum",
		},
		Exclude: []string{
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-lib",
			"alsa-tools-firmware",
			"biosdevname",
			"dnf-plugin-spacewalk",
			"dracut-config-rescue",
			"fedora-release",
			"fedora-repos",
			"firewalld",
			"iprutils",
			"ivtv-firmware",
			"iwl100-firmware",
			"iwl1000-firmware",
			"iwl105-firmware",
			"iwl135-firmware",
			"iwl2000-firmware",
			"iwl2030-firmware",
			"iwl3160-firmware",
			"iwl3945-firmware",
			"iwl4965-firmware",
			"iwl5000-firmware",
			"iwl5150-firmware",
			"iwl6000-firmware",
			"iwl6000g2a-firmware",
			"iwl6000g2b-firmware",
			"iwl6050-firmware",
			"iwl7260-firmware",
			"langpacks-en",
			"libertas-sd8686-firmware",
			"libertas-sd8787-firmware",
			"libertas-usb8388-firmware",
			"nss",
			"plymouth",
			"rng-tools",
			"udisks2",
		},
	},
}
