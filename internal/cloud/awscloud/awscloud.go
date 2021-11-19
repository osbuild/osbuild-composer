package awscloud

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type AWS struct {
	uploader *s3manager.Uploader
	ec2      *ec2.EC2
	s3       *s3.S3
}

// Create a new session from the credentials and the region and returns an *AWS object initialized with it.
func newAwsFromCreds(creds *credentials.Credentials, region string) (*AWS, error) {
	// Create a Session with a custom region
	sess, err := session.NewSession(&aws.Config{
		Credentials: creds,
		Region:      aws.String(region),
	})
	if err != nil {
		return nil, err
	}

	return &AWS{
		uploader: s3manager.NewUploader(sess),
		ec2:      ec2.New(sess),
		s3:       s3.New(sess),
	}, nil
}

// Initialize a new AWS object from individual bits. SessionToken is optional
func New(region string, accessKeyID string, accessKey string, sessionToken string) (*AWS, error) {
	return newAwsFromCreds(credentials.NewStaticCredentials(accessKeyID, accessKey, sessionToken), region)
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
	return newAwsFromCreds(credentials.NewSharedCredentials(filename, "default"), region)
}

func (a *AWS) Upload(filename, bucket, key string) (*s3manager.UploadOutput, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	log.Printf("[AWS] 🚀 Uploading image to S3: %s/%s", bucket, key)
	return a.uploader.Upload(
		&s3manager.UploadInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   file,
		},
	)
}

// WaitUntilImportSnapshotCompleted uses the Amazon EC2 API operation
// DescribeImportSnapshots to wait for a condition to be met before returning.
// If the condition is not met within the max attempt window, an error will
// be returned.
func WaitUntilImportSnapshotTaskCompleted(c *ec2.EC2, input *ec2.DescribeImportSnapshotTasksInput) error {
	return WaitUntilImportSnapshotTaskCompletedWithContext(c, aws.BackgroundContext(), input)
}

// WaitUntilImportSnapshotCompletedWithContext is an extended version of
// WaitUntilImportSnapshotCompleted. With the support for passing in a
// context and options to configure the Waiter and the underlying request
// options.
//
// The context must be non-nil and will be used for request cancellation. If
// the context is nil a panic will occur. In the future the SDK may create
// sub-contexts for http.Requests. See https://golang.org/pkg/context/
// for more information on using Contexts.
//
// NOTE(mhayden): The MaxAttempts is set to zero here so that we will keep
// checking the status of the image import until it succeeds or fails. This
// process can take anywhere from 5 to 60+ minutes depending on how quickly
// AWS can import the snapshot.
func WaitUntilImportSnapshotTaskCompletedWithContext(c *ec2.EC2, ctx aws.Context, input *ec2.DescribeImportSnapshotTasksInput, opts ...request.WaiterOption) error {
	w := request.Waiter{
		Name:        "WaitUntilImportSnapshotTaskCompleted",
		MaxAttempts: 0,
		Delay:       request.ConstantWaiterDelay(15 * time.Second),
		Acceptors: []request.WaiterAcceptor{
			{
				State:   request.SuccessWaiterState,
				Matcher: request.PathAllWaiterMatch, Argument: "ImportSnapshotTasks[].SnapshotTaskDetail.Status",
				Expected: "completed",
			},
			{
				State:   request.FailureWaiterState,
				Matcher: request.PathAllWaiterMatch, Argument: "ImportSnapshotTasks[].SnapshotTaskDetail.Status",
				Expected: "deleted",
			},
		},
		Logger: c.Config.Logger,
		NewRequest: func(opts []request.Option) (*request.Request, error) {
			var inCpy *ec2.DescribeImportSnapshotTasksInput
			if input != nil {
				tmp := *input
				inCpy = &tmp
			}
			req, _ := c.DescribeImportSnapshotTasksRequest(inCpy)
			req.SetContext(ctx)
			req.ApplyOptions(opts...)
			return req, nil
		},
	}
	w.ApplyOptions(opts...)

	return w.WaitWithContext(ctx)
}

