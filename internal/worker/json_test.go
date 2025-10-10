package worker

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

func TestOSBuildJobResultTargetErrors(t *testing.T) {
	testCases := []struct {
		jobResult    OSBuildJobResult
		targetErrors []*clienterrors.Error
	}{
		{
			jobResult: OSBuildJobResult{
				TargetResults: []*target.TargetResult{
					{
						Name:        target.TargetNameAWS,
						TargetError: clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameAWSS3,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", nil),
					},
				},
			},
			targetErrors: []*clienterrors.Error{
				clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", target.TargetNameAWS),
				clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", target.TargetNameVMWare),
				clienterrors.New(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", target.TargetNameAWSS3),
			},
		},
		{
			jobResult: OSBuildJobResult{
				TargetResults: []*target.TargetResult{
					{
						Name:        target.TargetNameAWS,
						TargetError: clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", nil),
					},
					{
						Name: target.TargetNameVMWare,
					},
					{
						Name:        target.TargetNameAWSS3,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", nil),
					},
				},
			},
			targetErrors: []*clienterrors.Error{
				clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", target.TargetNameAWS),
				clienterrors.New(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", target.TargetNameAWSS3),
			},
		},
		{
			jobResult: OSBuildJobResult{
				TargetResults: []*target.TargetResult{
					{
						Name: target.TargetNameAWS,
					},
					{
						Name: target.TargetNameVMWare,
					},
					{
						Name: target.TargetNameAWSS3,
					},
				},
			},
			targetErrors: []*clienterrors.Error{},
		},
		{
			jobResult:    OSBuildJobResult{},
			targetErrors: []*clienterrors.Error{},
		},
	}

	for _, testCase := range testCases {
		assert.EqualValues(t, testCase.targetErrors, testCase.jobResult.TargetErrors())
	}
}

func TestOSBuildJobResultTargetResultsByName(t *testing.T) {
	testCases := []struct {
		jobResult     OSBuildJobResult
		targetName    target.TargetName
		targetResults []*target.TargetResult
	}{
		// one target results of a given name
		{
			jobResult: OSBuildJobResult{
				TargetResults: []*target.TargetResult{
					{
						Name:        target.TargetNameAWS,
						TargetError: clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameAWSS3,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", nil),
					},
				},
			},
			targetName: target.TargetNameAWS,
			targetResults: []*target.TargetResult{
				{
					Name:        target.TargetNameAWS,
					TargetError: clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", nil),
				},
			},
		},
		// multiple target results of a given name
		{
			jobResult: OSBuildJobResult{
				TargetResults: []*target.TargetResult{
					{
						Name:        target.TargetNameAWS,
						TargetError: clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameAWSS3,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", nil),
					},
				},
			},
			targetName: target.TargetNameVMWare,
			targetResults: []*target.TargetResult{
				{
					Name:        target.TargetNameVMWare,
					TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
				},
				{
					Name:        target.TargetNameVMWare,
					TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
				},
			},
		},
		// no target result of a given name
		{
			jobResult: OSBuildJobResult{
				TargetResults: []*target.TargetResult{
					{
						Name:        target.TargetNameAWS,
						TargetError: clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameAWSS3,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", nil),
					},
				},
			},
			targetName:    target.TargetNameKoji,
			targetResults: []*target.TargetResult{},
		},
	}

	for _, testCase := range testCases {
		assert.EqualValues(t, testCase.targetResults, testCase.jobResult.TargetResultsByName(testCase.targetName))
	}
}

func TestOSBuildJobResultTargetResultsFilterByName(t *testing.T) {
	testCases := []struct {
		jobResult     OSBuildJobResult
		targetNames   []target.TargetName
		targetResults []*target.TargetResult
	}{
		{
			jobResult: OSBuildJobResult{
				TargetResults: []*target.TargetResult{
					{
						Name:        target.TargetNameAWS,
						TargetError: clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameAWSS3,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", nil),
					},
				},
			},
			targetNames: []target.TargetName{
				target.TargetNameVMWare,
			},
			targetResults: []*target.TargetResult{
				{
					Name:        target.TargetNameAWS,
					TargetError: clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", nil),
				},
				{
					Name:        target.TargetNameAWSS3,
					TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", nil),
				},
			},
		},
		{
			jobResult: OSBuildJobResult{
				TargetResults: []*target.TargetResult{
					{
						Name:        target.TargetNameAWS,
						TargetError: clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameAWSS3,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", nil),
					},
				},
			},
			targetNames: []target.TargetName{
				target.TargetNameVMWare,
				target.TargetNameAWSS3,
			},
			targetResults: []*target.TargetResult{
				{
					Name:        target.TargetNameAWS,
					TargetError: clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", nil),
				},
			},
		},
		{
			jobResult: OSBuildJobResult{
				TargetResults: []*target.TargetResult{
					{
						Name:        target.TargetNameAWS,
						TargetError: clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameAWSS3,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", nil),
					},
				},
			},
			targetNames: []target.TargetName{
				target.TargetNameAWS,
				target.TargetNameAWSS3,
			},
			targetResults: []*target.TargetResult{
				{
					Name:        target.TargetNameVMWare,
					TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
				},
				{
					Name:        target.TargetNameVMWare,
					TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
				},
			},
		},
		{
			jobResult: OSBuildJobResult{
				TargetResults: []*target.TargetResult{
					{
						Name:        target.TargetNameAWS,
						TargetError: clienterrors.New(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", nil),
					},
					{
						Name:        target.TargetNameAWSS3,
						TargetError: clienterrors.New(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", nil),
					},
				},
			},
			targetNames: []target.TargetName{
				target.TargetNameAWS,
				target.TargetNameVMWare,
				target.TargetNameAWSS3,
			},
			targetResults: []*target.TargetResult{},
		},
	}

	for _, testCase := range testCases {
		assert.EqualValues(t, testCase.targetResults, testCase.jobResult.TargetResultsFilterByName(testCase.targetNames))
	}
}

func TestOSBuildJobExports(t *testing.T) {
	testCases := []struct {
		job             *OSBuildJob
		expectedExports []string
	}{
		// one target with export set
		{
			job: &OSBuildJob{
				Manifest: []byte("manifest"),
				Targets: []*target.Target{
					{
						Name: target.TargetNameAWS,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportName: "archive",
						},
					},
				},
			},
			expectedExports: []string{"archive"},
		},
		// multiple targets with different exports set
		{
			job: &OSBuildJob{
				Manifest: []byte("manifest"),
				Targets: []*target.Target{
					{
						Name: target.TargetNameAWS,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportName: "archive",
						},
					},
					{
						Name: target.TargetNameAWSS3,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportName: "image",
						},
					},
				},
			},
			expectedExports: []string{"archive", "image"},
		},
		// multiple targets with the same export
		{
			job: &OSBuildJob{
				Manifest: []byte("manifest"),
				Targets: []*target.Target{
					{
						Name: target.TargetNameAWS,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportName: "archive",
						},
					},
					{
						Name: target.TargetNameAWSS3,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportName: "archive",
						},
					},
				},
			},
			expectedExports: []string{"archive"},
		},
	}

	for idx, testCase := range testCases {
		t.Run(fmt.Sprintf("case #%d", idx), func(t *testing.T) {
			assert.EqualValues(t, testCase.expectedExports, testCase.job.OsbuildExports())
		})
	}
}

