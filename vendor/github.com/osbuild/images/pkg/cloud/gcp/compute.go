package gcp

import (
	"context"
	"fmt"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/option"

	"github.com/osbuild/images/internal/common"
)

// Default Guest OS Features [1]. Note that officially Google creates the
// RHEL images in the rhel-cloud project in the rhel8,rhel9,etc image
// families. Periodically we'll want to make sure that the lists here
// are up to date with what is being produced there. You can see this
// with a command like:
//    gcloud compute images describe-from-family --project rhel-cloud rhel-9
//
// Note also for the time being that we should make sure the image upload
// code for CoreOS [2] should be kept in sync with this until CoreOS
// starts using OSBuild for image uploading.
//
// [1] https://cloud.google.com/compute/docs/images/create-custom#guest-os-features
// [2] https://github.com/coreos/coreos-assembler/blob/main/mantle/platform/api/gcloud/image.go

// Guest OS Features for RHEL8 images
var GuestOsFeaturesRHEL8 []*computepb.GuestOsFeature = []*computepb.GuestOsFeature{
	{Type: common.ToPtr(computepb.GuestOsFeature_UEFI_COMPATIBLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_VIRTIO_SCSI_MULTIQUEUE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_CAPABLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_SNP_CAPABLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_LIVE_MIGRATABLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_LIVE_MIGRATABLE_V2.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_GVNIC.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_IDPF.String())},
}

// Guest OS Features for RHEL9 images.
var GuestOsFeaturesRHEL9 []*computepb.GuestOsFeature = []*computepb.GuestOsFeature{
	{Type: common.ToPtr(computepb.GuestOsFeature_UEFI_COMPATIBLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_VIRTIO_SCSI_MULTIQUEUE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_CAPABLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_SNP_CAPABLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_LIVE_MIGRATABLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_LIVE_MIGRATABLE_V2.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_GVNIC.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_IDPF.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_TDX_CAPABLE.String())},
}

// Guest OS Features for RHEL images up to RHEL9.5.
// The TDX support was added since RHEL-9.6.
var GuestOsFeaturesRHEL95 []*computepb.GuestOsFeature = []*computepb.GuestOsFeature{
	{Type: common.ToPtr(computepb.GuestOsFeature_UEFI_COMPATIBLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_VIRTIO_SCSI_MULTIQUEUE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_CAPABLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_GVNIC.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_SNP_CAPABLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_LIVE_MIGRATABLE_V2.String())},
}

// Guest OS Features for RHEL9.1 images.
// The SEV_LIVE_MIGRATABLE_V2 support was added since RHEL-9.2
var GuestOsFeaturesRHEL91 []*computepb.GuestOsFeature = []*computepb.GuestOsFeature{
	{Type: common.ToPtr(computepb.GuestOsFeature_UEFI_COMPATIBLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_VIRTIO_SCSI_MULTIQUEUE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_CAPABLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_GVNIC.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_SNP_CAPABLE.String())},
}

// Guest OS Features for RHEL9.0 images.
// The SEV-SNP support was added since RHEL-9.1, so keeping this for RHEL-9.0 only.
var GuestOsFeaturesRHEL90 []*computepb.GuestOsFeature = []*computepb.GuestOsFeature{
	{Type: common.ToPtr(computepb.GuestOsFeature_UEFI_COMPATIBLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_VIRTIO_SCSI_MULTIQUEUE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_SEV_CAPABLE.String())},
	{Type: common.ToPtr(computepb.GuestOsFeature_GVNIC.String())},
}

// Guest OS Features for RHEL-10 images.
var GuestOsFeaturesRHEL10 []*computepb.GuestOsFeature = GuestOsFeaturesRHEL9

// GuestOsFeaturesByDistro returns the the list of Guest OS Features, which
// should be used when importing an image of the specified distribution.
//
// In case the provided distribution does not have any specific Guest OS
// Features list defined, nil is returned.
func GuestOsFeaturesByDistro(distroName string) []*computepb.GuestOsFeature {
	switch {
	case strings.HasPrefix(distroName, "centos-8"):
		fallthrough
	case strings.HasPrefix(distroName, "rhel-8"):
		return GuestOsFeaturesRHEL8

	case distroName == "rhel-9.0":
		return GuestOsFeaturesRHEL90
	case distroName == "rhel-9.1":
		return GuestOsFeaturesRHEL91
	case distroName == "rhel-9.2":
		fallthrough
	case distroName == "rhel-9.3":
		fallthrough
	case distroName == "rhel-9.4":
		fallthrough
	case distroName == "rhel-9.5":
		return GuestOsFeaturesRHEL95
	case strings.HasPrefix(distroName, "centos-9"):
		fallthrough
	case strings.HasPrefix(distroName, "rhel-9"):
		return GuestOsFeaturesRHEL9

	case strings.HasPrefix(distroName, "centos-10"):
		fallthrough
	case strings.HasPrefix(distroName, "rhel-10"):
		return GuestOsFeaturesRHEL10

	default:
		return nil
	}
}

