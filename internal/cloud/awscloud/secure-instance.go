package awscloud

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	smithy "github.com/aws/smithy-go"
	"github.com/sirupsen/logrus"
)

type SecureInstance struct {
	FleetID    string
	SGID       string
	LTID       string
	Instance   *ec2types.Instance
	InstanceID string
}

// SecureInstanceUserData returns the cloud-init user data for a secure instance.
func SecureInstanceUserData(cloudWatchGroup, hostname string) string {
	additionalFiles := ""

	if cloudWatchGroup != "" || hostname != "" {
		additionalFiles += `  - path: /tmp/cloud_init_vars
    content: |
`
	}
	if cloudWatchGroup != "" {
		additionalFiles += fmt.Sprintf(`      OSBUILD_EXECUTOR_CLOUDWATCH_GROUP='%s'
`, cloudWatchGroup)
	}
	if hostname != "" {
		additionalFiles += fmt.Sprintf(`      OSBUILD_EXECUTOR_HOSTNAME='%s'
`, hostname)
	}

	return fmt.Sprintf(`#cloud-config
write_files:
  - path: /tmp/worker-run-executor-service
    content: ''
%s`, additionalFiles)
}

// Runs an instance with a security group that only allows traffic to
// the host. Will replace resources if they already exists.
func (a *AWS) RunSecureInstance(iamProfile, keyName, cloudWatchGroup, hostname string) (*SecureInstance, error) {
	identity, err := a.ec2imds.GetInstanceIdentityDocument(context.Background(), &imds.GetInstanceIdentityDocumentInput{})
	if err != nil {
		logrus.Errorf("Error getting the identity document, %s", err)
		return nil, err
	}

	descrInstancesOutput, err := a.ec2.DescribeInstances(
		context.Background(),
		&ec2.DescribeInstancesInput{
			InstanceIds: []string{
				identity.InstanceID,
			},
		},
	)
	if err != nil {
		return nil, err
	}
	if len(descrInstancesOutput.Reservations) != 1 || len(descrInstancesOutput.Reservations[0].Instances) != 1 {
		return nil, fmt.Errorf("Expected exactly one reservation (got %d) with one instance (got %d)", len(descrInstancesOutput.Reservations), len(descrInstancesOutput.Reservations[0].Instances))
	}
	vpcID := *descrInstancesOutput.Reservations[0].Instances[0].VpcId
	imageID := *descrInstancesOutput.Reservations[0].Instances[0].ImageId
	subnetID := *descrInstancesOutput.Reservations[0].Instances[0].SubnetId

	secureInstance := &SecureInstance{}
	defer func() {
		if secureInstance.Instance == nil {
			logrus.Errorf("Unable to create secure instance, deleting resources")
			if err := a.TerminateSecureInstance(secureInstance); err != nil {
				logrus.Errorf("Deleting secure instance in defer unsuccessful: %v", err)
			}
		}
	}()

	previousSI, err := a.terminatePreviousSI(identity.InstanceID)
	if err != nil {
		logrus.Errorf("Unable to terminate previous secure instance %s (parent instance ID: %s): %v", previousSI, identity.InstanceID, err)
		return nil, fmt.Errorf("Unable to terminate previous secure instance %s (parent instance ID: %s): %v", previousSI, identity.InstanceID, err)
	}
	if previousSI != "" {
		logrus.Warningf("Previous instance (%s) terminated by parent instance (%s)", previousSI, identity.InstanceID)
	}

	sgID, err := a.createOrReplaceSG(identity.InstanceID, identity.PrivateIP, vpcID)
	if sgID != "" {
		secureInstance.SGID = sgID
	}
	if err != nil {
		return nil, err
	}

	ltID, err := a.createOrReplaceLT(identity.InstanceID, imageID, sgID, iamProfile, keyName, cloudWatchGroup, hostname)
	if ltID != "" {
		secureInstance.LTID = ltID
	}
	if err != nil {
		return nil, err
	}

	descrSubnetsOutput, err := a.ec2.DescribeSubnets(
		context.Background(),
		&ec2.DescribeSubnetsInput{
			Filters: []ec2types.Filter{
				ec2types.Filter{
					Name: aws.String("vpc-id"),
					Values: []string{
						vpcID,
					},
				},
			},
		})
	if err != nil {
		return nil, err
	}
	if len(descrSubnetsOutput.Subnets) == 0 {
		return nil, fmt.Errorf("Expected at least 1 subnet in the VPC, got 0")
	}

	createFleetOutput, err := a.createFleet(&ec2.CreateFleetInput{
		LaunchTemplateConfigs: []ec2types.FleetLaunchTemplateConfigRequest{
			ec2types.FleetLaunchTemplateConfigRequest{
				LaunchTemplateSpecification: &ec2types.FleetLaunchTemplateSpecificationRequest{
					LaunchTemplateId: aws.String(secureInstance.LTID),
					Version:          aws.String("1"),
				},
				Overrides: []ec2types.FleetLaunchTemplateOverridesRequest{
					ec2types.FleetLaunchTemplateOverridesRequest{
						SubnetId: aws.String(subnetID),
					},
				},
			},
		},
		TagSpecifications: []ec2types.TagSpecification{
			ec2types.TagSpecification{
				ResourceType: ec2types.ResourceTypeInstance,
				Tags: []ec2types.Tag{
					ec2types.Tag{
						Key:   aws.String("parent"),
						Value: aws.String(identity.InstanceID),
					},
				},
			},
		},
		TargetCapacitySpecification: &ec2types.TargetCapacitySpecificationRequest{
			DefaultTargetCapacityType: ec2types.DefaultTargetCapacityTypeSpot,
			TotalTargetCapacity:       aws.Int32(1),
		},
		SpotOptions: &ec2types.SpotOptionsRequest{
			AllocationStrategy: ec2types.SpotAllocationStrategyPriceCapacityOptimized,
		},
		Type: ec2types.FleetTypeInstant,
	})
	if err != nil {
		return nil, err
	}
	secureInstance.FleetID = *createFleetOutput.FleetId
	secureInstance.InstanceID = createFleetOutput.Instances[0].InstanceIds[0]

	instWaiter := ec2.NewInstanceStatusOkWaiter(a.ec2)
	err = instWaiter.Wait(
		context.Background(),
		&ec2.DescribeInstanceStatusInput{
			InstanceIds: []string{
				secureInstance.InstanceID,
			},
		},
		time.Hour,
	)
	if err != nil {
		return nil, err
	}

	descrInstOutput, err := a.ec2.DescribeInstances(
		context.Background(),
		&ec2.DescribeInstancesInput{
			InstanceIds: []string{
				secureInstance.InstanceID,
			},
		})
	if err != nil {
		return nil, err
	}
	if len(descrInstOutput.Reservations) != 1 {
		return nil, fmt.Errorf("Expected exactly 1 reservation for instance: %s, got %d", secureInstance.InstanceID, len(descrInstOutput.Reservations))
	}
	if len(descrInstOutput.Reservations[0].Instances) != 1 {
		return nil, fmt.Errorf("Expected exactly 1 instance for instance: %s, got %d", secureInstance.InstanceID, len(descrInstOutput.Reservations[0].Instances))
	}
	secureInstance.Instance = &descrInstOutput.Reservations[0].Instances[0]

	logrus.Infof(
		"Secure instance created: https://%s.console.aws.amazon.com/ec2/v2/home?region=%s#InstanceDetails:instanceId=%s",
		identity.Region,
		identity.Region,
		secureInstance.InstanceID)

	return secureInstance, nil
}

