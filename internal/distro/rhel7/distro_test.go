package rhel7_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel7"
)

type rhelFamilyDistro struct {
	name   string
	distro distro.Distro
}

var rhelFamilyDistros = []rhelFamilyDistro{
	{
		name:   "rhel",
		distro: rhel7.New(),
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
			name: "qcow2",
			args: args{"qcow2"},
			want: wantResult{
				filename: "disk.qcow2",
				mimeType: "application/x-qemu-disk",
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
	buildPackages := map[string][]string{
		"x86_64": x8664BuildPackages,
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
				"azure-rhui",
			},
		},
	}

	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, mapping := range imgMap {
				arch, err := dist.distro.GetArch(mapping.arch)
				if assert.NoError(t, err) {
					for _, imgName := range mapping.imgNames {
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

// Check that Manifest() function returns an error for unsupported
// configurations.
func TestDistro_ManifestError(t *testing.T) {
	r7distro := rhel7.New()
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Kernel: &blueprint.KernelCustomization{
				Append: "debug",
			},
		},
	}

	for _, archName := range r7distro.ListArches() {
		arch, _ := r7distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			imgOpts := distro.ImageOptions{
				Size: imgType.Size(0),
			}
			testPackageSpecSets := distro_test_common.GetTestingImagePackageSpecSets("kernel", imgType)
			_, err := imgType.Manifest(bp.Customizations, imgOpts, nil, testPackageSpecSets, nil, 0)
			assert.NoError(t, err)
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
				"azure-rhui",
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

func TestRhel7_ListArches(t *testing.T) {
	arches := rhel7.New().ListArches()
	assert.Equal(t, []string{"x86_64"}, arches)
}

func TestRhel7_GetArch(t *testing.T) {
	arches := []struct {
		name                  string
		errorExpected         bool
		errorExpectedInCentos bool
	}{
		{
			name: "x86_64",
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

func TestRhel7_Name(t *testing.T) {
	distro := rhel7.New()
	assert.Equal(t, "rhel-7", distro.Name())
}

func TestRhel7_ModulePlatformID(t *testing.T) {
	distro := rhel7.New()
	assert.Equal(t, "platform:el7", distro.ModulePlatformID())
}

func TestRhel7_KernelOption(t *testing.T) {
	distro_test_common.TestDistro_KernelOption(t, rhel7.New())
}

func TestDistro_CustomFileSystemManifestError(t *testing.T) {
	r7distro := rhel7.New()
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
	for _, archName := range r7distro.ListArches() {
		arch, _ := r7distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, nil, 0)
			assert.EqualError(t, err, "The following custom mountpoints are not supported [\"/boot\"]")
		}
	}
}

func TestDistro_TestRootMountPoint(t *testing.T) {
	r7distro := rhel7.New()
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
	for _, archName := range r7distro.ListArches() {
		arch, _ := r7distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			testPackageSpecSets := distro_test_common.GetTestingImagePackageSpecSets("kernel", imgType)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, testPackageSpecSets, nil, 0)
			assert.NoError(t, err)
		}
	}
}

func TestDistro_CustomFileSystemSubDirectories(t *testing.T) {
	r7distro := rhel7.New()
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
	for _, archName := range r7distro.ListArches() {
		arch, _ := r7distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			testPackageSpecSets := distro_test_common.GetTestingImagePackageSpecSets("kernel", imgType)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, testPackageSpecSets, nil, 0)
			assert.NoError(t, err)
		}
	}
}

func TestDistro_MountpointsWithArbitraryDepthAllowed(t *testing.T) {
	r7distro := rhel7.New()
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
	for _, archName := range r7distro.ListArches() {
		arch, _ := r7distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			testPackageSpecSets := distro_test_common.GetTestingImagePackageSpecSets("kernel", imgType)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, testPackageSpecSets, nil, 0)
			assert.NoError(t, err)
		}
	}
}

func TestDistro_DirtyMountpointsNotAllowed(t *testing.T) {
	r7distro := rhel7.New()
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
	for _, archName := range r7distro.ListArches() {
		arch, _ := r7distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, nil, 0)
			assert.EqualError(t, err, "The following custom mountpoints are not supported [\"//\" \"/var//\" \"/var//log/audit/\"]")
		}
	}
}

func TestDistro_CustomFileSystemPatternMatching(t *testing.T) {
	r7distro := rhel7.New()
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
	for _, archName := range r7distro.ListArches() {
		arch, _ := r7distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, nil, nil, 0)
			assert.EqualError(t, err, "The following custom mountpoints are not supported [\"/variable\" \"/variable/log/audit\"]")
		}
	}
}

func TestDistro_CustomUsrPartitionNotLargeEnough(t *testing.T) {
	r7distro := rhel7.New()
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
	for _, archName := range r7distro.ListArches() {
		arch, _ := r7distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			testPackageSpecSets := distro_test_common.GetTestingImagePackageSpecSets("kernel", imgType)
			_, err := imgType.Manifest(bp.Customizations, distro.ImageOptions{}, nil, testPackageSpecSets, nil, 0)
			assert.NoError(t, err)
		}
	}
}
