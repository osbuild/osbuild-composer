// +build integration

package main

import (
	"context"
	"net/http"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/cloudapi"
	"github.com/stretchr/testify/require"
	"github.com/google/uuid"
)

func TestCloud(t *testing.T) {
	client, err := cloudapi.NewClientWithResponses("http://127.0.0.1:8703/")
	if err != nil {
		panic(err)
	}

	response, err := client.ComposeWithResponse(context.Background(), cloudapi.ComposeJSONRequestBody{
		Distribution: "rhel-8",
		ImageRequests: []cloudapi.ImageRequest{
			{
				Architecture: "x86_64",
				ImageType:    "qcow2",
				Repositories: []cloudapi.Repository{
					{
						Baseurl: "https://cdn.redhat.com/content/dist/rhel8/8/x86_64/baseos/os",
					},
					{
						Baseurl: "https://cdn.redhat.com/content/dist/rhel8/8/x86_64/appstream/os",
					},
				},
				UploadRequests: []cloudapi.UploadRequest{
					{
						Options: cloudapi.AWSUploadRequestOptions{
							Ec2: cloudapi.AWSUploadRequestOptionsEc2{
								AccessKeyId: "access-key-id",
								SecretAccessKey: "my-secret-key",
							},
							Region: "eu-central-1",
							S3: cloudapi.AWSUploadRequestOptionsS3{
								AccessKeyId: "access-key-id",
								SecretAccessKey: "my-secret-key",
								Bucket: "bucket",
							},
						},
						Type: "aws",
					},
				},
			},
		},
		Customizations: &cloudapi.Customizations{
			Subscription: &cloudapi.Subscription {
				ActivationKey: "somekey",
				BaseUrl: "http://cdn.stage.redhat.com/",
				ServerUrl: "subscription.rhsm.stage.redhat.com",
				Organization: 00000,
				Insights: true,
			},
		},
	})

	require.NoError(t, err)
	require.Equalf(t, http.StatusCreated, response.StatusCode(), "Error: got non-201 status. Full response: %v", string(response.Body))
	require.NotNil(t, response.JSON201)

	response2, err := client.ComposeStatusWithResponse(context.Background(), response.JSON201.Id)
	require.NoError(t, err)
	require.Equalf(t, response2.StatusCode(), http.StatusOK, "Error: got non-200 status. Full response: %v", response2.Body)

	response2, err = client.ComposeStatusWithResponse(context.Background(), "invalid-id")
	require.NoError(t, err)
	require.Equalf(t, response2.StatusCode(), http.StatusBadRequest, "Error: got non-400 status. Full response: %v", response2.Body)

	response2, err = client.ComposeStatusWithResponse(context.Background(), uuid.New().String())
	require.NoError(t, err)
	require.Equalf(t, response2.StatusCode(), http.StatusNotFound, "Error: got non-404 status. Full response: %s", response2.Body)
}
