//go:build integration

package boot

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"time"


	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
)

type awsCredentials struct {
	AccessKeyId     string
	SecretAccessKey string
	sessionToken    string
	Region          string
	Bucket          string
}

// GetAWSCredentialsFromEnv gets the credentials from environment variables
// If none of the environment variables is set, it returns nil.
// If some but not all environment variables are set, it returns an error.
func GetAWSCredentialsFromEnv() (*awsCredentials, error) {
	accessKeyId, akExists := os.LookupEnv("V2_AWS_ACCESS_KEY_ID")
	secretAccessKey, sakExists := os.LookupEnv("V2_AWS_SECRET_ACCESS_KEY")
	region, regionExists := os.LookupEnv("AWS_REGION")
	bucket, bucketExists := os.LookupEnv("AWS_BUCKET")

	// If non of the variables is set, just ignore the test
	if !akExists && !sakExists && !bucketExists && !regionExists {
		return nil, nil
	}
	// If only one/two of them are not set, then fail
	if !akExists || !sakExists || !bucketExists || !regionExists {
		return nil, errors.New("not all required env variables were set")
	}

	return &awsCredentials{
		AccessKeyId:     accessKeyId,
		SecretAccessKey: secretAccessKey,
		Region:          region,
		Bucket:          bucket,
	}, nil
}

// encodeBase64 encodes string to base64-encoded string
func encodeBase64(input string) string {
	return base64.StdEncoding.EncodeToString([]byte(input))
}

// CreateUserData creates cloud-init's user-data that contains user redhat with
// the specified public key
func CreateUserData(publicKeyFile string) (string, error) {
	publicKey, err := os.ReadFile(publicKeyFile)
	if err != nil {
		return "", fmt.Errorf("cannot read the public key: %v", err)
	}

	userData := fmt.Sprintf(`#cloud-config
user: redhat
ssh_authorized_keys:
  - %s
`, string(publicKey))

	return userData, nil
}

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

// UploadImageToAWS mimics the upload feature of osbuild-composer.
// It takes an image and an image name and creates an ec2 instance from them.
// The s3 key is never returned - the same thing is done in osbuild-composer,
// the user has no way of getting the s3 key.
func UploadImageToAWS(c *awsCredentials, imagePath string, imageName string) error {
	uploader, err := awscloud.New(c.Region, c.AccessKeyId, c.SecretAccessKey, c.sessionToken)
	if err != nil {
		return fmt.Errorf("cannot create aws uploader: %w", err)
	}

	_, err = uploader.Upload(imagePath, c.Bucket, imageName)
	if err != nil {
		return fmt.Errorf("cannot upload the image: %w", err)
	}
	_, err = uploader.Register(imageName, c.Bucket, imageName, nil, arch.Current().String(), nil)
	if err != nil {
		return fmt.Errorf("cannot register the image: %w", err)
	}

	return nil
}

type imageDescription struct {
	Id         *string
	SnapshotId *string
	// this doesn't support multiple snapshots per one image,
	// because this feature is not supported in composer
	Img        *ec2types.Image
}

// DescribeEC2Image searches for EC2 image by its name and returns
// its id and snapshot id
func DescribeEC2Image(c *awsCredentials, imageName string) (*imageDescription, error) {
	awscl, err := awscloud.New(c.Region, c.AccessKeyId, c.SecretAccessKey, c.sessionToken)
	if err != nil {
		return nil, fmt.Errorf("cannot create aws client: %w", err)
	}

	imageDescriptions, err := awscl.DescribeImagesByName(imageName)
	if err != nil {
		return nil, fmt.Errorf("cannot describe images: %w", err)
	}
	imageId := imageDescriptions.Images[0].ImageId
	snapshotId := imageDescriptions.Images[0].BlockDeviceMappings[0].Ebs.SnapshotId

	return &imageDescription{
		Id:         imageId,
		SnapshotId: snapshotId,
		Img:        &imageDescriptions.Images[0],
	}, nil
}

// DeleteEC2Image deletes the specified image and its associated snapshot
func DeleteEC2Image(c *awsCredentials, imageDesc *imageDescription) error {
	awscl, err := awscloud.New(c.Region, c.AccessKeyId, c.SecretAccessKey, c.sessionToken)
	if err != nil {
		return fmt.Errorf("cannot create aws client: %w", err)
	}
	return awscl.RemoveSnapshotAndDeregisterImage(imageDesc.Img)
}