func TestDepsolvedPackageChecksumUnmarshalJSON(t *testing.T) {
	testCases := []struct {
		name                     string
		json                     string
		depsolvedPackageChecksum DepsolvedPackageChecksum
		expectedError            error
	}{
		{
			name: "struct",
			json: `{"type":"sha256","value":"17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76"}`,
			depsolvedPackageChecksum: DepsolvedPackageChecksum{
				Type:  "sha256",
				Value: "17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76",
			},
			expectedError: nil,
		},
		{
			name: "string",
			json: `"sha256:17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76"`,
			depsolvedPackageChecksum: DepsolvedPackageChecksum{
				Type:  "sha256",
				Value: "17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76",
			},
			expectedError: nil,
		},
		{
			name:                     "null",
			json:                     `null`,
			depsolvedPackageChecksum: DepsolvedPackageChecksum{},
		},
		{
			name:                     "empty",
			json:                     `":"`,
			depsolvedPackageChecksum: DepsolvedPackageChecksum{},
		},
		{
			name:          "invalid string",
			json:          `"17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76"`,
			expectedError: errors.New("invalid checksum format: \"17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76\""),
		},
		{
			name:          "invalid type",
			json:          `[{"type":"sha256","value":"17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76"}]`,
			expectedError: errors.New("unsupported checksum type: []interface {}"),
		},
		{
			name:          "invalid struct type",
			json:          `{"typo":"sha256","value":"17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76"}`,
			expectedError: errors.New("checksum type is required, got map[typo:sha256 value:17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76]"),
		},
		{
			name:          "invalid struct value",
			json:          `{"type":"sha256","typo":"17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76"}`,
			expectedError: errors.New("checksum value is required, got map[type:sha256 typo:17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76]"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var depsolvedPackageChecksum DepsolvedPackageChecksum
			err := json.Unmarshal([]byte(testCase.json), &depsolvedPackageChecksum)
			if testCase.expectedError != nil {
				require.Error(t, err)
				assert.EqualValues(t, testCase.expectedError, err)
			} else {
				require.NoError(t, err)
				assert.EqualValues(t, testCase.depsolvedPackageChecksum, depsolvedPackageChecksum)
			}
		})
	}
}

