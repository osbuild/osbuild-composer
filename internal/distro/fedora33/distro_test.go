package fedora33_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	fedora "github.com/osbuild/osbuild-composer/internal/distro/fedora33"
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
			dist := fedora.NewF35()
			arch, _ := dist.GetArch(distro.X86_64ArchName)
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
		"selinux-policy-targeted",
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
		"selinux-policy-targeted",
		"systemd",
		"tar",
		"xz",
	}
	buildPackages := map[string][]string{
		distro.X86_64ArchName:  x8664BuildPackages,
		distro.Aarch64ArchName: aarch64BuildPackages,
	}
	d := fedora.NewF35()
	for _, archLabel := range d.ListArches() {
		archStruct, err := d.GetArch(archLabel)
		if err != nil {
			t.Errorf("d.GetArch(%v) returned err = %v; expected nil", archLabel, err)
			continue
		}
		for _, itLabel := range archStruct.ListImageTypes() {
			itStruct, err := archStruct.GetImageType(itLabel)
			if err != nil {
				t.Errorf("d.GetArch(%v) returned err = %v; expected nil", archLabel, err)
				continue
			}
			buildPkgs := itStruct.PackageSets(blueprint.Blueprint{})["build-packages"]
			assert.NotNil(t, buildPkgs)
			if itLabel == "fedora-iot-commit" {
				// For now we only include rpm-ostree when building fedora-iot-commit image types, this we may want
				// to reconsider. The only reason to specia-case it is that it might pull in a lot of dependencies
				// for a niche usecase.
				assert.ElementsMatch(t, append(buildPackages[archLabel], "rpm-ostree"), buildPkgs.Include)
			} else {
				assert.ElementsMatch(t, buildPackages[archLabel], buildPkgs.Include)
			}
		}
	}
}

func TestImageType_Name(t *testing.T) {
	f35 := fedora.NewF35()
	imgMap := []struct {
		arch     string
		imgNames []string
	}{
		{
			arch: distro.X86_64ArchName,
			imgNames: []string{
				"ami",
				"qcow2",
				"openstack",
				"vhd",
				"vmdk",
			},
		},
		{
			arch: distro.Aarch64ArchName,
			imgNames: []string{
				"ami",
				"qcow2",
				"openstack",
			},
		},
	}
	for _, mapping := range imgMap {
		arch, err := f35.GetArch(mapping.arch)
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

	f35 := fedora.NewF35()
	arch, err := f35.GetArch(distro.X86_64ArchName)
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
				"@Core",
				"chrony",
				"selinux-policy-targeted",
				"langpacks-en",
				"libxcrypt-compat",
				"xfsprogs",
				"cloud-init",
				"checkpolicy",
				"net-tools",

				// Default from Blueprint
				"kernel",
			},
			bootloaderPackages: []string{
				"dracut-config-generic",
				"grub2-pc",
			},
			excludedPackages: []string{
				"dracut-config-rescue",
				"geolite2-city",
				"geolite2-country",
				"zram-generator-defaults",
			},
			bootable: true,
		},
		{
			name: "openstack",
			basePackages: []string{
				"@Core",
				"chrony",
				"selinux-policy-targeted",
				"spice-vdagent",
				"qemu-guest-agent",
				"xen-libs",
				"langpacks-en",
				"cloud-init",
				"libdrm",

				// Default from Blueprint
				"kernel",
			},
			bootloaderPackages: []string{
				"dracut-config-generic",
				"grub2-pc",
			},
			excludedPackages: []string{
				"dracut-config-rescue",
				"geolite2-city",
				"geolite2-country",
				"zram-generator-defaults",
			},
			bootable: true,
		},
	}
	f35 := fedora.NewF35()
	arch, err := f35.GetArch(distro.X86_64ArchName)
	assert.NoError(t, err)

	for _, pkgMap := range pkgMaps {
		imgType, err := arch.GetImageType(pkgMap.name)
		assert.NoError(t, err)
		packages := imgType.PackageSets(blueprint.Blueprint{})["packages"]
		assert.NotNil(t, packages)
		assert.Equalf(
			t,
			append(pkgMap.basePackages, pkgMap.bootloaderPackages...),
			packages.Include,
			"image type: %s",
			pkgMap.name,
		)
		assert.Equalf(t, pkgMap.excludedPackages, packages.Exclude, "image type: %s", pkgMap.name)
	}
}

