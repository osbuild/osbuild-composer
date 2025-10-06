package awscloud

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	s3manager "github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/olog"
	"github.com/osbuild/images/pkg/platform"
)

type AWS struct {
	ec2        ec2Client
	s3         s3Client
	s3uploader s3Uploader
	s3presign  s3Presign
}

// Allow to mock the EC2 SnapshotImportedWaiter for testing purposes
var newSnapshotImportedWaiterEC2 = func(client ec2.DescribeImportSnapshotTasksAPIClient, optFns ...func(*ec2.SnapshotImportedWaiterOptions)) snapshotImportedWaiterEC2 {
	return ec2.NewSnapshotImportedWaiter(client, optFns...)
}

// Allow to mock the EC2 NewInstanceRunningWaiter for testing purposes
var newInstanceRunningWaiterEC2 = func(client ec2.DescribeInstancesAPIClient, optFns ...func(*ec2.InstanceRunningWaiterOptions)) instanceRunningWaiterEC2 {
	return ec2.NewInstanceRunningWaiter(client, optFns...)
}

var newTerminateInstancesWaiterEC2 = func(client ec2.DescribeInstancesAPIClient, optFns ...func(*ec2.InstanceTerminatedWaiterOptions)) instanceTerminatedWaiterEC2 {
	return ec2.NewInstanceTerminatedWaiter(client, optFns...)
}

// S3PermissionsMatrix Maps a requested permission to all permissions that are sufficient for the requested one
var S3PermissionsMatrix = map[s3types.Permission][]s3types.Permission{
	s3types.PermissionRead:        {s3types.PermissionRead, s3types.PermissionWrite, s3types.PermissionFullControl},
	s3types.PermissionWrite:       {s3types.PermissionWrite, s3types.PermissionFullControl},
	s3types.PermissionFullControl: {s3types.PermissionFullControl},
	s3types.PermissionReadAcp:     {s3types.PermissionReadAcp, s3types.PermissionWriteAcp},
	s3types.PermissionWriteAcp:    {s3types.PermissionWriteAcp},
}

// Create a new session from the credentials and the region and returns an *AWS object initialized with it.
func newAwsFromConfig(cfg aws.Config) *AWS {
	s3cli := s3.NewFromConfig(cfg)
	return &AWS{
		ec2:        ec2.NewFromConfig(cfg),
		s3:         s3cli,
		s3uploader: s3manager.NewUploader(s3cli),
		s3presign:  s3.NewPresignClient(s3cli),
	}
}

// Initialize a new AWS object from individual bits. SessionToken is optional
func New(region string, accessKeyID string, accessKey string, sessionToken string) (*AWS, error) {
	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
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
		context.TODO(),
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
		context.TODO(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, err
	}
	aws := newAwsFromConfig(cfg)
	return aws, nil
}

// Create a new session from the credentials and the region and returns an *AWS object initialized with it.
func newAwsFromCredsWithEndpoint(optsFunc config.LoadOptionsFunc, region, endpoint, caBundle string, skipSSLVerification bool) (*AWS, error) {
	// Create a Session with a custom region
	optionFuncs := []func(*config.LoadOptions) error{
		config.WithRegion(region),
		optsFunc,
	}

	if caBundle != "" {
		caBundleReader, err := os.Open(caBundle)
		if err != nil {
			return nil, err
		}
		defer caBundleReader.Close()
		optionFuncs = append(optionFuncs, config.WithCustomCABundle(caBundleReader))
	}

	if skipSSLVerification {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // #nosec G402
		optionFuncs = append(optionFuncs, config.WithHTTPClient(&http.Client{
			Transport: transport,
		}))
	}

	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		optionFuncs...,
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
		s3:         s3cli,
		s3uploader: s3manager.NewUploader(s3cli),
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

func (a *AWS) Upload(filename, bucket, key string) (*s3manager.UploadOutput, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}

	defer func() {
		err := file.Close()
		if err != nil {
			olog.Printf("[AWS] ‼ Failed to close the file uploaded to S3️: %v", err)
		}
	}()
	return a.UploadFromReader(file, bucket, key)
}

func (a *AWS) UploadFromReader(r io.Reader, bucket, key string) (*s3manager.UploadOutput, error) {
	olog.Printf("[AWS] 🚀 Uploading image to S3: %s/%s", bucket, key)
	return a.s3uploader.Upload(
		context.TODO(),
		&s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
			Body:   r,
		},
	)
}