// WithBootedImageInEC2 runs the function f in the context of booted
// image in AWS EC2
func WithBootedImageInEC2(c *awsCredentials, securityGroupName string, imageDesc *imageDescription, publicKey string, instanceType string, f func(address string) error) (retErr error) {
	awscl, err := awscloud.New(c.Region, c.AccessKeyId, c.SecretAccessKey, c.sessionToken)
	if err != nil {
		return fmt.Errorf("cannot create aws client: %w", err)
	}

	// generate user data with given public key
	userData, err := CreateUserData(publicKey)
	if err != nil {
		return err
	}

	// Security group must be now generated, because by default
	// all traffic to EC2 instance is filtered.
	// Firstly create a security group
	securityGroup, err := awscl.EC2ForTestsOnly().CreateSecurityGroup(
		context.Background(),
		&ec2.CreateSecurityGroupInput{
			GroupName:   aws.String(securityGroupName),
			Description: aws.String("image-tests-security-group"),
		},
	)
	if err != nil {
		return fmt.Errorf("cannot create a new security group: %w", err)
	}

	defer func() {
		_, err = awscl.EC2ForTestsOnly().DeleteSecurityGroup(
			context.Background(),
			&ec2.DeleteSecurityGroupInput{
				GroupId: securityGroup.GroupId,
			},
		)

		if err != nil {
			retErr = wrapErrorf(retErr, "cannot delete the security group: %w", err)
		}
	}()

	// Authorize incoming SSH connections.
	_, err = awscl.EC2ForTestsOnly().AuthorizeSecurityGroupIngress(
		context.Background(),
		&ec2.AuthorizeSecurityGroupIngressInput{
			CidrIp:     aws.String("0.0.0.0/0"),
			GroupId:    securityGroup.GroupId,
			FromPort:   aws.Int32(22),
			ToPort:     aws.Int32(22),
			IpProtocol: aws.String("tcp"),
		},
	)
	if err != nil {
		return fmt.Errorf("canot add a rule to the security group: %w", err)
	}

	// Finally, run the instance from the given image and with the created security group
	res, err := awscl.EC2ForTestsOnly().RunInstances(
		context.Background(),
		&ec2.RunInstancesInput{
			MaxCount:         aws.Int32(1),
			MinCount:         aws.Int32(1),
			ImageId:          imageDesc.Id,
			InstanceType:     ec2types.InstanceType(instanceType),
			SecurityGroupIds: []string{*securityGroup.GroupId},
			UserData:         aws.String(encodeBase64(userData)),
		},
	)
	if err != nil {
		return fmt.Errorf("cannot create a new instance: %w", err)
	}

	describeInstanceInput := &ec2.DescribeInstancesInput{
		InstanceIds: []string{
			*res.Instances[0].InstanceId,
		},
	}

	defer func() {
		// We need to terminate the instance now and wait until the termination is done.
		// Otherwise, it wouldn't be possible to delete the image.
		_, err = awscl.EC2ForTestsOnly().TerminateInstances(
			context.Background(),
			&ec2.TerminateInstancesInput{
				InstanceIds: []string{
					*res.Instances[0].InstanceId,
				},
			},
		)
		if err != nil {
			retErr = wrapErrorf(retErr, "cannot terminate the instance: %w", err)
			return
		}

		instTermWaiter := ec2.NewInstanceTerminatedWaiter(awscl.EC2ForTestsOnly())
		err = instTermWaiter.Wait(
			context.Background(),
			describeInstanceInput,
			time.Hour,
		)
		if err != nil {
			retErr = wrapErrorf(retErr, "cannot terminate the instance: %w", err)
			return
		}
	}()

	// The instance has no IP address yet. It's assigned when the instance
	// is in the state "EXISTS". However, in this state the instance is not
	// much usable, therefore wait until "RUNNING" state, in which the instance
	// actually can do something useful for us.
	instWaiter := ec2.NewInstanceRunningWaiter(awscl.EC2ForTestsOnly())
	err = instWaiter.Wait(
		context.Background(),
		describeInstanceInput,
		time.Hour,
	)
	if err != nil {
		return fmt.Errorf("waiting for the instance to be running failed: %w", err)
	}

	// By describing the instance, we can get the ip address.
	out, err := awscl.EC2ForTestsOnly().DescribeInstances(context.Background(), describeInstanceInput)
	if err != nil {
		return fmt.Errorf("cannot describe the instance: %w", err)
	}

	return f(*out.Reservations[0].Instances[0].PublicIpAddress)
}
