package v2

import (
	"testing"

	"github.com/osbuild/images/pkg/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageRequestSize(t *testing.T) {
	distro := test_distro.New()
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