func ec2BootMode(bootMode *platform.BootMode) (ec2types.BootModeValues, error) {
	if bootMode == nil {
		return ec2types.BootModeValues(""), nil
	}

	switch *bootMode {
	case platform.BOOT_LEGACY:
		return ec2types.BootModeValuesLegacyBios, nil
	case platform.BOOT_UEFI:
		return ec2types.BootModeValuesUefi, nil
	case platform.BOOT_HYBRID:
		return ec2types.BootModeValuesUefiPreferred, nil
	default:
		return ec2types.BootModeValues(""), fmt.Errorf("invalid boot mode: %s", *bootMode)
	}
}

// Register is a function that imports a snapshot, waits for the snapshot to
// fully import, tags the snapshot, cleans up the image in S3, and registers
// an AMI in AWS.
// The caller can optionally specify the boot mode of the AMI. If the boot
// mode is not specified, then the instances launched from this AMI use the
// default boot mode value of the instance type.
func (a *AWS) Register(name, bucket, key string, tags [][2]string, shareWith []string, architecture arch.Arch, bootMode *platform.BootMode, importRole *string) (string, string, error) {
	rpmArchToEC2Arch := map[arch.Arch]ec2types.ArchitectureValues{
		arch.ARCH_X86_64:  ec2types.ArchitectureValuesX8664,
		arch.ARCH_AARCH64: ec2types.ArchitectureValuesArm64,
	}

	ec2Arch, validArch := rpmArchToEC2Arch[architecture]
	if !validArch {
		return "", "", fmt.Errorf("ec2 doesn't support the following arch: %s", architecture)
	}

	ec2BootMode, err := ec2BootMode(bootMode)
	if err != nil {
		return "", "", fmt.Errorf("ec2 doesn't support the following boot mode: %s", bootMode)
	}

	olog.Printf("[AWS] 📥 Importing snapshot from image: %s/%s", bucket, key)
	snapshotDescription := fmt.Sprintf("Image Builder AWS Import of %s", name)
	importTaskOutput, err := a.ec2.ImportSnapshot(
		context.TODO(),
		&ec2.ImportSnapshotInput{
			Description: aws.String(snapshotDescription),
			DiskContainer: &ec2types.SnapshotDiskContainer{
				UserBucket: &ec2types.UserBucket{
					S3Bucket: aws.String(bucket),
					S3Key:    aws.String(key),
				},
			},
			RoleName: importRole,
		},
	)
	if err != nil {
		olog.Printf("[AWS] error importing snapshot: %s", err)
		return "", "", err
	}

	olog.Printf("[AWS] 🚚 Waiting for snapshot to finish importing: %s", *importTaskOutput.ImportTaskId)
	snapWaiter := newSnapshotImportedWaiterEC2(a.ec2)
	snapWaitOutput, err := snapWaiter.WaitForOutput(
		context.TODO(),
		&ec2.DescribeImportSnapshotTasksInput{
			ImportTaskIds: []string{
				*importTaskOutput.ImportTaskId,
			},
		},
		time.Hour*24,
	)
	if err != nil {
		return "", "", err
	}

	snapshotTaskStatus := *snapWaitOutput.ImportSnapshotTasks[0].SnapshotTaskDetail.Status
	if snapshotTaskStatus != "completed" {
		return "", "", fmt.Errorf("Unable to import snapshot, task result: %v, msg: %v", snapshotTaskStatus, *snapWaitOutput.ImportSnapshotTasks[0].SnapshotTaskDetail.StatusMessage)
	}

	// we no longer need the object in s3, let's just delete it
	olog.Printf("[AWS] 🧹 Deleting image from S3: %s/%s", bucket, key)
	err = a.DeleteObject(bucket, key)
	if err != nil {
		return "", "", err
	}

	ec2Tags := []ec2types.Tag{
		{
			Key:   aws.String("Name"),
			Value: aws.String(name),
		},
	}
	for _, tag := range tags {
		ec2Tags = append(ec2Tags, ec2types.Tag{
			Key:   aws.String(tag[0]),
			Value: aws.String(tag[1]),
		})
	}

	snapshotID := *snapWaitOutput.ImportSnapshotTasks[0].SnapshotTaskDetail.SnapshotId
	// Tag the snapshot with the image name.
	_, err = a.ec2.CreateTags(
		context.TODO(),
		&ec2.CreateTagsInput{
			Resources: []string{snapshotID},
			Tags:      ec2Tags,
		},
	)
	if err != nil {
		return "", "", err
	}

	olog.Printf("[AWS] 📋 Registering AMI from imported snapshot: %s", snapshotID)
	registerOutput, err := a.ec2.RegisterImage(
		context.TODO(),
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
		return "", "", err
	}

	imageID := aws.ToString(registerOutput.ImageId)
	olog.Printf("[AWS] 🎉 AMI registered: %s", imageID)

	// Tag the image with the image name.
	_, err = a.ec2.CreateTags(
		context.TODO(),
		&ec2.CreateTagsInput{
			Resources: []string{imageID},
			Tags:      ec2Tags,
		},
	)
	if err != nil {
		return "", "", err
	}

	if len(shareWith) > 0 {
		err = a.ShareImage(imageID, []string{snapshotID}, shareWith)
		if err != nil {
			return "", "", err
		}
	}

	return imageID, snapshotID, nil
}

