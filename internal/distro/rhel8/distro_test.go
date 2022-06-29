package rhel8_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"
)

type rhelFamilyDistro struct {
	name   string
	distro distro.Distro
}

var rhelFamilyDistros = []rhelFamilyDistro{
	{
		name:   "rhel",
		distro: rhel8.New(),
	},
}

func TestFilenameFromType(t *testing.T) {
	type args struct {
		outputFormat string
	}
	type wantResult struct {
		filename string
		mimeType string
		wantErr  bool
	}
	tests := []struct {
		name string
		args args
		want wantResult
	}{
		{
			name: "ami",
			args: args{"ami"},
			want: wantResult{
				filename: "image.raw",
				mimeType: "application/octet-stream",
			},
		},
		{
			name: "ec2",
			args: args{"ec2"},
			want: wantResult{
				filename: "image.raw.xz",
				mimeType: "application/xz",
			},
		},
		{
			name: "ec2-ha",
			args: args{"ec2-ha"},
			want: wantResult{
				filename: "image.raw.xz",
				mimeType: "application/xz",
			},
		},
		{
			name: "ec2-sap",
			args: args{"ec2-sap"},
			want: wantResult{
				filename: "image.raw.xz",
				mimeType: "application/xz",
			},
		},
		{
			name: "qcow2",
			args: args{"qcow2"},
			want: wantResult{
				filename: "disk.qcow2",
				mimeType: "application/x-qemu-disk",
			},
		},
		{
			name: "openstack",
			args: args{"openstack"},
			want: wantResult{
				filename: "disk.qcow2",
				mimeType: "application/x-qemu-disk",
			},
		},
		{
			name: "vhd",
			args: args{"vhd"},
			want: wantResult{
				filename: "disk.vhd",
				mimeType: "application/x-vhd",
			},
		},
		{
			name: "azure-rhui",
			args: args{"azure-rhui"},
			want: wantResult{
				filename: "disk.vhd.xz",
				mimeType: "application/xz",
			},
		},
		{
			name: "vmdk",
			args: args{"vmdk"},
			want: wantResult{
				filename: "disk.vmdk",
				mimeType: "application/x-vmdk",
			},
		},
		{
			name: "tar",
			args: args{"tar"},
			want: wantResult{
				filename: "root.tar.xz",
				mimeType: "application/x-tar",
			},
		},
		{
			name: "image-installer",
			args: args{"image-installer"},
			want: wantResult{
				filename: "installer.iso",
				mimeType: "application/x-iso9660-image",
			},
		},
		{
			name: "edge-commit",
			args: args{"edge-commit"},
			want: wantResult{
				filename: "commit.tar",
				mimeType: "application/x-tar",
			},
		},
		// Alias
		{
			name: "rhel-edge-commit",
			args: args{"rhel-edge-commit"},
			want: wantResult{
				filename: "commit.tar",
				mimeType: "application/x-tar",
			},
		},
		{
			name: "edge-container",
			args: args{"edge-container"},
			want: wantResult{
				filename: "container.tar",
				mimeType: "application/x-tar",
			},
		},
		// Alias
		{
			name: "rhel-edge-container",
			args: args{"rhel-edge-container"},
			want: wantResult{
				filename: "container.tar",
				mimeType: "application/x-tar",
			},
		},
		{
			name: "edge-installer",
			args: args{"edge-installer"},
			want: wantResult{
				filename: "installer.iso",
				mimeType: "application/x-iso9660-image",
			},
		},
		// Alias
		{
			name: "rhel-edge-installer",
			args: args{"rhel-edge-installer"},
			want: wantResult{
				filename: "installer.iso",
				mimeType: "application/x-iso9660-image",
			},
		},
		{
			name: "gce",
			args: args{"gce"},
			want: wantResult{
				filename: "image.tar.gz",
				mimeType: "application/gzip",
			},
		},
		{
			name: "gce-rhui",
			args: args{"gce-rhui"},
			want: wantResult{
				filename: "image.tar.gz",
				mimeType: "application/gzip",
			},
		},
		{
			name: "invalid-output-type",
			args: args{"foobar"},
			want: wantResult{wantErr: true},
		},
	}
	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					dist := dist.distro
					arch, _ := dist.GetArch("x86_64")
					imgType, err := arch.GetImageType(tt.args.outputFormat)
					if (err != nil) != tt.want.wantErr {
						t.Errorf("Arch.GetImageType() error = %v, wantErr %v", err, tt.want.wantErr)
						return
					}
					if !tt.want.wantErr {
						gotFilename := imgType.Filename()
						gotMIMEType := imgType.MIMEType()
						if gotFilename != tt.want.filename {
							t.Errorf("ImageType.Filename()  got = %v, want %v", gotFilename, tt.want.filename)
						}
						if gotMIMEType != tt.want.mimeType {
							t.Errorf("ImageType.MIMEType() got1 = %v, want %v", gotMIMEType, tt.want.mimeType)
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
					buildPkgs := itStruct.PackageSets(blueprint.Blueprint{}, distro.ImageOptions{}, nil)["build"]
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
				"qcow2",
				"openstack",
				"vhd",
				"azure-rhui",
				"vmdk",
				"ami",
				"ec2",
				"ec2-ha",
				"ec2-sap",
				"gce",
				"gce-rhui",
				"edge-commit",
				"edge-container",
				"edge-installer",
				"tar",
				"image-installer",
			},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"qcow2",
				"openstack",
				"ami",
				"ec2",
				"edge-commit",
				"edge-container",
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
						if imgName == "edge-commit" && dist.name == "centos" {
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

func TestImageTypeAliases(t *testing.T) {
	type args struct {
		imageTypeAliases []string
	}
	type wantResult struct {
		imageTypeName string
	}
	tests := []struct {
		name string
		args args
		want wantResult
	}{
		{
			name: "edge-commit aliases",
			args: args{
				imageTypeAliases: []string{"rhel-edge-commit"},
			},
			want: wantResult{
				imageTypeName: "edge-commit",
			},
		},
		{
			name: "edge-container aliases",
			args: args{
				imageTypeAliases: []string{"rhel-edge-container"},
			},
			want: wantResult{
				imageTypeName: "edge-container",
			},
		},
		{
			name: "edge-installer aliases",
			args: args{
				imageTypeAliases: []string{"rhel-edge-installer"},
			},
			want: wantResult{
				imageTypeName: "edge-installer",
			},
		},
	}
	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					dist := dist.distro
					for _, archName := range dist.ListArches() {
						t.Run(archName, func(t *testing.T) {
							arch, err := dist.GetArch(archName)
							require.Nilf(t, err,
								"failed to get architecture '%s', previously listed as supported for the distro '%s'",
								archName, dist.Name())
							// Test image type aliases only if the aliased image type is supported for the arch
							if _, err = arch.GetImageType(tt.want.imageTypeName); err != nil {
								t.Skipf("aliased image type '%s' is not supported for architecture '%s'",
									tt.want.imageTypeName, archName)
							}
							for _, alias := range tt.args.imageTypeAliases {
								t.Run(fmt.Sprintf("'%s' alias for image type '%s'", alias, tt.want.imageTypeName),
									func(t *testing.T) {
										gotImage, err := arch.GetImageType(alias)
										require.Nilf(t, err, "arch.GetImageType() for image type alias '%s' failed: %v",
											alias, err)
										assert.Equalf(t, tt.want.imageTypeName, gotImage.Name(),
											"got unexpected image type name for alias '%s'. got = %s, want = %s",
											alias, tt.want.imageTypeName, gotImage.Name())
									})
							}
						})
					}
				})
			}
		})
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
			imgOpts := distro.ImageOptions{
				Size: imgType.Size(0),
			}
			testPackageSpecSets := distro_test_common.GetTestingImagePackageSpecSets("kernel", imgType)
			_, err := imgType.Manifest(bp.Customizations, imgOpts, nil, testPackageSpecSets, 0)
			if imgTypeName == "edge-commit" || imgTypeName == "edge-container" {
				assert.EqualError(t, err, "kernel boot parameter customizations are not supported for ostree types")
			} else if imgTypeName == "edge-raw-image" {
				assert.EqualError(t, err, "edge raw images require specifying a URL from which to retrieve the OSTree commit")
			} else if imgTypeName == "edge-installer" || imgTypeName == "edge-simplified-installer" {
				assert.EqualError(t, err, fmt.Sprintf("boot ISO image type \"%s\" requires specifying a URL from which to retrieve the OSTree commit", imgTypeName))
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
				"qcow2",
				"openstack",
				"vhd",
				"azure-rhui",
				"vmdk",
				"ami",
				"ec2",
				"ec2-ha",
				"ec2-sap",
				"gce",
				"gce-rhui",
				"edge-commit",
				"edge-container",
				"edge-installer",
				"edge-raw-image",
				"edge-simplified-installer",
				"tar",
				"image-installer",
				"oci",
			},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"qcow2",
				"openstack",
				"ami",
				"ec2",
				"edge-commit",
				"edge-container",
				"edge-installer",
				"edge-simplified-installer",
				"edge-raw-image",
				"tar",
				"image-installer",
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

