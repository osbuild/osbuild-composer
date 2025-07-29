//go:build integration

package azuretest

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	// NOTE these are deprecated and will need replacement, see issue #2977
	//nolint:staticcheck
	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-09-01/network"
	//nolint:staticcheck
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/go-autorest/autorest/azure/auth"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/images/pkg/upload/azure"
)

// wrapErrorf returns error constructed using fmt.Errorf from format and any
// other args. If innerError != nil, it's appended at the end of the new
// error.
func wrapErrorf(innerError error, format string, a ...interface{}) error {
	if innerError != nil {
		a = append(a, innerError)
		return fmt.Errorf(format+"\n\ninner error: %#s", a...)
	}

	return fmt.Errorf(format, a...)
}

type azureCredentials struct {
	StorageAccount   string
	StorageAccessKey string
	ContainerName    string
	SubscriptionID   string
	ClientID         string
	ClientSecret     string
	TenantID         string
	Location         string
	ResourceGroup    string
}

// getAzureCredentialsFromEnv gets the credentials from environment variables
// If none of the environment variables is set, it returns nil.
// If some but not all environment variables are set, it returns an error.
func GetAzureCredentialsFromEnv() (*azureCredentials, error) {
	storageAccount, saExists := os.LookupEnv("AZURE_STORAGE_ACCOUNT")
	storageAccessKey, sakExists := os.LookupEnv("AZURE_STORAGE_ACCESS_KEY")
	containerName, cExists := os.LookupEnv("AZURE_CONTAINER_NAME")
	subscriptionId, siExists := os.LookupEnv("AZURE_SUBSCRIPTION_ID")
	clientId, ciExists := os.LookupEnv("V2_AZURE_CLIENT_ID")
	clientSecret, csExists := os.LookupEnv("V2_AZURE_CLIENT_SECRET")
	tenantId, tiExists := os.LookupEnv("AZURE_TENANT_ID")
	location, lExists := os.LookupEnv("AZURE_LOCATION")
	resourceGroup, rgExists := os.LookupEnv("AZURE_RESOURCE_GROUP")

	// If non of the variables is set, just ignore the test
	if !saExists && !sakExists && !cExists && !siExists && !ciExists && !csExists && !tiExists && !lExists && !rgExists {
		return nil, nil
	}
	// If only one/two of them are not set, then fail
	if !saExists || !sakExists || !cExists || !siExists || !ciExists || !csExists || !tiExists || !lExists || !rgExists {
		return nil, errors.New("not all required env variables were set")
	}

	return &azureCredentials{
		StorageAccount:   storageAccount,
		StorageAccessKey: storageAccessKey,
		ContainerName:    containerName,
		SubscriptionID:   subscriptionId,
		ClientID:         clientId,
		ClientSecret:     clientSecret,
		TenantID:         tenantId,
		Location:         location,
		ResourceGroup:    resourceGroup,
	}, nil
}

// UploadImageToAzure mimics the upload feature of osbuild-composer.
func UploadImageToAzure(c *azureCredentials, imagePath string, imageName string) error {
	metadata := azure.BlobMetadata{
		StorageAccount: c.StorageAccount,
		ContainerName:  c.ContainerName,
		BlobName:       imageName,
	}
	client, err := azure.NewStorageClient(c.StorageAccount, c.StorageAccessKey)
	if err != nil {
		return err
	}
	err = client.UploadPageBlob(metadata, imagePath, 16)
	if err != nil {
		return fmt.Errorf("upload to azure failed: %v", err)
	}

	return nil
}

// DeleteImageFromAzure deletes the image uploaded by osbuild-composer
// (or UpluadImageToAzure method).
func DeleteImageFromAzure(c *azureCredentials, imageName string) error {
	// Create a default request pipeline using your storage account name and account key.
	credential, err := azblob.NewSharedKeyCredential(c.StorageAccount, c.StorageAccessKey)
	if err != nil {
		return err
	}

	// get blob URL endpoint.
	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", c.StorageAccount, c.ContainerName, imageName))

	client, err := blob.NewClientWithSharedKeyCredential(URL.String(), credential, nil)
	if err != nil {
		return fmt.Errorf("cannot create a new blob client: %w", err)
	}

	_, err = client.Delete(context.Background(), &blob.DeleteOptions{
		DeleteSnapshots: common.ToPtr(blob.DeleteSnapshotsOptionTypeInclude),
	})

	if err != nil {
		return fmt.Errorf("cannot delete the image: %v", err)
	}

	return nil
}