func TestDepsolvedPackageChecksumMarshalJSON(t *testing.T) {
	testCases := []struct {
		name                     string
		depsolvedPackageChecksum *DepsolvedPackageChecksum
		json                     string
	}{
		{
			name:                     "nil",
			depsolvedPackageChecksum: nil,
			json:                     `null`,
		},
		{
			name:                     "empty",
			depsolvedPackageChecksum: &DepsolvedPackageChecksum{},
			json:                     `":"`,
		},
		{
			name: "basic",
			depsolvedPackageChecksum: &DepsolvedPackageChecksum{
				Type:  "sha256",
				Value: "17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76",
			},
			json: `"sha256:17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76"`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			json, err := json.Marshal(testCase.depsolvedPackageChecksum)
			require.NoError(t, err)
			assert.EqualValues(t, testCase.json, string(json))
		})
	}
}

func TestDepsolvedPackageUnmarshalJSON(t *testing.T) {
	testCases := []struct {
		name             string
		json             string
		depsolvedPackage DepsolvedPackage
	}{
		{
			name: "minimal",
			json: `{"name":"test"}`,
			depsolvedPackage: DepsolvedPackage{
				Name: "test",
			},
		},
		// Test old worker response
		{
			name: "legacy-rpmmd-packagespec",
			json: `{"name":"test","epoch":1,"version":"1.0.0","release":"1","arch":"x86_64","remote_location":"https://example.com/rpms/test.rpm","checksum":"sha256:17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76","secrets":"org.osbuild.rhsm","check_gpg":true,"ignore_ssl":true,"path":"rpms/test.rpm","repo_id":"test-repo-id"}`,
			depsolvedPackage: DepsolvedPackage{
				Name:            "test",
				Epoch:           1,
				Version:         "1.0.0",
				Release:         "1",
				Arch:            "x86_64",
				RemoteLocations: []string{"https://example.com/rpms/test.rpm"},
				Checksum: &DepsolvedPackageChecksum{
					Type:  "sha256",
					Value: "17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76",
				},
				Secrets:   "org.osbuild.rhsm",
				CheckGPG:  true,
				IgnoreSSL: true,
				Location:  "rpms/test.rpm",
				RepoID:    "test-repo-id",
			},
		},
		{
			name: "basic from new worker with single remote location",
			json: `{"name":"test","epoch":1,"version":"1.0.0","release":"1","arch":"x86_64","remote_location":"https://example.com/rpms/test.rpm","remote_locations":["https://example.com/rpms/test.rpm"],"checksum":"sha256:17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76","secrets":"org.osbuild.rhsm","check_gpg":true,"ignore_ssl":true,"location":"rpms/test.rpm","path":"rpms/test.rpm","repo_id":"test-repo-id"}`,
			depsolvedPackage: DepsolvedPackage{
				Name:            "test",
				Epoch:           1,
				Version:         "1.0.0",
				Release:         "1",
				Arch:            "x86_64",
				RemoteLocations: []string{"https://example.com/rpms/test.rpm"},
				Checksum: &DepsolvedPackageChecksum{
					Type:  "sha256",
					Value: "17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76",
				},
				Secrets:   "org.osbuild.rhsm",
				CheckGPG:  true,
				IgnoreSSL: true,
				Location:  "rpms/test.rpm",
				RepoID:    "test-repo-id",
			},
		},
		{
			name: "basic from new worker with multiple remote locations",
			json: `{"name":"test","epoch":1,"version":"1.0.0","release":"1","arch":"x86_64","remote_location":"https://example.com/rpms/test.rpm","remote_locations":["https://example.com/rpms/test.rpm","http://example.com/rpms/test.rpm"],"checksum":"sha256:17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76","secrets":"org.osbuild.rhsm","check_gpg":true,"ignore_ssl":true,"location":"rpms/test.rpm","path":"rpms/test.rpm","repo_id":"test-repo-id"}`,
			depsolvedPackage: DepsolvedPackage{
				Name:            "test",
				Epoch:           1,
				Version:         "1.0.0",
				Release:         "1",
				Arch:            "x86_64",
				RemoteLocations: []string{"https://example.com/rpms/test.rpm", "http://example.com/rpms/test.rpm"},
				Checksum: &DepsolvedPackageChecksum{
					Type:  "sha256",
					Value: "17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76",
				},
				Secrets:   "org.osbuild.rhsm",
				CheckGPG:  true,
				IgnoreSSL: true,
				Location:  "rpms/test.rpm",
				RepoID:    "test-repo-id",
			},
		},
		{
			name: "basic from new worker - new properties not overwritten by old properties",
			json: `{"name":"test","epoch":1,"version":"1.0.0","release":"1","arch":"x86_64","remote_location":"should-not-override","remote_locations":["https://example.com/rpms/test.rpm","http://example.com/rpms/test.rpm"],"checksum":"sha256:17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76","secrets":"org.osbuild.rhsm","check_gpg":true,"ignore_ssl":true,"location":"rpms/test.rpm","path":"should-not-override","repo_id":"test-repo-id"}`,
			depsolvedPackage: DepsolvedPackage{
				Name:            "test",
				Epoch:           1,
				Version:         "1.0.0",
				Release:         "1",
				Arch:            "x86_64",
				RemoteLocations: []string{"https://example.com/rpms/test.rpm", "http://example.com/rpms/test.rpm"},
				Checksum: &DepsolvedPackageChecksum{
					Type:  "sha256",
					Value: "17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76",
				},
				Secrets:   "org.osbuild.rhsm",
				CheckGPG:  true,
				IgnoreSSL: true,
				Location:  "rpms/test.rpm",
				RepoID:    "test-repo-id",
			},
		},
		{
			name: "basic new, without backward compatibility",
			json: `{"name":"test","epoch":1,"version":"1.0.0","release":"1","arch":"x86_64","remote_locations":["https://example.com/rpms/test.rpm"],"checksum":"sha256:17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76","secrets":"org.osbuild.rhsm","check_gpg":true,"ignore_ssl":true,"location":"rpms/test.rpm","repo_id":"test-repo-id"}`,
			depsolvedPackage: DepsolvedPackage{
				Name:            "test",
				Epoch:           1,
				Version:         "1.0.0",
				Release:         "1",
				Arch:            "x86_64",
				RemoteLocations: []string{"https://example.com/rpms/test.rpm"},
				Checksum: &DepsolvedPackageChecksum{
					Type:  "sha256",
					Value: "17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76",
				},
				Secrets:   "org.osbuild.rhsm",
				CheckGPG:  true,
				IgnoreSSL: true,
				Location:  "rpms/test.rpm",
				RepoID:    "test-repo-id",
			},
		},
		{
			name: "basic after the checksum change to struct",
			json: `{"name":"test","epoch":1,"version":"1.0.0","release":"1","arch":"x86_64","remote_locations":["https://example.com/rpms/test.rpm"],"checksum":{"type":"sha256","value":"17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76"},"secrets":"org.osbuild.rhsm","check_gpg":true,"ignore_ssl":true,"location":"rpms/test.rpm","repo_id":"test-repo-id"}`,
			depsolvedPackage: DepsolvedPackage{
				Name:            "test",
				Epoch:           1,
				Version:         "1.0.0",
				Release:         "1",
				Arch:            "x86_64",
				RemoteLocations: []string{"https://example.com/rpms/test.rpm"},
				Checksum: &DepsolvedPackageChecksum{
					Type:  "sha256",
					Value: "17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76",
				},
				Secrets:   "org.osbuild.rhsm",
				CheckGPG:  true,
				IgnoreSSL: true,
				Location:  "rpms/test.rpm",
				RepoID:    "test-repo-id",
			},
		},
		{
			name: "checksum-null",
			json: `{"name": "test","checksum": null}`,
			depsolvedPackage: DepsolvedPackage{
				Name: "test",
			},
		},
		{
			name: "full",
			json: `{"name":"test","epoch":1,"version":"1.0.0","release":"1","arch":"x86_64","group":"test-group","download_size":100,"install_size":100,"license":"test-license","source_rpm":"test-source-rpm","build_time":"2021-01-01T00:00:00Z","packager":"test-packager","vendor":"test-vendor","url":"https://example.com/test_project","summary":"test-summary","description":"test-description","provides":[{"name":"test-provide","relationship":"\u003e=","version":"1.0.0"},{"name":"test-provide2"}],"requires":[{"name":"test-require","relationship":"\u003e=","version":"1.0.0"}],"requires_pre":[{"name":"test-require-pre","relationship":"\u003e=","version":"1.0.0"}],"conflicts":[{"name":"test-conflict","relationship":"\u003e=","version":"1.0.0"}],"obsoletes":[{"name":"test-obsolete","relationship":"\u003e=","version":"1.0.0"}],"regular_requires":[{"name":"test-regular-require","relationship":"\u003e=","version":"1.0.0"}],"recommends":[{"name":"test-recommend","relationship":"\u003e=","version":"1.0.0"}],"suggests":[{"name":"test-suggest","relationship":"\u003e=","version":"1.0.0"}],"enhances":[{"name":"test-enhance","relationship":"\u003e=","version":"1.0.0"}],"supplements":[{"name":"test-supplement","relationship":"\u003e=","version":"1.0.0"}],"files":["/usr/bin/test","/usr/lib/test","/usr/share/test","/usr/share/man/test","/usr/share/doc/test","/usr/share/doc/test.gz","/usr/share/doc/test.gz.sig"],"base_url":"https://example.com/test_project/repository","location":"rpms/test.rpm","remote_locations":["https://example.com/test_project/repository/rpms/test.rpm","http://example.com/test_project/repository/rpms/test.rpm"],"checksum":"sha256:17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76","header_checksum":"sha256:17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76","repo_id":"test-repo-id","reason":"test-reason","secrets":"org.osbuild.rhsm","check_gpg":true,"ignore_ssl":true}`,
			depsolvedPackage: DepsolvedPackage{
				Name:         "test",
				Epoch:        1,
				Version:      "1.0.0",
				Release:      "1",
				Arch:         "x86_64",
				Group:        "test-group",
				DownloadSize: 100,
				InstallSize:  100,
				License:      "test-license",
				SourceRpm:    "test-source-rpm",
				BuildTime:    common.ToPtr(time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC)),
				Packager:     "test-packager",
				Vendor:       "test-vendor",
				URL:          "https://example.com/test_project",
				Summary:      "test-summary",
				Description:  "test-description",
				Provides: []DepsolvedPackageRelDep{
					{
						Name:         "test-provide",
						Relationship: ">=",
						Version:      "1.0.0",
					},
					{
						Name: "test-provide2",
					},
				},
				Requires: []DepsolvedPackageRelDep{
					{
						Name:         "test-require",
						Relationship: ">=",
						Version:      "1.0.0",
					},
				},
				RequiresPre: []DepsolvedPackageRelDep{
					{
						Name:         "test-require-pre",
						Relationship: ">=",
						Version:      "1.0.0",
					},
				},
				Conflicts: []DepsolvedPackageRelDep{
					{
						Name:         "test-conflict",
						Relationship: ">=",
						Version:      "1.0.0",
					},
				},
				Obsoletes: []DepsolvedPackageRelDep{
					{
						Name:         "test-obsolete",
						Relationship: ">=",
						Version:      "1.0.0",
					},
				},
				RegularRequires: []DepsolvedPackageRelDep{
					{
						Name:         "test-regular-require",
						Relationship: ">=",
						Version:      "1.0.0",
					},
				},
				Recommends: []DepsolvedPackageRelDep{
					{
						Name:         "test-recommend",
						Relationship: ">=",
						Version:      "1.0.0",
					},
				},
				Suggests: []DepsolvedPackageRelDep{
					{
						Name:         "test-suggest",
						Relationship: ">=",
						Version:      "1.0.0",
					},
				},
				Enhances: []DepsolvedPackageRelDep{
					{
						Name:         "test-enhance",
						Relationship: ">=",
						Version:      "1.0.0",
					},
				},
				Supplements: []DepsolvedPackageRelDep{
					{
						Name:         "test-supplement",
						Relationship: ">=",
						Version:      "1.0.0",
					},
				},
				Files: []string{
					"/usr/bin/test",
					"/usr/lib/test",
					"/usr/share/test",
					"/usr/share/man/test",
					"/usr/share/doc/test",
					"/usr/share/doc/test.gz",
					"/usr/share/doc/test.gz.sig",
				},
				BaseURL:  "https://example.com/test_project/repository",
				Location: "rpms/test.rpm",
				RemoteLocations: []string{
					"https://example.com/test_project/repository/rpms/test.rpm",
					"http://example.com/test_project/repository/rpms/test.rpm",
				},
				Checksum: &DepsolvedPackageChecksum{
					Type:  "sha256",
					Value: "17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76",
				},
				HeaderChecksum: &DepsolvedPackageChecksum{
					Type:  "sha256",
					Value: "17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76",
				},
				RepoID:    "test-repo-id",
				Reason:    "test-reason",
				Secrets:   "org.osbuild.rhsm",
				CheckGPG:  true,
				IgnoreSSL: true,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var depsolvedPackage DepsolvedPackage
			err := json.Unmarshal([]byte(testCase.json), &depsolvedPackage)
			require.NoError(t, err)
			assert.EqualValues(t, testCase.depsolvedPackage, depsolvedPackage)
		})
	}
}