func TestRHEL8_ListArches(t *testing.T) {
	arches := rhel8.New().ListArches()
	assert.Equal(t, []string{"aarch64", "ppc64le", "s390x", "x86_64"}, arches)
}

func TestRHEL8_GetArch(t *testing.T) {
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
			name: "s390x",
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

func TestRhel8_Name(t *testing.T) {
	distro := rhel8.New()
	assert.Equal(t, "rhel-8", distro.Name())
}

func TestRhel8_ModulePlatformID(t *testing.T) {
	distro := rhel8.New()
	assert.Equal(t, "platform:el8", distro.ModulePlatformID())
}

func TestRhel86_KernelOption(t *testing.T) {
	distro_test_common.TestDistro_KernelOption(t, rhel8.New())
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
			if imgTypeName == "edge-commit" || imgTypeName == "edge-container" {
				assert.EqualError(t, err, "Custom mountpoints are not supported for ostree types")
			} else if imgTypeName == "edge-installer" || imgTypeName == "edge-simplified-installer" || imgTypeName == "edge-raw-image" {
				continue
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
			testPackageSpecSets := distro_test_common.GetTestingImagePackageSpecSets("kernel", imgType)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, testPackageSpecSets, 0)
			if imgTypeName == "edge-commit" || imgTypeName == "edge-container" {
				assert.EqualError(t, err, "Custom mountpoints are not supported for ostree types")
			} else if imgTypeName == "edge-installer" || imgTypeName == "edge-simplified-installer" || imgTypeName == "edge-raw-image" {
				continue
			} else {
				assert.NoError(t, err)
			}
		}
	}
}