// readPublicKey reads the public key from a file and returns it as a string
func readPublicKey(publicKeyFile string) (string, error) {
	publicKey, err := os.ReadFile(publicKeyFile)
	if err != nil {
		return "", fmt.Errorf("cannot read the public key file: %v", err)
	}

	return string(publicKey), nil
}

// deleteResource is a convenient wrapper around Azure SDK to delete a resource
func deleteResource(client resources.Client, id string, apiVersion string) error {
	deleteFuture, err := client.DeleteByID(context.Background(), id, apiVersion)
	if err != nil {
		return fmt.Errorf("cannot delete the resourceType %s: %v", id, err)
	}

	err = deleteFuture.WaitForCompletionRef(context.Background(), client.BaseClient.Client)
	if err != nil {
		return fmt.Errorf("waiting for the resourceType %s deletion failed: %v", id, err)
	}

	_, err = deleteFuture.Result(client)
	if err != nil {
		return fmt.Errorf("cannot retrieve the result of %s deletion: %v", id, err)
	}

	return nil
}

func NewDeploymentParameters(creds *azureCredentials, imageName, testId, publicKey string) DeploymentParameters {
	// Azure requires a lot of names - for a virtual machine, a virtual network,
	// a virtual interface and so on and so forth.
	// Let's create all of them here from the test id so we can delete them
	// later.

	imagePath := fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", creds.StorageAccount, creds.ContainerName, imageName)

	return DeploymentParameters{
		NetworkInterfaceName:     newDeploymentParameter("iface-" + testId),
		NetworkSecurityGroupName: newDeploymentParameter("nsg-" + testId),
		VirtualNetworkName:       newDeploymentParameter("vnet-" + testId),
		PublicIPAddressName:      newDeploymentParameter("ip-" + testId),
		VirtualMachineName:       newDeploymentParameter("vm-" + testId),
		DiskName:                 newDeploymentParameter("disk-" + testId),
		ImageName:                newDeploymentParameter("image-" + testId),
		Location:                 newDeploymentParameter(creds.Location),
		ImagePath:                newDeploymentParameter(imagePath),
		AdminUsername:            newDeploymentParameter("redhat"),
		AdminPublicKey:           newDeploymentParameter(publicKey),
	}
}