func TestDepsolvedPackageMarshalJSON(t *testing.T) {
	testCases := []struct {
		name             string
		depsolvedPackage DepsolvedPackage
		json             string
	}{
		{
			name: "minimal",
			depsolvedPackage: DepsolvedPackage{
				Name: "test",
			},
			json: `{"name":"test","epoch":0}`,
		},
		{
			name: "basic",
			depsolvedPackage: DepsolvedPackage{
				Name:            "test",
				Epoch:           1,
				Version:         "1.0.0",
				Release:         "1",
				Arch:            "x86_64",
				RemoteLocations: []string{"https://example.com/rpms/test.rpm"},
				Checksum: &DepsolvedPackageChecksum{
					Type:  "sha256",
					Value: "17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76",
				},
				Secrets:   "org.osbuild.rhsm",
				CheckGPG:  true,
				IgnoreSSL: true,
				Location:  "rpms/test.rpm",
				RepoID:    "test-repo-id",
			},
			json: `{"name":"test","epoch":1,"version":"1.0.0","release":"1","arch":"x86_64","location":"rpms/test.rpm","remote_locations":["https://example.com/rpms/test.rpm"],"checksum":"sha256:17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76","repo_id":"test-repo-id","secrets":"org.osbuild.rhsm","check_gpg":true,"ignore_ssl":true,"path":"rpms/test.rpm","remote_location":"https://example.com/rpms/test.rpm"}`,
		},
		{
			name: "basic with multiple remote locations",
			depsolvedPackage: DepsolvedPackage{
				Name:            "test",
				Epoch:           1,
				Version:         "1.0.0",
				Release:         "1",
				Arch:            "x86_64",
				RemoteLocations: []string{"https://example.com/rpms/test.rpm", "http://example.com/rpms/test.rpm"},
				Checksum: &DepsolvedPackageChecksum{
					Type:  "sha256",
					Value: "17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76",
				},
				Secrets:   "org.osbuild.rhsm",
				CheckGPG:  true,
				IgnoreSSL: true,
				Location:  "rpms/test.rpm",
				RepoID:    "test-repo-id",
			},
			json: `{"name":"test","epoch":1,"version":"1.0.0","release":"1","arch":"x86_64","location":"rpms/test.rpm","remote_locations":["https://example.com/rpms/test.rpm","http://example.com/rpms/test.rpm"],"checksum":"sha256:17e682f060b5f8e47ea04c5c4855908b0a5ad612022260fe50e11ecb0cc0ab76","repo_id":"test-repo-id","secrets":"org.osbuild.rhsm","check_gpg":true,"ignore_ssl":true,"path":"rpms/test.rpm","remote_location":"https://example.com/rpms/test.rpm"}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			json, err := json.Marshal(testCase.depsolvedPackage)
			require.NoError(t, err)
			assert.EqualValues(t, testCase.json, string(json))
		})
	}
}
