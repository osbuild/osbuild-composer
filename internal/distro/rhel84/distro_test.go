package rhel84_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel84"
)

type rhelFamilyDistro struct {
	name   string
	distro distro.Distro
}

var rhelFamilyDistros = []rhelFamilyDistro{
	{
		name:   "rhel",
		distro: rhel84.New(),
	},
	{
		name:   "centos",
		distro: rhel84.NewCentos(),
	},
}

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
			name:  "gce",
			args:  args{"gce"},
			want:  "image.tar.gz",
			want1: "application/gzip",
		},
		{
			name:  "gce-rhui",
			args:  args{"gce-rhui"},
			want:  "image.tar.gz",
			want1: "application/gzip",
		},
		{
			name:    "invalid-output-type",
			args:    args{"foobar"},
			wantErr: true,
		},
	}
	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					dist := dist.distro
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
	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			d := dist.distro
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
		})
	}
}

func TestImageType_Name(t *testing.T) {
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
				"rhel-edge-commit",
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
				"rhel-edge-commit",
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
				"qcow2",
				"tar",
			},
		},
	}

	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, mapping := range imgMap {
				if mapping.arch == distro.S390xArchName && dist.name == "centos" {
					continue
				}
				arch, err := dist.distro.GetArch(mapping.arch)
				if assert.NoError(t, err) {
					for _, imgName := range mapping.imgNames {
						if imgName == "rhel-edge-commit" && dist.name == "centos" {
							continue
						}
						imgType, err := arch.GetImageType(imgName)
						if assert.NoError(t, err) {
							assert.Equalf(t, imgName, imgType.Name(), "arch: %s", mapping.arch)
						}
					}
				}
			}
		})
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

	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			arch, err := dist.distro.GetArch("x86_64")
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
		})
	}
}

func TestImageType_BasePackages(t *testing.T) {
	pkgMaps := []struct {
		name                 string
		basePackages         []string
		bootloaderPackages   []string
		excludedPackages     []string
		bootable             bool
		rhelOnlyBasePackages []string
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
				"langpacks-en",
				"net-tools",
				"NetworkManager",
				"redhat-release",
				"redhat-release-eula",
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
				"rng-tools",

				// TODO this cannot be removed, because the kernel (?)
				// depends on it. The ec2 kickstart force-removes it.
				// "linux-firmware",

				// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
				// https://errata.devel.redhat.com/advisory/47339 lands
				"timedatex",
			},
			bootable: true,
			rhelOnlyBasePackages: []string{
				"insights-client",
			},
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
				"grub2-efi-x64",
				"shim-x64",
			},
			excludedPackages: []string{
				"dracut-config-rescue",
				"rng-tools",
			},
			bootable: true,
		},
	}

	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			arch, err := dist.distro.GetArch("x86_64")
			assert.NoError(t, err)

			for _, pkgMap := range pkgMaps {
				imgType, err := arch.GetImageType(pkgMap.name)
				assert.NoError(t, err)
				packages := imgType.PackageSets(blueprint.Blueprint{}, nil)["packages"]
				assert.NotNil(t, packages)
				assert.Len(t, packages, 1)
				expectedPackages := append(pkgMap.basePackages, pkgMap.bootloaderPackages...)
				if dist.name == "rhel" {
					expectedPackages = append(expectedPackages, pkgMap.rhelOnlyBasePackages...)
				}
				assert.ElementsMatchf(
					t,
					expectedPackages,
					packages[0].Include,
					"image type: %s",
					pkgMap.name,
				)
				assert.Equalf(t, pkgMap.excludedPackages, packages[0].Exclude, "image type: %s", pkgMap.name)
			}
		})
	}
}

// Check that Manifest() function returns an error for unsupported
// configurations.
func TestDistro_ManifestError(t *testing.T) {
	// Currently, the only unsupported configuration is OSTree commit types
	// with Kernel boot options
	r8distro := rhel84.New()
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
			if archName == distro.S390xArchName && imgTypeName == "tar" {
				// broken arch-imgType combination; see
				// https://github.com/osbuild/osbuild-composer/issues/1220
				continue
			}
			imgType, _ := arch.GetImageType(imgTypeName)
			imgOpts := distro.ImageOptions{
				Size: imgType.Size(0),
			}
			testPackageSpecSets := distro_test_common.GetTestingPackageSpecSets("kernel", arch.Name(), imgType.PayloadPackageSets())
			_, err := imgType.Manifest(bp.Customizations, imgOpts, nil, testPackageSpecSets, 0)
			if imgTypeName == "rhel-edge-commit" || imgTypeName == "rhel-edge-container" {
				assert.EqualError(t, err, "kernel boot parameter customizations are not supported for ostree types")
			} else if imgTypeName == "rhel-edge-installer" {
				assert.EqualError(t, err, "boot ISO image type \"rhel-edge-installer\" requires specifying a URL from which to retrieve the OSTree commit")
			} else {
				assert.NoError(t, err)
			}
		}
	}
}

