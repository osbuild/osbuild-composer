package v2

// ImageTypes methods to make it easier to use and test
import (
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/google/uuid"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"
)

// GetImageOptions returns the initial ImageOptions with Size set
// The size is set to the largest of:
//   - Default size for the image type
//   - Blueprint filesystem customizations
//   - Requested size
func (ir *ImageRequest) GetImageOptions(imageType distro.ImageType, bp blueprint.Blueprint) distro.ImageOptions {
	// NOTE: The size is in bytes
	var size uint64
	minSize := bp.Customizations.GetFilesystemsMinSize()
	if ir.Size == nil {
		size = imageType.Size(minSize)
	} else if bp.Customizations != nil && minSize > 0 && minSize > *ir.Size {
		size = imageType.Size(minSize)
	} else {
		size = imageType.Size(*ir.Size)
	}
	return distro.ImageOptions{Size: size}
}

// GetImageTarget returns the target for the selected image type
func (ir *ImageRequest) GetTarget(request *ComposeRequest, imageType distro.ImageType) (irTarget *target.Target, err error) {
	/* oneOf is not supported by the openapi generator so marshal and unmarshal the uploadrequest based on the type */
	switch ir.ImageType {
	case ImageTypesAws:
		fallthrough
	case ImageTypesAwsRhui:
		fallthrough
	case ImageTypesAwsHaRhui:
		fallthrough
	case ImageTypesAwsSapRhui:
		var awsUploadOptions AWSEC2UploadOptions
		jsonUploadOptions, err := json.Marshal(*ir.UploadOptions)
		if err != nil {
			return nil, HTTPError(ErrorJSONMarshallingError)
		}
		err = json.Unmarshal(jsonUploadOptions, &awsUploadOptions)
		if err != nil {
			return nil, HTTPError(ErrorJSONUnMarshallingError)
		}

		// For service maintenance, images are discovered by the "Name:composer-api-*"
		// tag filter. Currently all image names in the service are generated, so they're
		// guaranteed to be unique as well. If users are ever allowed to name their images,
		// an extra tag should be added.
		key := fmt.Sprintf("composer-api-%s", uuid.New().String())

		var amiBootMode *string
		switch imageType.BootMode() {
		case distro.BOOT_HYBRID:
			amiBootMode = common.ToPtr(ec2.BootModeValuesUefiPreferred)
		case distro.BOOT_UEFI:
			amiBootMode = common.ToPtr(ec2.BootModeValuesUefi)
		case distro.BOOT_LEGACY:
			amiBootMode = common.ToPtr(ec2.BootModeValuesLegacyBios)
		}

		t := target.NewAWSTarget(&target.AWSTargetOptions{
			Region:            awsUploadOptions.Region,
			Key:               key,
			ShareWithAccounts: awsUploadOptions.ShareWithAccounts,
			BootMode:          amiBootMode,
		})
		if awsUploadOptions.SnapshotName != nil {
			t.ImageName = *awsUploadOptions.SnapshotName
		} else {
			t.ImageName = key
		}
		t.OsbuildArtifact.ExportFilename = imageType.Filename()

		irTarget = t
	case ImageTypesGuestImage:
		fallthrough
	case ImageTypesVsphere:
		fallthrough
	case ImageTypesVsphereOva:
		fallthrough
	case ImageTypesWsl:
		fallthrough
	case ImageTypesImageInstaller:
		fallthrough
	case ImageTypesEdgeInstaller:
		fallthrough
	case ImageTypesIotInstaller:
		fallthrough
	case ImageTypesLiveInstaller:
		fallthrough
	case ImageTypesEdgeCommit:
		fallthrough
	case ImageTypesIotCommit:
		fallthrough
	case ImageTypesIotRawImage:
		var awsS3UploadOptions AWSS3UploadOptions
		jsonUploadOptions, err := json.Marshal(*ir.UploadOptions)
		if err != nil {
			return nil, HTTPError(ErrorJSONMarshallingError)
		}
		err = json.Unmarshal(jsonUploadOptions, &awsS3UploadOptions)
		if err != nil {
			return nil, HTTPError(ErrorJSONUnMarshallingError)
		}

		public := false
		if awsS3UploadOptions.Public != nil && *awsS3UploadOptions.Public {
			public = true
		}

		key := fmt.Sprintf("composer-api-%s", uuid.New().String())
		t := target.NewAWSS3Target(&target.AWSS3TargetOptions{
			Region: awsS3UploadOptions.Region,
			Key:    key,
			Public: public,
		})
		t.ImageName = key
		t.OsbuildArtifact.ExportFilename = imageType.Filename()

		irTarget = t
	case ImageTypesEdgeContainer:
		fallthrough
	case ImageTypesIotContainer:
		var containerUploadOptions ContainerUploadOptions
		jsonUploadOptions, err := json.Marshal(*ir.UploadOptions)
		if err != nil {
			return nil, HTTPError(ErrorJSONMarshallingError)
		}
		err = json.Unmarshal(jsonUploadOptions, &containerUploadOptions)
		if err != nil {
			return nil, HTTPError(ErrorJSONUnMarshallingError)
		}

		var name = request.Distribution
		var tag = uuid.New().String()
		if containerUploadOptions.Name != nil {
			name = *containerUploadOptions.Name
			if containerUploadOptions.Tag != nil {
				tag = *containerUploadOptions.Tag
			}
		}

		t := target.NewContainerTarget(&target.ContainerTargetOptions{})
		t.ImageName = fmt.Sprintf("%s:%s", name, tag)
		t.OsbuildArtifact.ExportFilename = imageType.Filename()

		irTarget = t
	case ImageTypesGcp:
		fallthrough
	case ImageTypesGcpRhui:
		var gcpUploadOptions GCPUploadOptions
		jsonUploadOptions, err := json.Marshal(*ir.UploadOptions)
		if err != nil {
			return nil, HTTPError(ErrorJSONMarshallingError)
		}
		err = json.Unmarshal(jsonUploadOptions, &gcpUploadOptions)
		if err != nil {
			return nil, HTTPError(ErrorJSONUnMarshallingError)
		}

		var share []string
		if gcpUploadOptions.ShareWithAccounts != nil {
			share = *gcpUploadOptions.ShareWithAccounts
		}

		imageName := fmt.Sprintf("composer-api-%s", uuid.New().String())
		var bucket string
		if gcpUploadOptions.Bucket != nil {
			bucket = *gcpUploadOptions.Bucket
		}
		t := target.NewGCPTarget(&target.GCPTargetOptions{
			Region: gcpUploadOptions.Region,
			Os:     imageType.Arch().Distro().Name(), // not exposed in cloudapi
			Bucket: bucket,
			// the uploaded object must have a valid extension
			Object:            fmt.Sprintf("%s.tar.gz", imageName),
			ShareWithAccounts: share,
		})
		// Import will fail if an image with this name already exists
		if gcpUploadOptions.ImageName != nil {
			t.ImageName = *gcpUploadOptions.ImageName
		} else {
			t.ImageName = imageName
		}
		t.OsbuildArtifact.ExportFilename = imageType.Filename()

		irTarget = t
	case ImageTypesAzure:
		fallthrough
	case ImageTypesAzureRhui:
		fallthrough
	case ImageTypesAzureEap7Rhui:
		fallthrough
	case ImageTypesAzureSapRhui:
		var azureUploadOptions AzureUploadOptions
		jsonUploadOptions, err := json.Marshal(*ir.UploadOptions)
		if err != nil {
			return nil, HTTPError(ErrorJSONMarshallingError)
		}
		err = json.Unmarshal(jsonUploadOptions, &azureUploadOptions)
		if err != nil {
			return nil, HTTPError(ErrorJSONUnMarshallingError)
		}
		rgLocation := ""
		if azureUploadOptions.Location != nil {
			rgLocation = *azureUploadOptions.Location
		}
		t := target.NewAzureImageTarget(&target.AzureImageTargetOptions{
			TenantID:       azureUploadOptions.TenantId,
			Location:       rgLocation,
			SubscriptionID: azureUploadOptions.SubscriptionId,
			ResourceGroup:  azureUploadOptions.ResourceGroup,
		})

		if azureUploadOptions.ImageName != nil {
			t.ImageName = *azureUploadOptions.ImageName
		} else {
			// if ImageName wasn't given, generate a random one
			t.ImageName = fmt.Sprintf("composer-api-%s", uuid.New().String())
		}
		t.OsbuildArtifact.ExportFilename = imageType.Filename()

		irTarget = t
	case ImageTypesOci:
		var ociUploadOptions OCIUploadOptions
		jsonUploadOptions, err := json.Marshal(*ir.UploadOptions)
		if err != nil {
			return nil, HTTPError(ErrorJSONMarshallingError)
		}
		err = json.Unmarshal(jsonUploadOptions, &ociUploadOptions)
		if err != nil {
			return nil, HTTPError(ErrorJSONUnMarshallingError)
		}

		key := fmt.Sprintf("composer-api-%s", uuid.New().String())
		t := target.NewOCIObjectStorageTarget(&target.OCIObjectStorageTargetOptions{})
		t.ImageName = key
		t.OsbuildArtifact.ExportFilename = imageType.Filename()
		irTarget = t
	default:
		return nil, HTTPError(ErrorUnsupportedImageType)
	}
	irTarget.OsbuildArtifact.ExportName = imageType.Exports()[0]

	return irTarget, nil
}

// GetOSTreeOptions returns the image ostree options when included in the request
// or nil if they are not present.
func (ir *ImageRequest) GetOSTreeOptions() (ostreeOptions *ostree.ImageOptions, err error) {

	if ir.Ostree == nil {
		return nil, nil
	}

	ostreeOptions = &ostree.ImageOptions{}
	if ir.Ostree.Ref != nil {
		ostreeOptions.ImageRef = *ir.Ostree.Ref
	}
	if ir.Ostree.Url != nil {
		ostreeOptions.URL = *ir.Ostree.Url
	}
	if ir.Ostree.Contenturl != nil {
		// URL must be set if content url is specified
		if ir.Ostree.Url == nil {
			return nil, HTTPError(ErrorInvalidOSTreeParams)
		}
		ostreeOptions.ContentURL = *ir.Ostree.Contenturl
	}
	if ir.Ostree.Parent != nil {
		ostreeOptions.ParentRef = *ir.Ostree.Parent
	}
	if ir.Ostree.Rhsm != nil {
		ostreeOptions.RHSM = *ir.Ostree.Rhsm
	}

	return ostreeOptions, nil
}
