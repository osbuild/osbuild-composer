// +build integration

package azuretest

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2019-09-01/network"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2019-05-01/resources"
	"github.com/Azure/azure-storage-blob-go/azblob"
	"github.com/Azure/go-autorest/autorest/azure/auth"

	"github.com/osbuild/osbuild-composer/internal/upload/azure"
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
	azure.Credentials
	ContainerName  string
	SubscriptionID string
	ClientID       string
	ClientSecret   string
	TenantID       string
	Location       string
	ResourceGroup  string
}

// getAzureCredentialsFromEnv gets the credentials from environment variables
// If none of the environment variables is set, it returns nil.
// If some but not all environment variables are set, it returns an error.
func GetAzureCredentialsFromEnv() (*azureCredentials, error) {
	storageAccount, saExists := os.LookupEnv("AZURE_STORAGE_ACCOUNT")
	storageAccessKey, sakExists := os.LookupEnv("AZURE_STORAGE_ACCESS_KEY")
	containerName, cExists := os.LookupEnv("AZURE_CONTAINER_NAME")
	subscriptionId, siExists := os.LookupEnv("AZURE_SUBSCRIPTION_ID")
	clientId, ciExists := os.LookupEnv("AZURE_CLIENT_ID")
	clientSecret, csExists := os.LookupEnv("AZURE_CLIENT_SECRET")
	tenantId, tiExists := os.LookupEnv("AZURE_TENANT_ID")
	location, lExists := os.LookupEnv("AZURE_LOCATION")
	resourceGroup, rgExists := os.LookupEnv("AZURE_RESOURCE_GROUP")

	// Workaround Travis security feature. If non of the variables is set, just ignore the test
	if !saExists && !sakExists && !cExists && !siExists && !ciExists && !csExists && !tiExists && !lExists && !rgExists {
		return nil, nil
	}
	// If only one/two of them are not set, then fail
	if !saExists || !sakExists || !cExists || !siExists || !ciExists || !csExists || !tiExists || !lExists || !rgExists {
		return nil, errors.New("not all required env variables were set")
	}

	return &azureCredentials{
		Credentials: azure.Credentials{
			StorageAccount:   storageAccount,
			StorageAccessKey: storageAccessKey,
		},
		ContainerName:  containerName,
		SubscriptionID: subscriptionId,
		ClientID:       clientId,
		ClientSecret:   clientSecret,
		TenantID:       tenantId,
		Location:       location,
		ResourceGroup:  resourceGroup,
	}, nil
}

// UploadImageToAzure mimics the upload feature of osbuild-composer.
func UploadImageToAzure(c *azureCredentials, imagePath string, imageName string) error {
	metadata := azure.ImageMetadata{
		ContainerName: c.ContainerName,
		ImageName:     imageName,
	}
	err := azure.UploadImage(c.Credentials, metadata, imagePath, 16)
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

	p := azblob.NewPipeline(credential, azblob.PipelineOptions{})

	// get storage account blob service URL endpoint.
	URL, _ := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", c.StorageAccount, c.ContainerName))

	// Create a ContainerURL object that wraps the container URL and a request
	// pipeline to make requests.
	containerURL := azblob.NewContainerURL(*URL, p)

	// Create the container, use a never-expiring context
	ctx := context.Background()

	blobURL := containerURL.NewPageBlobURL(imageName)

	_, err = blobURL.Delete(ctx, azblob.DeleteSnapshotsOptionInclude, azblob.BlobAccessConditions{})

	if err != nil {
		return fmt.Errorf("cannot delete the image: %v", err)
	}

	return nil
}

type resourceType struct {
	resType    string
	apiVersion string
}

