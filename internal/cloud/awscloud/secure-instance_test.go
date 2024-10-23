package awscloud_test

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
)

func TestSIUserData(t *testing.T) {
	type testCase struct {
		CloudWatchGroup  string
		Hostname         string
		ExpectedUserData string
	}

	testCases := []testCase{
		{
			ExpectedUserData: `#cloud-config
write_files:
  - path: /tmp/worker-run-executor-service
    content: ''
`,
		},
		{
			CloudWatchGroup: "test-group",
			Hostname:        "test-hostname",
			ExpectedUserData: `#cloud-config
write_files:
  - path: /tmp/worker-run-executor-service
    content: ''
  - path: /tmp/cloud_init_vars
    content: |
      OSBUILD_EXECUTOR_CLOUDWATCH_GROUP='test-group'
      OSBUILD_EXECUTOR_HOSTNAME='test-hostname'
`,
		},
		{
			Hostname: "test-hostname",
			ExpectedUserData: `#cloud-config
write_files:
  - path: /tmp/worker-run-executor-service
    content: ''
  - path: /tmp/cloud_init_vars
    content: |
      OSBUILD_EXECUTOR_HOSTNAME='test-hostname'
`,
		},
		{
			CloudWatchGroup: "test-group",
			ExpectedUserData: `#cloud-config
write_files:
  - path: /tmp/worker-run-executor-service
    content: ''
  - path: /tmp/cloud_init_vars
    content: |
      OSBUILD_EXECUTOR_CLOUDWATCH_GROUP='test-group'
`,
		}}

	for idx, tc := range testCases {
		t.Run(fmt.Sprintf("Test case %d", idx), func(t *testing.T) {
			userData := awscloud.SecureInstanceUserData(tc.CloudWatchGroup, tc.Hostname)
			if userData != tc.ExpectedUserData {
				t.Errorf("Expected: %s, got: %s", tc.ExpectedUserData, userData)
			}
		})
	}
}

func TestSIRunSecureInstance(t *testing.T) {
	m := newEc2Mock(t)
	aws := awscloud.NewForTest(m, &ec2imdsmock{t, "instance-id", "region1"}, nil, nil, nil)
	require.NotNil(t, aws)

	si, err := aws.RunSecureInstance("iam-profile", "key-name", "cw-group", "hostname")
	require.NoError(t, err)
	require.NotNil(t, si)
	require.Equal(t, 1, m.calledFn["CreateFleet"])
	require.Equal(t, 1, m.calledFn["CreateSecurityGroup"])
	require.Equal(t, 1, m.calledFn["CreateLaunchTemplate"])
}

func TestSITerminateSecureInstance(t *testing.T) {
	m := newEc2Mock(t)
	aws := awscloud.NewForTest(m, &ec2imdsmock{t, "instance-id", "region1"}, nil, nil, nil)
	require.NotNil(t, aws)

	// Small hack, describeinstances returns terminate/running
	// depending on how many times it was called.
	m.calledFn["DescribeInstances"] = 1

	err := aws.TerminateSecureInstance(&awscloud.SecureInstance{
		FleetID:    "fleet-id",
		SGID:       "sg-id",
		LTID:       "lt-id",
		InstanceID: "instance-id",
	})
	require.NoError(t, err)
	require.Equal(t, 1, m.calledFn["DeleteFleets"])
	require.Equal(t, 1, m.calledFn["DeleteSecurityGroup"])
	require.Equal(t, 1, m.calledFn["DeleteLaunchTemplate"])
	require.Equal(t, 2, m.calledFn["DescribeInstances"])
}

func TestSICreateSGFailures(t *testing.T) {
	m := newEc2Mock(t)
	aws := awscloud.NewForTest(m, &ec2imdsmock{t, "instance-id", "region1"}, nil, nil, nil)
	require.NotNil(t, aws)

	m.failFn["CreateSecurityGroup"] = fmt.Errorf("some-error")
	si, err := aws.RunSecureInstance("iam-profile", "key-name", "cw-group", "hostname")
	require.Error(t, err)
	require.Nil(t, si)
	require.Equal(t, 1, m.calledFn["CreateSecurityGroup"])
	require.Equal(t, 1, m.calledFn["DeleteSecurityGroup"])
	require.Equal(t, 0, m.calledFn["CreateFleet"])
	require.Equal(t, 0, m.calledFn["CreateLaunchTemplate"])
	require.Equal(t, 0, m.calledFn["DeleteLaunchTemplate"])
}