func (a *AWS) TerminateSecureInstance(si *SecureInstance) error {
	if err := a.deleteFleetIfExists(si); err != nil {
		return err
	}

	if err := a.deleteSGIfExists(si); err != nil {
		return err
	}

	if err := a.deleteLTIfExists(si); err != nil {
		return err
	}
	return nil
}

func (a *AWS) terminatePreviousSI(hostInstanceID string) (string, error) {
	var retryCount int
	const maxRetries = 10

	descrInstancesOutput, err := a.ec2.DescribeInstances(
		context.Background(),
		&ec2.DescribeInstancesInput{
			Filters: []ec2types.Filter{
				ec2types.Filter{
					Name:   aws.String("tag:parent"),
					Values: []string{hostInstanceID},
				},
			},
		})
	if err != nil {
		return "", err
	}
	if len(descrInstancesOutput.Reservations) == 0 || len(descrInstancesOutput.Reservations[0].Instances) == 0 {
		return "", nil
	}

	if descrInstancesOutput.Reservations[0].Instances[0].State.Name == ec2types.InstanceStateNameTerminated {
		return "", nil
	}

	instanceID := *descrInstancesOutput.Reservations[0].Instances[0].InstanceId
	_, err = a.ec2.TerminateInstances(
		context.Background(),
		&ec2.TerminateInstancesInput{
			InstanceIds: []string{instanceID},
		},
	)
	if err != nil {
		return instanceID, err
	}

	retryFunction := func(ctx context.Context, input *ec2.DescribeInstancesInput, output *ec2.DescribeInstancesOutput, err error) (bool, error) {

		if err == nil {
			var lastState ec2types.InstanceStateName = "unknown"
			for _, reservation := range output.Reservations {
				for _, instance := range reservation.Instances {
					if instance.State.Name == ec2types.InstanceStateNameTerminated {
						return false, nil
					}
					lastState = instance.State.Name
				}
			}
			logrus.Warnf("instance not terminated, but %s", lastState)
		} else {
			logrus.Errorf("Error encountered while waiting for termination: %v", err)
		}

		retryCount++
		if retryCount > maxRetries {
			logrus.Errorf("maximum retries reached, while waiting for %v to terminate", instanceID)
			return false, err
		}
		logrus.Warnf("retry waiting for termination %d/%d", retryCount, maxRetries)
		return true, nil

	}

	TerminatedWaiterOptions := func(o *ec2.InstanceTerminatedWaiterOptions) {
		o.Retryable = retryFunction
		o.MinDelay = 60 * time.Second
	}

	instTermWaiter := ec2.NewInstanceTerminatedWaiter(a.ec2)
	err = instTermWaiter.Wait(
		context.Background(),
		&ec2.DescribeInstancesInput{
			InstanceIds: []string{instanceID},
		},
		time.Hour,
		TerminatedWaiterOptions,
	)
	if err != nil {
		return instanceID, err
	}
	return instanceID, nil
}