func (a *AWS) DeleteObject(bucket, key string) error {
	_, err := a.s3.DeleteObject(
		context.TODO(),
		&s3.DeleteObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(key),
		},
	)
	return err
}

// ShareImage shares the AMI and its associated snapshots with the specified user IDs.
// If no snapshot IDs are provided, it will find the snapshot IDs associated with the AMI.
func (a *AWS) ShareImage(ami string, snapshotIDs, userIDs []string) error {
	// If no snapshot IDs are provided, we will try to find the snapshot IDs
	// associated with the AMI.
	if len(snapshotIDs) == 0 {
		imgs, err := a.ec2.DescribeImages(
			context.TODO(),
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
			snapshotIDs = append(snapshotIDs, *bdm.Ebs.SnapshotId)
		}
	}

	for _, snapshotID := range snapshotIDs {
		err := a.shareSnapshot(snapshotID, userIDs)
		if err != nil {
			return err
		}
	}

	err := a.shareImage(ami, userIDs)
	if err != nil {
		return err
	}
	return nil
}

func (a *AWS) shareImage(ami string, userIDs []string) error {
	olog.Println("[AWS] 💿 Sharing ec2 AMI")
	var launchPerms []ec2types.LaunchPermission

	for idx := range userIDs {
		launchPerms = append(launchPerms, ec2types.LaunchPermission{
			UserId: aws.String(userIDs[idx]),
		})
	}
	_, err := a.ec2.ModifyImageAttribute(
		context.TODO(),
		&ec2.ModifyImageAttributeInput{
			ImageId: &ami,
			LaunchPermission: &ec2types.LaunchPermissionModifications{
				Add: launchPerms,
			},
		},
	)
	if err != nil {
		olog.Printf("[AWS] 📨 Error sharing AMI: %v", err)
		return err
	}
	olog.Println("[AWS] 💿 Shared AMI")
	return nil
}

func (a *AWS) shareSnapshot(snapshotId string, userIds []string) error {
	olog.Println("[AWS] 🎥 Sharing ec2 snapshot")
	_, err := a.ec2.ModifySnapshotAttribute(
		context.TODO(),
		&ec2.ModifySnapshotAttributeInput{
			Attribute:     ec2types.SnapshotAttributeNameCreateVolumePermission,
			OperationType: ec2types.OperationTypeAdd,
			SnapshotId:    &snapshotId,
			UserIds:       userIds,
		},
	)
	if err != nil {
		olog.Printf("[AWS] 📨 Error sharing ec2 snapshot: %v", err)
		return err
	}
	olog.Println("[AWS] 📨 Shared ec2 snapshot")
	return nil
}

