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
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/sirupsen/logrus"
)

type AWS struct {
	ec2        EC2
	ec2imds    EC2Imds
	s3         S3
	s3uploader S3Manager
	s3presign  S3Presign
}

func newForTest(ec2cli EC2, ec2imds EC2Imds, s3cli S3, upldr S3Manager, sign S3Presign) *AWS {
	return &AWS{
		ec2:        ec2cli,
		ec2imds:    ec2imds,
		s3:         s3cli,
		s3uploader: upldr,
		s3presign:  sign,
	}
}

// Create a new session from the credentials and the region and returns an *AWS object initialized with it.
// /creds credentials.StaticCredentialsProvider, region string
func newAwsFromConfig(cfg aws.Config) *AWS {
	s3cli := s3.NewFromConfig(cfg)
	return &AWS{
		ec2:        ec2.NewFromConfig(cfg),
		ec2imds:    imds.NewFromConfig(cfg),
		s3:         s3cli,
		s3uploader: manager.NewUploader(s3cli),
		s3presign:  s3.NewPresignClient(s3cli),
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
	aws := newAwsFromConfig(cfg)
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
	aws := newAwsFromConfig(cfg)
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
	aws := newAwsFromConfig(cfg)
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
func newAwsFromCredsWithEndpoint(creds config.LoadOptionsFunc, region, endpoint, caBundle string, skipSSLVerification bool) (*AWS, error) {
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
		ec2:        ec2.NewFromConfig(cfg),
		ec2imds:    imds.NewFromConfig(cfg),
		s3:         s3cli,
		s3uploader: manager.NewUploader(s3cli),
		s3presign:  s3.NewPresignClient(s3cli),
	}, nil
}

// Initialize a new AWS object targeting a specific endpoint from individual bits. SessionToken is optional
func NewForEndpoint(endpoint, region, accessKeyID, accessKey, sessionToken, caBundle string, skipSSLVerification bool) (*AWS, error) {
	return newAwsFromCredsWithEndpoint(config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, accessKey, sessionToken)), region, endpoint, caBundle, skipSSLVerification)
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
	return newAwsFromCredsWithEndpoint(config.WithSharedCredentialsFiles([]string{filename, "default"}), region, endpoint, caBundle, skipSSLVerification)
}

