package gcp

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	cloudbuildpb "google.golang.org/genproto/googleapis/devtools/cloudbuild/v1"
)

// Structure with resources created by the Build job
// Intended only for internal use
type cloudbuildBuildResources struct {
	zone             string
	computeInstances []string
	computeDisks     []string
	storageCacheDir  struct {
		bucket string
		dir    string
	}
}

// CloudbuildBuildLog fetches the log for the provided Build ID and returns it as a string
//
// Uses:
//	- Storage API
//	- Cloud Build API
func (g *GCP) CloudbuildBuildLog(ctx context.Context, buildID string) (string, error) {
	cloudbuildClient, err := cloudbuild.NewClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return "", fmt.Errorf("failed to get Cloud Build client: %v", err)
	}
	defer cloudbuildClient.Close()

	storageClient, err := storage.NewClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return "", fmt.Errorf("failed to get Storage client: %v", err)
	}
	defer storageClient.Close()

	getBuldReq := &cloudbuildpb.GetBuildRequest{
		ProjectId: g.creds.ProjectID,
		Id:        buildID,
	}

	imageBuild, err := cloudbuildClient.GetBuild(ctx, getBuldReq)
	if err != nil {
		return "", fmt.Errorf("failed to get the build info: %v", err)
	}

	// Determine the log file's Bucket and Object name
	// Logs_bucket example: "gs://550072179371.cloudbuild-logs.googleusercontent.com"
	// Logs file names will be of the format `${logs_bucket}/log-${build_id}.txt`
	logBucket := imageBuild.LogsBucket
	logBucket = strings.TrimPrefix(logBucket, "gs://")
	// logBucket may contain directory in its name if set to a custom value
	var logObjectDir string
	if strings.Contains(logBucket, "/") {
		ss := strings.SplitN(logBucket, "/", 2)
		logBucket = ss[0]
		logObjectDir = fmt.Sprintf("%s/", ss[1])
	}
	logObject := fmt.Sprintf("%slog-%s.txt", logObjectDir, buildID)

	// Read the log
	logBuilder := new(strings.Builder)
	rd, err := storageClient.Bucket(logBucket).Object(logObject).NewReader(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to create a new Reader for object '%s/%s': %v", logBucket, logObject, err)
	}
	_, err = io.Copy(logBuilder, rd)
	if err != nil {
		return "", fmt.Errorf("reading data from object '%s/%s' failed: %v", logBucket, logObject, err)
	}

	return logBuilder.String(), nil
}

// CloudbuildBuildCleanup parses the logs for the specified Build job and tries to clean up all resources
// which were created as part of the job. It returns list of strings with all resources that were deleted
// as a result of calling this method.
//
// Uses:
//	- Storage API
//	- Cloud Build API
//	- Compute Engine API (indirectly)
func (g *GCP) CloudbuildBuildCleanup(ctx context.Context, buildID string) ([]string, error) {
	var deletedResources []string

	storageClient, err := storage.NewClient(ctx, option.WithCredentials(g.creds))
	if err != nil {
		return deletedResources, fmt.Errorf("failed to get Storage client: %v", err)
	}
	defer storageClient.Close()

	buildLog, err := g.CloudbuildBuildLog(ctx, buildID)
	if err != nil {
		return deletedResources, fmt.Errorf("failed to get log for build ID '%s': %v", buildID, err)
	}

	resources, err := cloudbuildResourcesFromBuildLog(buildLog)
	if err != nil {
		return deletedResources, fmt.Errorf("extracting created resources from build log failed: %v", err)
	}

	// Delete all Compute Engine instances
	for _, instance := range resources.computeInstances {
		err = g.ComputeInstanceDelete(ctx, resources.zone, instance)
		if err == nil {
			deletedResources = append(deletedResources, fmt.Sprintf("instance: %s (%s)", instance, resources.zone))
		}
	}

	// Deleting instances in reality takes some time. Deleting a disk while it is still used by the instance, will fail.
	// Iterate over the list of instances and wait until they are all deleted.
	for _, instance := range resources.computeInstances {
		for {
			instanceInfo, err := g.ComputeInstanceGet(ctx, resources.zone, instance)
			// Getting the instance information failed, it is ideleted.
			if err != nil {
				break
			}
			// Prevent an unlikely infinite loop of waiting on deletion of an instance which can't be deleted.
			if instanceInfo.GetDeletionProtection() {
				break
			}
			time.Sleep(1 * time.Second)
		}
	}

	// Delete all Compute Engine Disks
	for _, disk := range resources.computeDisks {
		err = g.ComputeDiskDelete(ctx, resources.zone, disk)
		if err == nil {
			deletedResources = append(deletedResources, fmt.Sprintf("disk: %s (%s)", disk, resources.zone))
		}
	}

	// Delete all Storage cache files
	bucket := storageClient.Bucket(resources.storageCacheDir.bucket)
	objects := bucket.Objects(ctx, &storage.Query{Prefix: resources.storageCacheDir.dir})
	for {
		objAttrs, err := objects.Next()
		if err == iterator.Done || err == storage.ErrBucketNotExist {
			break
		}
		if err != nil {
			// Do not return, just continue with the next object
			continue
		}

		object := storageClient.Bucket(objAttrs.Bucket).Object(objAttrs.Name)
		if err = object.Delete(ctx); err == nil {
			deletedResources = append(deletedResources, fmt.Sprintf("storage object: %s/%s", objAttrs.Bucket, objAttrs.Name))
		}
	}

	return deletedResources, nil
}

