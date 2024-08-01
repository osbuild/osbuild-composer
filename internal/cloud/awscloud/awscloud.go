package awscloud

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	credentialsv2 "github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/ec2metadata"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/sirupsen/logrus"
	"golang.org/x/exp/slices"
)

type AWS struct {
	ec2         *ec2.EC2
	ec2metadata *ec2metadata.EC2Metadata
	s3          S3
	s3uploader  S3Manager
	s3presign   S3Presign
}

func newForTest(s3cli S3, upldr S3Manager, sign S3Presign) *AWS {
	return &AWS{
		s3:         s3cli,
		s3uploader: upldr,
		s3presign:  sign,
	}
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

	credsValue, err := creds.Get()
	if err != nil {
		return nil, err
	}
	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentialsv2.NewStaticCredentialsProvider(
			credsValue.AccessKeyID,
			credsValue.SecretAccessKey,
			credsValue.SessionToken,
		)),
	)

	s3cli := s3.NewFromConfig(cfg)
	return &AWS{
		ec2:         ec2.New(sess),
		ec2metadata: ec2metadata.New(sess),
		s3:          s3cli,
		s3uploader:  manager.NewUploader(s3cli),
		s3presign:   s3.NewPresignClient(s3cli),
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

// Initialize a new AWS object from defaults.
// Looks for env variables, shared credential file, and EC2 Instance Roles.
func NewDefault(region string) (*AWS, error) {
	return newAwsFromCreds(nil, region)
}

func RegionFromInstanceMetadata() (string, error) {
	sess, err := session.NewSession()
	if err != nil {
		return "", err
	}
	identity, err := ec2metadata.New(sess).GetInstanceIdentityDocument()
	if err != nil {
		return "", err
	}
	return identity.Region, nil
}

// Create a new session from the credentials and the region and returns an *AWS object initialized with it.
func newAwsFromCredsWithEndpoint(creds *credentials.Credentials, region, endpoint, caBundle string, skipSSLVerification bool) (*AWS, error) {
	// Create a Session with a custom region
	s3ForcePathStyle := true
	sessionOptions := session.Options{
		Config: aws.Config{
			Credentials:      creds,
			Region:           aws.String(region),
			Endpoint:         &endpoint,
			S3ForcePathStyle: &s3ForcePathStyle,
		},
	}

	credsValue, err := creds.Get()
	if err != nil {
		return nil, err
	}
	v2OptionFuncs := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		config.WithCredentialsProvider(credentialsv2.NewStaticCredentialsProvider(
			credsValue.AccessKeyID,
			credsValue.SecretAccessKey,
			credsValue.SessionToken,
		)),
	}

	if caBundle != "" {
		caBundleReader, err := os.Open(caBundle)
		if err != nil {
			return nil, err
		}
		defer caBundleReader.Close()
		sessionOptions.CustomCABundle = caBundleReader
		v2OptionFuncs = append(v2OptionFuncs, config.WithCustomCABundle(caBundleReader))
	}

	if skipSSLVerification {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402
		sessionOptions.Config.HTTPClient = &http.Client{
			Transport: transport,
		}
		v2OptionFuncs = append(v2OptionFuncs, config.WithHTTPClient(&http.Client{
			Transport: transport,
		}))
	}

	sess, err := session.NewSessionWithOptions(sessionOptions)
	if err != nil {
		return nil, err
	}

	cfg, err := config.LoadDefaultConfig(
		context.Background(),
		v2OptionFuncs...,
	)

	s3cli := s3.NewFromConfig(cfg, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(endpoint)
		options.UsePathStyle = true
	})

	return &AWS{
		ec2:         ec2.New(sess),
		ec2metadata: ec2metadata.New(sess),
		s3:          s3cli,
		s3uploader:  manager.NewUploader(s3cli),
		s3presign:   s3.NewPresignClient(s3cli),
	}, nil
}

// Initialize a new AWS object targeting a specific endpoint from individual bits. SessionToken is optional
func NewForEndpoint(endpoint, region, accessKeyID, accessKey, sessionToken, caBundle string, skipSSLVerification bool) (*AWS, error) {
	return newAwsFromCredsWithEndpoint(credentials.NewStaticCredentials(accessKeyID, accessKey, sessionToken), region, endpoint, caBundle, skipSSLVerification)
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
	return newAwsFromCredsWithEndpoint(credentials.NewSharedCredentials(filename, "default"), region, endpoint, caBundle, skipSSLVerification)
}

