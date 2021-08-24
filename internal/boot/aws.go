// +build integration

package boot

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/upload/awsupload"
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
	accessKeyId, akExists := os.LookupEnv("AWS_ACCESS_KEY_ID")
	secretAccessKey, sakExists := os.LookupEnv("AWS_SECRET_ACCESS_KEY")
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
	publicKey, err := ioutil.ReadFile(publicKeyFile)
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
	uploader, err := awsupload.New(c.Region, c.AccessKeyId, c.SecretAccessKey, c.sessionToken)
	if err != nil {
		return fmt.Errorf("cannot create aws uploader: %v", err)
	}

	_, err = uploader.Upload(imagePath, c.Bucket, imageName)
	if err != nil {
		return fmt.Errorf("cannot upload the image: %v", err)
	}
	_, err = uploader.Register(imageName, c.Bucket, imageName, nil, common.CurrentArch())
	if err != nil {
		return fmt.Errorf("cannot register the image: %v", err)
	}

	return nil
}

// NewEC2 creates EC2 struct from given credentials
func NewEC2(c *awsCredentials) (*ec2.EC2, error) {
	creds := credentials.NewStaticCredentials(c.AccessKeyId, c.SecretAccessKey, "")
	sess, err := session.NewSession(&aws.Config{
		Credentials: creds,
		Region:      aws.String(c.Region),
	})
	if err != nil {
		return nil, fmt.Errorf("cannot create aws session: %v", err)
	}

	return ec2.New(sess), nil
}

type imageDescription struct {
	Id         *string
	SnapshotId *string
	// this doesn't support multiple snapshots per one image,
	// because this feature is not supported in composer
}

// DescribeEC2Image searches for EC2 image by its name and returns
// its id and snapshot id
func DescribeEC2Image(e *ec2.EC2, imageName string) (*imageDescription, error) {
	imageDescriptions, err := e.DescribeImages(&ec2.DescribeImagesInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("name"),
				Values: []*string{
					aws.String(imageName),
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("cannot describe the image: %v", err)
	}
	imageId := imageDescriptions.Images[0].ImageId
	snapshotId := imageDescriptions.Images[0].BlockDeviceMappings[0].Ebs.SnapshotId

	return &imageDescription{
		Id:         imageId,
		SnapshotId: snapshotId,
	}, nil
}

// DeleteEC2Image deletes the specified image and its associated snapshot
func DeleteEC2Image(e *ec2.EC2, imageDesc *imageDescription) error {
	var retErr error

	// firstly, deregister the image
	_, err := e.DeregisterImage(&ec2.DeregisterImageInput{
		ImageId: imageDesc.Id,
	})

	if err != nil {
		retErr = wrapErrorf(retErr, "cannot deregister the image: %v", err)
	}

	// now it's possible to delete the snapshot
	_, err = e.DeleteSnapshot(&ec2.DeleteSnapshotInput{
		SnapshotId: imageDesc.SnapshotId,
	})

	if err != nil {
		retErr = wrapErrorf(retErr, "cannot delete the snapshot: %v", err)
	}

	return retErr
}

// WithBootedImageInEC2 runs the function f in the context of booted
// image in AWS EC2
func WithBootedImageInEC2(e *ec2.EC2, securityGroupName string, imageDesc *imageDescription, publicKey string, instanceType string, f func(address string) error) (retErr error) {
	// generate user data with given public key
	userData, err := CreateUserData(publicKey)
	if err != nil {
		return err
	}

	// Security group must be now generated, because by default
	// all traffic to EC2 instance is filtered.
	// Firstly create a security group
	securityGroup, err := e.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
		GroupName:   aws.String(securityGroupName),
		Description: aws.String("image-tests-security-group"),
	})
	if err != nil {
		return fmt.Errorf("cannot create a new security group: %v", err)
	}

	defer func() {
		_, err = e.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
			GroupId: securityGroup.GroupId,
		})

		if err != nil {
			retErr = wrapErrorf(retErr, "cannot delete the security group: %v", err)
		}
	}()

	// Authorize incoming SSH connections.
	_, err = e.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		CidrIp:     aws.String("0.0.0.0/0"),
		GroupId:    securityGroup.GroupId,
		FromPort:   aws.Int64(22),
		ToPort:     aws.Int64(22),
		IpProtocol: aws.String("tcp"),
	})
	if err != nil {
		return fmt.Errorf("canot add a rule to the security group: %v", err)
	}

	// Finally, run the instance from the given image and with the created security group
	res, err := e.RunInstances(&ec2.RunInstancesInput{
		MaxCount:         aws.Int64(1),
		MinCount:         aws.Int64(1),
		ImageId:          imageDesc.Id,
		InstanceType:     aws.String(instanceType),
		SecurityGroupIds: []*string{securityGroup.GroupId},
		UserData:         aws.String(encodeBase64(userData)),
	})
	if err != nil {
		return fmt.Errorf("cannot create a new instance: %v", err)
	}

	describeInstanceInput := &ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			res.Instances[0].InstanceId,
		},
	}

	defer func() {
		// We need to terminate the instance now and wait until the termination is done.
		// Otherwise, it wouldn't be possible to delete the image.
		_, err = e.TerminateInstances(&ec2.TerminateInstancesInput{
			InstanceIds: []*string{
				res.Instances[0].InstanceId,
			},
		})
		if err != nil {
			retErr = wrapErrorf(retErr, "cannot terminate the instance: %v", err)
			return
		}

		err = e.WaitUntilInstanceTerminated(describeInstanceInput)
		if err != nil {
			retErr = wrapErrorf(retErr, "waiting for the instance termination failed: %v", err)
		}
	}()

	// The instance has no IP address yet. It's assigned when the instance
	// is in the state "EXISTS". However, in this state the instance is not
	// much usable, therefore wait until "RUNNING" state, in which the instance
	// actually can do something useful for us.
	err = e.WaitUntilInstanceRunning(describeInstanceInput)
	if err != nil {
		return fmt.Errorf("waiting for the instance to be running failed: %v", err)
	}

	// By describing the instance, we can get the ip address.
	out, err := e.DescribeInstances(describeInstanceInput)
	if err != nil {
		return fmt.Errorf("cannot describe the instance: %v", err)
	}

	return f(*out.Reservations[0].Instances[0].PublicIpAddress)
}
