package awscloud

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/sirupsen/logrus"
)

type SecureInstance struct {
	FleetID  string
	SGID     string
	LTID     string
	Instance *ec2.Instance
}

const UserData = `#cloud-config
write_files:
  - path: /tmp/worker-run-executor-service
    content: ''
`

// Runs an instance with a security group that only allows traffic to
// the host. Will replace resources if they already exists.
func (a *AWS) RunSecureInstance(iamProfile string) (*SecureInstance, error) {
	identity, err := a.ec2metadata.GetInstanceIdentityDocument()
	if err != nil {
		logrus.Errorf("Error getting the identity document, %s", err)
		return nil, err
	}

	descrInstancesOutput, err := a.ec2.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			aws.String(identity.InstanceID),
		},
	})
	if err != nil {
		return nil, err
	}
	if len(descrInstancesOutput.Reservations) != 1 || len(descrInstancesOutput.Reservations[0].Instances) != 1 {
		return nil, fmt.Errorf("Expected exactly one reservation (got %d) with one instance (got %d)", len(descrInstancesOutput.Reservations), len(descrInstancesOutput.Reservations[0].Instances))
	}
	vpcID := *descrInstancesOutput.Reservations[0].Instances[0].VpcId
	imageID := *descrInstancesOutput.Reservations[0].Instances[0].ImageId
	instanceType := *descrInstancesOutput.Reservations[0].Instances[0].InstanceType

	secureInstance := &SecureInstance{}
	defer func() {
		if secureInstance.Instance == nil {
			logrus.Errorf("Unable to create secure instance, deleting resources")
			if err := a.TerminateSecureInstance(secureInstance); err != nil {
				logrus.Errorf("Deleting secure instance in defer unsuccessful: %v", err)
			}
		}
	}()

	sgID, err := a.createOrReplaceSG(identity.InstanceID, identity.PrivateIP, vpcID)
	if sgID != "" {
		secureInstance.SGID = sgID
	}
	if err != nil {
		return nil, err
	}

	ltID, err := a.createOrReplaceLT(identity.InstanceID, imageID, sgID, instanceType, iamProfile)
	if ltID != "" {
		secureInstance.LTID = ltID
	}
	if err != nil {
		return nil, err
	}

	descrSubnetsOutput, err := a.ec2.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			&ec2.Filter{
				Name: aws.String("vpc-id"),
				Values: []*string{
					aws.String(vpcID),
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

	// For creating a fleet in a non-default VPC, AWS needs the subnets, and at most 1 subnet per AZ.
	// If a VPC has multiple subnets for a single AZ, only pick the first one.
	overrides := []*ec2.FleetLaunchTemplateOverridesRequest{}
	availZones := map[string]struct{}{}
	for _, subnet := range descrSubnetsOutput.Subnets {
		az := *subnet.AvailabilityZone
		if _, ok := availZones[az]; !ok {
			overrides = append(overrides, &ec2.FleetLaunchTemplateOverridesRequest{
				SubnetId: subnet.SubnetId,
			})
			availZones[az] = struct{}{}
		}
		// A maximum of 4 overrides are allowed
		if len(overrides) == 4 {
			break
		}
	}

	createFleetOutput, err := a.ec2.CreateFleet(&ec2.CreateFleetInput{
		LaunchTemplateConfigs: []*ec2.FleetLaunchTemplateConfigRequest{
			&ec2.FleetLaunchTemplateConfigRequest{
				LaunchTemplateSpecification: &ec2.FleetLaunchTemplateSpecificationRequest{
					LaunchTemplateId: aws.String(secureInstance.LTID),
					Version:          aws.String("1"),
				},
				Overrides: overrides,
			},
		},
		TagSpecifications: []*ec2.TagSpecification{
			&ec2.TagSpecification{
				ResourceType: aws.String(ec2.ResourceTypeInstance),
				Tags: []*ec2.Tag{
					&ec2.Tag{
						Key:   aws.String("parent"),
						Value: aws.String(identity.InstanceID),
					},
				},
			},
		},
		TargetCapacitySpecification: &ec2.TargetCapacitySpecificationRequest{
			DefaultTargetCapacityType: aws.String(ec2.DefaultTargetCapacityTypeSpot),
			TotalTargetCapacity:       aws.Int64(1),
		},
		SpotOptions: &ec2.SpotOptionsRequest{
			AllocationStrategy: aws.String(ec2.SpotAllocationStrategyPriceCapacityOptimized),
		},
		Type: aws.String(ec2.FleetTypeInstant),
	})
	if err != nil {
		return nil, err
	}
	if len(createFleetOutput.Errors) > 0 {
		fleetErrs := []string{}
		for _, fleetErr := range createFleetOutput.Errors {
			fleetErrs = append(fleetErrs, *fleetErr.ErrorMessage)
		}
		return nil, fmt.Errorf("Unable to create fleet: %v", strings.Join(fleetErrs, "; "))
	}
	secureInstance.FleetID = *createFleetOutput.FleetId

	if len(createFleetOutput.Instances) != 1 {
		return nil, fmt.Errorf("Unable to create fleet with exactly one instance, got %d instances", len(createFleetOutput.Instances))
	}
	if len(createFleetOutput.Instances[0].InstanceIds) != 1 {
		return nil, fmt.Errorf("Expected exactly one instance ID on fleet %v, got %d", secureInstance.FleetID, len(createFleetOutput.Instances[0].InstanceIds))
	}

	instanceID := createFleetOutput.Instances[0].InstanceIds[0]
	err = a.ec2.WaitUntilInstanceStatusOk(&ec2.DescribeInstanceStatusInput{
		InstanceIds: []*string{
			instanceID,
		},
	})
	if err != nil {
		return nil, err
	}

	descrInstOutput, err := a.ec2.DescribeInstances(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			instanceID,
		},
	})
	if err != nil {
		return nil, err
	}
	if len(descrInstOutput.Reservations) != 1 {
		return nil, fmt.Errorf("Expected exactly 1 reservation for instance: %s, got %d", *instanceID, len(descrInstOutput.Reservations))
	}
	if len(descrInstOutput.Reservations[0].Instances) != 1 {
		return nil, fmt.Errorf("Expected exactly 1 instance for instance: %s, got %d", *instanceID, len(descrInstOutput.Reservations[0].Instances))
	}
	secureInstance.Instance = descrInstOutput.Reservations[0].Instances[0]

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

