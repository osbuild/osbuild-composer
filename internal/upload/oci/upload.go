package oci

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/oracle/oci-go-sdk/v54/common"
	"github.com/oracle/oci-go-sdk/v54/core"
	"github.com/oracle/oci-go-sdk/v54/identity"
	"github.com/oracle/oci-go-sdk/v54/objectstorage"
	"github.com/oracle/oci-go-sdk/v54/objectstorage/transfer"
	"github.com/oracle/oci-go-sdk/v54/workrequests"
)

type Uploader interface {
	Upload(name string, bucketName string, namespace string, file *os.File, user, compartment string) (string, error)
}

type ImageCreator interface {
	create(imageName, bucketName, namespace, compartmentID string) (string, error)
}

type Client struct {
	Uploader
	ImageCreator
	ociClient
}

// Upload uploads a file into an objectName under the bucketName in the namespace.
func (c Client) Upload(objectName string, bucketName string, namespace string, file *os.File, compartmentID, imageName string) (string, error) {
	err := c.uploadToBucket(objectName, bucketName, namespace, file)
	// clean up the object even if we fail
	defer func() {
		if err := c.deleteObjectFromBucket(objectName, bucketName, namespace); err != nil {
			log.Printf("failed to clean up the object '%s' from bucket '%s'", objectName, bucketName)
		}
	}()
	if err != nil {
		return "", err
	}

	imageID, err := c.createImage(objectName, bucketName, namespace, compartmentID, imageName)
	if err != nil {
		return "", fmt.Errorf("failed to create a custom image using object '%s' bucket '%s' in namespace '%s': %w",
			objectName,
			bucketName,
			namespace,
			err)
	}
	return imageID, nil
}

func (c Client) uploadToBucket(objectName string, bucketName string, namespace string, file *os.File) error {
	req := transfer.UploadFileRequest{
		UploadRequest: transfer.UploadRequest{
			NamespaceName: common.String(namespace),
			BucketName:    common.String(bucketName),
			ObjectName:    common.String(objectName),
			CallBack: func(multiPartUploadPart transfer.MultiPartUploadPart) {
				if multiPartUploadPart.Err != nil {
					log.Printf("upload failure: %s\n", multiPartUploadPart.Err)
				}
				log.Printf("multipart upload stats: parts %d, total-parts %d\n",
					multiPartUploadPart.PartNum,
					multiPartUploadPart.TotalParts)
			},
			ObjectStorageClient: &c.storageClient,
		},
		FilePath: file.Name(),
	}

	uploadManager := transfer.NewUploadManager()
	ctx := context.Background()
	resp, err := uploadManager.UploadFile(ctx, req)
	if err != nil {
		if resp.IsResumable() {
			resp, err = uploadManager.ResumeUploadFile(ctx, *resp.MultipartUploadResponse.UploadID)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("failed to upload the file to object %s:  %w", objectName, err)
		}
	}
	return nil
}

// Create creates an image from the storageObjectName stored in the bucketName.
// The result is an image ID or an error if the operation failed.
func (c Client) createImage(objectName, bucketName, namespace, compartmentID, imageName string) (string, error) {
	request := core.CreateImageRequest{
		CreateImageDetails: core.CreateImageDetails{
			DisplayName:   common.String(imageName),
			CompartmentId: common.String(compartmentID),
			FreeformTags: map[string]string{
				"Uploaded-By": "osbuild-composer",
				"Name":        imageName,
			},
			ImageSourceDetails: core.ImageSourceViaObjectStorageTupleDetails{
				BucketName:      common.String(bucketName),
				NamespaceName:   common.String(namespace),
				ObjectName:      common.String(objectName),
				SourceImageType: core.ImageSourceDetailsSourceImageTypeQcow2,
			},
		},
	}

	createImageResponse, err := c.computeClient.CreateImage(context.Background(), request)
	if err != nil {
		return "", fmt.Errorf("failed to create an image from storage object: %w", err)
	}

	log.Printf("waiting for the work request to complete, this may take a while. Work request ID: %s\n", *createImageResponse.OpcWorkRequestId)
	for {
		r, err := c.workRequestsClient.GetWorkRequest(context.Background(), workrequests.GetWorkRequestRequest{
			WorkRequestId: createImageResponse.OpcWorkRequestId,
			OpcRequestId:  createImageResponse.OpcRequestId,
		})
		if err != nil {
			return "", fmt.Errorf("failed to fetch the work request for creating the image: %w", err)
		}
		switch r.Status {
		case workrequests.WorkRequestStatusSucceeded:
			return *createImageResponse.Id, nil
		case workrequests.WorkRequestStatusCanceled, workrequests.WorkRequestStatusFailed:
			return "", fmt.Errorf("the work request for creating an image is in status %s", r.Status)
		}
		time.Sleep(1 * time.Second)
	}
}

type ClientParams struct {
	User        string
	Region      string
	Tenancy     string
	PrivateKey  string
	Fingerprint string
}

type ociClient struct {
	storageClient      objectstorage.ObjectStorageClient
	identityClient     identity.IdentityClient
	computeClient      core.ComputeClient
	workRequestsClient workrequests.WorkRequestClient
}

// deleteObjectFromBucket deletes the object by name from the bucket.
func (c Client) deleteObjectFromBucket(name string, bucket string, namespace string) error {
	_, err := c.storageClient.DeleteObject(context.Background(), objectstorage.DeleteObjectRequest{
		NamespaceName: common.String(namespace),
		BucketName:    common.String(bucket),
		ObjectName:    common.String(name),
	})
	return err
}

// NewClient creates a new oci client from the passed in params.
// Pass nil clientParams if you want the client to automatically detect
// the configuration from the official disk location or and env file
// From the docs its: $HOME/.oci/config, $HOME/.obmcs/config and variable
// names starting with TF_VAR.
// Last is the environment variable OCI_CONFIG_FILE
func NewClient(clientParams *ClientParams) (Client, error) {
	var configProvider common.ConfigurationProvider
	if clientParams != nil {
		configProvider = common.NewRawConfigurationProvider(
			clientParams.Tenancy,
			clientParams.User,
			clientParams.Region,
			clientParams.Fingerprint,
			clientParams.PrivateKey,
			nil,
		)

	} else {
		configProvider = common.DefaultConfigProvider()
	}
	storageClient, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(configProvider)
	// this disables the default 60 seconds timeout, to support big files upload (the common scenario)
	storageClient.HTTPClient = &http.Client{}
	if err != nil {
		return Client{}, fmt.Errorf("failed to create an Oracle objectstorage client: %w", err)
	}
	identityClient, err := identity.NewIdentityClientWithConfigurationProvider(configProvider)
	if err != nil {
		return Client{}, fmt.Errorf("failed to create an Oracle identity client: %w", err)
	}
	computeClient, err := core.NewComputeClientWithConfigurationProvider(configProvider)
	if err != nil {
		return Client{}, fmt.Errorf("failed to create an Oracle compute client: %w", err)
	}
	workRequestsClient, err := workrequests.NewWorkRequestClientWithConfigurationProvider(configProvider)
	if err != nil {
		return Client{}, fmt.Errorf("failed to create an Oracle workrequests client: %w", err)
	}
	return Client{ociClient: ociClient{
		storageClient:      storageClient,
		identityClient:     identityClient,
		computeClient:      computeClient,
		workRequestsClient: workRequestsClient,
	}}, nil
}