// ComputeImageInsert imports a previously uploaded archive with raw image into Compute Engine.
//
// The image must be RAW image named 'disk.raw' inside a gzip-ed tarball.
//
// To delete the Storage object (image) used for the image import, use StorageObjectDelete().
//
// bucket - Google storage bucket name with the uploaded image archive
// object - Google storage object name of the uploaded image
// imageName - Desired image name after the import. This must be unique within the whole project.
// regions - A list of valid Google Storage regions where the resulting image should be located.
//
//	It is possible to specify multiple regions. Also multi and dual regions are allowed.
//	If not provided, the region of the used Storage object is used.
//	See: https://cloud.google.com/storage/docs/locations
//
// guestOsFeatures - A list of features supported by the Guest OS on the imported image.
//
// Uses:
//   - Compute Engine API
func (g *GCP) ComputeImageInsert(
	ctx context.Context,
	bucket, object, imageName string,
	regions []string,
	guestOsFeatures []*computepb.GuestOsFeature) (*computepb.Image, error) {
	imagesClient, err := compute.NewImagesRESTClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return nil, fmt.Errorf("failed to get Compute Engine Images client: %v", err)
	}
	defer imagesClient.Close()

	operationsClient, err := compute.NewGlobalOperationsRESTClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return nil, fmt.Errorf("failed to get Compute Engine Operations client: %v", err)
	}
	defer operationsClient.Close()

	imgInsertReq := &computepb.InsertImageRequest{
		Project: g.GetProjectID(),
		ImageResource: &computepb.Image{
			Name:             &imageName,
			StorageLocations: regions,
			GuestOsFeatures:  guestOsFeatures,
			RawDisk: &computepb.RawDisk{
				ContainerType: common.ToPtr(computepb.RawDisk_TAR.String()),
				Source:        common.ToPtr(fmt.Sprintf("https://storage.googleapis.com/%s/%s", bucket, object)),
			},
		},
	}

	operation, err := imagesClient.Insert(ctx, imgInsertReq)
	if err != nil {
		return nil, fmt.Errorf("failed to insert provided image into GCE: %v", err)
	}

	// wait for the operation to finish
	var operationResource *computepb.Operation
	for {
		waitOperationReq := &computepb.WaitGlobalOperationRequest{
			Operation: operation.Proto().GetName(),
			Project:   g.GetProjectID(),
		}

		operationResource, err = operationsClient.Wait(ctx, waitOperationReq)
		if err != nil {
			return nil, fmt.Errorf("failed to wait for an Image Import operation: %v", err)
		}

		// The operation finished
		if operationResource.GetStatus() != computepb.Operation_RUNNING && operationResource.GetStatus() != computepb.Operation_PENDING {
			break
		}
	}

	// If the operation failed, the HttpErrorStatusCode is set to a non-zero value
	if operationStatusCode := operationResource.GetHttpErrorStatusCode(); operationStatusCode != 0 {
		operationErrorMsg := operationResource.GetHttpErrorMessage()
		operationErrors := operationResource.GetError().GetErrors()
		return nil, fmt.Errorf("failed to insert image into GCE. HTTPErrorCode:%d HTTPErrorMsg:%v Errors:%v", operationStatusCode, operationErrorMsg, operationErrors)
	}

	getImageReq := &computepb.GetImageRequest{
		Image:   imageName,
		Project: g.GetProjectID(),
	}

	image, err := imagesClient.Get(ctx, getImageReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get information about the imported Image: %v", err)
	}

	return image, nil
}

// ComputeImageURL returns an image's URL to Google Cloud Console. The method does
// not check at all, if the image actually exists or not.
func (g *GCP) ComputeImageURL(imageName string) string {
	return fmt.Sprintf("https://console.cloud.google.com/compute/imagesDetail/projects/%s/global/images/%s", g.GetProjectID(), imageName)
}

