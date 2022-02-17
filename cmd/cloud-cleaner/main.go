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
	// api.sh chooses a random GCP Zone from the set Region. Since we
	// don't know which one it is, iterate over all Zones in the Region
	// and try to delete the instance. Unless the instance has set
	// "VmDnsSetting:ZonalOnly", which we don't do, this is safe and the
	// instance name must be unique for the whole GCP project.
	GCPZones, err := g.ComputeZonesInRegion(ctx, GCPRegion)
	if err != nil {
		log.Printf("[GCP] Error: Failed to get available Zones for the '%s' Region: %v", GCPRegion, err)
		return
	}
	for _, GCPZone := range GCPZones {
		log.Printf("[GCP] 🧹 Deleting VM instance %s in %s. "+
			"This should fail if the test succeeded.", GCPInstance, GCPZone)
		err = g.ComputeInstanceDelete(ctx, GCPZone, GCPInstance)
		if err == nil {
			// If an instance with the given name was successfully deleted in one of the Zones, we are done.
			break
		} else {
			log.Printf("[GCP] Error: %v", err)
		}
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
	log.Printf("[GCP] 🧹 Deleting image %s. This should fail if the test succeeded.", GCPImage)
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
	log.Println("[Azure] Deleting image. This should fail if the test succeeded.")
	err = azuretest.DeleteImageFromAzure(creds, imageName)
	if err != nil {
		log.Printf("[Azure] Error: %v", err)
	}

	// Delete all remaining resources (see the full list in the CleanUpBootedVM function)
	log.Println("[Azure] Cleaning up booted VM. This should fail if the test succeeded.")
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
	var wg sync.WaitGroup

	// Currently scheduled cloud-cleaner supports Azure only.
	// In case of scheduled cleanup get testID from env and run Azure cleanup.
	// If it's empty generate it and cleanup both GCP and Azure.
	testID := os.Getenv("TEST_ID")
	if testID == "" {
		testID, err := test.GenerateCIArtifactName("")
		if err != nil {
			log.Fatalf("Failed to get testID: %v", err)
		}
		log.Printf("TEST_ID=%s", testID)
		wg.Add(2)
		go cleanupAzure(testID, &wg)
		go cleanupGCP(testID, &wg)
		wg.Wait()
	} else {
		wg.Add(1)
		go cleanupAzure(testID, &wg)
		wg.Wait()
	}

}
