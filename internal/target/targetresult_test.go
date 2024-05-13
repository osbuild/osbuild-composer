package target

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
	"github.com/stretchr/testify/assert"
)

// Test that `Filename` set in the `Target` options gets set also in the
// `Target.ExportFilename`.
// This covers the case when new worker receives a job from old composer.
// This covers the case when new worker receives a job from new composer.
func TestTargetResultUnmarshal(t *testing.T) {
	testCases := []struct {
		resultJSON     []byte
		expectedResult *TargetResult
		err            bool
	}{
		{
			resultJSON: []byte(`{"name":"org.osbuild.aws","options":{"ami":"ami-123456789","region":"eu"}}`),
			expectedResult: &TargetResult{
				Name: TargetNameAWS,
				Options: &AWSTargetResultOptions{
					Ami:    "ami-123456789",
					Region: "eu",
				},
			},
		},
		{
			resultJSON: []byte(`{"name":"org.osbuild.aws.s3","options":{"url":"https://example.org/image"}}`),
			expectedResult: &TargetResult{
				Name: TargetNameAWSS3,
				Options: &AWSS3TargetResultOptions{
					URL: "https://example.org/image",
				},
			},
		},
		{
			resultJSON: []byte(`{"name":"org.osbuild.gcp","options":{"image_name":"image","project_id":"project"}}`),
			expectedResult: &TargetResult{
				Name: TargetNameGCP,
				Options: &GCPTargetResultOptions{
					ImageName: "image",
					ProjectID: "project",
				},
			},
		},
		{
			resultJSON: []byte(`{"name":"org.osbuild.azure.image","options":{"image_name":"image"}}`),
			expectedResult: &TargetResult{
				Name: TargetNameAzureImage,
				Options: &AzureImageTargetResultOptions{
					ImageName: "image",
				},
			},
		},
		{
			resultJSON: []byte(`{"name":"org.osbuild.koji","options":{"image":{"checksum_type":"md5","checksum":"hash","filename":"image.raw","size":123456}}}`),
			expectedResult: &TargetResult{
				Name: TargetNameKoji,
				Options: &KojiTargetResultOptions{
					Image: &KojiOutputInfo{
						Filename:     "image.raw",
						ChecksumType: ChecksumTypeMD5,
						Checksum:     "hash",
						Size:         123456,
					},
				},
			},
		},
		{
			resultJSON: []byte(`{"name":"org.osbuild.oci","options":{"region":"eu","image_id":"image"}}`),
			expectedResult: &TargetResult{
				Name: TargetNameOCI,
				Options: &OCITargetResultOptions{
					Region:  "eu",
					ImageID: "image",
				},
			},
		},
		{
			resultJSON: []byte(`{"name":"org.osbuild.vmware"}`),
			expectedResult: &TargetResult{
				Name: TargetNameVMWare,
			},
		},
		// target results with error without options
		{
			resultJSON: []byte(`{"name":"org.osbuild.aws","target_error":{"id":11,"reason":"failed to uplad image","details":"detail"}}`),
			expectedResult: &TargetResult{
				Name:        TargetNameAWS,
				TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "failed to uplad image", "detail"),
			},
		},
		// unknown target name
		{
			resultJSON: []byte(`{"name":"org.osbuild.made.up.target","options":{}}`),
			err:        true,
		},
	}

	for idx, testCase := range testCases {
		t.Run(fmt.Sprintf("Case #%d", idx), func(t *testing.T) {
			gotResult := TargetResult{}
			err := json.Unmarshal(testCase.resultJSON, &gotResult)
			if testCase.err {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.EqualValues(t, testCase.expectedResult, &gotResult)
			}
		})
	}
}
