package worker

import (
	"testing"

	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
	"github.com/stretchr/testify/assert"
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
						TargetError: clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS"),
					},
					{
						Name:        target.TargetNameVMWare,
						TargetError: clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, "can't upload image to VMWare"),
					},
					{
						Name:        target.TargetNameAWSS3,
						TargetError: clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3"),
					},
				},
			},
			targetErrors: []*clienterrors.Error{
				clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", target.TargetNameAWS),
				clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, "can't upload image to VMWare", target.TargetNameVMWare),
				clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", target.TargetNameAWSS3),
			},
		},
		{
			jobResult: OSBuildJobResult{
				TargetResults: []*target.TargetResult{
					{
						Name:        target.TargetNameAWS,
						TargetError: clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS"),
					},
					{
						Name: target.TargetNameVMWare,
					},
					{
						Name:        target.TargetNameAWSS3,
						TargetError: clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3"),
					},
				},
			},
			targetErrors: []*clienterrors.Error{
				clienterrors.WorkerClientError(clienterrors.ErrorInvalidTargetConfig, "can't login to AWS", target.TargetNameAWS),
				clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, "failed to upload image to AWS S3", target.TargetNameAWSS3),
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
