package azure

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v5"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/olog"
)

type GalleryImage struct {
	ResourceGroup string `json:"resourcegroup"`
	Gallery       string `json:"gallery"`
	ImageDef      string `json:"imagedefinition"`
	Image         string `json:"image"`
	ImageRef      string `json:"imageref"`
}

// RegisterGalleryImage creates an image gallery and registers the
// specified blob as a new image version.
func (ac Client) RegisterGalleryImage(ctx context.Context, resourceGroup, storageAccount, storageContainer, blobName, name, location string, hyperVGen HyperVGenerationType, architecture arch.Arch) (*GalleryImage, error) {
	galleryImage := GalleryImage{
		ResourceGroup: resourceGroup,
	}
	var err error
	defer func() {
		if err != nil {
			if err = ac.DeleteGalleryImage(ctx, &galleryImage); err != nil {
				olog.Printf("unable to delete image gallery: %s", err.Error())
			}
		}
	}()

	if location == "" {
		location, err = ac.GetResourceGroupLocation(ctx, resourceGroup)
		if err != nil {
			return nil, fmt.Errorf("retrieving resource group location failed: %w", err)
		}
	}

	// galleries do not support hypens in the name
	gallery, err := ac.createGallery(ctx, resourceGroup, location, fmt.Sprintf("%s_gallery", strings.ReplaceAll(name, "-", "_")))
	if err != nil {
		return nil, err
	}
	if gallery.Name == nil {
		return nil, fmt.Errorf("Gallery name in gallery create response is empty")
	}
	galleryImage.Gallery = *gallery.Name

	img, err := ac.createGalleryImageDef(ctx, resourceGroup, location, galleryImage.Gallery, hyperVGen, architecture, fmt.Sprintf("%s-img", name))
	if err != nil {
		return nil, err
	}
	if img.Name == nil {
		return nil, fmt.Errorf("Image definition name in gallery %s is empty", galleryImage.Gallery)
	}
	galleryImage.ImageDef = *img.Name

	managedImg := fmt.Sprintf("%s-mimg", name)
	err = ac.RegisterImage(ctx, resourceGroup, storageAccount, storageContainer, blobName, managedImg, location, hyperVGen)
	if err != nil {
		return nil, err
	}
	galleryImage.Image = managedImg

	imgVersion, err := ac.createGalleryImageVersion(
		ctx,
		resourceGroup,
		location,
		galleryImage.Gallery,
		galleryImage.ImageDef,
		fmt.Sprintf("/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/images/%s", ac.subscription, resourceGroup, managedImg),
	)
	if err != nil {
		return nil, err
	}
	if imgVersion.Name == nil {
		return nil, fmt.Errorf("Image version in image definition %s in gallery %s is empty", galleryImage.ImageDef, galleryImage.Gallery)
	}

	galleryImage.ImageRef = fmt.Sprintf(
		"/subscriptions/%s/resourceGroups/%s/providers/Microsoft.Compute/galleries/%s/images/%s/versions/%s",
		ac.subscription,
		resourceGroup,
		galleryImage.Gallery,
		galleryImage.ImageDef,
		*imgVersion.Name,
	)
	return &galleryImage, nil
}

