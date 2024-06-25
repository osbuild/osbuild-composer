package awscloud

import (
	"fmt"
	"testing"
)

func TestSecureInstanceUserData(t *testing.T) {
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
			userData := SecureInstanceUserData(tc.CloudWatchGroup, tc.Hostname)
			if userData != tc.ExpectedUserData {
				t.Errorf("Expected: %s, got: %s", tc.ExpectedUserData, userData)
			}
		})
	}
}
