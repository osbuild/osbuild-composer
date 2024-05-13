package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/pkg/jobqueue"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/test_distro"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/api"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

func newTestServer(t *testing.T, tempdir string, config worker.Config, acceptArtifacts bool) *worker.Server {
	q, err := fsjobqueue.New(tempdir)
	if err != nil {
		t.Fatalf("error creating fsjobqueue: %v", err)
	}

	if acceptArtifacts {
		artifactsDir := path.Join(tempdir, "artifacts")
		err := os.Mkdir(artifactsDir, 0755)
		if err != nil && !os.IsExist(err) {
			t.Fatalf("cannot create state directory %s: %v", artifactsDir, err)
		}
		config.ArtifactsDir = artifactsDir
	}

	return worker.NewServer(nil, q, config)
}

var defaultConfig = worker.Config{
	RequestJobTimeout: time.Duration(0),
	BasePath:          "/api/worker/v1",
}

// Ensure that the status request returns OK.
func TestStatus(t *testing.T) {
	server := newTestServer(t, t.TempDir(), defaultConfig, false)
	handler := server.Handler()
	test.TestRoute(t, handler, false, "GET", "/api/worker/v1/status", ``, http.StatusOK, `{"status":"OK", "href": "/api/worker/v1/status", "kind":"Status"}`, "message", "id")
}

func TestErrors(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
	}{
		// Bogus path
		{"GET", "/api/worker/v1/foo", ``, http.StatusNotFound},
		// Create job with invalid body
		{"POST", "/api/worker/v1/jobs", ``, http.StatusBadRequest},
		// Wrong method
		// Returns 404, but should be 405; see https://github.com/labstack/echo/issues/1981
		{"GET", "/api/worker/v1/jobs", ``, http.StatusNotFound},
		// Update job with invalid ID
		{"PATCH", "/api/worker/v1/jobs/foo", `{"status":"FINISHED"}`, http.StatusBadRequest},
		// Update job that does not exist, with invalid body
		{"PATCH", "/api/worker/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ``, http.StatusBadRequest},
		// Update job that does not exist
		{"PATCH", "/api/worker/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", `{"status":"FINISHED"}`, http.StatusNotFound},
	}

	tempdir := t.TempDir()

	for _, c := range cases {
		server := newTestServer(t, tempdir, defaultConfig, false)
		handler := server.Handler()
		test.TestRoute(t, handler, false, c.Method, c.Path, c.Body, c.ExpectedStatus, `{"kind":"Error"}`, "message", "href", "operation_id", "reason", "id", "code")
	}
}

func TestErrorsAlteredBasePath(t *testing.T) {
	var cases = []struct {
		Method         string
		Path           string
		Body           string
		ExpectedStatus int
	}{
		// Bogus path
		{"GET", "/api/image-builder-worker/v1/foo", ``, http.StatusNotFound},
		// Create job with invalid body
		{"POST", "/api/image-builder-worker/v1/jobs", ``, http.StatusBadRequest},
		// Wrong method
		// Returns 404, but should be 405; see https://github.com/labstack/echo/issues/1981
		{"GET", "/api/image-builder-worker/v1/jobs", ``, http.StatusNotFound},
		// Update job with invalid ID
		{"PATCH", "/api/image-builder-worker/v1/jobs/foo", `{"status":"FINISHED"}`, http.StatusBadRequest},
		// Update job that does not exist, with invalid body
		{"PATCH", "/api/image-builder-worker/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ``, http.StatusBadRequest},
		// Update job that does not exist
		{"PATCH", "/api/image-builder-worker/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", `{"status":"FINISHED"}`, http.StatusNotFound},
	}

	tempdir := t.TempDir()
	config := worker.Config{
		RequestJobTimeout: time.Duration(0),
		BasePath:          "/api/image-builder-worker/v1",
	}

	for _, c := range cases {
		server := newTestServer(t, tempdir, config, false)
		handler := server.Handler()
		test.TestRoute(t, handler, false, c.Method, c.Path, c.Body, c.ExpectedStatus, `{"kind":"Error"}`, "message", "href", "operation_id", "reason", "id", "code")
	}
}

func newTestDistro(t *testing.T) distro.Distro {
	distroStruct := test_distro.DistroFactory(test_distro.TestDistro1Name)
	require.NotNil(t, distroStruct)
	return distroStruct
}

func TestCreate(t *testing.T) {
	distroStruct := newTestDistro(t)
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
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	server := newTestServer(t, t.TempDir(), defaultConfig, false)
	handler := server.Handler()

	_, err = server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: mf}, "")
	require.NoError(t, err)

	emptyManifest := `{"version":"2","pipelines":[{"name":"build"},{"name":"os"}],"sources":{}}`
	test.TestRoute(t, handler, false, "POST", "/api/worker/v1/jobs",
		fmt.Sprintf(`{"types":["%s"],"arch":"%s"}`, worker.JobTypeOSBuild, test_distro.TestArchName), http.StatusCreated,
		fmt.Sprintf(`{"kind":"RequestJob","href":"/api/worker/v1/jobs","type":"%s","args":{"manifest":`+emptyManifest+`}}`, worker.JobTypeOSBuild), "id", "location", "artifact_location")
}

func TestCancel(t *testing.T) {
	distroStruct := newTestDistro(t)
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
	server := newTestServer(t, t.TempDir(), defaultConfig, false)
	mf, err := manifest.Serialize(nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	handler := server.Handler()

	jobId, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: mf}, "")
	require.NoError(t, err)

	j, token, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, jobId, j)
	require.Equal(t, worker.JobTypeOSBuild, typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusOK,
		fmt.Sprintf(`{"canceled":false,"href":"/api/worker/v1/jobs/%s","id":"%s","kind":"JobStatus"}`, token, token))

	err = server.Cancel(jobId)
	require.NoError(t, err)

	test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusOK,
		fmt.Sprintf(`{"canceled":true,"href":"/api/worker/v1/jobs/%s","id":"%s","kind":"JobStatus"}`, token, token))
}