func (a *AWS) Upload(filename, bucket, key string) (*manager.UploadOutput, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer func() {
		err := file.Close()
		if err != nil {
			logrus.Warnf("[AWS] ‼ Failed to close the file uploaded to S3️: %v", err)
		}
	}()

	logrus.Infof("[AWS] 🚀 Uploading image to S3: %s/%s", bucket, key)
	return a.s3uploader.Upload(
		context.Background(),
		&s3.PutObjectInput{
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
// The caller can optionally specify the boot mode of the AMI. If the boot
// mode is not specified, then the instances launched from this AMI use the
// default boot mode value of the instance type.
func (a *AWS) Register(name, bucket, key string, shareWith []string, rpmArch string, bootMode *string) (*string, error) {
	rpmArchToEC2Arch := map[string]string{
		"x86_64":  "x86_64",
		"aarch64": "arm64",
	}

	ec2Arch, validArch := rpmArchToEC2Arch[rpmArch]
	if !validArch {
		return nil, fmt.Errorf("ec2 doesn't support the following arch: %s", rpmArch)
	}

	if bootMode != nil {
		if !slices.Contains(ec2.BootModeValues_Values(), *bootMode) {
			return nil, fmt.Errorf("ec2 doesn't support the following boot mode: %s", *bootMode)
		}
	}

	logrus.Infof("[AWS] 📥 Importing snapshot from image: %s/%s", bucket, key)
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
		logrus.Warnf("[AWS] error importing snapshot: %s", err)
		return nil, err
	}

	logrus.Infof("[AWS] 🚚 Waiting for snapshot to finish importing: %s", *importTaskOutput.ImportTaskId)
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
	logrus.Infof("[AWS] 🧹 Deleting image from S3: %s/%s", bucket, key)
	_, err = a.s3.DeleteObject(
		context.Background(),
		&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		},
	)
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

	logrus.Infof("[AWS] 📋 Registering AMI from imported snapshot: %s", *snapshotID)
	registerOutput, err := a.ec2.RegisterImage(
		&ec2.RegisterImageInput{
			Architecture:       aws.String(ec2Arch),
			BootMode:           bootMode,
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

	logrus.Infof("[AWS] 🎉 AMI registered: %s", *registerOutput.ImageId)

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
		err = a.shareSnapshot(snapshotID, shareWith)
		if err != nil {
			return nil, err
		}
		err = a.shareImage(registerOutput.ImageId, shareWith)
		if err != nil {
			return nil, err
		}
	}

	return registerOutput.ImageId, nil
}

// target region is determined by the region configured in the aws session
func (a *AWS) CopyImage(name, ami, sourceRegion string) (string, error) {
	result, err := a.ec2.CopyImage(
		&ec2.CopyImageInput{
			Name:          aws.String(name),
			SourceImageId: aws.String(ami),
			SourceRegion:  aws.String(sourceRegion),
		},
	)
	if err != nil {
		return "", err
	}

	dIInput := &ec2.DescribeImagesInput{
		ImageIds: []*string{result.ImageId},
	}

	// Custom waiter which waits indefinitely until a final state
	w := request.Waiter{
		Name:        "WaitUntilImageAvailable",
		MaxAttempts: 0,
		Delay:       request.ConstantWaiterDelay(15 * time.Second),
		Acceptors: []request.WaiterAcceptor{
			{
				State:   request.SuccessWaiterState,
				Matcher: request.PathAllWaiterMatch, Argument: "Images[].State",
				Expected: "available",
			},
			{
				State:   request.FailureWaiterState,
				Matcher: request.PathAnyWaiterMatch, Argument: "Images[].State",
				Expected: "failed",
			},
		},
		Logger: a.ec2.Config.Logger,
		NewRequest: func(opts []request.Option) (*request.Request, error) {
			var inCpy *ec2.DescribeImagesInput
			if dIInput != nil {
				tmp := *dIInput
				inCpy = &tmp
			}
			req, _ := a.ec2.DescribeImagesRequest(inCpy)
			req.SetContext(aws.BackgroundContext())
			req.ApplyOptions(opts...)
			return req, nil
		},
	}
	err = w.WaitWithContext(aws.BackgroundContext())
	if err != nil {
		return *result.ImageId, err
	}

	// Tag image with name
	_, err = a.ec2.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{result.ImageId},
		Tags: []*ec2.Tag{
			{
				Key:   aws.String("Name"),
				Value: aws.String(name),
			},
		},
	})

	if err != nil {
		return *result.ImageId, err
	}

	imgs, err := a.ec2.DescribeImages(dIInput)
	if err != nil {
		return *result.ImageId, err
	}
	if len(imgs.Images) == 0 {
		return *result.ImageId, fmt.Errorf("Unable to find image with id: %v", ami)
	}

	// Tag snapshot with name
	for _, bdm := range imgs.Images[0].BlockDeviceMappings {
		_, err = a.ec2.CreateTags(&ec2.CreateTagsInput{
			Resources: []*string{bdm.Ebs.SnapshotId},
			Tags: []*ec2.Tag{
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

func (a *AWS) ShareImage(ami string, userIds []string) error {
	imgs, err := a.ec2.DescribeImages(
		&ec2.DescribeImagesInput{
			ImageIds: []*string{aws.String(ami)},
		},
	)
	if err != nil {
		return err
	}
	if len(imgs.Images) == 0 {
		return fmt.Errorf("Unable to find image with id: %v", ami)
	}

	for _, bdm := range imgs.Images[0].BlockDeviceMappings {
		err = a.shareSnapshot(bdm.Ebs.SnapshotId, userIds)
		if err != nil {
			return err
		}
	}

	err = a.shareImage(aws.String(ami), userIds)
	if err != nil {
		return err
	}
	return nil
}

func (a *AWS) shareImage(ami *string, userIds []string) error {
	logrus.Info("[AWS] 🎥 Sharing ec2 snapshot")
	var uIds []*string
	for i := range userIds {
		uIds = append(uIds, &userIds[i])
	}

	logrus.Info("[AWS] 💿 Sharing ec2 AMI")
	var launchPerms []*ec2.LaunchPermission
	for _, id := range uIds {
		launchPerms = append(launchPerms, &ec2.LaunchPermission{
			UserId: id,
		})
	}
	_, err := a.ec2.ModifyImageAttribute(
		&ec2.ModifyImageAttributeInput{
			ImageId: ami,
			LaunchPermission: &ec2.LaunchPermissionModifications{
				Add: launchPerms,
			},
		},
	)
	if err != nil {
		logrus.Warnf("[AWS] 📨 Error sharing AMI: %v", err)
		return err
	}
	logrus.Info("[AWS] 💿 Shared AMI")
	return nil
}

func (a *AWS) shareSnapshot(snapshotId *string, userIds []string) error {
	logrus.Info("[AWS] 🎥 Sharing ec2 snapshot")
	var uIds []*string
	for i := range userIds {
		uIds = append(uIds, &userIds[i])
	}
	_, err := a.ec2.ModifySnapshotAttribute(
		&ec2.ModifySnapshotAttributeInput{
			Attribute:     aws.String(ec2.SnapshotAttributeNameCreateVolumePermission),
			OperationType: aws.String("add"),
			SnapshotId:    snapshotId,
			UserIds:       uIds,
		},
	)
	if err != nil {
		logrus.Warnf("[AWS] 📨 Error sharing ec2 snapshot: %v", err)
		return err
	}
	logrus.Info("[AWS] 📨 Shared ec2 snapshot")
	return nil
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
			logrus.Warn("Unable to remove snapshot", s)
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
	logrus.Infof("[AWS] 📋 Generating Presigned URL for S3 object %s/%s", bucket, objectKey)

	req, err := a.s3presign.PresignGetObject(
		context.Background(),
		&s3.GetObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(objectKey),
		},
		func(opts *s3.PresignOptions) {
			opts.Expires = time.Duration(7 * 24 * time.Hour)
		},
	)
	if err != nil {
		return "", err
	}

	logrus.Info("[AWS] 🎉 S3 Presigned URL ready")
	return req.URL, nil
}

func (a *AWS) MarkS3ObjectAsPublic(bucket, objectKey string) error {
	logrus.Infof("[AWS] 👐 Making S3 object public %s/%s", bucket, objectKey)
	_, err := a.s3.PutObjectAcl(
		context.Background(),
		&s3.PutObjectAclInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(objectKey),
			ACL:    s3types.ObjectCannedACL(s3types.ObjectCannedACLPublicRead),
		},
	)
	if err != nil {
		return err
	}
	logrus.Info("[AWS] ✔️ Making S3 object public successful")
	return nil
}

func (a *AWS) Regions() ([]string, error) {
	out, err := a.ec2.DescribeRegions(&ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, err
	}

	result := []string{}
	for _, r := range out.Regions {
		result = append(result, aws.StringValue(r.RegionName))
	}
	return result, nil
}
