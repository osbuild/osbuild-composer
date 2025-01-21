package v2

// ImageTypes methods to make it easier to use and test
import (
	"encoding/json"
	"fmt"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/uuid"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/cloud/gcp"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"
)

// GetImageOptions returns the initial ImageOptions with Size and PartitioningMode set
// The size is set to the largest of:
//   - Default size for the image type
//   - Blueprint filesystem customizations
//   - Requested size
//
// The partitioning mode is set to AutoLVM which will select LVM if there are additional mountpoints
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
	return distro.ImageOptions{Size: size, PartitioningMode: disk.AutoLVMPartitioningMode}
}

func newAWSTarget(options UploadOptions, imageType distro.ImageType) (*target.Target, error) {
	var awsUploadOptions AWSEC2UploadOptions
	jsonUploadOptions, err := json.Marshal(options)
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
	case platform.BOOT_HYBRID:
		amiBootMode = common.ToPtr(string(ec2types.BootModeValuesUefiPreferred))
	case platform.BOOT_UEFI:
		amiBootMode = common.ToPtr(string(ec2types.BootModeValuesUefi))
	case platform.BOOT_LEGACY:
		amiBootMode = common.ToPtr(string(ec2types.BootModeValuesLegacyBios))
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
	return t, nil
}

func newAWSS3Target(options UploadOptions, imageType distro.ImageType) (*target.Target, error) {
	var awsS3UploadOptions AWSS3UploadOptions
	jsonUploadOptions, err := json.Marshal(options)
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
	return t, nil
}

func newContainerTarget(options UploadOptions, request *ComposeRequest, imageType distro.ImageType) (*target.Target, error) {
	var containerUploadOptions ContainerUploadOptions
	jsonUploadOptions, err := json.Marshal(options)
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

	return t, nil
}

func newGCPTarget(options UploadOptions, imageType distro.ImageType) (*target.Target, error) {
	var gcpUploadOptions GCPUploadOptions
	jsonUploadOptions, err := json.Marshal(options)
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
	osName := imageType.Arch().Distro().Name()
	t := target.NewGCPTarget(&target.GCPTargetOptions{
		Region: gcpUploadOptions.Region,
		Os:     osName, // not exposed in cloudapi
		Bucket: bucket,
		// the uploaded object must have a valid extension
		Object:            fmt.Sprintf("%s.tar.gz", imageName),
		ShareWithAccounts: share,
		GuestOsFeatures:   gcp.GuestOsFeaturesByDistro(osName), // not exposed in cloudapi
	})
	// Import will fail if an image with this name already exists
	if gcpUploadOptions.ImageName != nil {
		t.ImageName = *gcpUploadOptions.ImageName
	} else {
		t.ImageName = imageName
	}
	t.OsbuildArtifact.ExportFilename = imageType.Filename()
	return t, nil
}

func newAzureTarget(options UploadOptions, imageType distro.ImageType) (*target.Target, error) {
	var azureUploadOptions AzureUploadOptions
	jsonUploadOptions, err := json.Marshal(options)
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

	hypvgen := target.HyperVGenV1
	if azureUploadOptions.HyperVGeneration != nil &&
		*azureUploadOptions.HyperVGeneration == AzureUploadOptionsHyperVGenerationV2 {
		hypvgen = target.HyperVGenV2
	}

	t := target.NewAzureImageTarget(&target.AzureImageTargetOptions{
		TenantID:         azureUploadOptions.TenantId,
		Location:         rgLocation,
		SubscriptionID:   azureUploadOptions.SubscriptionId,
		ResourceGroup:    azureUploadOptions.ResourceGroup,
		HyperVGeneration: hypvgen,
	})

	if azureUploadOptions.ImageName != nil {
		t.ImageName = *azureUploadOptions.ImageName
	} else {
		// if ImageName wasn't given, generate a random one
		t.ImageName = fmt.Sprintf("composer-api-%s", uuid.New().String())
	}
	t.OsbuildArtifact.ExportFilename = imageType.Filename()
	return t, nil
}

func newOCITarget(options UploadOptions, imageType distro.ImageType) (*target.Target, error) {
	var ociUploadOptions OCIUploadOptions
	jsonUploadOptions, err := json.Marshal(options)
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
	return t, nil
}