func isInvalidGroupNotFoundErr(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == "InvalidGroup.NotFound" {
			return true
		}
	}
	return false
}

func (a *AWS) createOrReplaceSG(hostInstanceID, hostIP, vpcID string) (string, error) {
	sgName := fmt.Sprintf("SG for %s (%s)", hostInstanceID, hostIP)
	descrSGOutput, err := a.ec2.DescribeSecurityGroups(
		context.Background(),
		&ec2.DescribeSecurityGroupsInput{
			Filters: []ec2types.Filter{
				ec2types.Filter{
					Name: aws.String("group-name"),
					Values: []string{
						sgName,
					},
				},
			},
		})
	if err != nil && !isInvalidGroupNotFoundErr(err) {
		return "", err
	}
	if descrSGOutput != nil {
		for _, sg := range descrSGOutput.SecurityGroups {
			_, err := a.ec2.DeleteSecurityGroup(
				context.Background(),
				&ec2.DeleteSecurityGroupInput{
					GroupId: sg.GroupId,
				},
			)
			if err != nil {
				return "", err
			}
		}
	}

	cSGOutput, err := a.ec2.CreateSecurityGroup(
		context.Background(),
		&ec2.CreateSecurityGroupInput{
			Description: aws.String(sgName),
			GroupName:   aws.String(sgName),
			VpcId:       aws.String(vpcID),
		},
	)
	if err != nil {
		return "", err
	}
	sgID := *cSGOutput.GroupId

	sgIngressOutput, err := a.ec2.AuthorizeSecurityGroupIngress(
		context.Background(),
		&ec2.AuthorizeSecurityGroupIngressInput{
			GroupId: aws.String(sgID),
			IpPermissions: []ec2types.IpPermission{
				ec2types.IpPermission{
					IpProtocol: aws.String(string(ec2types.ProtocolTcp)),
					FromPort:   aws.Int32(1),
					ToPort:     aws.Int32(65535),
					IpRanges: []ec2types.IpRange{
						ec2types.IpRange{
							CidrIp: aws.String(fmt.Sprintf("%s/32", hostIP)),
						},
					},
				},
			},
		})
	if err != nil {
		return sgID, err
	}
	if !*sgIngressOutput.Return {
		return sgID, fmt.Errorf("Unable to attach ingress rules to SG")
	}

	describeSGOutput, err := a.ec2.DescribeSecurityGroups(
		context.Background(),
		&ec2.DescribeSecurityGroupsInput{
			GroupIds: []string{
				sgID,
			},
		},
	)
	if err != nil {
		return sgID, err
	}

	// SGs are created with a predefind egress rule that allows all outgoing traffic, so expecting 1 outbound rule
	if len(describeSGOutput.SecurityGroups[0].IpPermissions) != 1 || len(describeSGOutput.SecurityGroups[0].IpPermissionsEgress) != 1 {
		return sgID, fmt.Errorf("Expected 2 security group rules: 1 inbound (got %d) and 1 outbound (got %d)",
			len(describeSGOutput.SecurityGroups[0].IpPermissions), len(describeSGOutput.SecurityGroups[0].IpPermissionsEgress))
	}

	return sgID, nil
}

