package rhel90_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel90"
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
			name:  "qcow2",
			args:  args{"qcow2"},
			want:  "disk.qcow2",
			want1: "application/x-qemu-disk",
		},
		{
			name:    "invalid-output-type",
			args:    args{"foobar"},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist := rhel90.New()
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
	d := rhel90.New()
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
			buildPkgs := itStruct.PackageSets(blueprint.Blueprint{})["build-packages"]
			assert.NotNil(t, buildPkgs)
			assert.ElementsMatch(t, buildPackages[archLabel], buildPkgs.Include)
		}
	}
}

func TestImageType_Name(t *testing.T) {
	distro := rhel90.New()
	imgMap := []struct {
		arch     string
		imgNames []string
	}{
		{
			arch: "x86_64",
			imgNames: []string{
				"qcow2",
			},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"qcow2",
			},
		},
		{
			arch: "ppc64le",
			imgNames: []string{
				"qcow2",
			},
		},
		{
			arch: "s390x",
			imgNames: []string{
				"qcow2",
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

func TestImageType_BasePackages(t *testing.T) {
	pkgMaps := []struct {
		name               string
		basePackages       []string
		bootloaderPackages []string
		excludedPackages   []string
		bootable           bool
	}{
		{
			name: "qcow2",
			basePackages: []string{
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
				"qemu-guest-agent",
				"redhat-release",
				"redhat-release-eula",
				"rsync",
				"subscription-manager-cockpit",
				"tar",
				"tcpdump",
				"yum",
				"kernel",
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
			bootable: true,
		},
	}
	distro := rhel90.New()
	arch, err := distro.GetArch("x86_64")
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
	r9distro := rhel90.New()
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Kernel: &blueprint.KernelCustomization{
				Append: "debug",
			},
		},
	}
	for _, archName := range r9distro.ListArches() {
		arch, _ := r9distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, 0)
			assert.NoError(t, err)
		}
	}
}

func TestArchitecture_ListImageTypes(t *testing.T) {
	distro := rhel90.New()
	imgMap := []struct {
		arch     string
		imgNames []string
	}{
		{
			arch: "x86_64",
			imgNames: []string{
				"qcow2",
			},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"qcow2",
			},
		},
		{
			arch: "ppc64le",
			imgNames: []string{
				"qcow2",
			},
		},
		{
			arch: "s390x",
			imgNames: []string{
				"qcow2",
			},
		},
	}

	for _, mapping := range imgMap {
		arch, err := distro.GetArch(mapping.arch)
		require.NoError(t, err)
		imageTypes := arch.ListImageTypes()

		var expectedImageTypes []string
		expectedImageTypes = append(expectedImageTypes, mapping.imgNames...)

		require.ElementsMatch(t, expectedImageTypes, imageTypes)
	}
}

func TestRhel90_ListArches(t *testing.T) {
	arches := rhel90.New().ListArches()
	assert.Equal(t, []string{"aarch64", "ppc64le", "s390x", "x86_64"}, arches)
}

func TestRhel90_GetArch(t *testing.T) {
	distro := rhel90.New()
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
		if a.errorExpected {
			assert.Nil(t, actualArch)
			assert.Error(t, err)
		} else {
			assert.Equal(t, a.name, actualArch.Name())
			assert.NoError(t, err)
		}
	}
}

func TestRhel90_Name(t *testing.T) {
	distro := rhel90.New()
	assert.Equal(t, "rhel-90", distro.Name())
}

func TestRhel84_ModulePlatformID(t *testing.T) {
	distro := rhel90.New()
	assert.Equal(t, "platform:el9", distro.ModulePlatformID())
}

func TestRhel90_KernelOption(t *testing.T) {
	distro_test_common.TestDistro_KernelOption(t, rhel90.New())
}

func TestDistro_CustomFileSystemManifestError(t *testing.T) {
	r9distro := rhel90.New()
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
	for _, archName := range r9distro.ListArches() {
		arch, _ := r9distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, 0)
			assert.EqualError(t, err, "The following custom mountpoints are not supported [\"/boot\"]")
		}
	}
}

func TestDistro_TestRootMountPoint(t *testing.T) {
	r9distro := rhel90.New()
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
	for _, archName := range r9distro.ListArches() {
		arch, _ := r9distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, 0)
			assert.NoError(t, err)
		}
	}
}