func newPulpOSTreeTarget(options UploadOptions, imageType distro.ImageType) (*target.Target, error) {
	var pulpUploadOptions PulpOSTreeUploadOptions
	jsonUploadOptions, err := json.Marshal(options)
	if err != nil {
		return nil, HTTPError(ErrorJSONMarshallingError)
	}
	err = json.Unmarshal(jsonUploadOptions, &pulpUploadOptions)
	if err != nil {
		return nil, HTTPError(ErrorJSONUnMarshallingError)
	}
	serverAddress := ""
	if pulpUploadOptions.ServerAddress != nil {
		serverAddress = *pulpUploadOptions.ServerAddress
	}
	repository := ""
	if pulpUploadOptions.Repository != nil {
		repository = *pulpUploadOptions.Repository
	}
	t := target.NewPulpOSTreeTarget(&target.PulpOSTreeTargetOptions{
		ServerAddress: serverAddress,
		Repository:    repository,
		BasePath:      pulpUploadOptions.Basepath,
	})

	t.ImageName = fmt.Sprintf("composer-api-%s", uuid.New().String())
	t.OsbuildArtifact.ExportFilename = imageType.Filename()
	return t, nil
}

// Returns the name of the default target for a given image type name or error
// if the image type name is unknown.
func getDefaultTarget(imageType ImageTypes) (UploadTypes, error) {
	switch imageType {
	case ImageTypesAws:
		fallthrough
	case ImageTypesAwsHaRhui:
		fallthrough
	case ImageTypesAwsRhui:
		fallthrough
	case ImageTypesAwsSapRhui:
		return UploadTypesAws, nil

	case ImageTypesEdgeCommit:
		fallthrough
	case ImageTypesEdgeInstaller:
		fallthrough
	case ImageTypesGuestImage:
		fallthrough
	case ImageTypesImageInstaller:
		fallthrough
	case ImageTypesIotCommit:
		fallthrough
	case ImageTypesIotInstaller:
		fallthrough
	case ImageTypesIotRawImage:
		fallthrough
	case ImageTypesIotSimplifiedInstaller:
		fallthrough
	case ImageTypesLiveInstaller:
		fallthrough
	case ImageTypesMinimalRaw:
		fallthrough
	case ImageTypesVsphere:
		fallthrough
	case ImageTypesVsphereOva:
		fallthrough
	case ImageTypesWsl:
		return UploadTypesAwsS3, nil

	case ImageTypesEdgeContainer:
		fallthrough
	case ImageTypesIotBootableContainer:
		fallthrough
	case ImageTypesIotContainer:
		return UploadTypesContainer, nil

	case ImageTypesGcp:
		fallthrough
	case ImageTypesGcpRhui:
		return UploadTypesGcp, nil

	case ImageTypesAzure:
		fallthrough
	case ImageTypesAzureEap7Rhui:
		fallthrough
	case ImageTypesAzureRhui:
		fallthrough
	case ImageTypesAzureSapRhui:
		return UploadTypesAzure, nil

	case ImageTypesOci:
		return UploadTypesOciObjectstorage, nil

	default:
		return "", HTTPError(ErrorUnsupportedImageType)
	}
}