func (ac Client) createGallery(ctx context.Context, resourceGroup, location, name string) (*armcompute.Gallery, error) {
	poller, err := ac.galleries.BeginCreateOrUpdate(ctx, resourceGroup, name, armcompute.Gallery{
		Location: &location,
	}, nil)
	if err != nil {
		return nil, err
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.Gallery, nil
}

// the name "image definition" should not be confused with osbuild's term, it is just a container for image versions.
func (ac Client) createGalleryImageDef(ctx context.Context, resourceGroup, location, gallery string, hyperVGen HyperVGenerationType, architecture arch.Arch, name string) (*armcompute.GalleryImage, error) {
	var hypvgen armcompute.HyperVGeneration
	switch hyperVGen {
	case HyperVGenV1:
		hypvgen = armcompute.HyperVGenerationV1
	case HyperVGenV2:
		hypvgen = armcompute.HyperVGenerationV2
	default:
		return nil, fmt.Errorf("Unknown hyper v generation type %v", hyperVGen)
	}

	var azArch armcompute.Architecture
	switch architecture {
	case arch.ARCH_X86_64:
		azArch = armcompute.ArchitectureX64
	case arch.ARCH_AARCH64:
		azArch = armcompute.ArchitectureArm64
	default:
		return nil, fmt.Errorf("Unknown hyper v generation type %v", hyperVGen)
	}

	poller, err := ac.galleryImgs.BeginCreateOrUpdate(ctx, resourceGroup, gallery, name, armcompute.GalleryImage{
		Location: &location,
		Properties: &armcompute.GalleryImageProperties{
			Identifier: &armcompute.GalleryImageIdentifier{
				Publisher: common.ToPtr("image-builder"),
				Offer:     common.ToPtr("image-builder"),
				SKU:       common.ToPtr(fmt.Sprintf("IB-SKU-%s", name)),
			},
			Architecture:     &azArch,
			HyperVGeneration: &hypvgen,
			OSType:           common.ToPtr(armcompute.OperatingSystemTypesLinux),
			OSState:          common.ToPtr(armcompute.OperatingSystemStateTypesGeneralized),
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.GalleryImage, nil
}

func (ac Client) createGalleryImageVersion(ctx context.Context, resourceGroup, location, gallery, image, uri string) (*armcompute.GalleryImageVersion, error) {
	poller, err := ac.galleryImgVs.BeginCreateOrUpdate(ctx, resourceGroup, gallery, image, "1.0.0", armcompute.GalleryImageVersion{
		Location: &location,
		Properties: &armcompute.GalleryImageVersionProperties{
			PublishingProfile: &armcompute.GalleryImageVersionPublishingProfile{
				TargetRegions: []*armcompute.TargetRegion{
					{
						Name: &location,
					},
				},
			},
			StorageProfile: &armcompute.GalleryImageVersionStorageProfile{
				Source: &armcompute.GalleryArtifactVersionFullSource{
					ID: &uri,
				},
			},
		},
	}, nil)
	if err != nil {
		return nil, err
	}
	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &resp.GalleryImageVersion, nil
}

func (ac Client) deleteGalleryImageVersion(ctx context.Context, gi *GalleryImage) error {
	if gi.ImageRef == "" {
		return nil
	}

	poller, err := ac.galleryImgVs.BeginDelete(ctx, gi.ResourceGroup, gi.Gallery, gi.ImageDef, "1.0.0", nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ac Client) deleteGalleryImageDef(ctx context.Context, gi *GalleryImage) error {
	if gi.ImageDef == "" {
		return nil
	}

	poller, err := ac.galleryImgs.BeginDelete(ctx, gi.ResourceGroup, gi.Gallery, gi.ImageDef, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ac Client) deleteImageGallery(ctx context.Context, gi *GalleryImage) error {
	if gi.Gallery == "" {
		return nil
	}

	poller, err := ac.galleries.BeginDelete(ctx, gi.ResourceGroup, gi.Gallery, nil)
	if err != nil {
		return err
	}
	_, err = poller.PollUntilDone(ctx, nil)
	if err != nil {
		return err
	}
	return nil
}

func (ac Client) DeleteGalleryImage(ctx context.Context, gi *GalleryImage) error {
	if err := ac.deleteGalleryImageVersion(ctx, gi); err != nil {
		return err
	}
	if err := ac.deleteGalleryImageDef(ctx, gi); err != nil {
		return err
	}

	var err error
	// Even though the gallery image definition has been deleted, azure returns 409
	// (conflict because the definition still exists) sometimes in spite of the poller in deleteImageGallery.
	for tries := 0; tries < 10; tries++ {
		if err = ac.deleteImageGallery(ctx, gi); err == nil {
			break
		}
		time.Sleep(20 * time.Second)
	}
	if err != nil {
		return err
	}

	if err = ac.DeleteImage(ctx, gi.ResourceGroup, gi.Image); err != nil {
		return err
	}
	return nil
}