func (a *AWS) S3ObjectPresignedURL(bucket, objectKey string) (string, error) {
	olog.Printf("[AWS] 📋 Generating Presigned URL for S3 object %s/%s", bucket, objectKey)
	req, err := a.s3presign.PresignGetObject(
		context.TODO(),
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

	olog.Println("[AWS] 🎉 S3 Presigned URL ready")
	return req.URL, nil
}

func (a *AWS) MarkS3ObjectAsPublic(bucket, objectKey string) error {
	olog.Printf("[AWS] 👐 Making S3 object public %s/%s", bucket, objectKey)
	_, err := a.s3.PutObjectAcl(
		context.TODO(),
		&s3.PutObjectAclInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(objectKey),
			ACL:    s3types.ObjectCannedACL(s3types.ObjectCannedACLPublicRead),
		},
	)
	if err != nil {
		return err
	}

	olog.Println("[AWS] ✔️ Making S3 object public successful")
	return nil
}

func (a *AWS) Regions() ([]string, error) {
	out, err := a.ec2.DescribeRegions(
		context.TODO(),
		&ec2.DescribeRegionsInput{},
	)
	if err != nil {
		return nil, err
	}

	result := []string{}
	for _, r := range out.Regions {
		result = append(result, aws.ToString(r.RegionName))
	}

	return result, nil
}

func (a *AWS) Buckets() ([]string, error) {
	out, err := a.s3.ListBuckets(
		context.TODO(),
		nil,
	)
	if err != nil {
		return nil, err
	}

	result := []string{}
	for _, b := range out.Buckets {
		result = append(result, aws.ToString(b.Name))
	}

	return result, nil
}

// checkAWSPermissionMatrix internal helper function, checks if the requiredPermission is
// covered by the currentPermission (consulting the PermissionsMatrix)
func checkAWSPermissionMatrix(requiredPermission s3types.Permission, currentPermission s3types.Permission) bool {
	requiredPermissions, exists := S3PermissionsMatrix[requiredPermission]
	if !exists {
		return false
	}

	for _, permission := range requiredPermissions {
		if permission == currentPermission {
			return true
		}
	}
	return false
}

// CheckBucketPermission check if the current account (of a.s3) has the `permission` on the given bucket
func (a *AWS) CheckBucketPermission(bucketName string, permission s3types.Permission) (bool, error) {
	resp, err := a.s3.GetBucketAcl(
		context.TODO(),
		&s3.GetBucketAclInput{
			Bucket: aws.String(bucketName),
		},
	)
	if err != nil {
		return false, err
	}

	for _, grant := range resp.Grants {
		if checkAWSPermissionMatrix(permission, grant.Permission) {
			return true, nil
		}
	}
	return false, nil
}

func (a *AWS) CreateSecurityGroupEC2(name, description string) (*ec2.CreateSecurityGroupOutput, error) {
	return a.ec2.CreateSecurityGroup(
		context.TODO(),
		&ec2.CreateSecurityGroupInput{
			GroupName:   aws.String(name),
			Description: aws.String(description),
		},
	)
}

func (a *AWS) DeleteSecurityGroupEC2(groupID string) (*ec2.DeleteSecurityGroupOutput, error) {
	return a.ec2.DeleteSecurityGroup(
		context.TODO(),
		&ec2.DeleteSecurityGroupInput{
			GroupId: &groupID,
		})
}

func (a *AWS) AuthorizeSecurityGroupIngressEC2(groupID, address string, from, to int32, proto string) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	return a.ec2.AuthorizeSecurityGroupIngress(
		context.TODO(),
		&ec2.AuthorizeSecurityGroupIngressInput{
			CidrIp:     aws.String(address),
			GroupId:    &groupID,
			FromPort:   aws.Int32(from),
			ToPort:     aws.Int32(to),
			IpProtocol: aws.String(proto),
		})
}

func (a *AWS) RunInstanceEC2(imageID, secGroupID, userData, instanceType string) (*ec2types.Reservation, error) {
	ec2InstanceType := ec2types.InstanceType(instanceType)
	if !slices.Contains(ec2InstanceType.Values(), ec2InstanceType) {
		return nil, fmt.Errorf("ec2 doesn't support the following instance type: %s", instanceType)
	}

	runInstanceOutput, err := a.ec2.RunInstances(
		context.TODO(),
		&ec2.RunInstancesInput{
			MaxCount:         aws.Int32(1),
			MinCount:         aws.Int32(1),
			ImageId:          &imageID,
			InstanceType:     ec2InstanceType,
			SecurityGroupIds: []string{secGroupID},
			UserData:         aws.String(encodeBase64(userData)),
		})
	if err != nil {
		return nil, err
	}

	if len(runInstanceOutput.Instances) == 0 {
		return nil, fmt.Errorf("no instances were created")
	}
	instanceWaiter := newInstanceRunningWaiterEC2(a.ec2)
	err = instanceWaiter.Wait(
		context.TODO(),
		&ec2.DescribeInstancesInput{
			InstanceIds: []string{aws.ToString(runInstanceOutput.Instances[0].InstanceId)},
		},
		time.Hour,
	)
	if err != nil {
		return nil, err
	}

	reservation, err := a.instanceReservation(aws.ToString(runInstanceOutput.Instances[0].InstanceId))
	if err != nil {
		return nil, fmt.Errorf("failed to get reservation for instance %s: %w", aws.ToString(runInstanceOutput.Instances[0].InstanceId), err)
	}

	return reservation, nil
}

