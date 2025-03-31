package v2

import (
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro/rhel/rhel9"
	"github.com/osbuild/images/pkg/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageRequestSize(t *testing.T) {
	distro := test_distro.DistroFactory(test_distro.TestDistro1Name)
	require.NotNil(t, distro)
	arch, err := distro.GetArch(test_distro.TestArchName)
	if err != nil {
		panic(err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		panic(err)
	}

	// A blueprint with no filesystem customizations
	bp := blueprint.Blueprint{
		Name:        "image-request-test",
		Description: "Empty Blueprint",
		Version:     "0.0.1",
	}
	// #1 With no size request
	ir := ImageRequest{
		Architecture: test_distro.TestArchName,
		ImageType:    test_distro.TestImageTypeName,
		Size:         nil,
	}
	imageOptions := ir.GetImageOptions(imageType, bp)

	// The test_distro default size is 1GiB
	assert.Equal(t, uint64(1073741824), imageOptions.Size)

	// #2 With size request
	ir = ImageRequest{
		Architecture: test_distro.TestArchName,
		ImageType:    test_distro.TestImageTypeName,
		Size:         common.ToPtr(uint64(5368709120)),
	}
	imageOptions = ir.GetImageOptions(imageType, bp)

	// The test_distro default size is actually 5GiB
	assert.Equal(t, uint64(5368709120), imageOptions.Size)

	// A blueprint with filesystem customizations
	bp = blueprint.Blueprint{
		Name:        "image-request-test",
		Description: "Customized Filesystem",
		Version:     "0.0.1",
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				blueprint.FilesystemCustomization{
					Mountpoint: "/",
					MinSize:    2147483648,
				},
			},
		},
	}

	// #3 With no size request
	ir = ImageRequest{
		Architecture: test_distro.TestArchName,
		ImageType:    test_distro.TestImageTypeName,
		Size:         nil,
	}
	imageOptions = ir.GetImageOptions(imageType, bp)

	// The test_distro default size is actually 2GiB
	assert.Equal(t, uint64(2147483648), imageOptions.Size)

	// #4 With size request
	ir = ImageRequest{
		Architecture: test_distro.TestArchName,
		ImageType:    test_distro.TestImageTypeName,
		Size:         common.ToPtr(uint64(5368709120)),
	}
	imageOptions = ir.GetImageOptions(imageType, bp)

	// The test_distro default size is actually 5GiB
	assert.Equal(t, uint64(5368709120), imageOptions.Size)
}

func TestGetOstreeOptions(t *testing.T) {
	// No Ostree settings
	ir := ImageRequest{
		Architecture: test_distro.TestArchName,
		ImageType:    test_distro.TestImageTypeName,
	}
	options, err := ir.GetOSTreeOptions()
	assert.Nil(t, options)
	assert.Nil(t, err)

	// Populated Ostree settings
	ir = ImageRequest{
		Architecture: test_distro.TestArchName,
		ImageType:    test_distro.TestImageTypeName,
		Ostree: &OSTree{
			Contenturl: common.ToPtr("http://url.to.content/"),
			Parent:     common.ToPtr("02604b2da6e954bd34b8b82a835e5a77d2b60ffa"),
			Ref:        common.ToPtr("reference"),
			Url:        common.ToPtr("http://the.url/"),
		},
	}
	options, err = ir.GetOSTreeOptions()
	assert.Nil(t, err)
	require.NotNil(t, options)
	assert.Equal(t, "http://url.to.content/", options.ContentURL)
	assert.Equal(t, "02604b2da6e954bd34b8b82a835e5a77d2b60ffa", options.ParentRef)
	assert.Equal(t, "reference", options.ImageRef)
	assert.Equal(t, "http://the.url/", options.URL)

	// Populated Ostree settings with no url
	ir = ImageRequest{
		Architecture: test_distro.TestArchName,
		ImageType:    test_distro.TestImageTypeName,
		Ostree: &OSTree{
			Contenturl: common.ToPtr("http://url.to.content/"),
		},
	}
	_, err = ir.GetOSTreeOptions()
	require.NotNil(t, err)
	assert.Error(t, err)
}