func isLaunchTemplateNotFoundError(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		if apiErr.ErrorCode() == "InvalidLaunchTemplateId.NotFound" || apiErr.ErrorCode() == "InvalidLaunchTemplateName.NotFoundException" {
			return true
		}
	}
	return false
}

func (a *AWS) createOrReplaceLT(hostInstanceID, imageID, sgID, iamProfile, keyName, cloudWatchGroup, hostname string) (string, error) {
	ltName := fmt.Sprintf("launch-template-for-%s-runner-instance", hostInstanceID)
	descrLTOutput, err := a.ec2.DescribeLaunchTemplates(
		context.Background(),
		&ec2.DescribeLaunchTemplatesInput{
			LaunchTemplateNames: []string{
				ltName,
			},
		},
	)

	if err != nil && !isLaunchTemplateNotFoundError(err) {
		return "", err
	}

	if descrLTOutput != nil && len(descrLTOutput.LaunchTemplates) == 1 {
		_, err := a.ec2.DeleteLaunchTemplate(
			context.Background(),
			&ec2.DeleteLaunchTemplateInput{
				LaunchTemplateId: descrLTOutput.LaunchTemplates[0].LaunchTemplateId,
			},
		)
		if err != nil {
			return "", err
		}
	}

	input := &ec2.CreateLaunchTemplateInput{
		LaunchTemplateData: &ec2types.RequestLaunchTemplateData{
			ImageId:                           aws.String(imageID),
			InstanceInitiatedShutdownBehavior: ec2types.ShutdownBehaviorTerminate,
			InstanceRequirements: &ec2types.InstanceRequirementsRequest{
				AcceleratorCount: &ec2types.AcceleratorCountRequest{
					Max: aws.Int32(0),
				},
				BareMetal: ec2types.BareMetalExcluded,
				MemoryMiB: &ec2types.MemoryMiBRequest{
					Min: aws.Int32(4096),
				},
				NetworkInterfaceCount: &ec2types.NetworkInterfaceCountRequest{
					Min: aws.Int32(1),
				},
				VCpuCount: &ec2types.VCpuCountRangeRequest{
					Min: aws.Int32(2),
				},
			},
			BlockDeviceMappings: []ec2types.LaunchTemplateBlockDeviceMappingRequest{
				ec2types.LaunchTemplateBlockDeviceMappingRequest{
					DeviceName: aws.String("/dev/sda1"),
					Ebs: &ec2types.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(true),
						Encrypted:           aws.Bool(true),
						VolumeSize:          aws.Int32(50),
						VolumeType:          ec2types.VolumeTypeGp3,
					},
				},
			},
			SecurityGroupIds: []string{
				sgID,
			},
			UserData: aws.String(base64.StdEncoding.EncodeToString([]byte(SecureInstanceUserData(cloudWatchGroup, hostname)))),
		},
		TagSpecifications: []ec2types.TagSpecification{
			ec2types.TagSpecification{
				ResourceType: ec2types.ResourceTypeLaunchTemplate,
				Tags: []ec2types.Tag{
					ec2types.Tag{
						Key:   aws.String("parent"),
						Value: aws.String(hostInstanceID),
					},
				},
			},
		},
		LaunchTemplateName: aws.String(ltName),
	}

	if iamProfile != "" {
		input.LaunchTemplateData.IamInstanceProfile = &ec2types.LaunchTemplateIamInstanceProfileSpecificationRequest{
			Name: aws.String(iamProfile),
		}
	}

	if keyName != "" {
		input.LaunchTemplateData.KeyName = aws.String(keyName)
	}

	createLaunchTemplateOutput, err := a.ec2.CreateLaunchTemplate(context.Background(), input)
	if err != nil {
		return "", err
	}
	return *createLaunchTemplateOutput.LaunchTemplate.LaunchTemplateId, nil
}

func (a *AWS) deleteFleetIfExists(si *SecureInstance) error {
	if si.FleetID == "" {
		return nil
	}

	delFlOutput, err := a.ec2.DeleteFleets(
		context.Background(),
		&ec2.DeleteFleetsInput{
			FleetIds: []string{
				si.FleetID,
			},
			TerminateInstances: aws.Bool(true),
		})
	if err != nil {
		return err
	}
	if len(delFlOutput.UnsuccessfulFleetDeletions) != 0 || len(delFlOutput.SuccessfulFleetDeletions) != 1 {
		return fmt.Errorf("Deleting fleet unsuccessful")
	}

	if si.InstanceID != "" {
		instTermWaiter := ec2.NewInstanceTerminatedWaiter(a.ec2)
		err = instTermWaiter.Wait(
			context.Background(),
			&ec2.DescribeInstancesInput{
				InstanceIds: []string{si.InstanceID},
			},
			time.Hour,
		)
		if err != nil {
			return err
		}
		si.FleetID = ""
	}
	return nil
}