func isInvalidGroupNotFoundErr(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == "InvalidGroup.NotFound" {
			return true
		}
	}
	return false
}

func (a *AWS) createOrReplaceSG(hostInstanceID, hostIP, vpcID string) (string, error) {
	sgName := fmt.Sprintf("SG for %s (%s)", hostInstanceID, hostIP)
	descrSGOutput, err := a.ec2.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		GroupNames: []*string{
			aws.String(sgName),
		},
	})
	if err != nil && !isInvalidGroupNotFoundErr(err) {
		return "", err
	}
	for _, sg := range descrSGOutput.SecurityGroups {
		_, err := a.ec2.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
			GroupId: sg.GroupId,
		})
		if err != nil {
			return "", err
		}
	}

	cSGOutput, err := a.ec2.CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
		Description: aws.String(sgName),
		GroupName:   aws.String(sgName),
		VpcId:       aws.String(vpcID),
	})
	if err != nil {
		return "", err
	}
	sgID := *cSGOutput.GroupId

	sgIngressOutput, err := a.ec2.AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
		GroupId: aws.String(sgID),
		IpPermissions: []*ec2.IpPermission{
			&ec2.IpPermission{
				IpProtocol: aws.String(ec2.ProtocolTcp),
				FromPort:   aws.Int64(1),
				ToPort:     aws.Int64(65535),
				IpRanges: []*ec2.IpRange{
					&ec2.IpRange{
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

	describeSGOutput, err := a.ec2.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		GroupIds: []*string{
			aws.String(sgID),
		},
	})
	if err != nil {
		return sgID, err
	}
	if len(describeSGOutput.SecurityGroups) != 1 {
		return sgID, fmt.Errorf("Expected 1 security group, got %d", len(describeSGOutput.SecurityGroups))
	}
	// SGs are created with a predefind egress rule that allows all outgoing traffic, so expecting 1 outbound rule
	if len(describeSGOutput.SecurityGroups[0].IpPermissions) != 1 || len(describeSGOutput.SecurityGroups[0].IpPermissionsEgress) != 1 {
		return sgID, fmt.Errorf("Expected 3 security group rules: 1 inbound (got %d) and 1 outbound (got %d)",
			len(describeSGOutput.SecurityGroups[0].IpPermissions), len(describeSGOutput.SecurityGroups[0].IpPermissionsEgress))
	}

	return sgID, nil
}

func isLaunchTemplateNotFoundError(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		if awsErr.Code() == "InvalidLaunchTemplateId.NotFound" || awsErr.Code() == "InvalidLaunchTemplateName.NotFoundException" {
			return true
		}
	}
	return false

}

