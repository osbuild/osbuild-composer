package gcp

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
	cloudbuildpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
	"google.golang.org/protobuf/types/known/durationpb"
)

// ComputeImageImport imports a previously uploaded image by submitting a Cloud Build API
// job. The job builds an image into Compute Engine from an image uploaded to the
// storage.
//
// The Build job usually creates a number of cache files in the Storage.
// This method does not do any cleanup, regardless if the image import succeeds or fails.
//
// To delete the Storage object (image) used for the image import, use StorageObjectDelete().
//
// To delete all potentially left over resources after the Build job, use CloudbuildBuildCleanup().
// This is especially important in case the image import is cancelled via the passed Context.
// Cancelling the build leaves behind all resources that it created - instances and disks.
// Therefore if you don't clean up the resources, they'll continue running and costing you money.
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
func (g *GCP) ComputeImageImport(ctx context.Context, bucket, object, imageName, os, region string) (*cloudbuildpb.Build, error) {
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

		// Set the region to be used by the daisy workflow when creating resources
		// If not specified, the workflow seems to always default to us-central1.

		// The Region passed as the argument is a Google Storage Region, which can be a multi or dual region.
		// Multi and Dual regions don't work with GCE API, therefore we need to get the list of GCE regions
		// that they map to. If the passed Region is not a multi or dual Region, then the returned slice contains
		// only the Region passed as an argument.
		gceRegions, err := g.storageRegionToComputeRegions(ctx, region)
		if err != nil {
			return nil, fmt.Errorf("failed to translate Google Storage Region to GCE Region: %v", err)
		}
		// Pick a random GCE Region to be used by the image import workflow
		gceRegionIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(gceRegions))))
		if err != nil {
			return nil, fmt.Errorf("failed to pick random GCE Region: %v", err)
		}
		// The expecation is that Google won't have more regions listed for multi/dual
		// regions than what can potentially fit into int32.
		gceRegion := gceRegions[int(gceRegionIndex.Int64())]

		availableZones, err := g.ComputeZonesInRegion(ctx, gceRegion)
		if err != nil {
			return nil, fmt.Errorf("failed to get available GCE Zones within Region '%s': %v", region, err)
		}
		// Pick random zone from the list
		gceZoneIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(availableZones))))
		if err != nil {
			return nil, fmt.Errorf("failed to pick random GCE Zone: %v", err)
		}
		// The expecation is that Google won't have more zones in a region than what can potentially fit into int32
		zone := availableZones[int(gceZoneIndex.Int64())]

		buildStepArgs = append(buildStepArgs, fmt.Sprintf("-zone=%s", zone))
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
	if err := resp.Metadata.UnmarshalTo(buildOpMetadata); err != nil {
		return nil, err
	}
	imageBuild = buildOpMetadata.Build

	getBuldReq := &cloudbuildpb.GetBuildRequest{
		ProjectId: imageBuild.ProjectId,
		Id:        imageBuild.Id,
	}

	// Wait for the build to finish
	for {
		select {
		case <-time.After(30 * time.Second):
			// Just check the build status below
		case <-ctx.Done():
			// cancel the build
			cancelBuildReq := &cloudbuildpb.CancelBuildRequest{
				ProjectId: imageBuild.ProjectId,
				Id:        imageBuild.Id,
			}
			// since the provided ctx has been canceled, create a new one to cancel the build
			ctx = context.Background()
			// Cancelling the build leaves behind all resources that it created (instances and disks)
			imageBuild, err = cloudbuildClient.CancelBuild(ctx, cancelBuildReq)
			if err != nil {
				return imageBuild, fmt.Errorf("failed to cancel the image import build job: %v", err)
			}
		}

		imageBuild, err = cloudbuildClient.GetBuild(ctx, getBuldReq)
		if err != nil {
			return imageBuild, fmt.Errorf("failed to get the build info: %v", err)
		}
		// The build finished
		if imageBuild.Status != cloudbuildpb.Build_WORKING && imageBuild.Status != cloudbuildpb.Build_QUEUED {
			break
		}
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
func (g *GCP) ComputeImageShare(ctx context.Context, imageName string, shareWith []string) error {
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
//	- Compute Engine API
func (g *GCP) ComputeImageDelete(ctx context.Context, resourceId string) error {
	computeService, err := compute.NewService(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("failed to get Compute Engine client: %v", err)
	}

	_, err = computeService.Images.Delete(g.creds.ProjectID, resourceId).Context(ctx).Do()

	return err
}

// ComputeExecuteFunctionForImages will pass all the compute images in the account to a function,
// which is able to iterate over the images. Useful if something needs to be execute for each image.
// Uses:
//	- Compute Engine API
func (g *GCP) ComputeExecuteFunctionForImages(ctx context.Context, f func(*compute.ImageList) error) error {
	computeService, err := compute.NewService(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("failed to get Compute Engine client: %v", err)
	}

	imagesService := compute.NewImagesService(computeService)
	return imagesService.List(g.creds.ProjectID).Pages(ctx, f)
}

// ComputeInstanceDelete deletes a Compute Engine instance with the given name and
// running in the given zone. If the instance existed and was successfully deleted,
// no error is returned.
//
// Uses:
//	- Compute Engine API
func (g *GCP) ComputeInstanceDelete(ctx context.Context, zone, instance string) error {
	computeService, err := compute.NewService(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("failed to get Compute Engine client: %v", err)
	}

	_, err = computeService.Instances.Delete(g.creds.ProjectID, zone, instance).Context(ctx).Do()

	return err
}

// ComputeInstanceGet fetches a Compute Engine instance information. If fetching the information
// was successful, it is returned to the caller, otherwise <nil> is returned with a proper error.
//
// Uses:
//	- Compute Engine API
func (g *GCP) ComputeInstanceGet(ctx context.Context, zone, instance string) (*compute.Instance, error) {
	computeService, err := compute.NewService(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return nil, fmt.Errorf("failed to get Compute Engine client: %v", err)
	}

	resp, err := computeService.Instances.Get(g.creds.ProjectID, zone, instance).Context(ctx).Do()

	return resp, err
}

// ComputeDiskDelete deletes a Compute Engine disk with the given name and
// running in the given zone. If the disk existed and was successfully deleted,
// no error is returned.
//
// Uses:
//	- Compute Engine API
func (g *GCP) ComputeDiskDelete(ctx context.Context, zone, disk string) error {
	computeService, err := compute.NewService(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return fmt.Errorf("failed to get Compute Engine client: %v", err)
	}

	_, err = computeService.Disks.Delete(g.creds.ProjectID, zone, disk).Context(ctx).Do()

	return err
}

// storageRegionToComputeRegion translates a Google Storage Region to GCE Region.
// This is useful mainly for multi and dual Storage Regions. For each valid multi
// or dual Region name, a slice with relevant GCE Regions is returned. If the
// Region provided as an argument is not multi or dual Region, a slice with the
// provided argument as the only item is returned.
//
// In general, Storage Regions correspond to the Compute Engine Regions. However,
// Storage allows also Multi and Dual regions, which must be mapped to GCE Regions,
// since these can not be used with GCE API calls.
//
// Uses:
//  - Compute Engine API
func (g *GCP) storageRegionToComputeRegions(ctx context.Context, region string) ([]string, error) {
	regionLower := strings.ToLower(region)

	// Handle Dual-Regions
	// https://cloud.google.com/storage/docs/locations#location-dr
	if regionLower == "asia1" {
		return []string{"asia-northeast1", "asia-northeast2"}, nil
	} else if regionLower == "eur4" {
		return []string{"europe-north1", "europe-west4"}, nil
	} else if regionLower == "nam4" {
		return []string{"us-central1", "us-east1"}, nil
	}

	// Handle Regular Region
	if regionLower != "asia" && regionLower != "eu" && regionLower != "us" {
		// Just return a slice with the region, which we got as
		return []string{regionLower}, nil
	}

	// Handle Multi-Regions
	// https://cloud.google.com/storage/docs/locations#location-mr
	computeService, err := compute.NewService(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return nil, fmt.Errorf("failed to get Compute Engine client: %v", err)
	}

	regionObjList, err := computeService.Regions.List(g.creds.ProjectID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get available Compute Engine Regions: %v", err)
	}

	regionsMap := make(map[string][]string)
	for _, regionObj := range regionObjList.Items {
		regionPrefix := strings.Split(regionObj.Name, "-")[0]
		regionsMap[regionPrefix] = append(regionsMap[regionPrefix], regionObj.Name)
	}

	switch regionLower {
	case "asia", "us":
		return regionsMap[regionLower], nil
	case "eu":
		var euRegions []string
		for _, euRegion := range regionsMap["europe"] {
			// "europe-west2" (London) and "europe-west6" (Zurich) are excluded
			// see https://cloud.google.com/storage/docs/locations#location-mr
			if euRegion != "europe-west2" && euRegion != "europe-west6" {
				euRegions = append(euRegions, euRegion)
			}
		}
		return euRegions, nil
	default:
		// This case should never happen, since the "default" case is handled above by
		// if regionLower != "asia" && regionLower != "eu" && regionLower != "us"
		return nil, fmt.Errorf("failed to translate Google Storage Region '%s' to Compute Engine Region", regionLower)
	}
}

// ComputeZonesInRegion returns list of zones within the given GCE Region, which are "UP".
//
// Uses:
//  - Compute Engine API
func (g *GCP) ComputeZonesInRegion(ctx context.Context, region string) ([]string, error) {
	var zones []string

	computeService, err := compute.NewService(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return nil, fmt.Errorf("failed to get Compute Engine client: %v", err)
	}

	// Get available zones in the given region
	regionObj, err := computeService.Regions.Get(g.creds.ProjectID, region).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get information about Compute Engine region '%s': %v", region, err)
	}

	for _, zoneURL := range regionObj.Zones {
		// zone URL example - "https://www.googleapis.com/compute/v1/projects/<PROJECT_ID>/zones/us-central1-a"
		zoneNameSs := strings.Split(zoneURL, "/")
		zoneName := zoneNameSs[len(zoneNameSs)-1]

		zoneObj, err := computeService.Zones.Get(g.creds.ProjectID, zoneName).Context(ctx).Do()
		if err != nil {
			return nil, fmt.Errorf("failed to get information about Compute Engine zone '%s': %v", zoneName, err)
		}

		// Make sure to return only Zones, which can be used
		if zoneObj.Status == "UP" {
			zones = append(zones, zoneName)
		}
	}

	return zones, nil
}