func targetSupportMap() map[UploadTypes]map[ImageTypes]bool {
	return map[UploadTypes]map[ImageTypes]bool{
		UploadTypesAws: {
			ImageTypesAws:        true,
			ImageTypesAwsRhui:    true,
			ImageTypesAwsHaRhui:  true,
			ImageTypesAwsSapRhui: true,
		},
		UploadTypesAwsS3: {
			ImageTypesEdgeCommit:           true,
			ImageTypesEdgeInstaller:        true,
			ImageTypesGuestImage:           true,
			ImageTypesImageInstaller:       true,
			ImageTypesIotBootableContainer: true,
			ImageTypesIotCommit:            true,
			ImageTypesIotInstaller:         true,
			ImageTypesIotRawImage:          true,
			ImageTypesLiveInstaller:        true,
			ImageTypesMinimalRaw:           true,
			ImageTypesVsphereOva:           true,
			ImageTypesVsphere:              true,
			ImageTypesWsl:                  true,
		},
		UploadTypesContainer: {
			ImageTypesEdgeContainer:        true,
			ImageTypesIotBootableContainer: true,
			ImageTypesIotContainer:         true,
		},
		UploadTypesGcp: {
			ImageTypesGcp:     true,
			ImageTypesGcpRhui: true,
		},
		UploadTypesAzure: {
			ImageTypesAzure:         true,
			ImageTypesAzureRhui:     true,
			ImageTypesAzureEap7Rhui: true,
			ImageTypesAzureSapRhui:  true,
		},
		UploadTypesOciObjectstorage: {
			ImageTypesOci: true,
		},
		UploadTypesPulpOstree: {
			ImageTypesEdgeCommit: true,
			ImageTypesIotCommit:  true,
		},
		UploadTypesLocal: {
			ImageTypesAws:                  true,
			ImageTypesAwsRhui:              true,
			ImageTypesAwsHaRhui:            true,
			ImageTypesAwsSapRhui:           true,
			ImageTypesAzure:                true,
			ImageTypesAzureRhui:            true,
			ImageTypesAzureEap7Rhui:        true,
			ImageTypesAzureSapRhui:         true,
			ImageTypesEdgeCommit:           true,
			ImageTypesEdgeContainer:        true,
			ImageTypesEdgeInstaller:        true,
			ImageTypesGuestImage:           true,
			ImageTypesImageInstaller:       true,
			ImageTypesIotBootableContainer: true,
			ImageTypesIotCommit:            true,
			ImageTypesIotInstaller:         true,
			ImageTypesIotRawImage:          true,
			ImageTypesLiveInstaller:        true,
			ImageTypesMinimalRaw:           true,
			ImageTypesOci:                  true,
			ImageTypesVsphereOva:           true,
			ImageTypesVsphere:              true,
			ImageTypesWsl:                  true,
		},
	}
}

// GetTargets returns the targets for the ImageRequest. Merges the
// UploadTargets with the top-level default UploadOptions if specified.
func (ir *ImageRequest) GetTargets(request *ComposeRequest, imageType distro.ImageType) ([]*target.Target, error) {
	tsm := targetSupportMap()
	targets := make([]*target.Target, 0)
	if ir.UploadTargets != nil {
		for _, ut := range *ir.UploadTargets {
			// check if the target type is valid for the image type
			if !tsm[ut.Type][ir.ImageType] {
				return nil, HTTPError(ErrorInvalidUploadTarget)
			}
			trgt, err := getTarget(ut.Type, ut.UploadOptions, request, imageType)
			if err != nil {
				return nil, err
			}
			// prepend the top-level target
			targets = append([]*target.Target{trgt}, targets...)
		}
	}

	if ir.UploadOptions != nil {
		// default upload target options also defined: append them to the targets
		defTargetType, err := getDefaultTarget(ir.ImageType)
		if err != nil {
			return nil, err
		}
		trgt, err := getTarget(defTargetType, *ir.UploadOptions, request, imageType)
		if err != nil {
			return nil, err
		}
		targets = append(targets, trgt)
	}

	return targets, nil
}

func getTarget(targetType UploadTypes, options UploadOptions, request *ComposeRequest, imageType distro.ImageType) (irTarget *target.Target, err error) {
	switch targetType {
	case UploadTypesAws:
		irTarget, err = newAWSTarget(options, imageType)

	case UploadTypesAwsS3:
		irTarget, err = newAWSS3Target(options, imageType)

	case UploadTypesContainer:
		irTarget, err = newContainerTarget(options, request, imageType)

	case UploadTypesGcp:
		irTarget, err = newGCPTarget(options, imageType)

	case UploadTypesAzure:
		irTarget, err = newAzureTarget(options, imageType)

	case UploadTypesOciObjectstorage:
		irTarget, err = newOCITarget(options, imageType)

	case UploadTypesPulpOstree:
		irTarget, err = newPulpOSTreeTarget(options, imageType)

	case UploadTypesLocal:
		irTarget = target.NewWorkerServerTarget()
	default:
		return nil, HTTPError(ErrorInvalidUploadTarget)
	}
	if err != nil {
		return nil, err
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