func TestUpdate(t *testing.T) {
	distroStruct := newTestDistro(t)
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
	server := newTestServer(t, t.TempDir(), defaultConfig, false)
	mf, err := manifest.Serialize(nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	handler := server.Handler()

	jobId, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: mf}, "")
	require.NoError(t, err)

	j, token, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, jobId, j)
	require.Equal(t, worker.JobTypeOSBuild, typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusOK,
		fmt.Sprintf(`{"href":"/api/worker/v1/jobs/%s","id":"%s","kind":"UpdateJobResponse"}`, token, token))
	test.TestRoute(t, handler, false, "PATCH", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusNotFound,
		`{"href":"/api/worker/v1/errors/5","code":"IMAGE-BUILDER-WORKER-5","id":"5","kind":"Error","message":"Token not found","reason":"Token not found"}`,
		"operation_id")
}

func TestArgs(t *testing.T) {
	distroStruct := newTestDistro(t)
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	require.NoError(t, err)
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	require.NoError(t, err)
	manifest, _, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, 0)
	require.NoError(t, err)

	mf, err := manifest.Serialize(nil, nil, nil, nil)
	require.NoError(t, err)

	job := worker.OSBuildJob{
		Manifest: mf,
		PipelineNames: &worker.PipelineNames{
			Build:   []string{"b"},
			Payload: []string{"x", "y", "z"},
		},
		Targets: []*target.Target{
			{
				Name:      target.TargetNameWorkerServer,
				ImageName: "test-image",
				OsbuildArtifact: target.OsbuildArtifact{
					ExportFilename: "test-image",
					ExportName:     "assembler",
				},
				Options: &target.WorkerServerTargetOptions{},
			},
		},
	}

	server := newTestServer(t, t.TempDir(), defaultConfig, false)
	jobId, err := server.EnqueueOSBuild(arch.Name(), &job, "")
	require.NoError(t, err)

	_, _, _, args, _, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
	require.NoError(t, err)
	require.NotNil(t, args)

	var jobArgs worker.OSBuildJob
	err = server.OSBuildJob(jobId, &jobArgs)
	require.NoError(t, err)
	require.Equal(t, job, jobArgs)
}

func TestUpload(t *testing.T) {
	distroStruct := newTestDistro(t)
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
	server := newTestServer(t, t.TempDir(), defaultConfig, true)
	mf, err := manifest.Serialize(nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	handler := server.Handler()

	jobID, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: mf}, "")
	require.NoError(t, err)

	j, token, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, jobID, j)
	require.Equal(t, worker.JobTypeOSBuild, typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "PUT", fmt.Sprintf("/api/worker/v1/jobs/%s/artifacts/foobar", token), `this is my artifact`, http.StatusOK, `?`)
}

func TestUploadNotAcceptingArtifacts(t *testing.T) {
	distroStruct := newTestDistro(t)
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
	server := newTestServer(t, t.TempDir(), defaultConfig, false)
	handler := server.Handler()
	mf, _ := manifest.Serialize(nil, nil, nil, nil)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}

	jobID, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: mf}, "")
	require.NoError(t, err)

	j, token, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, jobID, j)
	require.Equal(t, worker.JobTypeOSBuild, typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "PUT", fmt.Sprintf("/api/worker/v1/jobs/%s/artifacts/foobar", token), `this is my artifact`, http.StatusBadRequest, `?`)
}

func TestUploadAlteredBasePath(t *testing.T) {
	distroStruct := newTestDistro(t)
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
		t.Fatalf("error creating osbuild manifest: %v", err)
	}

	config := worker.Config{
		RequestJobTimeout: time.Duration(0),
		BasePath:          "/api/image-builder-worker/v1",
	}

	server := newTestServer(t, t.TempDir(), config, true)
	handler := server.Handler()

	jobID, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: mf}, "")
	require.NoError(t, err)

	j, token, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, jobID, j)
	require.Equal(t, worker.JobTypeOSBuild, typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "PUT", fmt.Sprintf("/api/image-builder-worker/v1/jobs/%s/artifacts/foobar", token), `this is my artifact`, http.StatusOK, `?`)
}

func TestTimeout(t *testing.T) {
	distroStruct := newTestDistro(t)
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	config := worker.Config{
		RequestJobTimeout: time.Millisecond * 10,
		BasePath:          "/api/image-builder-worker/v1",
	}
	server := newTestServer(t, t.TempDir(), config, false)

	_, _, _, _, _, err = server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
	require.Equal(t, jobqueue.ErrDequeueTimeout, err)

	test.TestRoute(t, server.Handler(), false, "POST", "/api/image-builder-worker/v1/jobs", `{"arch":"arch","types":["types"]}`, http.StatusNoContent,
		`{"href":"/api/image-builder-worker/v1/jobs","id":"00000000-0000-0000-0000-000000000000","kind":"RequestJob"}`)
}