func CleanUpBootedVM(creds *azureCredentials, parameters DeploymentParameters, authorizer autorest.Authorizer, testId string) (retErr error) {
	deploymentName := testId

	deploymentsClient := resources.NewDeploymentsClient(creds.SubscriptionID)
	deploymentsClient.Authorizer = authorizer

	resourcesClient := resources.NewClient(creds.SubscriptionID)
	resourcesClient.Authorizer = authorizer

	// This array specifies all the resources we need to delete. The
	// order is important, e.g. one cannot delete a network interface
	// that is still attached to a virtual machine.
	resourcesToDelete := []struct {
		resType    string
		name       string
		apiVersion string
	}{
		{
			resType:    "Microsoft.Compute/virtualMachines",
			name:       parameters.VirtualMachineName.Value,
			apiVersion: "2019-07-01",
		},
		{
			resType:    "Microsoft.Network/networkInterfaces",
			name:       parameters.NetworkInterfaceName.Value,
			apiVersion: "2019-09-01",
		},
		{
			resType:    "Microsoft.Network/publicIPAddresses",
			name:       parameters.PublicIPAddressName.Value,
			apiVersion: "2019-09-01",
		},
		{
			resType:    "Microsoft.Network/networkSecurityGroups",
			name:       parameters.NetworkSecurityGroupName.Value,
			apiVersion: "2019-09-01",
		},
		{
			resType:    "Microsoft.Network/virtualNetworks",
			name:       parameters.VirtualNetworkName.Value,
			apiVersion: "2019-09-01",
		},
		{
			resType:    "Microsoft.Compute/disks",
			name:       parameters.DiskName.Value,
			apiVersion: "2019-07-01",
		},
		{
			resType:    "Microsoft.Compute/images",
			name:       parameters.ImageName.Value,
			apiVersion: "2019-07-01",
		},
	}

	// Delete all the resources
	for _, resourceToDelete := range resourcesToDelete {
		resourceID := fmt.Sprintf(
			"subscriptions/%s/resourceGroups/%s/providers/%s/%s",
			creds.SubscriptionID,
			creds.ResourceGroup,
			resourceToDelete.resType,
			resourceToDelete.name,
		)

		err := deleteResource(resourcesClient, resourceID, resourceToDelete.apiVersion)
		if err != nil {
			log.Printf("deleting the resource %s errored: %v", resourceToDelete.name, err)
			retErr = wrapErrorf(retErr, "cannot delete the resource %s: %v", resourceToDelete.name, err)
			// do not return here, try deleting as much as possible
		}
	}

	// Delete the deployment
	// This actually does not delete any resources created by the
	// deployment as one might think. Therefore the code above
	// is needed.
	result, err := deploymentsClient.Delete(context.Background(), creds.ResourceGroup, deploymentName)
	if err != nil {
		retErr = wrapErrorf(retErr, "cannot create the request for the deployment deletion: %v", err)
		return
	}

	err = result.WaitForCompletionRef(context.Background(), deploymentsClient.Client)
	if err != nil {
		retErr = wrapErrorf(retErr, "waiting for the deployment deletion failed: %v", err)
		return
	}

	_, err = result.Result(deploymentsClient)
	if err != nil {
		retErr = wrapErrorf(retErr, "cannot retrieve the deployment deletion result: %v", err)
		return
	}
	return
}

// WithBootedImageInAzure runs the function f in the context of booted
// image in Azure
func WithBootedImageInAzure(creds *azureCredentials, imageName, testId, publicKeyFile string, f func(address string) error) (retErr error) {
	publicKey, err := readPublicKey(publicKeyFile)
	if err != nil {
		return err
	}

	clientCredentialsConfig := auth.NewClientCredentialsConfig(creds.ClientID, creds.ClientSecret, creds.TenantID)
	authorizer, err := clientCredentialsConfig.Authorizer()
	if err != nil {
		return fmt.Errorf("cannot create the authorizer: %v", err)
	}

	template, err := loadDeploymentTemplate()
	if err != nil {
		return err
	}

	deploymentsClient := resources.NewDeploymentsClient(creds.SubscriptionID)
	deploymentsClient.Authorizer = authorizer

	deploymentName := testId
	parameters := NewDeploymentParameters(creds, imageName, testId, publicKey)

	deploymentFuture, err := deploymentsClient.CreateOrUpdate(context.Background(), creds.ResourceGroup, deploymentName, resources.Deployment{
		Properties: &resources.DeploymentProperties{
			Mode:       resources.Incremental,
			Template:   template,
			Parameters: parameters,
		},
	})

	// Let's registed the clean-up function as soon as possible.
	defer func() {
		retErr = CleanUpBootedVM(creds, parameters, authorizer, testId)
	}()

	if err != nil {
		return fmt.Errorf("creating a deployment failed: %v", err)
	}

	err = deploymentFuture.WaitForCompletionRef(context.Background(), deploymentsClient.Client)
	if err != nil {
		return fmt.Errorf("waiting for deployment completion failed: %v", err)
	}

	_, err = deploymentFuture.Result(deploymentsClient)
	if err != nil {
		return fmt.Errorf("retrieving the deployment result failed: %v", err)
	}

	// get the IP address
	publicIPAddressClient := network.NewPublicIPAddressesClient(creds.SubscriptionID)
	publicIPAddressClient.Authorizer = authorizer

	publicIPAddress, err := publicIPAddressClient.Get(context.Background(), creds.ResourceGroup, parameters.PublicIPAddressName.Value, "")
	if err != nil {
		return fmt.Errorf("cannot get the ip address details: %v", err)
	}

	return f(*publicIPAddress.IPAddress)
}