// TerminateInstancesEC2 terminates the specified EC2 instances and waits for them to be terminated if timeout is greater than 0.
func (a *AWS) TerminateInstancesEC2(instanceIDs []string, timeout time.Duration) (*ec2.TerminateInstancesOutput, error) {
	res, err := a.ec2.TerminateInstances(
		context.TODO(),
		&ec2.TerminateInstancesInput{
			InstanceIds: slices.Clone(instanceIDs),
		})
	if err != nil {
		return nil, err
	}

	if timeout > 0 {
		instanceWaiter := newTerminateInstancesWaiterEC2(a.ec2)
		err = instanceWaiter.Wait(
			context.TODO(),
			&ec2.DescribeInstancesInput{
				InstanceIds: slices.Clone(instanceIDs),
			},
			timeout,
		)
		if err != nil {
			return nil, err
		}
	}

	return res, nil
}

func (a *AWS) GetInstanceAddress(instanceID string) (string, error) {
	reservation, err := a.instanceReservation(instanceID)
	if err != nil {
		return "", err
	}

	return *reservation.Instances[0].PublicIpAddress, nil
}

// DeleteEC2Image deletes the specified image and all of its associated snapshots
func (a *AWS) DeleteEC2Image(imageID string) error {
	img, err := a.ec2.DescribeImages(
		context.TODO(),
		&ec2.DescribeImagesInput{
			ImageIds: []string{imageID},
		},
	)
	if err != nil {
		return err
	}

	if len(img.Images) == 0 {
		return fmt.Errorf("image %s not found", imageID)
	}

	var snapshotIDs []string
	for _, bdm := range img.Images[0].BlockDeviceMappings {
		if bdm.Ebs != nil && bdm.Ebs.SnapshotId != nil {
			snapshotIDs = append(snapshotIDs, *bdm.Ebs.SnapshotId)
		}
	}

	var retErr error
	// firstly, deregister the image
	_, err = a.ec2.DeregisterImage(
		context.TODO(),
		&ec2.DeregisterImageInput{
			ImageId: &imageID,
		})

	if err != nil {
		retErr = fmt.Errorf("failed to deregister image %s: %w", imageID, err)
	}

	// now it's possible to delete snapshots
	for _, snapshotID := range snapshotIDs {
		_, err = a.ec2.DeleteSnapshot(
			context.TODO(),
			&ec2.DeleteSnapshotInput{
				SnapshotId: &snapshotID,
			})

		if err != nil {
			if retErr != nil {
				retErr = fmt.Errorf("%w; failed to delete snapshot %s: %v", retErr, snapshotID, err)
				continue
			}
			retErr = fmt.Errorf("failed to delete snapshot %s: %w", snapshotID, err)
		}
	}

	return retErr
}

func (a *AWS) instanceReservation(id string) (*ec2types.Reservation, error) {
	describeInstancesOutput, err := a.ec2.DescribeInstances(
		context.TODO(),
		&ec2.DescribeInstancesInput{
			InstanceIds: []string{id},
		},
	)
	if err != nil {
		return nil, err
	}

	if len(describeInstancesOutput.Reservations) == 0 || len(describeInstancesOutput.Reservations[0].Instances) == 0 {
		return nil, fmt.Errorf("no reservation found for instance %s", id)
	}

	return &describeInstancesOutput.Reservations[0], nil
}

// encodeBase64 encodes string to base64-encoded string
func encodeBase64(input string) string {
	return base64.StdEncoding.EncodeToString([]byte(input))
}
