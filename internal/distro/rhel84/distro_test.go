package rhel84_test

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel84"
	"github.com/stretchr/testify/assert"
)

func TestFilenameFromType(t *testing.T) {
	type args struct {
		outputFormat string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		want1   string
		wantErr bool
	}{
		{
			name:  "ami",
			args:  args{"ami"},
			want:  "image.raw",
			want1: "application/octet-stream",
		},
		{
			name:  "openstack",
			args:  args{"openstack"},
			want:  "disk.qcow2",
			want1: "application/x-qemu-disk",
		},
		{
			name:  "qcow2",
			args:  args{"qcow2"},
			want:  "disk.qcow2",
			want1: "application/x-qemu-disk",
		},
		{
			name:  "tar",
			args:  args{"tar"},
			want:  "root.tar.xz",
			want1: "application/x-tar",
		},

		{
			name:  "vhd",
			args:  args{"vhd"},
			want:  "disk.vhd",
			want1: "application/x-vhd",
		},
		{
			name:  "vmdk",
			args:  args{"vmdk"},
			want:  "disk.vmdk",
			want1: "application/x-vmdk",
		},
		{
			name:    "invalid-output-type",
			args:    args{"foobar"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist := rhel84.New()
			arch, _ := dist.GetArch("x86_64")
			imgType, err := arch.GetImageType(tt.args.outputFormat)
			if (err != nil) != tt.wantErr {
				t.Errorf("Arch.GetImageType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				got := imgType.Filename()
				got1 := imgType.MIMEType()
				if got != tt.want {
					t.Errorf("ImageType.Filename()  got = %v, want %v", got, tt.want)
				}
				if got1 != tt.want1 {
					t.Errorf("ImageType.MIMEType() got1 = %v, want %v", got1, tt.want1)
				}
			}
		})
	}
}

func TestImageType_BuildPackages(t *testing.T) {
	x8664BuildPackages := []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"grub2-efi-x64",
		"grub2-pc",
		"policycoreutils",
		"shim-x64",
		"systemd",
		"tar",
		"qemu-img",
		"xz",
	}
	aarch64BuildPackages := []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"policycoreutils",
		"qemu-img",
		"systemd",
		"tar",
		"xz",
	}
	buildPackages := map[string][]string{
		"x86_64":  x8664BuildPackages,
		"aarch64": aarch64BuildPackages,
		"ppc64le": nil,
		"s390x":   nil,
	}
	d := rhel84.New()
	for _, archLabel := range d.ListArches() {
		archStruct, err := d.GetArch(archLabel)
		if assert.NoErrorf(t, err, "d.GetArch(%v) returned err = %v; expected nil", archLabel, err) {
			continue
		}
		for _, itLabel := range archStruct.ListImageTypes() {
			itStruct, err := archStruct.GetImageType(itLabel)
			if assert.NoErrorf(t, err, "d.GetArch(%v) returned err = %v; expected nil", archLabel, err) {
				continue
			}
			assert.ElementsMatch(t, buildPackages[archLabel], itStruct.BuildPackages())
		}
	}
}

func TestImageType_Name(t *testing.T) {
	distro := rhel84.New()
	imgMap := []struct {
		arch     string
		imgNames []string
	}{
		{
			arch: "x86_64",
			imgNames: []string{
				"ami",
				"qcow2",
				"openstack",
				"tar",
				"vhd",
				"vmdk",
			},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"ami",
				"qcow2",
				"openstack",
				"tar",
			},
		},
		{
			arch: "ppc64le",
			imgNames: []string{
				"qcow2",
				"tar",
			},
		},
		{
			arch: "s390x",
			imgNames: []string{
				"tar",
			},
		},
	}
	for _, mapping := range imgMap {
		arch, err := distro.GetArch(mapping.arch)
		if assert.NoError(t, err) {
			for _, imgName := range mapping.imgNames {
				imgType, err := arch.GetImageType(imgName)
				if assert.NoError(t, err) {
					assert.Equalf(t, imgName, imgType.Name(), "arch: %s", mapping.arch)
				}
			}
		}
	}
}

func TestImageType_Size(t *testing.T) {
	const gigaByte = 1024 * 1024 * 1024
	sizeMap := []struct {
		name       string
		inputSize  uint64
		outputSize uint64
	}{
		{
			name:       "ami",
			inputSize:  6*gigaByte + 1,
			outputSize: 6*gigaByte + 1,
		},
		{
			name:       "ami",
			inputSize:  0,
			outputSize: 6 * gigaByte,
		},
		{
			name:       "vhd",
			inputSize:  10 * gigaByte,
			outputSize: 10 * gigaByte,
		},
		{
			name:       "vhd",
			inputSize:  10*gigaByte - 1,
			outputSize: 10 * gigaByte,
		},
	}

	distro := rhel84.New()
	arch, err := distro.GetArch("x86_64")
	if assert.NoError(t, err) {
		for _, mapping := range sizeMap {
			imgType, err := arch.GetImageType(mapping.name)
			if assert.NoError(t, err) {
				size := imgType.Size(mapping.inputSize)
				assert.Equalf(t, mapping.outputSize, size, "Image type: %s, input size: %d, expected: %d, got: %d",
					mapping.name, mapping.inputSize, mapping.outputSize, size)
			}
		}
	}
}