func TestRequestJobById(t *testing.T) {
	distroStruct := newTestDistro(t)
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	server := newTestServer(t, t.TempDir(), defaultConfig, false)
	handler := server.Handler()

	depsolveJobId, err := server.EnqueueDepsolve(&worker.DepsolveJob{}, "")
	require.NoError(t, err)

	jobId, err := server.EnqueueManifestJobByID(&worker.ManifestJobByID{}, []uuid.UUID{depsolveJobId}, "")
	require.NoError(t, err)

	test.TestRoute(t, server.Handler(), false, "POST", "/api/worker/v1/jobs", fmt.Sprintf(`{"arch":"arch","types":["%s"]}`, worker.JobTypeManifestIDOnly), http.StatusBadRequest,
		`{"href":"/api/worker/v1/errors/15","kind":"Error","id": "15","code":"IMAGE-BUILDER-WORKER-15"}`, "operation_id", "reason", "message")

	_, _, _, _, _, err = server.RequestJobById(context.Background(), arch.Name(), jobId)
	require.Error(t, jobqueue.ErrNotPending, err)

	_, token, _, _, _, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeDepsolve}, []string{""}, uuid.Nil)
	require.NoError(t, err)

	depsolveJR, err := json.Marshal(worker.DepsolveJobResult{})
	require.NoError(t, err)
	err = server.FinishJob(token, depsolveJR)
	require.NoError(t, err)

	j, token, typ, args, dynamicArgs, err := server.RequestJobById(context.Background(), arch.Name(), jobId)
	require.NoError(t, err)
	require.Equal(t, jobId, j)
	require.Equal(t, worker.JobTypeManifestIDOnly, typ)
	require.NotNil(t, args)
	require.NotNil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "GET", fmt.Sprintf("/api/worker/v1/jobs/%s", token), `{}`, http.StatusOK,
		fmt.Sprintf(`{"canceled":false,"href":"/api/worker/v1/jobs/%s","id":"%s","kind":"JobStatus"}`, token, token))
}

// Enqueue OSBuild jobs with and without additional data and read them off the queue to
// check if the fallbacks are added for the old job and the new data are kept
// for the new job.
func TestMixedOSBuildJob(t *testing.T) {
	require := require.New(t)

	emptyManifestV2 := manifest.OSBuildManifest(`{"version":"2","pipelines":{}}`)
	config := worker.Config{
		RequestJobTimeout: time.Millisecond * 10,
		BasePath:          "/",
	}
	server := newTestServer(t, t.TempDir(), config, false)
	fbPipelines := &worker.PipelineNames{Build: distro.BuildPipelinesFallback(), Payload: distro.PayloadPipelinesFallback()}

	oldJob := worker.OSBuildJob{
		Manifest: emptyManifestV2,
		Targets: []*target.Target{
			{
				Name:      target.TargetNameWorkerServer,
				ImageName: "no-pipeline-names",
				OsbuildArtifact: target.OsbuildArtifact{
					ExportFilename: "no-pipeline-names",
					ExportName:     "assembler",
				},
				Options: &target.WorkerServerTargetOptions{},
			},
		},
	}
	oldJobID, err := server.EnqueueOSBuild("x", &oldJob, "")
	require.NoError(err)

	newJob := worker.OSBuildJob{
		Manifest: emptyManifestV2,
		PipelineNames: &worker.PipelineNames{
			Build:   []string{"build"},
			Payload: []string{"other", "pipelines"},
		},
		Targets: []*target.Target{
			{
				Name:      target.TargetNameWorkerServer,
				ImageName: "with-pipeline-names",
				OsbuildArtifact: target.OsbuildArtifact{
					ExportFilename: "with-pipeline-names",
					ExportName:     "assembler",
				},
				Options: &target.WorkerServerTargetOptions{},
			},
		},
	}
	newJobID, err := server.EnqueueOSBuild("x", &newJob, "")
	require.NoError(err)

	var oldJobRead worker.OSBuildJob
	err = server.OSBuildJob(oldJobID, &oldJobRead)
	require.NoError(err)
	require.NotNil(oldJobRead.PipelineNames)
	// OldJob gets default pipeline names when read
	require.Equal(fbPipelines, oldJobRead.PipelineNames)
	require.Equal(oldJob.Manifest, oldJobRead.Manifest)
	require.Equal(oldJob.Targets, oldJobRead.Targets)
	// Not entirely equal
	require.NotEqual(oldJob, oldJobRead)

	// NewJob the same when read back
	var newJobRead worker.OSBuildJob
	err = server.OSBuildJob(newJobID, &newJobRead)
	require.NoError(err)
	require.NotNil(newJobRead.PipelineNames)
	require.Equal(newJob.PipelineNames, newJobRead.PipelineNames)

	// Dequeue the jobs (via RequestJob) to get their tokens and update them to
	// test the result retrieval

	getJob := func() (uuid.UUID, uuid.UUID) {
		// don't block forever if the jobs weren't added or can't be retrieved
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		id, token, _, _, _, err := server.RequestJob(ctx, "x", []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
		require.NoError(err)
		return id, token
	}

	getJobTokens := func(n uint) map[uuid.UUID]uuid.UUID {
		tokens := make(map[uuid.UUID]uuid.UUID, n)
		for idx := uint(0); idx < n; idx++ {
			id, token := getJob()
			tokens[id] = token
		}
		return tokens
	}

	jobTokens := getJobTokens(2)
	// make sure we got them both as expected
	require.Contains(jobTokens, oldJobID)
	require.Contains(jobTokens, newJobID)

	oldJobResult := &worker.OSBuildJobResult{
		Success: true,
		OSBuildOutput: &osbuild.Result{
			Type:    "result",
			Success: true,
			Log: map[string]osbuild.PipelineResult{
				"build-old": {
					osbuild.StageResult{
						ID:      "---",
						Type:    "org.osbuild.test",
						Output:  "<test output>",
						Success: true,
					},
				},
			},
		},
	}
	oldJobResultRaw, err := json.Marshal(oldJobResult)
	require.NoError(err)
	oldJobToken := jobTokens[oldJobID]
	err = server.FinishJob(oldJobToken, oldJobResultRaw)
	require.NoError(err)

	oldJobResultRead := new(worker.OSBuildJobResult)
	_, err = server.OSBuildJobInfo(oldJobID, oldJobResultRead)
	require.NoError(err)

	// oldJobResultRead should have PipelineNames now
	require.NotEqual(oldJobResult, oldJobResultRead)
	require.Equal(fbPipelines, oldJobResultRead.PipelineNames)
	require.NotNil(oldJobResultRead.PipelineNames)
	require.Equal(oldJobResult.OSBuildOutput, oldJobResultRead.OSBuildOutput)
	require.Equal(oldJobResult.Success, oldJobResultRead.Success)

	newJobResult := &worker.OSBuildJobResult{
		Success: true,
		PipelineNames: &worker.PipelineNames{
			Build:   []string{"build-result"},
			Payload: []string{"result-test-payload", "result-test-assembler"},
		},
		OSBuildOutput: &osbuild.Result{
			Type:    "result",
			Success: true,
			Log: map[string]osbuild.PipelineResult{
				"build-new": {
					osbuild.StageResult{
						ID:      "---",
						Type:    "org.osbuild.test",
						Output:  "<test output new>",
						Success: true,
					},
				},
			},
		},
	}
	newJobResultRaw, err := json.Marshal(newJobResult)
	require.NoError(err)
	newJobToken := jobTokens[newJobID]
	err = server.FinishJob(newJobToken, newJobResultRaw)
	require.NoError(err)

	newJobResultRead := new(worker.OSBuildJobResult)
	_, err = server.OSBuildJobInfo(newJobID, newJobResultRead)
	require.NoError(err)
	require.Equal(newJobResult, newJobResultRead)
}

func TestDepsolveLegacyErrorConversion(t *testing.T) {
	distroStruct := newTestDistro(t)
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	server := newTestServer(t, t.TempDir(), defaultConfig, false)

	depsolveJobId, err := server.EnqueueDepsolve(&worker.DepsolveJob{}, "")
	require.NoError(t, err)

	_, _, _, _, _, err = server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeDepsolve}, []string{""}, uuid.Nil)
	require.NoError(t, err)

	reason := "Depsolve failed"
	errType := worker.DepsolveErrorType

	expectedResult := worker.DepsolveJobResult{
		Error:     reason,
		ErrorType: errType,
		JobResult: worker.JobResult{
			JobError: clienterrors.New(clienterrors.ErrorDNFDepsolveError, reason, nil),
		},
	}

	depsolveJobResult := worker.DepsolveJobResult{
		Error:     reason,
		ErrorType: errType,
	}

	_, err = server.DepsolveJobInfo(depsolveJobId, &depsolveJobResult)
	require.NoError(t, err)
	require.Equal(t, expectedResult, depsolveJobResult)
}