// Register is a function that imports a snapshot, waits for the snapshot to
// fully import, tags the snapshot, cleans up the image in S3, and registers
// an AMI in AWS.
func (a *AWS) Register(name, bucket, key string, shareWith []string, rpmArch string) (*string, error) {
	rpmArchToEC2Arch := map[string]string{
		"x86_64":  "x86_64",
		"aarch64": "arm64",
	}

	ec2Arch, validArch := rpmArchToEC2Arch[rpmArch]
	if !validArch {
		return nil, fmt.Errorf("ec2 doesn't support the following arch: %s", rpmArch)
	}

	log.Printf("[AWS] 📥 Importing snapshot from image: %s/%s", bucket, key)
	snapshotDescription := fmt.Sprintf("Image Builder AWS Import of %s", name)
	importTaskOutput, err := a.ec2.ImportSnapshot(
		&ec2.ImportSnapshotInput{
			Description: aws.String(snapshotDescription),
			DiskContainer: &ec2.SnapshotDiskContainer{
				UserBucket: &ec2.UserBucket{
					S3Bucket: aws.String(bucket),
					S3Key:    aws.String(key),
				},
			},
		},
	)
	if err != nil {
		log.Printf("[AWS] error importing snapshot: %s", err)
		return nil, err
	}

	log.Printf("[AWS] 🚚 Waiting for snapshot to finish importing: %s", *importTaskOutput.ImportTaskId)
	err = WaitUntilImportSnapshotTaskCompleted(
		a.ec2,
		&ec2.DescribeImportSnapshotTasksInput{
			ImportTaskIds: []*string{
				importTaskOutput.ImportTaskId,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	// we no longer need the object in s3, let's just delete it
	log.Printf("[AWS] 🧹 Deleting image from S3: %s/%s", bucket, key)
	_, err = a.s3.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	importOutput, err := a.ec2.DescribeImportSnapshotTasks(
		&ec2.DescribeImportSnapshotTasksInput{
			ImportTaskIds: []*string{
				importTaskOutput.ImportTaskId,
			},
		},
	)
	if err != nil {
		return nil, err
	}

	snapshotID := importOutput.ImportSnapshotTasks[0].SnapshotTaskDetail.SnapshotId

	if len(shareWith) > 0 {
		log.Printf("[AWS] 🎥 Sharing ec2 snapshot")
		var userIds []*string
		for _, v := range shareWith {
			userIds = append(userIds, &v)
		}
		_, err := a.ec2.ModifySnapshotAttribute(
			&ec2.ModifySnapshotAttributeInput{
				Attribute:     aws.String("createVolumePermission"),
				OperationType: aws.String("add"),
				SnapshotId:    snapshotID,
				UserIds:       userIds,
			},
		)
		if err != nil {
			return nil, err
		}
		log.Println("[AWS] 📨 Shared ec2 snapshot")
	}

	// Tag the snapshot with the image name.
	req, _ := a.ec2.CreateTagsRequest(
		&ec2.CreateTagsInput{
			Resources: []*string{snapshotID},
			Tags: []*ec2.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String(name),
				},
			},
		},
	)
	err = req.Send()
	if err != nil {
		return nil, err
	}

	log.Printf("[AWS] 📋 Registering AMI from imported snapshot: %s", *snapshotID)
	registerOutput, err := a.ec2.RegisterImage(
		&ec2.RegisterImageInput{
			Architecture:       aws.String(ec2Arch),
			VirtualizationType: aws.String("hvm"),
			Name:               aws.String(name),
			RootDeviceName:     aws.String("/dev/sda1"),
			EnaSupport:         aws.Bool(true),
			BlockDeviceMappings: []*ec2.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/sda1"),
					Ebs: &ec2.EbsBlockDevice{
						SnapshotId: snapshotID,
					},
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}

	log.Printf("[AWS] 🎉 AMI registered: %s", *registerOutput.ImageId)

	// Tag the image with the image name.
	req, _ = a.ec2.CreateTagsRequest(
		&ec2.CreateTagsInput{
			Resources: []*string{registerOutput.ImageId},
			Tags: []*ec2.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String(name),
				},
			},
		},
	)
	err = req.Send()
	if err != nil {
		return nil, err
	}

	if len(shareWith) > 0 {
		log.Println("[AWS] 💿 Sharing ec2 AMI")
		var launchPerms []*ec2.LaunchPermission
		for _, id := range shareWith {
			launchPerms = append(launchPerms, &ec2.LaunchPermission{
				UserId: &id,
			})
		}
		_, err := a.ec2.ModifyImageAttribute(
			&ec2.ModifyImageAttributeInput{
				ImageId: registerOutput.ImageId,
				LaunchPermission: &ec2.LaunchPermissionModifications{
					Add: launchPerms,
				},
			},
		)
		if err != nil {
			return nil, err
		}
		log.Println("[AWS] 💿 Shared AMI")
	}

	return registerOutput.ImageId, nil
}

func (a *AWS) RemoveSnapshotAndDeregisterImage(image *ec2.Image) error {
	if image == nil {
		return fmt.Errorf("image is nil")
	}

	var snapshots []*string
	for _, bdm := range image.BlockDeviceMappings {
		snapshots = append(snapshots, bdm.Ebs.SnapshotId)
	}

	_, err := a.ec2.DeregisterImage(
		&ec2.DeregisterImageInput{
			ImageId: image.ImageId,
		},
	)
	if err != nil {
		return err
	}

	for _, s := range snapshots {
		_, err = a.ec2.DeleteSnapshot(
			&ec2.DeleteSnapshotInput{
				SnapshotId: s,
			},
		)
		if err != nil {
			// TODO return err?
			log.Println("Unable to remove snapshot", s)
		}
	}
	return err
}

// For service maintenance images are discovered by the "Name:composer-api-*" tag filter. Currently
// all image names in the service are generated, so they're guaranteed to be unique as well. If
// users are ever allowed to name their images, an extra tag should be added.
func (a *AWS) DescribeImagesByTag(tagKey, tagValue string) ([]*ec2.Image, error) {
	imgs, err := a.ec2.DescribeImages(
		&ec2.DescribeImagesInput{
			Filters: []*ec2.Filter{
				{
					Name:   aws.String(fmt.Sprintf("tag:%s", tagKey)),
					Values: []*string{aws.String(tagValue)},
				},
			},
		},
	)
	return imgs.Images, err
}

func (a *AWS) S3ObjectPresignedURL(bucket, objectKey string) (string, error) {
	log.Printf("[AWS] 📋 Generating Presigned URL for S3 object %s/%s", bucket, objectKey)
	req, _ := a.s3.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(objectKey),
	})
	url, err := req.Presign(7 * 24 * time.Hour) // maximum allowed
	if err != nil {
		return "", err
	}
	log.Print("[AWS] 🎉 S3 Presigned URL ready")
	return url, nil
}