func (a *AWS) createOrReplaceLT(hostInstanceID, imageID, sgID, instanceType, iamProfile string) (string, error) {
	ltName := fmt.Sprintf("launch-template-for-%s-runner-instance", hostInstanceID)
	descrLTOutput, err := a.ec2.DescribeLaunchTemplates(&ec2.DescribeLaunchTemplatesInput{
		LaunchTemplateNames: []*string{
			aws.String(ltName),
		},
	})
	if len(descrLTOutput.LaunchTemplates) == 1 {
		_, err := a.ec2.DeleteLaunchTemplate(&ec2.DeleteLaunchTemplateInput{
			LaunchTemplateId: descrLTOutput.LaunchTemplates[0].LaunchTemplateId,
		})
		if err != nil {
			return "", err
		}
	}
	if err != nil && !isLaunchTemplateNotFoundError(err) {
		return "", err
	}

	input := &ec2.CreateLaunchTemplateInput{
		LaunchTemplateData: &ec2.RequestLaunchTemplateData{
			ImageId:                           aws.String(imageID),
			InstanceInitiatedShutdownBehavior: aws.String(ec2.ShutdownBehaviorTerminate),
			InstanceType:                      aws.String(instanceType),
			BlockDeviceMappings: []*ec2.LaunchTemplateBlockDeviceMappingRequest{
				&ec2.LaunchTemplateBlockDeviceMappingRequest{
					DeviceName: aws.String("/dev/sda1"),
					Ebs: &ec2.LaunchTemplateEbsBlockDeviceRequest{
						DeleteOnTermination: aws.Bool(true),
						Encrypted:           aws.Bool(true),
						VolumeSize:          aws.Int64(50),
						VolumeType:          aws.String(ec2.VolumeTypeGp3),
					},
				},
			},
			SecurityGroupIds: []*string{
				aws.String(sgID),
			},
			UserData: aws.String(base64.StdEncoding.EncodeToString([]byte(UserData))),
		},
		TagSpecifications: []*ec2.TagSpecification{
			&ec2.TagSpecification{
				ResourceType: aws.String(ec2.ResourceTypeLaunchTemplate),
				Tags: []*ec2.Tag{
					&ec2.Tag{
						Key:   aws.String("parent"),
						Value: aws.String(hostInstanceID),
					},
				},
			},
		},
		LaunchTemplateName: aws.String(ltName),
	}

	if iamProfile != "" {
		input.LaunchTemplateData.IamInstanceProfile = &ec2.LaunchTemplateIamInstanceProfileSpecificationRequest{
			Name: aws.String(iamProfile),
		}
	}

	createLaunchTemplateOutput, err := a.ec2.CreateLaunchTemplate(input)
	if err != nil {
		return "", err
	}
	return *createLaunchTemplateOutput.LaunchTemplate.LaunchTemplateId, nil
}

func (a *AWS) deleteFleetIfExists(si *SecureInstance) error {
	if si.FleetID == "" {
		return nil
	}

	delFlOutput, err := a.ec2.DeleteFleets(&ec2.DeleteFleetsInput{
		FleetIds: []*string{
			aws.String(si.FleetID),
		},
		TerminateInstances: aws.Bool(true),
	})
	if err != nil {
		return err
	}
	if len(delFlOutput.UnsuccessfulFleetDeletions) != 0 || len(delFlOutput.SuccessfulFleetDeletions) != 1 {
		return fmt.Errorf("Deleting fleet unsuccessful")
	}

	err = a.ec2.WaitUntilInstanceTerminated(&ec2.DescribeInstancesInput{
		InstanceIds: []*string{
			si.Instance.InstanceId,
		},
	})
	if err == nil {
		si.FleetID = ""
	}
	return err
}

func (a *AWS) deleteLTIfExists(si *SecureInstance) error {
	if si.LTID == "" {
		return nil
	}

	_, err := a.ec2.DeleteLaunchTemplate(&ec2.DeleteLaunchTemplateInput{
		LaunchTemplateId: aws.String(si.LTID),
	})
	if err == nil {
		si.LTID = ""
	}
	return err
}

func (a *AWS) deleteSGIfExists(si *SecureInstance) error {
	if si.SGID == "" {
		return nil
	}

	_, err := a.ec2.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
		GroupId: aws.String(si.SGID),
	})
	if err == nil {
		si.SGID = ""
	}
	return err
}