func TestSICreateLTFailures(t *testing.T) {
	m := newEc2Mock(t)
	aws := awscloud.NewForTest(m, &ec2imdsmock{t, "instance-id", "region1"}, nil, nil, nil)
	require.NotNil(t, aws)

	m.failFn["CreateLaunchTemplate"] = fmt.Errorf("some-error")
	si, err := aws.RunSecureInstance("iam-profile", "key-name", "cw-group", "hostname")
	require.Error(t, err)
	require.Nil(t, si)
	require.Equal(t, 1, m.calledFn["CreateSecurityGroup"])
	require.Equal(t, 2, m.calledFn["DeleteSecurityGroup"])
	require.Equal(t, 1, m.calledFn["CreateLaunchTemplate"])
	require.Equal(t, 1, m.calledFn["DeleteLaunchTemplate"])
	require.Equal(t, 0, m.calledFn["CreateFleet"])
}

func TestSICreateFleetFailures(t *testing.T) {
	m := newEc2Mock(t)
	aws := awscloud.NewForTest(m, &ec2imdsmock{t, "instance-id", "region1"}, nil, nil, nil)
	require.NotNil(t, aws)

	// create fleet error should call create fleet thrice
	m.failFn["CreateFleet"] = nil
	si, err := aws.RunSecureInstance("iam-profile", "key-name", "cw-group", "hostname")
	require.Error(t, err)
	require.Nil(t, si)
	require.Equal(t, 3, m.calledFn["CreateFleet"])
	require.Equal(t, 1, m.calledFn["CreateSecurityGroup"])
	require.Equal(t, 1, m.calledFn["CreateLaunchTemplate"])
	require.Equal(t, 2, m.calledFn["DeleteSecurityGroup"])
	require.Equal(t, 2, m.calledFn["DeleteLaunchTemplate"])

	// other errors should just fail immediately
	m.failFn["CreateFleet"] = fmt.Errorf("random error")
	si, err = aws.RunSecureInstance("iam-profile", "key-name", "cw-group", "hostname")
	require.Error(t, err)
	require.Nil(t, si)
	require.Equal(t, 4, m.calledFn["CreateFleet"])
	require.Equal(t, 2, m.calledFn["CreateSecurityGroup"])
	require.Equal(t, 2, m.calledFn["CreateLaunchTemplate"])
	require.Equal(t, 4, m.calledFn["DeleteSecurityGroup"])
	require.Equal(t, 4, m.calledFn["DeleteLaunchTemplate"])
}

func TestDoCreateFleetRetry(t *testing.T) {
	cfOutput := &ec2.CreateFleetOutput{
		Errors: []ec2types.CreateFleetError{
			{
				ErrorCode:    aws.String("UnfulfillableCapacity"),
				ErrorMessage: aws.String("Msg"),
			},
		},
	}
	retry, fmtErrs := awscloud.DoCreateFleetRetry(cfOutput)
	require.True(t, retry)
	require.Equal(t, []string{"UnfulfillableCapacity: Msg"}, fmtErrs)

	cfOutput = &ec2.CreateFleetOutput{
		Errors: []ec2types.CreateFleetError{
			{
				ErrorCode:    aws.String("Bogus"),
				ErrorMessage: aws.String("Msg"),
			},
			{
				ErrorCode:    aws.String("InsufficientInstanceCapacity"),
				ErrorMessage: aws.String("Msg"),
			},
		},
	}
	retry, fmtErrs = awscloud.DoCreateFleetRetry(cfOutput)
	require.True(t, retry)
	require.Equal(t, []string{"Bogus: Msg", "InsufficientInstanceCapacity: Msg"}, fmtErrs)

	cfOutput = &ec2.CreateFleetOutput{
		Errors: []ec2types.CreateFleetError{
			{
				ErrorCode:    aws.String("Bogus"),
				ErrorMessage: aws.String("Msg"),
			},
		},
	}
	retry, fmtErrs = awscloud.DoCreateFleetRetry(cfOutput)
	require.False(t, retry)
	require.Equal(t, []string{"Bogus: Msg"}, fmtErrs)

	cfOutput = &ec2.CreateFleetOutput{
		Errors: []ec2types.CreateFleetError{
			{
				ErrorCode:    aws.String("InsufficientInstanceCapacity"),
				ErrorMessage: aws.String("Msg"),
			},
		},
		Instances: []ec2types.CreateFleetInstance{
			{
				InstanceIds: []string{
					"instance-id",
				},
			},
		},
	}
	retry, fmtErrs = awscloud.DoCreateFleetRetry(cfOutput)
	require.False(t, retry)
	require.Equal(t, []string{"InsufficientInstanceCapacity: Msg", "Already launched instance ([instance-id]), aborting create fleet"}, fmtErrs)
}
