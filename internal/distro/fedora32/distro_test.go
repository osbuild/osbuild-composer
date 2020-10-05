package fedora32_test

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
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
			dist := fedora32.New()
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
		"x86_64":  x8664BuildPackages,
		"aarch64": aarch64BuildPackages,
	}
	d := fedora32.New()
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
			if itLabel == "fedora-iot-commit" {
				// For now we only include rpm-ostree when building fedora-iot-commit image types, this we may want
				// to reconsider. The only reason to specia-case it is that it might pull in a lot of dependencies
				// for a niche usecase.
				assert.ElementsMatch(t, append(buildPackages[archLabel], "rpm-ostree"), itStruct.BuildPackages())
			} else {
				assert.ElementsMatch(t, buildPackages[archLabel], itStruct.BuildPackages())
			}
		}
	}
}

func TestImageType_Name(t *testing.T) {
	distro := fedora32.New()
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

	distro := fedora32.New()
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
				"@Core",
				"chrony",
				"kernel",
				"selinux-policy-targeted",
				"langpacks-en",
				"libxcrypt-compat",
				"xfsprogs",
				"cloud-init",
				"checkpolicy",
				"net-tools",
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
		{
			name: "openstack",
			basePackages: []string{
				"@Core",
				"chrony",
				"kernel",
				"selinux-policy-targeted",
				"spice-vdagent",
				"qemu-guest-agent",
				"xen-libs",
				"langpacks-en",
				"cloud-init",
				"libdrm",
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
	distro := fedora32.New()
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
	distro_test_common.TestDistro_Manifest(t, "../../../test/data/cases/", "fedora_32*", fedora32.New())
}

func TestFedora32_ListArches(t *testing.T) {
	distro := fedora32.New()
	arches := distro.ListArches()
	assert.Equal(t, []string{"aarch64", "x86_64"}, arches)
}

func TestFedora32_GetArch(t *testing.T) {
	distro := fedora32.New()
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

func TestFedora32_Name(t *testing.T) {
	distro := fedora32.New()
	assert.Equal(t, "fedora-32", distro.Name())
}

func TestFedora32_ModulePlatformID(t *testing.T) {
	distro := fedora32.New()
	assert.Equal(t, "platform:f32", distro.ModulePlatformID())
}
