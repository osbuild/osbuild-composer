package v2

import (
	"testing"

	"github.com/osbuild/images/pkg/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"

	"github.com/stretchr/testify/assert"
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
