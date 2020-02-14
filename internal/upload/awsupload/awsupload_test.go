package awsupload_test

import (
	"bytes"
	"fmt"
	"github.com/osbuild/osbuild-composer/internal/upload/awsupload"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

func handleErrors(t *testing.T, err error) {
	if err != nil {
		t.Fatal(err)
	}
}

func loadEnvVar(t *testing.T, envVarName string) (string, bool) {
	variable, exists := os.LookupEnv(envVarName)
	if !exists {
		t.Logf("Environment variable does not exist: %s", envVarName)
		return "", false
	}
	return variable, true
}

func generateRandomFile(pattern string, length int) (string, []byte, error) {
	f, err := ioutil.TempFile("/tmp", pattern)
	if err != nil {
		return "", []byte{}, err
	}
	fileName := f.Name()

	countArg := fmt.Sprintf("count=%d", length)
	cmd := exec.Command("dd", "bs=1", countArg, "if=/dev/urandom", fmt.Sprintf("of=%s", fileName))
	err = cmd.Run()

	if err != nil {
		return "", []byte{}, err
	}

	contents, err := ioutil.ReadAll(f)
	if err != nil {
		return "", []byte{}, err
	}

	if err := f.Close(); err != nil {
		return "", []byte{}, err
	}

	return fileName, contents, err
}

func loadTestSettings(t *testing.T) (string, string, string, string) {
	accessKeyId, akExists := loadEnvVar(t, "AWS_ACCESS_KEY_ID")
	secretAccessKey, sakExists := loadEnvVar(t, "AWS_SECRET_ACCESS_KEY")
	region, regionExists := loadEnvVar(t, "AWS_REGION")
	bucket, bucketExists := loadEnvVar(t, "AWS_BUCKET")

	// Workaround Travis security feature. If non of the variables is set, just ignore the test
	if !akExists && !sakExists && !bucketExists && !regionExists {
		t.Skip("No AWS configuration provided, assuming that this is running in CI. Skipping the test.")
	}
	// If only one/two of them are not set, then fail
	if !akExists || !sakExists || !bucketExists || !regionExists {
		t.Fatal("You need to define all variables for AWS connection.")
	}
	return accessKeyId, secretAccessKey, region, bucket
}

func TestAWS_S3Upload(t *testing.T) {
	// Load test settings
	accessKeyId, secretAccessKey, region, bucket := loadTestSettings(t)

	// There's no way to retrieve session from awsupload, therefore we need to create an extra one in tests
	sess, err := createSession(accessKeyId, secretAccessKey, region)
	handleErrors(t, err)

	// Generate random file to be uploaded
	fileLength := 512 * 512
	filePath, content, err := generateRandomFile("osbuild-composer-test-aws-upload-", fileLength)
	handleErrors(t, err)
	defer os.Remove(filePath)

	// Use filename as s3 key
	s3Key := path.Base(filePath)

	// Test uploader
	awsUpload, err := awsupload.New(region, accessKeyId, secretAccessKey)
	if err != nil {
		t.Fatalf("obtaining session failed: %s", err.Error())
	}
	handleErrors(t, err)

	_, err = awsUpload.Upload(filePath, bucket, s3Key)

	if err != nil {
		t.Fatalf("upload failed: %s", err.Error())
	}

	// Delete the object from S3 after the test is finished
	defer func() {
		s := s3.New(sess)
		s.DeleteObject(&s3.DeleteObjectInput{
			Bucket: &bucket,
			Key:    &s3Key,
		})
	}()

	// Set up temporary file for downloaded
	downloadFile, err := ioutil.TempFile("/tmp", "osbuild-composer-test-aws-download-")
	handleErrors(t, err)
	defer os.Remove(downloadFile.Name())

	// Download the object from S3
	downloader := s3manager.NewDownloader(sess)

	n, err := downloader.Download(downloadFile, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &s3Key,
	})

	if err != nil {
		t.Fatalf("downloader cannot download s3 object: %s", err.Error())
	}

	if n != int64(fileLength) {
		t.Fatalf("downloaded file length is wrong, expected: %d, got: %d", fileLength, n)
	}

	// Seek the downloaded file to the beginning
	_, err = downloadFile.Seek(0, 0)
	handleErrors(t, err)

	downloadedFileContent, err := ioutil.ReadAll(downloadFile)
	handleErrors(t, err)

	if !bytes.Equal(content, downloadedFileContent) {
		t.Errorf("downloaded file isn't matching the uploaded one")
	}
}

func createSession(accessKeyId string, secretAccessKey string, region string) (*session.Session, error) {
	creds := credentials.NewStaticCredentials(accessKeyId, secretAccessKey, "")

	return session.NewSession(&aws.Config{
		Credentials: creds,
		Region:      &region,
	})

}