type testJob struct {
	main   interface{}
	deps   []testJob
	result interface{}
}

func enqueueAndFinishTestJobDependencies(s *worker.Server, deps []testJob) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}

	for _, dep := range deps {
		var depUUIDs []uuid.UUID
		var err error
		if len(dep.deps) > 0 {
			depUUIDs, err = enqueueAndFinishTestJobDependencies(s, dep.deps)
			if err != nil {
				return nil, err
			}
		}

		var id uuid.UUID
		switch dep.main.(type) {
		case *worker.OSBuildJob:
			job := dep.main.(*worker.OSBuildJob)
			id, err = s.EnqueueOSBuildAsDependency(arch.ARCH_X86_64.String(), job, depUUIDs, "")
			if err != nil {
				return nil, err
			}

		case *worker.ManifestJobByID:
			job := dep.main.(*worker.ManifestJobByID)
			if len(depUUIDs) < 1 {
				return nil, fmt.Errorf("at least one dependency is expected for ManifestJobByID, got: %d", len(depUUIDs))
			}
			id, err = s.EnqueueManifestJobByID(job, depUUIDs, "")
			if err != nil {
				return nil, err
			}

		case *worker.DepsolveJob:
			job := dep.main.(*worker.DepsolveJob)
			if len(depUUIDs) != 0 {
				return nil, fmt.Errorf("dependencies are not supported for DepsolveJob, got: %d", len(depUUIDs))
			}
			id, err = s.EnqueueDepsolve(job, "")
			if err != nil {
				return nil, err
			}

		case *worker.KojiInitJob:
			job := dep.main.(*worker.KojiInitJob)
			if len(depUUIDs) != 0 {
				return nil, fmt.Errorf("dependencies are not supported for KojiInitJob, got: %d", len(depUUIDs))
			}
			id, err = s.EnqueueKojiInit(job, "")
			if err != nil {
				return nil, err
			}

		case *worker.KojiFinalizeJob:
			job := dep.main.(*worker.KojiFinalizeJob)
			if len(depUUIDs) < 2 {
				return nil, fmt.Errorf("at least two dependencies are expected for KojiFinalizeJob, got: %d", len(depUUIDs))
			}
			id, err = s.EnqueueKojiFinalize(job, depUUIDs[0], depUUIDs[1:], "")
			if err != nil {
				return nil, err
			}

		case *worker.ContainerResolveJob:
			job := dep.main.(*worker.ContainerResolveJob)
			if len(depUUIDs) != 0 {
				return nil, fmt.Errorf("dependencies are not supported for ContainerResolveJob, got: %d", len(depUUIDs))
			}
			id, err = s.EnqueueContainerResolveJob(job, "")
			if err != nil {
				return nil, err
			}

		case *worker.OSTreeResolveJob:
			job := dep.main.(*worker.OSTreeResolveJob)
			if len(depUUIDs) != 0 {
				return nil, fmt.Errorf("dependencies are not supported for OSTreeResolveJob, got: %d", len(depUUIDs))
			}
			id, err = s.EnqueueOSTreeResolveJob(job, "")
			if err != nil {
				return nil, err
			}

		default:
			return nil, fmt.Errorf("unexpected job type")
		}

		// request the previously added Job
		_, token, _, _, _, err := s.RequestJobById(context.Background(), arch.ARCH_X86_64.String(), id)
		if err != nil {
			return nil, err
		}
		result, err := json.Marshal(dep.result)
		if err != nil {
			return nil, err
		}
		// mark the job as finished using the defined job result
		err = s.FinishJob(token, result)
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func TestJobDependencyChainErrors(t *testing.T) {
	var cases = []struct {
		job           testJob
		expectedError *clienterrors.Error
	}{
		// osbuild + manifest + depsolve
		// failed depsolve
		{
			job: testJob{
				main: &worker.OSBuildJob{},
				deps: []testJob{
					{
						main: &worker.ManifestJobByID{},
						deps: []testJob{
							{
								main: &worker.DepsolveJob{},
								result: &worker.DepsolveJobResult{
									JobResult: worker.JobResult{
										JobError: &clienterrors.Error{
											ID:     clienterrors.ErrorDNFDepsolveError,
											Reason: "package X not found",
										},
									},
								},
							},
						},
						result: &worker.ManifestJobByIDResult{
							JobResult: worker.JobResult{
								JobError: &clienterrors.Error{
									ID:     clienterrors.ErrorDepsolveDependency,
									Reason: "depsolve dependency job failed",
								},
							},
						},
					},
				},
				result: &worker.OSBuildJobResult{
					JobResult: worker.JobResult{
						JobError: &clienterrors.Error{
							ID:     clienterrors.ErrorManifestDependency,
							Reason: "manifest dependency job failed",
						},
					},
				},
			},
			expectedError: &clienterrors.Error{
				ID:     clienterrors.ErrorManifestDependency,
				Reason: "manifest dependency job failed",
				Details: []*clienterrors.Error{
					{
						ID:     clienterrors.ErrorDepsolveDependency,
						Reason: "depsolve dependency job failed",
						Details: []*clienterrors.Error{
							{
								ID:     clienterrors.ErrorDNFDepsolveError,
								Reason: "package X not found",
							},
						},
					},
				},
			},
		},
		// osbuild + manifest + depsolve
		// failed manifest
		{
			job: testJob{
				main: &worker.OSBuildJob{},
				deps: []testJob{
					{
						main: &worker.ManifestJobByID{},
						deps: []testJob{
							{
								main:   &worker.DepsolveJob{},
								result: &worker.DepsolveJobResult{},
							},
						},
						result: &worker.ManifestJobByIDResult{
							JobResult: worker.JobResult{
								JobError: &clienterrors.Error{
									ID:     clienterrors.ErrorManifestGeneration,
									Reason: "failed to generate manifest",
								},
							},
						},
					},
				},
				result: &worker.OSBuildJobResult{
					JobResult: worker.JobResult{
						JobError: &clienterrors.Error{
							ID:     clienterrors.ErrorManifestDependency,
							Reason: "manifest dependency job failed",
						},
					},
				},
			},
			expectedError: &clienterrors.Error{
				ID:     clienterrors.ErrorManifestDependency,
				Reason: "manifest dependency job failed",
				Details: []*clienterrors.Error{
					{
						ID:     clienterrors.ErrorManifestGeneration,
						Reason: "failed to generate manifest",
					},
				},
			},
		},
		// osbuild + manifest + depsolve
		// failed osbuild
		{
			job: testJob{
				main: &worker.OSBuildJob{},
				deps: []testJob{
					{
						main: &worker.ManifestJobByID{},
						deps: []testJob{
							{
								main:   &worker.DepsolveJob{},
								result: &worker.DepsolveJobResult{},
							},
						},
						result: &worker.ManifestJobByIDResult{
							JobResult: worker.JobResult{},
						},
					},
				},
				result: &worker.OSBuildJobResult{
					JobResult: worker.JobResult{
						JobError: &clienterrors.Error{
							ID:     clienterrors.ErrorEmptyManifest,
							Reason: "empty manifest received",
						},
					},
				},
			},
			expectedError: &clienterrors.Error{
				ID:     clienterrors.ErrorEmptyManifest,
				Reason: "empty manifest received",
			},
		},
		// osbuild + manifest + depsolve + container resolve
		// failed container resolve
		{
			job: testJob{
				main: &worker.OSBuildJob{},
				deps: []testJob{
					{
						main:   &worker.KojiInitJob{},
						result: &worker.KojiInitJobResult{},
					},
					{
						main: &worker.ManifestJobByID{},
						deps: []testJob{
							{
								main: &worker.ContainerResolveJob{},
								result: &worker.ContainerResolveJobResult{
									JobResult: worker.JobResult{
										JobError: &clienterrors.Error{
											ID:     clienterrors.ErrorContainerResolution,
											Reason: "remote container not found",
										},
									},
								},
							},
							{
								main:   &worker.DepsolveJob{},
								result: &worker.DepsolveJobResult{},
							},
						},
						result: &worker.ManifestJobByIDResult{
							JobResult: worker.JobResult{
								JobError: &clienterrors.Error{
									ID:     clienterrors.ErrorContainerDependency,
									Reason: "container dependency job failed",
								},
							},
						},
					},
				},
				result: &worker.OSBuildJobResult{
					JobResult: worker.JobResult{
						JobError: &clienterrors.Error{
							ID:     clienterrors.ErrorManifestDependency,
							Reason: "manifest dependency job failed",
						},
					},
				},
			},
			expectedError: &clienterrors.Error{
				ID:     clienterrors.ErrorManifestDependency,
				Reason: "manifest dependency job failed",
				Details: []*clienterrors.Error{
					{
						ID:     clienterrors.ErrorContainerDependency,
						Reason: "container dependency job failed",
						Details: []*clienterrors.Error{
							{
								ID:     clienterrors.ErrorContainerResolution,
								Reason: "remote container not found",
							},
						},
					},
				},
			},
		},
		// koji-init + osbuild + manifest + depsolve
		// failed depsolve
		{
			job: testJob{
				main: &worker.OSBuildJob{},
				deps: []testJob{
					{
						main:   &worker.KojiInitJob{},
						result: &worker.KojiInitJobResult{},
					},
					{
						main: &worker.ManifestJobByID{},
						deps: []testJob{
							{
								main: &worker.DepsolveJob{},
								result: &worker.DepsolveJobResult{
									JobResult: worker.JobResult{
										JobError: &clienterrors.Error{
											ID:     clienterrors.ErrorDNFDepsolveError,
											Reason: "package X not found",
										},
									},
								},
							},
						},
						result: &worker.ManifestJobByIDResult{
							JobResult: worker.JobResult{
								JobError: &clienterrors.Error{
									ID:     clienterrors.ErrorDepsolveDependency,
									Reason: "depsolve dependency job failed",
								},
							},
						},
					},
				},
				result: &worker.OSBuildJobResult{
					JobResult: worker.JobResult{
						JobError: &clienterrors.Error{
							ID:     clienterrors.ErrorManifestDependency,
							Reason: "manifest dependency job failed",
						},
					},
				},
			},
			expectedError: &clienterrors.Error{
				ID:     clienterrors.ErrorManifestDependency,
				Reason: "manifest dependency job failed",
				Details: []*clienterrors.Error{
					{
						ID:     clienterrors.ErrorDepsolveDependency,
						Reason: "depsolve dependency job failed",
						Details: []*clienterrors.Error{
							{
								ID:     clienterrors.ErrorDNFDepsolveError,
								Reason: "package X not found",
							},
						},
					},
				},
			},
		},
		// koji-init + (osbuild + manifest + depsolve) + (osbuild + manifest + depsolve) + koji-finalize
		// failed one depsolve
		{
			job: testJob{
				main: &worker.KojiFinalizeJob{},
				deps: []testJob{
					{
						main:   &worker.KojiInitJob{},
						result: &worker.KojiInitJobResult{},
					},
					// failed build
					{
						main: &worker.OSBuildJob{},
						deps: []testJob{
							{
								main:   &worker.KojiInitJob{},
								result: &worker.KojiInitJobResult{},
							},
							{
								main: &worker.ManifestJobByID{},
								deps: []testJob{
									{
										main: &worker.DepsolveJob{},
										result: &worker.DepsolveJobResult{
											JobResult: worker.JobResult{
												JobError: &clienterrors.Error{
													ID:     clienterrors.ErrorDNFDepsolveError,
													Reason: "package X not found",
												},
											},
										},
									},
								},
								result: &worker.ManifestJobByIDResult{
									JobResult: worker.JobResult{
										JobError: &clienterrors.Error{
											ID:     clienterrors.ErrorDepsolveDependency,
											Reason: "depsolve dependency job failed",
										},
									},
								},
							},
						},
						result: &worker.OSBuildJobResult{
							JobResult: worker.JobResult{
								JobError: &clienterrors.Error{
									ID:     clienterrors.ErrorManifestDependency,
									Reason: "manifest dependency job failed",
								},
							},
						},
					},
					// passed build
					{
						main: &worker.OSBuildJob{},
						deps: []testJob{
							{
								main:   &worker.KojiInitJob{},
								result: &worker.KojiInitJobResult{},
							},
							{
								main: &worker.ManifestJobByID{},
								deps: []testJob{
									{
										main:   &worker.DepsolveJob{},
										result: &worker.DepsolveJobResult{},
									},
								},
								result: &worker.ManifestJobByIDResult{},
							},
						},
						result: &worker.OSBuildJobResult{
							OSBuildOutput: &osbuild.Result{},
						},
					},
				},
				result: &worker.KojiFinalizeJobResult{
					JobResult: worker.JobResult{
						JobError: &clienterrors.Error{
							ID:     clienterrors.ErrorKojiFailedDependency,
							Reason: "one build failed",
						},
					},
				},
			},
			expectedError: &clienterrors.Error{
				ID:     clienterrors.ErrorKojiFailedDependency,
				Reason: "one build failed",
				Details: []*clienterrors.Error{
					{
						ID:     clienterrors.ErrorManifestDependency,
						Reason: "manifest dependency job failed",
						Details: []*clienterrors.Error{
							{
								ID:     clienterrors.ErrorDepsolveDependency,
								Reason: "depsolve dependency job failed",
								Details: []*clienterrors.Error{
									{
										ID:     clienterrors.ErrorDNFDepsolveError,
										Reason: "package X not found",
									},
								},
							},
						},
					},
				},
			},
		},
		// koji-init + (osbuild + manifest + depsolve) + (osbuild + manifest + depsolve) + koji-finalize
		// failed both depsolve
		{
			job: testJob{
				main: &worker.KojiFinalizeJob{},
				deps: []testJob{
					{
						main:   &worker.KojiInitJob{},
						result: &worker.KojiInitJobResult{},
					},
					// failed build
					{
						main: &worker.OSBuildJob{},
						deps: []testJob{
							{
								main:   &worker.KojiInitJob{},
								result: &worker.KojiInitJobResult{},
							},
							{
								main: &worker.ManifestJobByID{},
								deps: []testJob{
									{
										main: &worker.DepsolveJob{},
										result: &worker.DepsolveJobResult{
											JobResult: worker.JobResult{
												JobError: &clienterrors.Error{
													ID:     clienterrors.ErrorDNFDepsolveError,
													Reason: "package X not found",
												},
											},
										},
									},
								},
								result: &worker.ManifestJobByIDResult{
									JobResult: worker.JobResult{
										JobError: &clienterrors.Error{
											ID:     clienterrors.ErrorDepsolveDependency,
											Reason: "depsolve dependency job failed",
										},
									},
								},
							},
						},
						result: &worker.OSBuildJobResult{
							JobResult: worker.JobResult{
								JobError: &clienterrors.Error{
									ID:     clienterrors.ErrorManifestDependency,
									Reason: "manifest dependency job failed",
								},
							},
						},
					},
					// failed build
					{
						main: &worker.OSBuildJob{},
						deps: []testJob{
							{
								main:   &worker.KojiInitJob{},
								result: &worker.KojiInitJobResult{},
							},
							{
								main: &worker.ManifestJobByID{},
								deps: []testJob{
									{
										main: &worker.DepsolveJob{},
										result: &worker.DepsolveJobResult{
											JobResult: worker.JobResult{
												JobError: &clienterrors.Error{
													ID:     clienterrors.ErrorDNFDepsolveError,
													Reason: "package Y not found",
												},
											},
										},
									},
								},
								result: &worker.ManifestJobByIDResult{
									JobResult: worker.JobResult{
										JobError: &clienterrors.Error{
											ID:     clienterrors.ErrorDepsolveDependency,
											Reason: "depsolve dependency job failed",
										},
									},
								},
							},
						},
						result: &worker.OSBuildJobResult{
							JobResult: worker.JobResult{
								JobError: &clienterrors.Error{
									ID:     clienterrors.ErrorManifestDependency,
									Reason: "manifest dependency job failed",
								},
							},
						},
					},
				},
				result: &worker.KojiFinalizeJobResult{
					JobResult: worker.JobResult{
						JobError: &clienterrors.Error{
							ID:     clienterrors.ErrorKojiFailedDependency,
							Reason: "two builds failed",
						},
					},
				},
			},
			expectedError: &clienterrors.Error{
				ID:     clienterrors.ErrorKojiFailedDependency,
				Reason: "two builds failed",
				Details: []*clienterrors.Error{
					{
						ID:     clienterrors.ErrorManifestDependency,
						Reason: "manifest dependency job failed",
						Details: []*clienterrors.Error{
							{
								ID:     clienterrors.ErrorDepsolveDependency,
								Reason: "depsolve dependency job failed",
								Details: []*clienterrors.Error{
									{
										ID:     clienterrors.ErrorDNFDepsolveError,
										Reason: "package X not found",
									},
								},
							},
						},
					},
					{
						ID:     clienterrors.ErrorManifestDependency,
						Reason: "manifest dependency job failed",
						Details: []*clienterrors.Error{
							{
								ID:     clienterrors.ErrorDepsolveDependency,
								Reason: "depsolve dependency job failed",
								Details: []*clienterrors.Error{
									{
										ID:     clienterrors.ErrorDNFDepsolveError,
										Reason: "package Y not found",
									},
								},
							},
						},
					},
				},
			},
		},
		// koji-init + (osbuild + manifest + depsolve) + (osbuild + manifest + depsolve) + koji-finalize
		// failed koji-finalize
		{
			job: testJob{
				main: &worker.KojiFinalizeJob{},
				deps: []testJob{
					{
						main:   &worker.KojiInitJob{},
						result: &worker.KojiInitJobResult{},
					},
					// passed build
					{
						main: &worker.OSBuildJob{},
						deps: []testJob{
							{
								main:   &worker.KojiInitJob{},
								result: &worker.KojiInitJobResult{},
							},
							{
								main: &worker.ManifestJobByID{},
								deps: []testJob{
									{
										main:   &worker.DepsolveJob{},
										result: &worker.DepsolveJobResult{},
									},
								},
								result: &worker.ManifestJobByIDResult{},
							},
						},
						result: &worker.OSBuildJobResult{
							OSBuildOutput: &osbuild.Result{},
						},
					},
					// passed build
					{
						main: &worker.OSBuildJob{},
						deps: []testJob{
							{
								main:   &worker.KojiInitJob{},
								result: &worker.KojiInitJobResult{},
							},
							{
								main: &worker.ManifestJobByID{},
								deps: []testJob{
									{
										main:   &worker.DepsolveJob{},
										result: &worker.DepsolveJobResult{},
									},
								},
								result: &worker.ManifestJobByIDResult{},
							},
						},
						result: &worker.OSBuildJobResult{
							OSBuildOutput: &osbuild.Result{},
						},
					},
				},
				result: &worker.KojiFinalizeJobResult{
					JobResult: worker.JobResult{
						JobError: &clienterrors.Error{
							ID:     clienterrors.ErrorKojiFinalize,
							Reason: "koji-finalize failed",
						},
					},
				},
			},
			expectedError: &clienterrors.Error{
				ID:     clienterrors.ErrorKojiFinalize,
				Reason: "koji-finalize failed",
			},
		},
		// koji-init + (osbuild + manifest + depsolve + container resolve) + (osbuild + manifest + depsolve) + koji-finalize
		// all passed
		{
			job: testJob{
				main: &worker.KojiFinalizeJob{},
				deps: []testJob{
					{
						main:   &worker.KojiInitJob{},
						result: &worker.KojiInitJobResult{},
					},
					// passed build
					{
						main: &worker.OSBuildJob{},
						deps: []testJob{
							{
								main:   &worker.KojiInitJob{},
								result: &worker.KojiInitJobResult{},
							},
							{
								main: &worker.ManifestJobByID{},
								deps: []testJob{
									{
										main:   &worker.ContainerResolveJob{},
										result: &worker.ContainerResolveJobResult{},
									},
									{
										main:   &worker.DepsolveJob{},
										result: &worker.DepsolveJobResult{},
									},
								},
								result: &worker.ManifestJobByIDResult{},
							},
						},
						result: &worker.OSBuildJobResult{
							OSBuildOutput: &osbuild.Result{},
						},
					},
					// passed build
					{
						main: &worker.OSBuildJob{},
						deps: []testJob{
							{
								main:   &worker.KojiInitJob{},
								result: &worker.KojiInitJobResult{},
							},
							{
								main: &worker.ManifestJobByID{},
								deps: []testJob{
									{
										main:   &worker.DepsolveJob{},
										result: &worker.DepsolveJobResult{},
									},
								},
								result: &worker.ManifestJobByIDResult{},
							},
						},
						result: &worker.OSBuildJobResult{
							OSBuildOutput: &osbuild.Result{},
						},
					},
				},
				result: &worker.KojiFinalizeJobResult{
					JobResult: worker.JobResult{},
				},
			},
			expectedError: nil,
		},
		// osbuild + manifest + depsolve + ostree resolve
		// failed ostree resolve
		{
			job: testJob{
				main: &worker.OSBuildJob{},
				deps: []testJob{
					{
						main:   &worker.KojiInitJob{},
						result: &worker.KojiInitJobResult{},
					},
					{
						main: &worker.ManifestJobByID{},
						deps: []testJob{
							{
								main: &worker.OSTreeResolveJob{},
								result: &worker.OSTreeResolveJobResult{
									JobResult: worker.JobResult{
										JobError: &clienterrors.Error{
											ID:     clienterrors.ErrorOSTreeRefResolution,
											Reason: "remote ostree ref not found",
										},
									},
								},
							},
							{
								main:   &worker.DepsolveJob{},
								result: &worker.DepsolveJobResult{},
							},
						},
						result: &worker.ManifestJobByIDResult{
							JobResult: worker.JobResult{
								JobError: &clienterrors.Error{
									ID:     clienterrors.ErrorOSTreeDependency,
									Reason: "ostree dependency job failed",
								},
							},
						},
					},
				},
				result: &worker.OSBuildJobResult{
					JobResult: worker.JobResult{
						JobError: &clienterrors.Error{
							ID:     clienterrors.ErrorManifestDependency,
							Reason: "manifest dependency job failed",
						},
					},
				},
			},
			expectedError: &clienterrors.Error{
				ID:     clienterrors.ErrorManifestDependency,
				Reason: "manifest dependency job failed",
				Details: []*clienterrors.Error{
					{
						ID:     clienterrors.ErrorOSTreeDependency,
						Reason: "ostree dependency job failed",
						Details: []*clienterrors.Error{
							{
								ID:     clienterrors.ErrorOSTreeRefResolution,
								Reason: "remote ostree ref not found",
							},
						},
					},
				},
			},
		},
	}

	for idx, c := range cases {
		t.Logf("Test case #%d", idx)
		server := newTestServer(t, t.TempDir(), defaultConfig, false)
		ids, err := enqueueAndFinishTestJobDependencies(server, []testJob{c.job})
		require.Nil(t, err)
		require.Len(t, ids, 1)

		mainJobID := ids[0]
		errors, err := server.JobDependencyChainErrors(mainJobID)
		require.Nil(t, err)
		assert.EqualValues(t, c.expectedError, errors)
	}
}

func TestWorkerWatch(t *testing.T) {
	config := worker.Config{
		RequestJobTimeout: time.Duration(0),
		BasePath:          "/api/worker/v1",
		WorkerTimeout:     time.Millisecond * 200,
		WorkerWatchFreq:   time.Millisecond * 100,
	}

	server := newTestServer(t, t.TempDir(), config, false)

	reply := test.TestRouteWithReply(t, server.Handler(), false, "POST", "/api/worker/v1/workers", fmt.Sprintf(`{"arch":"%s"}`, arch.Current().String()), 201, `{"href":"/api/worker/v1/workers","kind":"WorkerID","id": "15"}`, "id", "worker_id")
	var resp api.PostWorkersResponse
	require.NoError(t, json.Unmarshal(reply, &resp))
	workerID, err := uuid.Parse(resp.WorkerId)
	require.NoError(t, err)

	test.TestRoute(t, server.Handler(), false, "POST", fmt.Sprintf("/api/worker/v1/workers/%s/status", workerID), "{}", 200, "")
	time.Sleep(time.Millisecond * 400)
	test.TestRoute(t, server.Handler(), false, "POST", fmt.Sprintf("/api/worker/v1/workers/%s/status", workerID), "", 400,
		`{"href":"/api/worker/v1/errors/18","code":"IMAGE-BUILDER-WORKER-18","id":"18","kind":"Error","message":"Given worker id doesn't exist","reason":"Given worker id doesn't exist"}`,
		"operation_id")
}

func TestRequestJobForWorker(t *testing.T) {
	server := newTestServer(t, t.TempDir(), defaultConfig, false)
	reply := test.TestRouteWithReply(t, server.Handler(), false, "POST", "/api/worker/v1/workers", fmt.Sprintf(`{"arch":"%s"}`, arch.Current().String()), 201, `{"href":"/api/worker/v1/workers","kind":"WorkerID","id": "15"}`, "id", "worker_id")
	var resp api.PostWorkersResponse
	require.NoError(t, json.Unmarshal(reply, &resp))
	workerID, err := uuid.Parse(resp.WorkerId)
	require.NoError(t, err)
	test.TestRoute(t, server.Handler(), false, "POST", fmt.Sprintf("/api/worker/v1/workers/%s/status", workerID), "{}", 200, "")

	distroStruct := newTestDistro(t)
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
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	jobId, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: mf}, "")
	require.NoError(t, err)

	// Can request a job with worker ID
	j, _, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""}, workerID)
	require.NoError(t, err)
	require.Equal(t, jobId, j)
	require.Equal(t, worker.JobTypeOSBuild, typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)
}