func (a *AWS) deleteLTIfExists(si *SecureInstance) error {
	if si.LTID == "" {
		return nil
	}

	_, err := a.ec2.DeleteLaunchTemplate(
		context.Background(),
		&ec2.DeleteLaunchTemplateInput{
			LaunchTemplateId: aws.String(si.LTID),
		},
	)
	if err == nil {
		si.LTID = ""
	}
	return err
}

func (a *AWS) deleteSGIfExists(si *SecureInstance) error {
	if si.SGID == "" {
		return nil
	}

	_, err := a.ec2.DeleteSecurityGroup(
		context.Background(),
		&ec2.DeleteSecurityGroupInput{
			GroupId: aws.String(si.SGID),
		},
	)
	if err == nil {
		si.SGID = ""
	}
	return err
}

func (a *AWS) createFleet(input *ec2.CreateFleetInput) (*ec2.CreateFleetOutput, error) {
	createFleetOutput, err := a.ec2.CreateFleet(context.Background(), input)
	if err != nil {
		return nil, fmt.Errorf("Unable to create spot fleet: %w", err)
	}

	retry, fleetErrs := doCreateFleetRetry(createFleetOutput)
	if len(fleetErrs) > 0 && retry {
		logrus.Warnf("Received errors (%s) from CreateFleet, retrying CreateFleet with OnDemand instance", strings.Join(fleetErrs, "; "))
		input.SpotOptions = nil
		input.TargetCapacitySpecification.DefaultTargetCapacityType = ec2types.DefaultTargetCapacityTypeOnDemand
		createFleetOutput, err = a.ec2.CreateFleet(context.Background(), input)
		if err != nil {
			return nil, fmt.Errorf("Unable to create on demand fleet: %w", err)
		}
	}

	retry, fleetErrs = doCreateFleetRetry(createFleetOutput)
	if len(fleetErrs) > 0 && retry {
		logrus.Warnf("Received errors (%s) from CreateFleet with OnDemand instance option, retrying across availability zones", strings.Join(fleetErrs, "; "))
		input.LaunchTemplateConfigs[0].Overrides = nil
		createFleetOutput, err = a.ec2.CreateFleet(context.Background(), input)
		if err != nil {
			return nil, fmt.Errorf("Unable to create on demand fleet across AZs: %w", err)
		}
	}

	if len(createFleetOutput.Errors) > 0 {
		fleetErrs := []string{}
		for _, fleetErr := range createFleetOutput.Errors {
			fleetErrs = append(fleetErrs, fmt.Sprintf("%s: %s", *fleetErr.ErrorCode, *fleetErr.ErrorMessage))
		}
		return nil, fmt.Errorf("Unable to create fleet: %v", strings.Join(fleetErrs, "; "))
	}

	if len(createFleetOutput.Instances) != 1 {
		return nil, fmt.Errorf("Unable to create fleet with exactly one instance, got %d instances", len(createFleetOutput.Instances))
	}
	if len(createFleetOutput.Instances[0].InstanceIds) != 1 {
		return nil, fmt.Errorf("Expected exactly one instance ID on fleet, got %d", len(createFleetOutput.Instances[0].InstanceIds))
	}
	return createFleetOutput, nil
}

func doCreateFleetRetry(cfOutput *ec2.CreateFleetOutput) (bool, []string) {
	if cfOutput == nil {
		return false, nil
	}

	retryCodes := []string{
		"UnfulfillableCapacity",
		"InsufficientInstanceCapacity",
	}
	msg := []string{}
	retry := false
	for _, err := range cfOutput.Errors {
		if slices.Contains(retryCodes, *err.ErrorCode) {
			retry = true
		}
		msg = append(msg, fmt.Sprintf("%s: %s", *err.ErrorCode, *err.ErrorMessage))
	}

	// Do not retry in case an instance already exists, in that case just fail and let the worker terminate the SI
	if len(cfOutput.Instances) > 0 && len(cfOutput.Instances[0].InstanceIds) > 0 {
		retry = false
		msg = append(msg, fmt.Sprintf("Already launched instance (%s), aborting create fleet", cfOutput.Instances[0].InstanceIds))
	}

	return retry, msg
}
