package weldr

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/test_distro"
	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

func TestComposeStatusFromLegacyError(t *testing.T) {

	if len(os.Getenv("OSBUILD_COMPOSER_TEST_EXTERNAL")) > 0 {
		t.Skip("This test is for internal testing only")
	}

	api, sf := createTestWeldrAPI(t.TempDir(), test_distro.TestDistro1Name, test_distro.TestArchName, rpmmd_mock.BaseFixture, nil)
	t.Cleanup(sf.Cleanup)

	distroStruct := test_distro.DistroFactory(test_distro.TestDistro1Name)
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, _, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	mf, err := manifest.Serialize(nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("error serializing osbuild manifest: %v", err)
	}

	_, err = api.workers.RegisterWorker("", arch.Name())
	require.NoError(t, err)
	jobId, err := api.workers.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: mf}, "")
	require.NoError(t, err)

	j, token, _, _, _, err := api.workers.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
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

	api, sf := createTestWeldrAPI(t.TempDir(), test_distro.TestDistro1Name, test_distro.TestArchName, rpmmd_mock.BaseFixture, nil)
	t.Cleanup(sf.Cleanup)

	distroStruct := test_distro.DistroFactory(test_distro.TestDistro1Name)
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, _, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	mf, err := manifest.Serialize(nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("error serializing osbuild manifest: %v", err)
	}

	_, err = api.workers.RegisterWorker("", arch.Name())
	require.NoError(t, err)
	jobId, err := api.workers.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: mf}, "")
	require.NoError(t, err)

	j, token, _, _, _, err := api.workers.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, jobId, j)

	jobResult := worker.OSBuildJobResult{}
	jobResult.JobError = clienterrors.New(clienterrors.ErrorUploadingImage, "Upload error", nil)
	rawResult, err := json.Marshal(jobResult)
	require.NoError(t, err)
	err = api.workers.FinishJob(token, rawResult)
	require.NoError(t, err)

	jobInfo, err := api.workers.OSBuildJobInfo(jobId, &jobResult)
	require.NoError(t, err)

	state := composeStateFromJobStatus(jobInfo.JobStatus, &jobResult)
	require.Equal(t, "FAILED", state.ToString())
}
