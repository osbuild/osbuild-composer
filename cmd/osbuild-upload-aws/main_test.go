package main

import (
	"os"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
	"github.com/stretchr/testify/require"
)

func TestMinio(t *testing.T) {
	endpoint := "http://127.0.0.1:9000"
	region := "eu-central-1"
	accessKeyID := "ACCESS_KEY_ID"
	secretAccessKey := "SECRET_ACCESS_KEY"

	a, err := awscloud.New(region, endpoint, accessKeyID, secretAccessKey, "")
	require.NoError(t, err)

	file, err := os.Create("minioTestUploadFile")
	require.NoError(t, err)
	defer os.Remove(file.Name())

	bucketName := "BUCKET_NAME"
	keyName := "KEY_NAME"

	_, err = a.Upload(file.Name(), bucketName, keyName)
	require.NoError(t, err)
}
