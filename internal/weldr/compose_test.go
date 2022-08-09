package weldr

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

func TestComposeStatusFromLegacyError(t *testing.T) {

	if len(os.Getenv("OSBUILD_COMPOSER_TEST_EXTERNAL")) > 0 {
		t.Skip("This test is for internal testing only")
	}

	api, _ := createWeldrAPI(t.TempDir(), rpmmd_mock.BaseFixture)

	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}

	jobId, err := api.workers.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest}, "")
	require.NoError(t, err)

	j, token, _, _, _, err := api.workers.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""})
	require.NoError(t, err)
	require.Equal(t, jobId, j)

	jobResult := worker.OSBuildJobResult{
		JobResult: worker.JobResult{
			JobError: &clienterrors.Error{
				ID:     clienterrors.ErrorUploadingImage,
				Reason: "Upload error",
			},
		},
	}
	rawResult, err := json.Marshal(jobResult)
	require.NoError(t, err)
	err = api.workers.FinishJob(token, rawResult)
	require.NoError(t, err)

	jobInfo, err := api.workers.OSBuildJobInfo(jobId, &jobResult)
	require.NoError(t, err)

	state := composeStateFromJobStatus(jobInfo.JobStatus, &jobResult)
	require.Equal(t, "FAILED", state.ToString())
}

func TestComposeStatusFromJobError(t *testing.T) {

	if len(os.Getenv("OSBUILD_COMPOSER_TEST_EXTERNAL")) > 0 {
		t.Skip("This test is for internal testing only")
	}

	api, _ := createWeldrAPI(t.TempDir(), rpmmd_mock.BaseFixture)

	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}

	jobId, err := api.workers.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest}, "")
	require.NoError(t, err)

	j, token, _, _, _, err := api.workers.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""})
	require.NoError(t, err)
	require.Equal(t, jobId, j)

	jobResult := worker.OSBuildJobResult{}
	jobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, "Upload error", nil)
	rawResult, err := json.Marshal(jobResult)
	require.NoError(t, err)
	err = api.workers.FinishJob(token, rawResult)
	require.NoError(t, err)

	jobInfo, err := api.workers.OSBuildJobInfo(jobId, &jobResult)
	require.NoError(t, err)

	state := composeStateFromJobStatus(jobInfo.JobStatus, &jobResult)
	require.Equal(t, "FAILED", state.ToString())
}