// ComputeImageShare shares the specified Compute Engine image with list of accounts.
//
// "shareWith" is a list of accounts to share the image with. Items can be one
// of the following options:
//
//   - `user:{emailid}`: An email address that represents a specific
//     Google account. For example, `alice@example.com`.
//
//   - `serviceAccount:{emailid}`: An email address that represents a
//     service account. For example, `my-other-app@appspot.gserviceaccount.com`.
//
//   - `group:{emailid}`: An email address that represents a Google group.
//     For example, `admins@example.com`.
//
//   - `domain:{domain}`: The G Suite domain (primary) that represents all
//     the users of that domain. For example, `google.com` or `example.com`.
//
// Uses:
//   - Compute Engine API
func (g *GCP) ComputeImageShare(ctx context.Context, imageName string, shareWith []string) error {
	imagesClient, err := compute.NewImagesRESTClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("failed to get Compute Engine Images client: %v", err)
	}
	defer imagesClient.Close()

	// Standard role to enable account to view and use a specific Image
	imageDesiredRole := "roles/compute.imageUser"

	// Get the current Policy set on the Image
	getIamPolicyReq := &computepb.GetIamPolicyImageRequest{
		Project:  g.GetProjectID(),
		Resource: imageName,
	}
	policy, err := imagesClient.GetIamPolicy(ctx, getIamPolicyReq)
	if err != nil {
		return fmt.Errorf("failed to get image's policy: %v", err)
	}

	// Add new members, who can use the image
	// Completely override the old policy
	userBinding := &computepb.Binding{
		Members: shareWith,
		Role:    common.ToPtr(imageDesiredRole),
	}
	newPolicy := &computepb.Policy{
		Bindings: []*computepb.Binding{userBinding},
		Etag:     policy.Etag,
	}
	setIamPolicyReq := &computepb.SetIamPolicyImageRequest{
		Project:  g.GetProjectID(),
		Resource: imageName,
		GlobalSetPolicyRequestResource: &computepb.GlobalSetPolicyRequest{
			Policy: newPolicy,
		},
	}
	_, err = imagesClient.SetIamPolicy(ctx, setIamPolicyReq)
	if err != nil {
		return fmt.Errorf("failed to set new image policy: %v", err)
	}

	// Users won't see the shared image in their images.list requests, unless
	// they are also granted a specific "imagesList" role on the project. If you
	// don't need users to be able to view the list of shared images, this
	// step can be skipped.
	//
	// Downside of granting the "imagesList" role to a project is that the user
	// will be able to list all available images in the project, even those that
	// they can't use because of insufficient permissions.
	//
	// Even without the ability to view / list shared images, the user can still
	// create a Compute Engine instance using the image via API or 'gcloud' tool.
	//
	// Custom role to enable account to only list images in the project.
	// Without this role, the account won't be able to list and see the image
	// in the GCP Web UI.

	// For now, the decision is that the account should not get any role to the
	// project, where the image has been imported.

	return nil
}

// ComputeImageDelete deletes a Compute Engine image with the given name. If the
// image existed and was successfully deleted, no error is returned.
//
// Uses:
//   - Compute Engine API
func (g *GCP) ComputeImageDelete(ctx context.Context, name string) error {
	imagesClient, err := compute.NewImagesRESTClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("failed to get Compute Engine Images client: %v", err)
	}
	defer imagesClient.Close()

	req := &computepb.DeleteImageRequest{
		Project: g.GetProjectID(),
		Image:   name,
	}
	_, err = imagesClient.Delete(ctx, req)

	return err
}

// ComputeExecuteFunctionForImages will pass all the compute images in the account to a function,
// which is able to iterate over the images. Useful if something needs to be execute for each image.
// Uses:
//   - Compute Engine API
func (g *GCP) ComputeExecuteFunctionForImages(ctx context.Context, f func(*compute.ImageIterator) error) error {
	imagesClient, err := compute.NewImagesRESTClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("failed to get Compute Engine Images client: %v", err)
	}
	defer imagesClient.Close()

	req := &computepb.ListImagesRequest{
		Project: g.GetProjectID(),
	}
	imagesIterator := imagesClient.List(ctx, req)
	return f(imagesIterator)
}
