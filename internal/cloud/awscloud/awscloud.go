package awscloud

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	images_awscloud "github.com/osbuild/images/pkg/cloud/awscloud"
)

type AWS struct {
	// awscloud.AWS from the osbuild/images package implements all of the methods
	// related to image upload and sharing.
	*images_awscloud.AWS

	ec2     EC2
	ec2imds EC2Imds
	s3      S3
	asg     ASG
}

func newForTest(ec2cli EC2, ec2imds EC2Imds, s3cli S3) *AWS {
	return &AWS{
		ec2:     ec2cli,
		ec2imds: ec2imds,
		s3:      s3cli,
		asg:     nil,
	}
}

// Create a new session from the credentials and the region and returns an *AWS object initialized with it.
// /creds credentials.StaticCredentialsProvider, region string
func newAwsFromConfig(cfg aws.Config, imagesAWS *images_awscloud.AWS) *AWS {
	s3cli := s3.NewFromConfig(cfg)
	return &AWS{
		AWS:     imagesAWS,
		ec2:     ec2.NewFromConfig(cfg),
		ec2imds: imds.NewFromConfig(cfg),
		s3:      s3cli,
		asg:     autoscaling.NewFromConfig(cfg),
	}
}

// Initialize a new AWS object from individual bits. SessionToken is optional
func New(region string, accessKeyID string, accessKey string, sessionToken string) (*AWS, error) {
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, accessKey, sessionToken)),
	)
	if err != nil {
		return nil, err
	}

	imagesAWS, err := images_awscloud.New(region, accessKeyID, accessKey, sessionToken)
	if err != nil {
		return nil, fmt.Errorf("failed to create images AWS client: %w", err)
	}

	aws := newAwsFromConfig(cfg, imagesAWS)
	return aws, nil
}

// Initializes a new AWS object with the credentials info found at filename's location.
// The credential files should match the AWS format, such as:
// [default]
// aws_access_key_id = secretString1
// aws_secret_access_key = secretString2
//
// If filename is empty the underlying function will look for the
// "AWS_SHARED_CREDENTIALS_FILE" env variable or will default to
// $HOME/.aws/credentials.
func NewFromFile(filename string, region string) (*AWS, error) {
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
		config.WithSharedCredentialsFiles([]string{
			filename,
			"default",
		}),
	)
	if err != nil {
		return nil, err
	}

	imagesAWS, err := images_awscloud.NewFromFile(filename, region)
	if err != nil {
		return nil, fmt.Errorf("failed to create images AWS client: %w", err)
	}

	aws := newAwsFromConfig(cfg, imagesAWS)
	return aws, nil
}

// Initialize a new AWS object from defaults.
// Looks for env variables, shared credential file, and EC2 Instance Roles.
func NewDefault(region string) (*AWS, error) {
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, err
	}

	imagesAWS, err := images_awscloud.NewDefault(region)
	if err != nil {
		return nil, fmt.Errorf("failed to create images AWS client: %w", err)
	}

	aws := newAwsFromConfig(cfg, imagesAWS)
	return aws, nil
}

func RegionFromInstanceMetadata() (string, error) {
	identity, err := imds.New(imds.Options{}).GetInstanceIdentityDocument(
		context.Background(),
		&imds.GetInstanceIdentityDocumentInput{},
	)
	if err != nil {
		return "", err
	}
	return identity.Region, nil
}

// Create a new session from the credentials and the region and returns an *AWS object initialized with it.
func newAwsFromCredsWithEndpoint(creds config.LoadOptionsFunc, region, endpoint, caBundle string, skipSSLVerification bool, imagesAWS *images_awscloud.AWS) (*AWS, error) {
	// Create a Session with a custom region
	v2OptionFuncs := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		creds,
	}

	if caBundle != "" {
		caBundleReader, err := os.Open(caBundle)
		if err != nil {
			return nil, err
		}
		defer caBundleReader.Close()
		v2OptionFuncs = append(v2OptionFuncs, config.WithCustomCABundle(caBundleReader))
	}

	if skipSSLVerification {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402
		v2OptionFuncs = append(v2OptionFuncs, config.WithHTTPClient(&http.Client{
			Transport: transport,
		}))
	}

	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		v2OptionFuncs...,
	)
	if err != nil {
		return nil, err
	}

	s3cli := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})

	return &AWS{
		AWS:     imagesAWS,
		ec2:     ec2.NewFromConfig(cfg),
		ec2imds: imds.NewFromConfig(cfg),
		s3:      s3cli,
		asg:     autoscaling.NewFromConfig(cfg),
	}, nil
}