func TestDistro_CustomFileSystemSubDirectories(t *testing.T) {
	r8distro := rhel8.New()
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "/var/log",
				},
				{
					MinSize:    1024,
					Mountpoint: "/var/log/audit",
				},
			},
		},
	}
	for _, archName := range r8distro.ListArches() {
		arch, _ := r8distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			testPackageSpecSets := distro_test_common.GetTestingImagePackageSpecSets("kernel", imgType)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, testPackageSpecSets, 0)
			if strings.HasPrefix(imgTypeName, "edge-") {
				continue
			} else {
				assert.NoError(t, err)
			}
		}
	}
}

func TestDistro_MountpointsWithArbitraryDepthAllowed(t *testing.T) {
	r8distro := rhel8.New()
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "/var/a",
				},
				{
					MinSize:    1024,
					Mountpoint: "/var/a/b",
				},
				{
					MinSize:    1024,
					Mountpoint: "/var/a/b/c",
				},
				{
					MinSize:    1024,
					Mountpoint: "/var/a/b/c/d",
				},
			},
		},
	}
	for _, archName := range r8distro.ListArches() {
		arch, _ := r8distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			testPackageSpecSets := distro_test_common.GetTestingImagePackageSpecSets("kernel", imgType)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, testPackageSpecSets, 0)
			if strings.HasPrefix(imgTypeName, "edge-") {
				continue
			} else {
				assert.NoError(t, err)
			}
		}
	}
}

func TestDistro_DirtyMountpointsNotAllowed(t *testing.T) {
	r8distro := rhel8.New()
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "//",
				},
				{
					MinSize:    1024,
					Mountpoint: "/var//",
				},
				{
					MinSize:    1024,
					Mountpoint: "/var//log/audit/",
				},
			},
		},
	}
	for _, archName := range r8distro.ListArches() {
		arch, _ := r8distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, 0)
			if strings.HasPrefix(imgTypeName, "edge-") {
				continue
			} else {
				assert.EqualError(t, err, "The following custom mountpoints are not supported [\"//\" \"/var//\" \"/var//log/audit/\"]")
			}
		}
	}
}

func TestDistro_CustomFileSystemPatternMatching(t *testing.T) {
	r8distro := rhel8.New()
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "/variable",
				},
				{
					MinSize:    1024,
					Mountpoint: "/variable/log/audit",
				},
			},
		},
	}
	for _, archName := range r8distro.ListArches() {
		arch, _ := r8distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, 0)
			if imgTypeName == "edge-commit" || imgTypeName == "edge-container" {
				assert.EqualError(t, err, "Custom mountpoints are not supported for ostree types")
			} else if imgTypeName == "edge-installer" || imgTypeName == "edge-simplified-installer" || imgTypeName == "edge-raw-image" {
				continue
			} else {
				assert.EqualError(t, err, "The following custom mountpoints are not supported [\"/variable\" \"/variable/log/audit\"]")
			}
		}
	}
}

func TestDistro_CustomUsrPartitionNotLargeEnough(t *testing.T) {
	r8distro := rhel8.New()
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "/usr",
				},
			},
		},
	}
	for _, archName := range r8distro.ListArches() {
		arch, _ := r8distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			testPackageSpecSets := distro_test_common.GetTestingImagePackageSpecSets("kernel", imgType)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, testPackageSpecSets, 0)
			if imgTypeName == "edge-commit" || imgTypeName == "edge-container" {
				assert.EqualError(t, err, "Custom mountpoints are not supported for ostree types")
			} else if imgTypeName == "edge-installer" || imgTypeName == "edge-simplified-installer" || imgTypeName == "edge-raw-image" {
				continue
			} else {
				assert.NoError(t, err)
			}
		}
	}
}
