// +build integration

package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/Azure/go-autorest/autorest/azure/auth"

	"github.com/osbuild/osbuild-composer/internal/boot/azuretest"
	"github.com/osbuild/osbuild-composer/internal/cloud/gcp"
	"github.com/osbuild/osbuild-composer/internal/test"
)

func cleanupGCP(testID string, wg *sync.WaitGroup) {
	defer wg.Done()

	log.Println("[GCP] Running clean up")

	GCPRegion, ok := os.LookupEnv("GCP_REGION")
	if !ok {
		log.Println("[GCP] Error: 'GCP_REGION' is not set in the environment.")
		return
	}
	// api.sh test uses '--zone="$GCP_REGION-a"'
	GCPZone := fmt.Sprintf("%s-a", GCPRegion)
	GCPBucket, ok := os.LookupEnv("GCP_BUCKET")
	if !ok {
		log.Println("[GCP] Error: 'GCP_BUCKET' is not set in the environment.")
		return
	}
	// max 62 characters
	// Must be a match of regex '[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?|[1-9][0-9]{0,19}'
	// use sha224sum to get predictable testID without invalid characters
	testIDhash := fmt.Sprintf("%x", sha256.Sum224([]byte(testID)))

	// Resource names to clean up
	GCPInstance := fmt.Sprintf("vm-%s", testIDhash)
	GCPImage := fmt.Sprintf("image-%s", testIDhash)

	// It does not matter if there was any error. If the credentials file was
	// read successfully then 'creds' should be non-nil, otherwise it will be
	// nil. Both values are acceptable for creating a new "GCP" instance.
	// If 'creds' is nil, then GCP library will try to authenticate using
	// the instance permissions.
	creds, err := gcp.GetCredentialsFromEnv()
	if err != nil {
		log.Printf("[GCP] Error: %v. This may not be an issue.", err)
	}

	// If this fails, there is no point in continuing
	g, err := gcp.New(creds)
	if err != nil {
		log.Printf("[GCP] Error: %v", err)
		return
	}

	ctx := context.Background()

	// Try to delete potentially running instance
	log.Printf("[GCP] 🧹 Deleting VM instance %s in %s. "+
		"This should fail if the test succedded.", GCPInstance, GCPZone)
	err = g.ComputeInstanceDelete(ctx, GCPZone, GCPInstance)
	if err != nil {
		log.Printf("[GCP] Error: %v", err)
	}

	// Try to clean up storage of cache objects after image import job
	log.Println("[GCP] 🧹 Cleaning up cache objects from storage after image " +
		"import. This should fail if the test succedded.")
	cacheObjects, errs := g.StorageImageImportCleanup(ctx, GCPImage)
	for _, err = range errs {
		log.Printf("[GCP] Error: %v", err)
	}
	for _, cacheObject := range cacheObjects {
		log.Printf("[GCP] 🧹 Deleted image import job file %s", cacheObject)
	}

	// Try to find the potentially uploaded Storage objects using custom metadata
	objects, err := g.StorageListObjectsByMetadata(ctx, GCPBucket, map[string]string{gcp.MetadataKeyImageName: GCPImage})
	if err != nil {
		log.Printf("[GCP] Error: %v", err)
	}
	for _, obj := range objects {
		if err = g.StorageObjectDelete(ctx, obj.Bucket, obj.Name); err != nil {
			log.Printf("[GCP] Error: %v", err)
		}
		log.Printf("[GCP] 🧹 Deleted object %s/%s related to build of image %s", obj.Bucket, obj.Name, GCPImage)
	}

	// Try to delete the imported image
	log.Printf("[GCP] 🧹 Deleting image %s. This should fail if the test succedded.", GCPImage)
	err = g.ComputeImageDelete(ctx, GCPImage)
	if err != nil {
		log.Printf("[GCP] Error: %v", err)
	}
}

func cleanupAzure(testID string, wg *sync.WaitGroup) {
	defer wg.Done()

	log.Println("[Azure] Running clean up")

	// Load Azure credentials
	creds, err := azuretest.GetAzureCredentialsFromEnv()
	if err != nil {
		log.Printf("[Azure] Error: %v", err)
		return
	}
	if creds == nil {
		log.Println("[Azure] Error: empty credentials")
		return
	}

	// Delete the vhd image
	imageName := "image-" + testID + ".vhd"
	log.Println("[Azure] Deleting image. This should fail if the test succedded.")
	err = azuretest.DeleteImageFromAzure(creds, imageName)
	if err != nil {
		log.Printf("[Azure] Error: %v", err)
	}

	// Delete all remaining resources (see the full list in the CleanUpBootedVM function)
	log.Println("[Azure] Cleaning up booted VM. This should fail if the test succedded.")
	parameters := azuretest.NewDeploymentParameters(creds, imageName, testID, "")
	clientCredentialsConfig := auth.NewClientCredentialsConfig(creds.ClientID, creds.ClientSecret, creds.TenantID)
	authorizer, err := clientCredentialsConfig.Authorizer()
	if err != nil {
		log.Printf("[Azure] Error: %v", err)
		return
	}

	err = azuretest.CleanUpBootedVM(creds, parameters, authorizer, testID)
	if err != nil {
		log.Printf("[Azure] Error: %v", err)
	}
}

func main() {
	log.Println("Running a cloud cleanup")

	// Get test ID
	testID, err := test.GenerateCIArtifactName("")
	if err != nil {
		log.Fatalf("Failed to get testID: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go cleanupAzure(testID, &wg)
	go cleanupGCP(testID, &wg)
	wg.Wait()
}