// cloudbuildResourcesFromBuildLog parses the provided Cloud Build log for any
// resources that were created by the job as part of its work. The list of extracted
// resources is returned as cloudbuildBuildResources struct instance
func cloudbuildResourcesFromBuildLog(buildLog string) (*cloudbuildBuildResources, error) {
	var resources cloudbuildBuildResources

	// extract the used zone
	// [inflate]: 2021-02-17T12:42:10Z Workflow Zone: europe-west1-b
	zoneRe, err := regexp.Compile(`(?m)^.+Workflow Zone: (?P<zone>.+)$`)
	if err != nil {
		return &resources, err
	}
	zoneMatch := zoneRe.FindStringSubmatch(buildLog)
	if zoneMatch != nil {
		resources.zone = zoneMatch[1]
	}

	// extract Storage cache directory
	// [inflate]: 2021-03-12T13:13:10Z Workflow GCSPath: gs://ascendant-braid-303513-daisy-bkt-us-central1/gce-image-import-2021-03-12T13:13:08Z-btgtd
	cacheDirRe, err := regexp.Compile(`(?m)^.+Workflow GCSPath: gs://(?P<bucket>.+)/(?P<dir>.+)$`)
	if err != nil {
		return &resources, err
	}
	cacheDirMatch := cacheDirRe.FindStringSubmatch(buildLog)
	if cacheDirMatch != nil {
		resources.storageCacheDir.bucket = cacheDirMatch[1]
		resources.storageCacheDir.dir = cacheDirMatch[2]
	}

	// extract Compute disks
	// [inflate.setup-disks]: 2021-03-12T13:13:11Z CreateDisks: Creating disk "disk-importer-inflate-7366y".
	// [inflate.setup-disks]: 2021-03-12T13:13:11Z CreateDisks: Creating disk "disk-inflate-scratch-7366y".
	// [inflate.setup-disks]: 2021-03-12T13:13:11Z CreateDisks: Creating disk "disk-btgtd".
	// [shadow-disk-checksum.create-disks]: 2021-03-12T17:29:54Z CreateDisks: Creating disk "disk-shadow-disk-checksum-shadow-disk-checksum-r3qxv".
	disksRe, err := regexp.Compile(`(?m)^.+CreateDisks: Creating disk "(?P<disk>.+)".*$`)
	if err != nil {
		return &resources, err
	}
	disksMatches := disksRe.FindAllStringSubmatch(buildLog, -1)
	for _, disksMatch := range disksMatches {
		diskName := disksMatch[1]
		if diskName != "" {
			resources.computeDisks = append(resources.computeDisks, diskName)
		}
	}

	// extract Compute instances
	// [inflate.import-virtual-disk]: 2021-03-12T13:13:12Z CreateInstances: Creating instance "inst-importer-inflate-7366y".
	// [shadow-disk-checksum.create-instance]: 2021-03-12T17:29:55Z CreateInstances: Creating instance "inst-shadow-disk-checksum-shadow-disk-checksum-r3qxv".
	instancesRe, err := regexp.Compile(`(?m)^.+CreateInstances: Creating instance "(?P<instance>.+)".*$`)
	if err != nil {
		return &resources, err
	}
	instancesMatches := instancesRe.FindAllStringSubmatch(buildLog, -1)
	for _, instanceMatch := range instancesMatches {
		instanceName := instanceMatch[1]
		if instanceName != "" {
			resources.computeInstances = append(resources.computeInstances, instanceName)
		}
	}

	return &resources, nil
}