// Check that Manifest() function returns an error for unsupported
// configurations.
func TestDistro_ManifestError(t *testing.T) {
	// Currently, the only unsupported configuration is OSTree commit types
	// with Kernel boot options
	f35distro := fedora.NewF35()
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Kernel: &blueprint.KernelCustomization{
				Append: "debug",
			},
		},
	}

	for _, archName := range f35distro.ListArches() {
		arch, _ := f35distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, 0)
			if imgTypeName == "fedora-iot-commit" {
				assert.EqualError(t, err, "kernel boot parameter customizations are not supported for ostree types")
			} else {
				assert.NoError(t, err)
			}
		}
	}
}

func TestFedora35_ListArches(t *testing.T) {
	f35 := fedora.NewF35()
	arches := f35.ListArches()
	assert.Equal(t, []string{distro.Aarch64ArchName, distro.X86_64ArchName}, arches)
}

func TestFedora35_GetArch(t *testing.T) {
	f35 := fedora.NewF35()
	arches := []struct {
		name          string
		errorExpected bool
	}{
		{
			name: distro.X86_64ArchName,
		},
		{
			name: distro.Aarch64ArchName,
		},
		{
			name:          "foo-arch",
			errorExpected: true,
		},
	}

	for _, a := range arches {
		actualArch, err := f35.GetArch(a.name)
		if !a.errorExpected {
			assert.Equal(t, a.name, actualArch.Name())
			assert.NoError(t, err)
		} else {
			assert.Nil(t, actualArch)
			assert.Error(t, err)
		}
	}
}

func TestFedora35_Name(t *testing.T) {
	distro := fedora.NewF35()
	assert.Equal(t, "fedora-35", distro.Name())
}

func TestFedora35_ModulePlatformID(t *testing.T) {
	distro := fedora.NewF35()
	assert.Equal(t, "platform:f35", distro.ModulePlatformID())
}

func TestFedora35_OSTreeRef(t *testing.T) {
	f35 := fedora.NewF35()
	assert.Equal(t, "fedora/35/%s/iot", f35.OSTreeRef())

	x86_64, err := f35.GetArch(distro.X86_64ArchName)
	assert.Nilf(t, err, "failed to get %q architecture of %q distribution", distro.X86_64ArchName, f35.Name())
	ostreeImgName := "fedora-iot-commit"
	ostreeImg, err := x86_64.GetImageType(ostreeImgName)
	assert.Nilf(t, err, "failed to get %q image type for %q architecture of %q distribution", ostreeImgName, distro.X86_64ArchName, f35.Name())
	assert.Equal(t, "fedora/35/x86_64/iot", ostreeImg.OSTreeRef())
}

func TestFedora35_KernelOption(t *testing.T) {
	distro_test_common.TestDistro_KernelOption(t, fedora.NewF35())
}

func TestDistro_CustomFileSystemManifestError(t *testing.T) {
	f35distro := fedora.NewF35()
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
	for _, archName := range f35distro.ListArches() {
		arch, _ := f35distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, 0)
			if imgTypeName == "fedora-iot-commit" {
				assert.EqualError(t, err, "Custom mountpoints are not supported for ostree types")
			} else {
				assert.EqualError(t, err, "The following custom mountpoints are not supported [\"/boot\"]")
			}
		}
	}
}

func TestDistro_TestRootMountPoint(t *testing.T) {
	f35distro := fedora.NewF35()
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
	for _, archName := range f35distro.ListArches() {
		arch, _ := f35distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, 0)
			if imgTypeName == "fedora-iot-commit" {
				assert.EqualError(t, err, "Custom mountpoints are not supported for ostree types")
			} else {
				assert.NoError(t, err)
			}
		}
	}
}