func TestGetTargets(t *testing.T) {
	at := assert.New(t)

	r9 := rhel9.DistroFactory("rhel-9.3")
	require.NotNil(t, r9)
	arch, err := r9.GetArch(arch.ARCH_X86_64.String())
	at.NoError(err)

	cr := &ComposeRequest{
		Distribution: r9.Name(),
	}

	it, err := arch.GetImageType("qcow2")
	at.NoError(err)

	var uploadOptions UploadOptions // doesn't need a concrete value or type for this test

	type testCase struct {
		imageType      ImageTypes
		targets        []UploadTypes
		includeDefault bool
		expected       []target.TargetName
		fail           bool
	}

	testCases := map[string]testCase{
		"guest:default": {
			imageType:      ImageTypesGuestImage,
			targets:        nil,
			includeDefault: true,
			expected:       []target.TargetName{target.TargetNameAWSS3},
		},
		"guest:s3": {
			imageType:      ImageTypesGuestImage,
			targets:        []UploadTypes{UploadTypesAwsS3},
			includeDefault: false,
			expected:       []target.TargetName{target.TargetNameAWSS3},
		},
		"guest:s3+default": {
			imageType:      ImageTypesGuestImage,
			targets:        []UploadTypes{UploadTypesAwsS3},
			includeDefault: true,
			expected:       []target.TargetName{target.TargetNameAWSS3, target.TargetNameAWSS3},
		},
		"guest:local": {
			imageType:      ImageTypesGuestImage,
			targets:        []UploadTypes{UploadTypesLocal},
			includeDefault: false,
			expected:       []target.TargetName{target.TargetNameWorkerServer},
		},
		"guest:azure:fail": {
			imageType: ImageTypesGuestImage,
			targets:   []UploadTypes{UploadTypesAzure},
			expected:  []target.TargetName{""},
			fail:      true,
		},
		"azure:nil": {
			imageType:      ImageTypesAzure,
			targets:        nil,
			includeDefault: true,
			expected:       []target.TargetName{target.TargetNameAzureImage},
		},
		"azure:azure": {
			imageType: ImageTypesAzure,
			targets:   []UploadTypes{UploadTypesAzure},
			expected:  []target.TargetName{target.TargetNameAzureImage},
		},
		"azure:gcp:fail": {
			imageType: ImageTypesAzure,
			targets:   []UploadTypes{UploadTypesGcp},
			expected:  []target.TargetName{""},
			fail:      true,
		},
		"edge:default": {
			imageType:      ImageTypesEdgeCommit,
			targets:        nil,
			includeDefault: true,
			expected:       []target.TargetName{target.TargetNameAWSS3},
		},
		"edge:s3": {
			imageType: ImageTypesEdgeCommit,
			targets:   []UploadTypes{UploadTypesAwsS3},
			expected:  []target.TargetName{target.TargetNameAWSS3},
		},
		"edge:pulp": {
			imageType: ImageTypesEdgeCommit,
			targets:   []UploadTypes{UploadTypesPulpOstree},
			expected:  []target.TargetName{target.TargetNamePulpOSTree},
		},
		"edge:pulp+default": {
			imageType:      ImageTypesEdgeCommit,
			targets:        []UploadTypes{UploadTypesPulpOstree},
			includeDefault: true,
			expected:       []target.TargetName{target.TargetNamePulpOSTree, target.TargetNameAWSS3},
		},
		"edge:gcp:fail": {
			imageType: ImageTypesEdgeCommit,
			targets:   []UploadTypes{UploadTypesGcp},
			expected:  []target.TargetName{""},
			fail:      true,
		},
	}

	for name := range testCases {
		t.Run(name, func(t *testing.T) {
			at := assert.New(t)
			testCase := testCases[name]
			uploadTargets := make([]UploadTarget, len(testCase.targets))
			for idx := range testCase.targets {
				uploadTargets[idx] = UploadTarget{
					Type:          testCase.targets[idx],
					UploadOptions: uploadOptions,
				}
			}
			ir := ImageRequest{
				Architecture:  arch.Name(),
				ImageType:     testCase.imageType,
				UploadTargets: &uploadTargets,
			}
			if testCase.includeDefault {
				// add UploadOptions for the default target too
				ir.UploadOptions = &uploadOptions
			}

			targets, err := ir.GetTargets(cr, it)
			if !testCase.fail {
				at.NoError(err)
				at.Equal(len(targets), len(testCase.expected))
				for idx := range targets {
					at.Equal(targets[idx].Name, testCase.expected[idx])
				}
			} else {
				at.Error(err)
			}
		})
	}
}