// Initialize a new AWS object targeting a specific endpoint from individual bits. SessionToken is optional
func NewForEndpoint(endpoint, region, accessKeyID, accessKey, sessionToken, caBundle string, skipSSLVerification bool) (*AWS, error) {
	imagesAWS, err := images_awscloud.NewForEndpoint(endpoint, region, accessKeyID, accessKey, sessionToken, caBundle, skipSSLVerification)
	if err != nil {
		return nil, fmt.Errorf("failed to create images AWS client: %w", err)
	}
	return newAwsFromCredsWithEndpoint(config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, accessKey, sessionToken)), region, endpoint, caBundle, skipSSLVerification, imagesAWS)
}

// Initializes a new AWS object targeting a specific endpoint with the credentials info found at filename's location.
// The credential files should match the AWS format, such as:
// [default]
// aws_access_key_id = secretString1
// aws_secret_access_key = secretString2
//
// If filename is empty the underlying function will look for the
// "AWS_SHARED_CREDENTIALS_FILE" env variable or will default to
// $HOME/.aws/credentials.
func NewForEndpointFromFile(filename, endpoint, region, caBundle string, skipSSLVerification bool) (*AWS, error) {
	imagesAWS, err := images_awscloud.NewForEndpointFromFile(filename, endpoint, region, caBundle, skipSSLVerification)
	if err != nil {
		return nil, fmt.Errorf("failed to create images AWS client: %w", err)
	}
	return newAwsFromCredsWithEndpoint(config.WithSharedCredentialsFiles([]string{filename, "default"}), region, endpoint, caBundle, skipSSLVerification, imagesAWS)
}

// target region is determined by the region configured in the aws session
func (a *AWS) CopyImage(name, ami, sourceRegion string) (string, error) {
	result, err := a.ec2.CopyImage(
		context.Background(),
		&ec2.CopyImageInput{
			Name:          aws.String(name),
			SourceImageId: aws.String(ami),
			SourceRegion:  aws.String(sourceRegion),
		},
	)
	if err != nil {
		return "", err
	}

	imgWaiter := ec2.NewImageAvailableWaiter(a.ec2)
	imgWaitOutput, err := imgWaiter.WaitForOutput(
		context.Background(),
		&ec2.DescribeImagesInput{
			ImageIds: []string{*result.ImageId},
		},
		time.Hour*24,
	)
	if err != nil {
		return *result.ImageId, err
	}

	if imgWaitOutput.Images[0].State != ec2types.ImageStateAvailable {
		return *result.ImageId, fmt.Errorf("Image not available after waiting: %s, Code: %v reason: %v",
			imgWaitOutput.Images[0].State, *imgWaitOutput.Images[0].StateReason.Code, *imgWaitOutput.Images[0].StateReason.Message)
	}

	// Tag image with name
	_, err = a.ec2.CreateTags(
		context.Background(),
		&ec2.CreateTagsInput{
			Resources: []string{*result.ImageId},
			Tags: []ec2types.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String(name),
				},
			},
		},
	)
	if err != nil {
		return *result.ImageId, err
	}

	imgs, err := a.ec2.DescribeImages(
		context.Background(),
		&ec2.DescribeImagesInput{
			ImageIds: []string{*result.ImageId},
		},
	)
	if err != nil {
		return *result.ImageId, err
	}
	if len(imgs.Images) == 0 {
		return *result.ImageId, fmt.Errorf("Unable to find image with id: %v", ami)
	}

	// Tag snapshot with name
	for _, bdm := range imgs.Images[0].BlockDeviceMappings {
		_, err = a.ec2.CreateTags(
			context.Background(),
			&ec2.CreateTagsInput{
				Resources: []string{*bdm.Ebs.SnapshotId},
				Tags: []ec2types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(name),
					},
				},
			})
		if err != nil {
			return *result.ImageId, err
		}
	}

	return *result.ImageId, nil
}

func (a *AWS) DescribeImagesByName(name string) (*ec2.DescribeImagesOutput, error) {
	return a.ec2.DescribeImages(
		context.Background(),
		&ec2.DescribeImagesInput{
			Filters: []ec2types.Filter{
				{
					Name: aws.String("name"),
					Values: []string{
						name,
					},
				},
			},
		},
	)
}