func TestImageType_BasePackages(t *testing.T) {
	pkgMaps := []struct {
		name               string
		basePackages       []string
		bootloaderPackages []string
		excludedPackages   []string
		bootable           bool
	}{
		{
			name: "ami",
			basePackages: []string{
				"checkpolicy",
				"chrony",
				"cloud-init",
				"cloud-init",
				"cloud-utils-growpart",
				"@core",
				"dhcp-client",
				"gdisk",
				"insights-client",
				"kernel",
				"langpacks-en",
				"net-tools",
				"NetworkManager",
				"redhat-release",
				"redhat-release-eula",
				"rng-tools",
				"rsync",
				"selinux-policy-targeted",
				"tar",
				"yum-utils",
			},
			bootloaderPackages: []string{
				"dracut-config-generic",
				"grub2-pc",
				"grub2-efi-x64",
				"shim-x64",
			},
			excludedPackages: []string{
				"aic94xx-firmware",
				"alsa-firmware",
				"alsa-lib",
				"alsa-tools-firmware",
				"biosdevname",
				"dracut-config-rescue",
				"firewalld",
				"iprutils",
				"ivtv-firmware",
				"iwl1000-firmware",
				"iwl100-firmware",
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
				"libertas-sd8686-firmware",
				"libertas-sd8787-firmware",
				"libertas-usb8388-firmware",
				"plymouth",

				// TODO this cannot be removed, because the kernel (?)
				// depends on it. The ec2 kickstart force-removes it.
				// "linux-firmware",

				// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
				// https://errata.devel.redhat.com/advisory/47339 lands
				"timedatex",
			},
			bootable: true,
		},
		{
			name: "openstack",
			basePackages: []string{
				// Defaults
				"@Core",
				"langpacks-en",
				// From the lorax kickstart
				"kernel",
				"selinux-policy-targeted",
				"cloud-init",
				"qemu-guest-agent",
				"spice-vdagent",
			},
			bootloaderPackages: []string{
				"dracut-config-generic",
				"grub2-pc",
				"grub2-efi-x64",
				"shim-x64",
			},
			excludedPackages: []string{
				"dracut-config-rescue",
			},
			bootable: true,
		},
	}
	distro := rhel84.New()
	arch, err := distro.GetArch("x86_64")
	assert.NoError(t, err)

	for _, pkgMap := range pkgMaps {
		imgType, err := arch.GetImageType(pkgMap.name)
		assert.NoError(t, err)
		basePackages, excludedPackages := imgType.Packages(blueprint.Blueprint{})
		assert.Equalf(
			t,
			append(pkgMap.basePackages, pkgMap.bootloaderPackages...),
			basePackages,
			"image type: %s",
			pkgMap.name,
		)
		assert.Equalf(t, pkgMap.excludedPackages, excludedPackages, "image type: %s", pkgMap.name)
	}
}

func TestDistro_Manifest(t *testing.T) {
	distro_test_common.TestDistro_Manifest(t, "../../../test/data/manifests/", "rhel_84*", rhel84.New())
}

func TestRhel84_ListArches(t *testing.T) {
	distro := rhel84.New()
	arches := distro.ListArches()
	assert.Equal(t, []string{"aarch64", "ppc64le", "s390x", "x86_64"}, arches)
}

func TestRhel84_GetArch(t *testing.T) {
	distro := rhel84.New()
	arches := []struct {
		name          string
		errorExpected bool
	}{
		{
			name: "x86_64",
		},
		{
			name: "aarch64",
		},
		{
			name: "ppc64le",
		},
		{
			name: "s390x",
		},
		{
			name:          "foo-arch",
			errorExpected: true,
		},
	}

	for _, a := range arches {
		actualArch, err := distro.GetArch(a.name)
		if !a.errorExpected {
			assert.Equal(t, a.name, actualArch.Name())
			assert.NoError(t, err)
		} else {
			assert.Nil(t, actualArch)
			assert.Error(t, err)
		}
	}
}

func TestRhel84_Name(t *testing.T) {
	distro := rhel84.New()
	assert.Equal(t, "rhel-84", distro.Name())
}

func TestRhel84_ModulePlatformID(t *testing.T) {
	distro := rhel84.New()
	assert.Equal(t, "platform:el8", distro.ModulePlatformID())
}