func TestArchitecture_ListImageTypes(t *testing.T) {
	imgMap := []struct {
		arch                     string
		imgNames                 []string
		rhelAdditionalImageTypes []string
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
				"gce",
				"gce-rhui",
			},
			rhelAdditionalImageTypes: []string{"rhel-edge-commit", "rhel-edge-container", "rhel-edge-installer"},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"ami",
				"qcow2",
				"openstack",
				"tar",
			},
			rhelAdditionalImageTypes: []string{"rhel-edge-commit", "rhel-edge-container"},
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
				"qcow2",
				"tar",
			},
		},
	}

	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, mapping := range imgMap {
				if mapping.arch == distro.S390xArchName && dist.name == "centos" {
					continue
				}
				arch, err := dist.distro.GetArch(mapping.arch)
				require.NoError(t, err)
				imageTypes := arch.ListImageTypes()

				var expectedImageTypes []string
				expectedImageTypes = append(expectedImageTypes, mapping.imgNames...)
				if dist.name == "rhel" {
					expectedImageTypes = append(expectedImageTypes, mapping.rhelAdditionalImageTypes...)
				}

				require.ElementsMatch(t, expectedImageTypes, imageTypes)
			}
		})
	}
}

func TestRhel84_ListArches(t *testing.T) {
	arches := rhel84.New().ListArches()
	assert.Equal(t, []string{"aarch64", "ppc64le", "s390x", "x86_64"}, arches)
}

func TestCentos_ListArches(t *testing.T) {
	arches := rhel84.NewCentos().ListArches()
	assert.Equal(t, []string{"aarch64", "ppc64le", "x86_64"}, arches)
}

func TestRhel84_GetArch(t *testing.T) {
	arches := []struct {
		name                  string
		errorExpected         bool
		errorExpectedInCentos bool
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
			name:                  "s390x",
			errorExpectedInCentos: true,
		},
		{
			name:          "foo-arch",
			errorExpected: true,
		},
	}

	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, a := range arches {
				actualArch, err := dist.distro.GetArch(a.name)
				if a.errorExpected || (a.errorExpectedInCentos && dist.name == "centos") {
					assert.Nil(t, actualArch)
					assert.Error(t, err)
				} else {
					assert.Equal(t, a.name, actualArch.Name())
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestRhel84_Name(t *testing.T) {
	distro := rhel84.New()
	assert.Equal(t, "rhel-84", distro.Name())
}

func TestCentos_Name(t *testing.T) {
	distro := rhel84.NewCentos()
	assert.Equal(t, "centos-8", distro.Name())
}

func TestRhel84_ModulePlatformID(t *testing.T) {
	distro := rhel84.New()
	assert.Equal(t, "platform:el8", distro.ModulePlatformID())

	centos := rhel84.NewCentos()
	assert.Equal(t, "platform:el8", centos.ModulePlatformID())
}

func TestRhel84_KernelOption(t *testing.T) {
	distro_test_common.TestDistro_KernelOption(t, rhel84.New())
}

func TestDistro_CustomFileSystemManifestError(t *testing.T) {
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
	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			d := dist.distro
			for _, archName := range d.ListArches() {
				arch, _ := d.GetArch(archName)
				for _, imgTypeName := range arch.ListImageTypes() {
					if (archName == distro.S390xArchName && imgTypeName == "tar") || imgTypeName == "rhel-edge-installer" {
						continue
					}
					imgType, _ := arch.GetImageType(imgTypeName)
					imgOpts := distro.ImageOptions{
						Size: imgType.Size(0),
					}
					testPackageSpecSets := distro_test_common.GetTestingPackageSpecSets("kernel", arch.Name(), imgType.PayloadPackageSets())
					_, err := imgType.Manifest(bp.Customizations, imgOpts, nil, testPackageSpecSets, 0)
					if imgTypeName == "rhel-edge-commit" || imgTypeName == "rhel-edge-container" {
						assert.EqualError(t, err, "Custom mountpoints are not supported for ostree types")
					} else {
						assert.EqualError(t, err, "The following custom mountpoints are not supported [\"/boot\"]")
					}
				}
			}
		})
	}
}

func TestDistro_TestRootMountPoint(t *testing.T) {
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
	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			d := dist.distro
			for _, archName := range d.ListArches() {
				arch, _ := d.GetArch(archName)
				for _, imgTypeName := range arch.ListImageTypes() {
					if (archName == distro.S390xArchName && imgTypeName == "tar") || imgTypeName == "rhel-edge-installer" {
						continue
					}
					imgType, _ := arch.GetImageType(imgTypeName)
					imgOpts := distro.ImageOptions{
						Size: imgType.Size(0),
					}
					testPackageSpecSets := distro_test_common.GetTestingPackageSpecSets("kernel", arch.Name(), imgType.PayloadPackageSets())
					_, err := imgType.Manifest(bp.Customizations, imgOpts, nil, testPackageSpecSets, 0)
					if imgTypeName == "rhel-edge-commit" || imgTypeName == "rhel-edge-container" {
						assert.EqualError(t, err, "Custom mountpoints are not supported for ostree types")
					} else {
						assert.NoError(t, err)
					}
				}
			}
		})
	}
}