// This is used by the internal/boot test, which access the ec2 apis directly
func (a *AWS) EC2ForTestsOnly() EC2 {
	return a.ec2
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

// Register is a function that imports a snapshot, waits for the snapshot to
// fully import, tags the snapshot, cleans up the image in S3, and registers
// an AMI in AWS.
// The caller can optionally specify the boot mode of the AMI. If the boot
// mode is not specified, then the instances launched from this AMI use the
// default boot mode value of the instance type.
func (a *AWS) Register(name, bucket, key string, shareWith []string, rpmArch string, bootMode *string) (*string, error) {
	rpmArchToEC2Arch := map[string]ec2types.ArchitectureValues{
		"x86_64":  ec2types.ArchitectureValuesX8664,
		"aarch64": ec2types.ArchitectureValuesArm64,
	}

	ec2Arch, validArch := rpmArchToEC2Arch[rpmArch]
	if !validArch {
		return nil, fmt.Errorf("ec2 doesn't support the following arch: %s", rpmArch)
	}

	bootModeToEC2BootMode := map[string]ec2types.BootModeValues{
		string(ec2types.BootModeValuesLegacyBios):    ec2types.BootModeValuesLegacyBios,
		string(ec2types.BootModeValuesUefi):          ec2types.BootModeValuesUefi,
		string(ec2types.BootModeValuesUefiPreferred): ec2types.BootModeValuesUefiPreferred,
	}
	ec2BootMode := ec2types.BootModeValuesUefiPreferred
	if bootMode != nil {
		bm, validBootMode := bootModeToEC2BootMode[*bootMode]
		if !validBootMode {
			return nil, fmt.Errorf("ec2 doesn't support the following boot mode: %s", *bootMode)
		}
		ec2BootMode = bm
	}

	logrus.Infof("[AWS] 📥 Importing snapshot from image: %s/%s", bucket, key)
	snapshotDescription := fmt.Sprintf("Image Builder AWS Import of %s", name)
	importTaskOutput, err := a.ec2.ImportSnapshot(
		context.Background(),
		&ec2.ImportSnapshotInput{
			Description: aws.String(snapshotDescription),
			DiskContainer: &ec2types.SnapshotDiskContainer{
				UserBucket: &ec2types.UserBucket{
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

	// importTaskOutput.
	snapWaiter := ec2.NewSnapshotImportedWaiter(a.ec2)
	snapWaitOutput, err := snapWaiter.WaitForOutput(
		context.Background(),
		&ec2.DescribeImportSnapshotTasksInput{
			ImportTaskIds: []string{
				*importTaskOutput.ImportTaskId,
			},
		},
		time.Hour*24,
	)
	if err != nil {
		return nil, err
	}

	snapshotTaskStatus := *snapWaitOutput.ImportSnapshotTasks[0].SnapshotTaskDetail.Status
	if snapshotTaskStatus != "completed" {
		return nil, fmt.Errorf("Unable to import snapshot, task result: %v, msg: %v", snapshotTaskStatus, *snapWaitOutput.ImportSnapshotTasks[0].SnapshotTaskDetail.StatusMessage)
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

	snapshotID := *snapWaitOutput.ImportSnapshotTasks[0].SnapshotTaskDetail.SnapshotId
	// Tag the snapshot with the image name.
	_, err = a.ec2.CreateTags(
		context.Background(),
		&ec2.CreateTagsInput{
			Resources: []string{snapshotID},
			Tags: []ec2types.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String(name),
				},
			},
		},
	)
	if err != nil {
		return nil, err
	}

	logrus.Infof("[AWS] 📋 Registering AMI from imported snapshot: %s", snapshotID)
	registerOutput, err := a.ec2.RegisterImage(
		context.Background(),
		&ec2.RegisterImageInput{
			Architecture:       ec2Arch,
			BootMode:           ec2BootMode,
			VirtualizationType: aws.String("hvm"),
			Name:               aws.String(name),
			RootDeviceName:     aws.String("/dev/sda1"),
			EnaSupport:         aws.Bool(true),
			BlockDeviceMappings: []ec2types.BlockDeviceMapping{
				{
					DeviceName: aws.String("/dev/sda1"),
					Ebs: &ec2types.EbsBlockDevice{
						SnapshotId: aws.String(snapshotID),
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
	_, err = a.ec2.CreateTags(
		context.Background(),
		&ec2.CreateTagsInput{
			Resources: []string{*registerOutput.ImageId},
			Tags: []ec2types.Tag{
				{
					Key:   aws.String("Name"),
					Value: aws.String(name),
				},
			},
		},
	)
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

func (a *AWS) ShareImage(ami string, userIds []string) error {
	imgs, err := a.ec2.DescribeImages(
		context.Background(),
		&ec2.DescribeImagesInput{
			ImageIds: []string{ami},
		},
	)
	if err != nil {
		return err
	}
	if len(imgs.Images) == 0 {
		return fmt.Errorf("Unable to find image with id: %v", ami)
	}

	for _, bdm := range imgs.Images[0].BlockDeviceMappings {
		err = a.shareSnapshot(*bdm.Ebs.SnapshotId, userIds)
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
	var launchPerms []ec2types.LaunchPermission
	for _, id := range uIds {
		launchPerms = append(launchPerms, ec2types.LaunchPermission{
			UserId: id,
		})
	}
	_, err := a.ec2.ModifyImageAttribute(
		context.Background(),
		&ec2.ModifyImageAttributeInput{
			ImageId: ami,
			LaunchPermission: &ec2types.LaunchPermissionModifications{
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

func (a *AWS) shareSnapshot(snapshotId string, userIds []string) error {
	logrus.Info("[AWS] 🎥 Sharing ec2 snapshot")
	_, err := a.ec2.ModifySnapshotAttribute(
		context.Background(),
		&ec2.ModifySnapshotAttributeInput{
			Attribute:     ec2types.SnapshotAttributeNameCreateVolumePermission,
			OperationType: ec2types.OperationTypeAdd,
			SnapshotId:    aws.String(snapshotId),
			UserIds:       userIds,
		},
	)
	if err != nil {
		logrus.Warnf("[AWS] 📨 Error sharing ec2 snapshot: %v", err)
		return err
	}
	logrus.Info("[AWS] 📨 Shared ec2 snapshot")
	return nil
}

func (a *AWS) RemoveSnapshotAndDeregisterImage(image *ec2types.Image) error {
	if image == nil {
		return fmt.Errorf("image is nil")
	}

	var snapshots []*string
	for _, bdm := range image.BlockDeviceMappings {
		snapshots = append(snapshots, bdm.Ebs.SnapshotId)
	}

	_, err := a.ec2.DeregisterImage(
		context.Background(),
		&ec2.DeregisterImageInput{
			ImageId: image.ImageId,
		},
	)
	if err != nil {
		return err
	}

	for _, s := range snapshots {
		_, err = a.ec2.DeleteSnapshot(
			context.Background(),
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
func (a *AWS) DescribeImagesByTag(tagKey, tagValue string) ([]ec2types.Image, error) {
	imgs, err := a.ec2.DescribeImages(
		context.Background(),
		&ec2.DescribeImagesInput{
			Filters: []ec2types.Filter{
				{
					Name:   aws.String(fmt.Sprintf("tag:%s", tagKey)),
					Values: []string{tagValue},
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
	out, err := a.ec2.DescribeRegions(context.Background(), &ec2.DescribeRegionsInput{})
	if err != nil {
		return nil, err
	}

	result := []string{}
	for _, r := range out.Regions {
		result = append(result, *r.RegionName)
	}
	return result, nil
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
