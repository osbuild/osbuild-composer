package awsupload

import (
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
	importer *ec2.EC2
	s3       *s3.S3
}

func New(region, accessKeyID, accessKey string) (*AWS, error) {
	// Session credentials
	creds := credentials.NewStaticCredentials(accessKeyID, accessKey, "")

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
		importer: ec2.New(sess),
		s3:       s3.New(sess),
	}, nil
}

func (a *AWS) Upload(filename, bucket, key string) (*s3manager.UploadOutput, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	log.Printf("[AWS] Uploading image to S3: %s/%s", bucket, key)
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

func (a *AWS) Register(name, bucket, key string) (*string, error) {
	log.Printf("[AWS] Importing snapshot from image: %s/%s", bucket, key)
	importTaskOutput, err := a.importer.ImportSnapshot(
		&ec2.ImportSnapshotInput{
			DiskContainer: &ec2.SnapshotDiskContainer{
				UserBucket: &ec2.UserBucket{
					S3Bucket: aws.String(bucket),
					S3Key:    aws.String(key),
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}

	log.Printf("[AWS] Waiting for snapshot to finish importing: %s", *importTaskOutput.ImportTaskId)
	err = WaitUntilImportSnapshotTaskCompleted(
		a.importer,
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
	log.Printf("[AWS] Deleting image from S3: %s/%s", bucket, key)
	_, err = a.s3.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}

	importOutput, err := a.importer.DescribeImportSnapshotTasks(
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
	log.Printf("[AWS] Registering AMI from imported snapshot: %s", *snapshotID)
	registerOutput, err := a.importer.RegisterImage(
		&ec2.RegisterImageInput{
			Architecture:       aws.String("x86_64"),
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
	return registerOutput.ImageId, nil
}