// resourcesTypesToDelete serves two purposes:
// 1) The WithBootedImageInAzure method tags all the created resources and
//    we can get the list of resources with that tag. However, it's needed to
//    delete them in right order because of inner dependencies.
// 2) The resources.Client.DeleteByID method requires the API version to be
//    passed in. Therefore we need to way to get API version for a given
//    resource type.
var resourcesTypesToDelete = []resourceType{
	{
		resType:    "Microsoft.Compute/virtualMachines",
		apiVersion: "2019-07-01",
	},
	{
		resType:    "Microsoft.Network/networkInterfaces",
		apiVersion: "2019-09-01",
	},
	{
		resType:    "Microsoft.Network/publicIPAddresses",
		apiVersion: "2019-09-01",
	},
	{
		resType:    "Microsoft.Network/networkSecurityGroups",
		apiVersion: "2019-09-01",
	},
	{
		resType:    "Microsoft.Network/virtualNetworks",
		apiVersion: "2019-09-01",
	},
	{
		resType:    "Microsoft.Compute/disks",
		apiVersion: "2019-07-01",
	},
	{
		resType:    "Microsoft.Compute/images",
		apiVersion: "2019-07-01",
	},
}

// readPublicKey reads the public key from a file and returns it as a string
func readPublicKey(publicKeyFile string) (string, error) {
	publicKey, err := ioutil.ReadFile(publicKeyFile)
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

// withBootedImageInAzure runs the function f in the context of booted
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

	// Azure requires a lot of names - for a virtual machine, a virtual network,
	// a virtual interface and so on and so forth.
	// Let's create all of them here from the test id.
	deploymentName := testId
	tag := "tag-" + testId
	imagePath := fmt.Sprintf("https://%s.blob.core.windows.net/%s/%s", creds.StorageAccount, creds.ContainerName, imageName)

	parameters := deploymentParameters{
		NetworkInterfaceName:     newDeploymentParameter("iface-" + testId),
		NetworkSecurityGroupName: newDeploymentParameter("nsg-" + testId),
		VirtualNetworkName:       newDeploymentParameter("vnet-" + testId),
		PublicIPAddressName:      newDeploymentParameter("ip-" + testId),
		VirtualMachineName:       newDeploymentParameter("vm-" + testId),
		DiskName:                 newDeploymentParameter("disk-" + testId),
		ImageName:                newDeploymentParameter("image-" + testId),
		Tag:                      newDeploymentParameter(tag),
		Location:                 newDeploymentParameter(creds.Location),
		ImagePath:                newDeploymentParameter(imagePath),
		AdminUsername:            newDeploymentParameter("redhat"),
		AdminPublicKey:           newDeploymentParameter(publicKey),
	}

	deploymentsClient := resources.NewDeploymentsClient(creds.SubscriptionID)
	deploymentsClient.Authorizer = authorizer

	deploymentFuture, err := deploymentsClient.CreateOrUpdate(context.Background(), creds.ResourceGroup, deploymentName, resources.Deployment{
		Properties: &resources.DeploymentProperties{
			Mode:       resources.Incremental,
			Template:   template,
			Parameters: parameters,
		},
	})

	// Let's registed the clean-up function as soon as possible.
	defer func() {
		resourcesClient := resources.NewClient(creds.SubscriptionID)
		resourcesClient.Authorizer = authorizer

		// find all the resources we marked with a tag during the deployment
		filter := fmt.Sprintf("tagName eq 'osbuild-composer-image-test' and tagValue eq '%s'", tag)
		resourceList, err := resourcesClient.ListByResourceGroup(context.Background(), creds.ResourceGroup, filter, "", nil)
		if err != nil {
			retErr = wrapErrorf(retErr, "listing of resources failed: %v", err)
		} else {

			// delete all the found resources
			for _, resourceType := range resourcesTypesToDelete {
				for _, resource := range resourceList.Values() {
					if *resource.Type != resourceType.resType {
						continue
					}

					err := deleteResource(resourcesClient, *resource.ID, resourceType.apiVersion)
					if err != nil {
						retErr = wrapErrorf(retErr, "cannot delete the resource %s: %v", *resource.ID, err)
						// do not return here, try deleting as much as possible
					}
				}
			}
		}

		// Delete the deployment
		// This actually does not delete any resources created by the
		// deployment as one might think. Therefore the code above
		// and the tagging are needed.
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
