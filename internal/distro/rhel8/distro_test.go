package rhel8_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"
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
			dist := rhel8.New()
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
		"grub2-pc",
		"policycoreutils",
		"qemu-img",
		"systemd",
		"tar",
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
	d := rhel8.New()
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
			buildPkgs := itStruct.PackageSets(blueprint.Blueprint{}, nil)["build-packages"]
			assert.NotNil(t, buildPkgs)
			assert.Len(t, buildPkgs, 1)
			assert.ElementsMatch(t, buildPackages[archLabel], buildPkgs[0].Include)
		}
	}
}

func TestImageType_Name(t *testing.T) {
	distro := rhel8.New()
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

	distro := rhel8.New()
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

				// Default from Blueprint
				"kernel",
			},
			bootloaderPackages: []string{
				"dracut-config-generic",
				"grub2-pc",
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
				"selinux-policy-targeted",
				"cloud-init",
				"qemu-guest-agent",
				"spice-vdagent",

				// Default from Blueprint
				"kernel",
			},
			bootloaderPackages: []string{
				"dracut-config-generic",
				"grub2-pc",
			},
			excludedPackages: []string{
				"dracut-config-rescue",
			},
			bootable: true,
		},
	}
	distro := rhel8.New()
	arch, err := distro.GetArch("x86_64")
	assert.NoError(t, err)

	for _, pkgMap := range pkgMaps {
		imgType, err := arch.GetImageType(pkgMap.name)
		assert.NoError(t, err)
		packages := imgType.PackageSets(blueprint.Blueprint{}, nil)["packages"]
		assert.NotNil(t, packages)
		assert.Len(t, packages, 1)
		assert.Equalf(
			t,
			append(pkgMap.basePackages, pkgMap.bootloaderPackages...),
			packages[0].Include,
			"image type: %s",
			pkgMap.name,
		)
		assert.Equalf(t, pkgMap.excludedPackages, packages[0].Exclude, "image type: %s", pkgMap.name)
	}
}

// Check that Manifest() function returns an error for unsupported
// configurations.
func TestDistro_ManifestError(t *testing.T) {
	// Currently, the only unsupported configuration is OSTree commit types
	// with Kernel boot options
	r8distro := rhel8.New()
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Kernel: &blueprint.KernelCustomization{
				Append: "debug",
			},
		},
	}
	for _, archName := range r8distro.ListArches() {
		arch, _ := r8distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, 0)
			if imgTypeName == "rhel-edge-commit" {
				assert.EqualError(t, err, "kernel boot parameter customizations are not supported for ostree types")
			} else {
				assert.NoError(t, err)
			}
		}
	}
}
func TestDistro_CustomFileSystemManifestError(t *testing.T) {
	r8distro := rhel8.New()
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "/boot",
				},
			},
		},
	}
	for _, archName := range r8distro.ListArches() {
		arch, _ := r8distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, 0)
			if imgTypeName == "rhel-edge-commit" {
				assert.EqualError(t, err, "Custom mountpoints are not supported for ostree types")
			} else {
				assert.EqualError(t, err, "The following custom mountpoints are not supported [\"/boot\"]")
			}
		}
	}
}

func TestDistro_TestRootMountPoint(t *testing.T) {
	r8distro := rhel8.New()
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "/",
				},
			},
		},
	}
	for _, archName := range r8distro.ListArches() {
		arch, _ := r8distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, 0)
			if imgTypeName == "rhel-edge-commit" {
				assert.EqualError(t, err, "Custom mountpoints are not supported for ostree types")
			} else {
				assert.NoError(t, err)
			}
		}
	}
}

func TestRhel8_ListArches(t *testing.T) {
	distro := rhel8.New()
	arches := distro.ListArches()
	assert.Equal(t, []string{"aarch64", "ppc64le", "s390x", "x86_64"}, arches)
}

func TestRhel8_GetArch(t *testing.T) {
	distro := rhel8.New()
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

func TestRhel8_Name(t *testing.T) {
	distro := rhel8.New()
	assert.Equal(t, "rhel-8", distro.Name())
}

func TestRhel8_ModulePlatformID(t *testing.T) {
	distro := rhel8.New()
	assert.Equal(t, "platform:el8", distro.ModulePlatformID())
}

func TestRhel8_KernelOption(t *testing.T) {
	distro_test_common.TestDistro_KernelOption(t, rhel8.New())
}
