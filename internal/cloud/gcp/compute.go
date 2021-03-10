package gcp

import (
	"context"
	"fmt"
	"time"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	cloudbuildpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

// ComputeImageImport imports a previously uploaded image by submitting a Cloud Build API
// job. The job builds an image into Compute Node from an image uploaded to the
// storage.
//
// The Build job usually creates a number of cache files in the Storage.
// This method does not do any cleanup, regardless if the image import succeeds or fails.
//
// To delete the Storage object used for image import, use StorageObjectDelete().
// To cleanup cache files after the Build job, use StorageImageImportCleanup().
//
// bucket - Google storage bucket name with the uploaded image
// object - Google storage object name of the uploaded image
// imageName - Desired image name after the import. This must be unique within the whole project.
// os - Specifies the OS type used when installing GCP guest tools.
//      If empty (""), then the image is imported without the installation of GCP guest tools.
// 		Valid values are: centos-7, centos-8, debian-8, debian-9, opensuse-15, rhel-6,
//                        rhel-6-byol, rhel-7, rhel-7-byol, rhel-8, rhel-8-byol, sles-12,
//                        sles-12-byol, sles-15, sles-15-byol, sles-sap-12, sles-sap-12-byol,
//                        sles-sap-15, sles-sap-15-byol, ubuntu-1404, ubuntu-1604, ubuntu-1804,
//                        ubuntu-2004, windows-10-x64-byol, windows-10-x86-byol,
//                        windows-2008r2, windows-2008r2-byol, windows-2012, windows-2012-byol,
//                        windows-2012r2, windows-2012r2-byol, windows-2016, windows-2016-byol,
//                        windows-2019, windows-2019-byol, windows-7-x64-byol,
//                        windows-7-x86-byol, windows-8-x64-byol, windows-8-x86-byol
// region - A valid region where the resulting image should be located. If empty,
//          the multi-region location closest to the source is chosen automatically.
//          See: https://cloud.google.com/storage/docs/locations
//
// Uses:
//	- Cloud Build API
func (g *GCP) ComputeImageImport(bucket, object, imageName, os, region string) (*cloudbuildpb.Build, error) {
	ctx := context.Background()
	cloudbuildClient, err := cloudbuild.NewClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return nil, fmt.Errorf("failed to get Cloud Build client: %v", err)
	}
	defer cloudbuildClient.Close()

	buildStepArgs := []string{
		fmt.Sprintf("-source_file=gs://%s/%s", bucket, object),
		fmt.Sprintf("-image_name=%s", imageName),
		"-timeout=7000s",
		"-client_id=api",
	}
	if region != "" {
		buildStepArgs = append(buildStepArgs, fmt.Sprintf("-storage_location=%s", region))
	}
	if os != "" {
		buildStepArgs = append(buildStepArgs, fmt.Sprintf("-os=%s", os))
	} else {
		// This effectively marks the image as non-bootable for the import process,
		// but it has no effect on the later use or booting in Compute Engine other
		// than the GCP guest tools not being installed.
		buildStepArgs = append(buildStepArgs, "-data_disk")
	}

	imageBuild := &cloudbuildpb.Build{
		Steps: []*cloudbuildpb.BuildStep{{
			Name: "gcr.io/compute-image-tools/gce_vm_image_import:release",
			Args: buildStepArgs,
		}},
		Tags: []string{
			"gce-daisy",
			"gce-daisy-image-import",
		},
		Timeout: durationpb.New(time.Second * 7200),
	}

	createBuildReq := &cloudbuildpb.CreateBuildRequest{
		ProjectId: g.creds.ProjectID,
		Build:     imageBuild,
	}

	resp, err := cloudbuildClient.CreateBuild(ctx, createBuildReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create image import build job: %v", err)
	}

	// Get the returned Build struct
	buildOpMetadata := &cloudbuildpb.BuildOperationMetadata{}
	if err := ptypes.UnmarshalAny(resp.Metadata, buildOpMetadata); err != nil {
		return nil, err
	}
	imageBuild = buildOpMetadata.Build

	getBuldReq := &cloudbuildpb.GetBuildRequest{
		ProjectId: imageBuild.ProjectId,
		Id:        imageBuild.Id,
	}

	// Wait for the build to finish
	for {
		imageBuild, err = cloudbuildClient.GetBuild(ctx, getBuldReq)
		if err != nil {
			return imageBuild, fmt.Errorf("failed to get the build info: %v", err)
		}
		// The build finished
		if imageBuild.Status != cloudbuildpb.Build_WORKING && imageBuild.Status != cloudbuildpb.Build_QUEUED {
			break
		}
		time.Sleep(time.Second * 30)
	}

	if imageBuild.Status != cloudbuildpb.Build_SUCCESS {
		return imageBuild, fmt.Errorf("image import didn't finish successfully: %s", imageBuild.Status)
	}

	return imageBuild, nil
}

// ComputeImageURL returns an image's URL to Google Cloud Console. The method does
// not check at all, if the image actually exists or not.
func (g *GCP) ComputeImageURL(imageName string) string {
	return fmt.Sprintf("https://console.cloud.google.com/compute/imagesDetail/projects/%s/global/images/%s", g.creds.ProjectID, imageName)
}

// ComputeImageShare shares the specified Compute Engine image with list of accounts.
//
// "shareWith" is a list of accounts to share the image with. Items can be one
// of the following options:
//
// - `user:{emailid}`: An email address that represents a specific
//	 Google account. For example, `alice@example.com`.
//
// - `serviceAccount:{emailid}`: An email address that represents a
//   service account. For example, `my-other-app@appspot.gserviceaccount.com`.
//
// - `group:{emailid}`: An email address that represents a Google group.
//   For example, `admins@example.com`.
//
// - `domain:{domain}`: The G Suite domain (primary) that represents all
//   the users of that domain. For example, `google.com` or `example.com`.
//
// Uses:
//	- Compute Engine API
func (g *GCP) ComputeImageShare(imageName string, shareWith []string) error {
	ctx := context.Background()

	computeService, err := compute.NewService(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("failed to get Compute Engine client: %v", err)
	}

	// Standard role to enable account to view and use a specific Image
	imageDesiredRole := "roles/compute.imageUser"

	// Get the current Policy set on the Image
	policy, err := computeService.Images.GetIamPolicy(g.creds.ProjectID, imageName).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get image's policy: %v", err)
	}

	// Add new members, who can use the image
	// Completely override the old policy
	userBinding := &compute.Binding{
		Members: shareWith,
		Role:    imageDesiredRole,
	}
	newPolicy := &compute.Policy{
		Bindings: []*compute.Binding{userBinding},
		Etag:     policy.Etag,
	}
	req := &compute.GlobalSetPolicyRequest{
		Policy: newPolicy,
	}
	_, err = computeService.Images.SetIamPolicy(g.creds.ProjectID, imageName, req).Context(ctx).Do()
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
	// create a Compute Node instance using the image via API or 'gcloud' tool.
	//
	// Custom role to enable account to only list images in the project.
	// Without this role, the account won't be able to list and see the image
	// in the GCP Web UI.

	// For now, the decision is that the account should not get any role to the
	// project, where the image has been imported.

	return nil
}
