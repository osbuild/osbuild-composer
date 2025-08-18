package worker

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

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

// Test that that exports set in the OSBuildJob get added to all targets
// defined in the job.
// This covers the case when new worker receives a job from old composer.
// This covers the case when new worker receives a job from new composer.
func TestOSBuildJobExportsCompatibilityUnmarshal(t *testing.T) {
	testCases := []struct {
		jobJSON         []byte
		job             *OSBuildJob
		expectedExports []string
		err             bool
	}{
		// Test that one export specified on the job level gets added to each target
		{
			jobJSON: []byte(`{"export_stages":["archive"],"targets":[{"name":"org.osbuild.aws","options":{}}]}`),
			job: &OSBuildJob{
				Targets: []*target.Target{
					{
						Name: target.TargetNameAWS,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportName: "archive",
						},
						Options: &target.AWSTargetOptions{},
					},
				},
				PipelineNames: &PipelineNames{
					Build:   BuildPipelinesFallback,
					Payload: PayloadPipelinesFallback,
				},
			},
			expectedExports: []string{"archive"},
		},
		// Test that one export specified on the job level gets added to each target
		{
			jobJSON: []byte(`{"export_stages":["archive"],"targets":[{"name":"org.osbuild.aws","options":{}},{"name":"org.osbuild.aws.s3","options":{}}]}`),
			job: &OSBuildJob{
				Targets: []*target.Target{
					{
						Name: target.TargetNameAWS,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportName: "archive",
						},
						Options: &target.AWSTargetOptions{},
					},
					{
						Name: target.TargetNameAWSS3,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportName: "archive",
						},
						Options: &target.AWSS3TargetOptions{},
					},
				},
				PipelineNames: &PipelineNames{
					Build:   BuildPipelinesFallback,
					Payload: PayloadPipelinesFallback,
				},
			},
			expectedExports: []string{"archive"},
		},
		// Test that the job as Marshalled by the current compatibility code is also acceptable/
		// Such job has exports set on the job level, but also in the targets
		{
			jobJSON: []byte(`{"export_stages":["archive"],"targets":[{"name":"org.osbuild.aws","options":{},"osbuild_export":"archive"},{"name":"org.osbuild.aws.s3","options":{},"osbuild_export":"archive"}]}`),
			job: &OSBuildJob{
				Targets: []*target.Target{
					{
						Name: target.TargetNameAWS,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportName: "archive",
						},
						Options: &target.AWSTargetOptions{},
					},
					{
						Name: target.TargetNameAWSS3,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportName: "archive",
						},
						Options: &target.AWSS3TargetOptions{},
					},
				},
				PipelineNames: &PipelineNames{
					Build:   BuildPipelinesFallback,
					Payload: PayloadPipelinesFallback,
				},
			},
			expectedExports: []string{"archive"},
		},
		// Test that different exports defined on the job and target level generate an error
		{
			jobJSON: []byte(`{"export_stages":["archive"],"targets":[{"name":"org.osbuild.aws","options":{},"osbuild_artifact":{"export_name":"image"}},{"name":"org.osbuild.aws.s3","options":{},"osbuild_artifact":{"export_name":"archive"}}]}`),
			err:     true,
		},
		// Test that different exports defined on the job and target level generate an error
		{
			jobJSON: []byte(`{"export_stages":["image"],"targets":[{"name":"org.osbuild.aws","options":{},"osbuild_artifact":{"export_name":"archive"}},{"name":"org.osbuild.aws.s3","options":{},"osbuild_artifact":{"export_name":"archive"}}]}`),
			err:     true,
		},
		// Test that multiple exports defined on the job level generate an error
		{
			jobJSON: []byte(`{"export_stages":["archive","image"],"targets":[{"name":"org.osbuild.aws","options":{},"osbuild_export":"archive"},{"name":"org.osbuild.aws.s3","options":{},"osbuild_export":"archive"}]}`),
			err:     true,
		},
	}

	for idx, testCase := range testCases {
		t.Run(fmt.Sprintf("Case #%d", idx), func(t *testing.T) {
			gotJob := OSBuildJob{}
			err := json.Unmarshal(testCase.jobJSON, &gotJob)
			if testCase.err {
				assert.Error(t, err)
			} else {
				assert.Nil(t, err)
				assert.EqualValues(t, testCase.job, &gotJob)
				assert.EqualValues(t, testCase.expectedExports, gotJob.OsbuildExports())
			}
		})
	}
}

// Test that that exports set in the OSBuildJob target get added to the job
// definition itself.
// This covers the case when jobs from new composer are to be picked by old worker.
// This covers the case when new worker receives a job from new composer.
func TestOSBuildJobExportsCompatibilityMarshal(t *testing.T) {
	testCases := []struct {
		jobJSON []byte
		job     *OSBuildJob
	}{
		// Test that the export specified in the target is set also on the job level as it used to be in the past
		{
			jobJSON: []byte(`{"targets":[{"uuid":"00000000-0000-0000-0000-000000000000","image_name":"","name":"org.osbuild.aws","created":"0001-01-01T00:00:00Z","status":"WAITING","options":{"region":"","accessKeyID":"","secretAccessKey":"","sessionToken":"","bucket":"","key":"","shareWithAccounts":null,"filename":""},"osbuild_artifact":{"export_filename":"","export_name":"archive"}}],"export_stages":["archive"]}`),
			job: &OSBuildJob{
				Targets: []*target.Target{
					{
						Name: target.TargetNameAWS,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportName: "archive",
						},
						Options: &target.AWSTargetOptions{},
					},
				},
			},
		},
		// Test that the export specified in multiple targets is set also on the job level as it used to be in the past.
		// We do not test with multiple different exports, because multiple exports in job definition were never allowed.
		{
			jobJSON: []byte(`{"targets":[{"uuid":"00000000-0000-0000-0000-000000000000","image_name":"","name":"org.osbuild.aws","created":"0001-01-01T00:00:00Z","status":"WAITING","options":{"region":"","accessKeyID":"","secretAccessKey":"","sessionToken":"","bucket":"","key":"","shareWithAccounts":null,"filename":""},"osbuild_artifact":{"export_filename":"","export_name":"archive"}},{"uuid":"00000000-0000-0000-0000-000000000000","image_name":"","name":"org.osbuild.aws.s3","created":"0001-01-01T00:00:00Z","status":"WAITING","options":{"region":"","accessKeyID":"","secretAccessKey":"","sessionToken":"","bucket":"","key":"","endpoint":"","ca_bundle":"","skip_ssl_verification":false,"filename":""},"osbuild_artifact":{"export_filename":"","export_name":"archive"}}],"export_stages":["archive"]}`),
			job: &OSBuildJob{
				Targets: []*target.Target{
					{
						Name: target.TargetNameAWS,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportName: "archive",
						},
						Options: &target.AWSTargetOptions{},
					},
					{
						Name: target.TargetNameAWSS3,
						OsbuildArtifact: target.OsbuildArtifact{
							ExportName: "archive",
						},
						Options: &target.AWSS3TargetOptions{},
					},
				},
			},
		},
	}

	for idx, testCase := range testCases {
		t.Run(fmt.Sprintf("Case #%d", idx), func(t *testing.T) {
			gotJSON, err := json.Marshal(testCase.job)
			assert.Nil(t, err)
			assert.EqualValues(t, testCase.jobJSON, gotJSON)
		})
	}
}
