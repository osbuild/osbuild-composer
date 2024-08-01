package awscloud_test

import (
	"fmt"
	"testing"

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
