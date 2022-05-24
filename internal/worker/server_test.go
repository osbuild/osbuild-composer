package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

func newTestServer(t *testing.T, tempdir string, jobRequestTimeout time.Duration, basePath string) *worker.Server {
	q, err := fsjobqueue.New(tempdir)
	if err != nil {
		t.Fatalf("error creating fsjobqueue: %v", err)
	}

	config := worker.Config{
		RequestJobTimeout: jobRequestTimeout,
		BasePath:          basePath,
	}
	return worker.NewServer(nil, q, config)
}

// Ensure that the status request returns OK.
func TestStatus(t *testing.T) {
	server := newTestServer(t, t.TempDir(), time.Duration(0), "/api/worker/v1")
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
		{"GET", "/api/worker/v1/jobs", ``, http.StatusMethodNotAllowed},
		// Update job with invalid ID
		{"PATCH", "/api/worker/v1/jobs/foo", `{"status":"FINISHED"}`, http.StatusBadRequest},
		// Update job that does not exist, with invalid body
		{"PATCH", "/api/worker/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ``, http.StatusBadRequest},
		// Update job that does not exist
		{"PATCH", "/api/worker/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", `{"status":"FINISHED"}`, http.StatusNotFound},
	}

	tempdir := t.TempDir()

	for _, c := range cases {
		server := newTestServer(t, tempdir, time.Duration(0), "/api/worker/v1")
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
		{"GET", "/api/image-builder-worker/v1/jobs", ``, http.StatusMethodNotAllowed},
		// Update job with invalid ID
		{"PATCH", "/api/image-builder-worker/v1/jobs/foo", `{"status":"FINISHED"}`, http.StatusBadRequest},
		// Update job that does not exist, with invalid body
		{"PATCH", "/api/image-builder-worker/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ``, http.StatusBadRequest},
		// Update job that does not exist
		{"PATCH", "/api/image-builder-worker/v1/jobs/aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", `{"status":"FINISHED"}`, http.StatusNotFound},
	}

	tempdir := t.TempDir()

	for _, c := range cases {
		server := newTestServer(t, tempdir, time.Duration(0), "/api/image-builder-worker/v1")
		handler := server.Handler()
		test.TestRoute(t, handler, false, c.Method, c.Path, c.Body, c.ExpectedStatus, `{"kind":"Error"}`, "message", "href", "operation_id", "reason", "id", "code")
	}
}

func TestCreate(t *testing.T) {
	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	server := newTestServer(t, t.TempDir(), time.Duration(0), "/api/worker/v1")
	handler := server.Handler()

	_, err = server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest}, "")
	require.NoError(t, err)

	test.TestRoute(t, handler, false, "POST", "/api/worker/v1/jobs",
		fmt.Sprintf(`{"types":["%s"],"arch":"%s"}`, worker.JobTypeOSBuild, test_distro.TestArchName), http.StatusCreated,
		fmt.Sprintf(`{"kind":"RequestJob","href":"/api/worker/v1/jobs","type":"%s","args":{"manifest":{"pipeline":{},"sources":{}}}}`, worker.JobTypeOSBuild), "id", "location", "artifact_location")
}

func TestCancel(t *testing.T) {
	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	server := newTestServer(t, t.TempDir(), time.Duration(0), "/api/worker/v1")
	handler := server.Handler()

	jobId, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest}, "")
	require.NoError(t, err)

	j, token, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""})
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
	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	server := newTestServer(t, t.TempDir(), time.Duration(0), "/api/worker/v1")
	handler := server.Handler()

	jobId, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest}, "")
	require.NoError(t, err)

	j, token, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""})
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
	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	require.NoError(t, err)
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	require.NoError(t, err)
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, 0)
	require.NoError(t, err)

	server := newTestServer(t, t.TempDir(), time.Duration(0), "/api/worker/v1")

	job := worker.OSBuildJob{
		Manifest:  manifest,
		ImageName: "test-image",
		PipelineNames: &worker.PipelineNames{
			Build:   []string{"b"},
			Payload: []string{"x", "y", "z"},
		},
	}
	jobId, err := server.EnqueueOSBuild(arch.Name(), &job, "")
	require.NoError(t, err)

	_, _, _, args, _, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""})
	require.NoError(t, err)
	require.NotNil(t, args)

	var jobArgs worker.OSBuildJob
	err = server.OSBuildJob(jobId, &jobArgs)
	require.NoError(t, err)
	require.Equal(t, job, jobArgs)
}

func TestUpload(t *testing.T) {
	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	server := newTestServer(t, t.TempDir(), time.Duration(0), "/api/worker/v1")
	handler := server.Handler()

	jobID, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest}, "")
	require.NoError(t, err)

	j, token, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""})
	require.NoError(t, err)
	require.Equal(t, jobID, j)
	require.Equal(t, worker.JobTypeOSBuild, typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "PUT", fmt.Sprintf("/api/worker/v1/jobs/%s/artifacts/foobar", token), `this is my artifact`, http.StatusOK, `?`)
}

func TestUploadAlteredBasePath(t *testing.T) {
	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	imageType, err := arch.GetImageType(test_distro.TestImageTypeName)
	if err != nil {
		t.Fatalf("error getting image type from arch: %v", err)
	}
	manifest, err := imageType.Manifest(nil, distro.ImageOptions{Size: imageType.Size(0)}, nil, nil, 0)
	if err != nil {
		t.Fatalf("error creating osbuild manifest: %v", err)
	}
	server := newTestServer(t, t.TempDir(), time.Duration(0), "/api/image-builder-worker/v1")
	handler := server.Handler()

	jobID, err := server.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest}, "")
	require.NoError(t, err)

	j, token, typ, args, dynamicArgs, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""})
	require.NoError(t, err)
	require.Equal(t, jobID, j)
	require.Equal(t, worker.JobTypeOSBuild, typ)
	require.NotNil(t, args)
	require.Nil(t, dynamicArgs)

	test.TestRoute(t, handler, false, "PUT", fmt.Sprintf("/api/image-builder-worker/v1/jobs/%s/artifacts/foobar", token), `this is my artifact`, http.StatusOK, `?`)
}

func TestTimeout(t *testing.T) {
	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	server := newTestServer(t, t.TempDir(), time.Millisecond*10, "/api/image-builder-worker/v1")

	_, _, _, _, _, err = server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeOSBuild}, []string{""})
	require.Equal(t, jobqueue.ErrDequeueTimeout, err)

	test.TestRoute(t, server.Handler(), false, "POST", "/api/image-builder-worker/v1/jobs", `{"arch":"arch","types":["types"]}`, http.StatusNoContent,
		`{"href":"/api/image-builder-worker/v1/jobs","id":"00000000-0000-0000-0000-000000000000","kind":"RequestJob"}`)
}

func TestRequestJobById(t *testing.T) {
	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	server := newTestServer(t, t.TempDir(), time.Duration(0), "/api/worker/v1")
	handler := server.Handler()

	depsolveJobId, err := server.EnqueueDepsolve(&worker.DepsolveJob{}, "")
	require.NoError(t, err)

	jobId, err := server.EnqueueManifestJobByID(&worker.ManifestJobByID{}, depsolveJobId, "")
	require.NoError(t, err)

	test.TestRoute(t, server.Handler(), false, "POST", "/api/worker/v1/jobs", fmt.Sprintf(`{"arch":"arch","types":["%s"]}`, worker.JobTypeManifestIDOnly), http.StatusBadRequest,
		`{"href":"/api/worker/v1/errors/15","kind":"Error","id": "15","code":"IMAGE-BUILDER-WORKER-15"}`, "operation_id", "reason", "message")

	_, _, _, _, _, err = server.RequestJobById(context.Background(), arch.Name(), jobId)
	require.Error(t, jobqueue.ErrNotPending, err)

	_, token, _, _, _, err := server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeDepsolve}, []string{""})
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

	emptyManifestV2 := distro.Manifest(`{"version":"2","pipelines":{}}`)
	server := newTestServer(t, t.TempDir(), time.Millisecond*10, "/")
	fbPipelines := &worker.PipelineNames{Build: distro.BuildPipelinesFallback(), Payload: distro.PayloadPipelinesFallback()}

	oldJob := worker.OSBuildJob{
		Manifest:  emptyManifestV2,
		ImageName: "no-pipeline-names",
	}
	oldJobID, err := server.EnqueueOSBuild("x", &oldJob, "")
	require.NoError(err)

	newJob := worker.OSBuildJob{
		Manifest:  emptyManifestV2,
		ImageName: "with-pipeline-names",
		PipelineNames: &worker.PipelineNames{
			Build:   []string{"build"},
			Payload: []string{"other", "pipelines"},
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
	require.Equal(oldJob.ImageName, oldJobRead.ImageName)
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
		id, token, _, _, _, err := server.RequestJob(ctx, "x", []string{worker.JobTypeOSBuild}, []string{""})
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
		OSBuildOutput: &osbuild2.Result{
			Type:    "result",
			Success: true,
			Log: map[string]osbuild2.PipelineResult{
				"build-old": {
					osbuild2.StageResult{
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
	_, _, err = server.OSBuildJobStatus(oldJobID, oldJobResultRead)
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
		OSBuildOutput: &osbuild2.Result{
			Type:    "result",
			Success: true,
			Log: map[string]osbuild2.PipelineResult{
				"build-new": {
					osbuild2.StageResult{
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
	_, _, err = server.OSBuildJobStatus(newJobID, newJobResultRead)
	require.NoError(err)
	require.Equal(newJobResult, newJobResultRead)
}

// Enqueue Koji jobs with and without additional data and read them off the queue to
// check if the fallbacks are added for the old job and the new data are kept
// for the new job.
func TestMixedOSBuildKojiJob(t *testing.T) {
	require := require.New(t)

	emptyManifestV2 := distro.Manifest(`{"version":"2","pipelines":{}}`)
	server := newTestServer(t, t.TempDir(), time.Duration(0), "/api/worker/v1")
	fbPipelines := &worker.PipelineNames{Build: distro.BuildPipelinesFallback(), Payload: distro.PayloadPipelinesFallback()}

	enqueueKojiJob := func(job *worker.OSBuildKojiJob) uuid.UUID {
		initJob := new(worker.KojiInitJob)
		initJobID, err := server.EnqueueKojiInit(initJob, "")
		require.NoError(err)
		jobID, err := server.EnqueueOSBuildKoji("k", job, initJobID, "")
		require.NoError(err)
		return jobID
	}
	oldJob := worker.OSBuildKojiJob{
		Manifest:  emptyManifestV2,
		ImageName: "no-pipeline-names",
	}
	oldJobID := enqueueKojiJob(&oldJob)

	newJob := worker.OSBuildKojiJob{
		Manifest:  emptyManifestV2,
		ImageName: "with-pipeline-names",
		PipelineNames: &worker.PipelineNames{
			Build:   []string{"build"},
			Payload: []string{"other", "pipelines"},
		},
	}
	newJobID := enqueueKojiJob(&newJob)

	var oldJobRead worker.OSBuildKojiJob
	err := server.OSBuildKojiJob(oldJobID, &oldJobRead)
	require.NoError(err)
	require.NotNil(oldJobRead.PipelineNames)
	// OldJob gets default pipeline names when read
	require.Equal(fbPipelines, oldJobRead.PipelineNames)
	require.Equal(oldJob.Manifest, oldJobRead.Manifest)
	require.Equal(oldJob.ImageName, oldJobRead.ImageName)
	// Not entirely equal
	require.NotEqual(oldJob, oldJobRead)

	// NewJob the same when read back
	var newJobRead worker.OSBuildKojiJob
	err = server.OSBuildKojiJob(newJobID, &newJobRead)
	require.NoError(err)
	require.NotNil(newJobRead.PipelineNames)
	require.Equal(newJob.PipelineNames, newJobRead.PipelineNames)

	// Dequeue the jobs (via RequestJob) to get their tokens and update them to
	// test the result retrieval

	// Finish init jobs
	for idx := uint(0); idx < 2; idx++ {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_, token, _, _, _, err := server.RequestJob(ctx, "k", []string{worker.JobTypeKojiInit}, []string{""})
		require.NoError(err)
		require.NoError(server.FinishJob(token, nil))
	}

	getJob := func() (uuid.UUID, uuid.UUID) {
		// don't block forever if the jobs weren't added or can't be retrieved
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		id, token, _, _, _, err := server.RequestJob(ctx, "k", []string{worker.JobTypeOSBuildKoji}, []string{""})
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

	oldJobResult := &worker.OSBuildKojiJobResult{
		HostOS: "rhel-10",
		Arch:   "k",
		OSBuildOutput: &osbuild2.Result{
			Type:    "result",
			Success: true,
			Log: map[string]osbuild2.PipelineResult{
				"build-old": {
					osbuild2.StageResult{
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

	oldJobResultRead := new(worker.OSBuildKojiJobResult)
	_, _, err = server.OSBuildKojiJobStatus(oldJobID, oldJobResultRead)
	require.NoError(err)

	// oldJobResultRead should have PipelineNames now
	require.NotEqual(oldJobResult, oldJobResultRead)
	require.Equal(fbPipelines, oldJobResultRead.PipelineNames)
	require.NotNil(oldJobResultRead.PipelineNames)
	require.Equal(oldJobResult.OSBuildOutput, oldJobResultRead.OSBuildOutput)
	require.Equal(oldJobResult.HostOS, oldJobResultRead.HostOS)
	require.Equal(oldJobResult.Arch, oldJobResultRead.Arch)

	newJobResult := &worker.OSBuildKojiJobResult{
		HostOS: "rhel-10",
		Arch:   "k",
		PipelineNames: &worker.PipelineNames{
			Build:   []string{"build-result"},
			Payload: []string{"result-test-payload", "result-test-assembler"},
		},
		OSBuildOutput: &osbuild2.Result{
			Type:    "result",
			Success: true,
			Log: map[string]osbuild2.PipelineResult{
				"build-new": {
					osbuild2.StageResult{
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

	newJobResultRead := new(worker.OSBuildKojiJobResult)
	_, _, err = server.OSBuildKojiJobStatus(newJobID, newJobResultRead)
	require.NoError(err)
	require.Equal(newJobResult, newJobResultRead)
}

func TestDepsolveLegacyErrorConversion(t *testing.T) {
	distroStruct := test_distro.New()
	arch, err := distroStruct.GetArch(test_distro.TestArchName)
	if err != nil {
		t.Fatalf("error getting arch from distro: %v", err)
	}
	server := newTestServer(t, t.TempDir(), time.Duration(0), "/api/worker/v1")

	depsolveJobId, err := server.EnqueueDepsolve(&worker.DepsolveJob{}, "")
	require.NoError(t, err)

	_, _, _, _, _, err = server.RequestJob(context.Background(), arch.Name(), []string{worker.JobTypeDepsolve}, []string{""})
	require.NoError(t, err)

	reason := "Depsolve failed"
	errType := worker.DepsolveErrorType

	expectedResult := worker.DepsolveJobResult{
		Error:     reason,
		ErrorType: errType,
		JobResult: worker.JobResult{
			JobError: clienterrors.WorkerClientError(clienterrors.ErrorDNFDepsolveError, reason),
		},
	}

	depsolveJobResult := worker.DepsolveJobResult{
		Error:     reason,
		ErrorType: errType,
	}

	_, _, err = server.DepsolveJobStatus(depsolveJobId, &depsolveJobResult)
	require.NoError(t, err)
	require.Equal(t, expectedResult, depsolveJobResult)
}

// Enquueue OSBuild jobs and save both kinds of
// error types (old & new) to the queue and ensure
// that both kinds of errors can be read back
func TestMixedOSBuildJobErrors(t *testing.T) {
	require := require.New(t)

	emptyManifestV2 := distro.Manifest(`{"version":"2","pipelines":{}}`)
	server := newTestServer(t, t.TempDir(), time.Millisecond*10, "/")

	oldJob := worker.OSBuildJob{
		Manifest:  emptyManifestV2,
		ImageName: "no-pipeline-names",
	}
	oldJobID, err := server.EnqueueOSBuild("x", &oldJob, "")
	require.NoError(err)

	newJob := worker.OSBuildJob{
		Manifest:  emptyManifestV2,
		ImageName: "with-pipeline-names",
		PipelineNames: &worker.PipelineNames{
			Build:   []string{"build"},
			Payload: []string{"other", "pipelines"},
		},
	}
	newJobID, err := server.EnqueueOSBuild("x", &newJob, "")
	require.NoError(err)

	oldJobRead := new(worker.OSBuildJob)
	err = server.OSBuildJob(oldJobID, oldJobRead)
	require.NoError(err)
	// Not entirely equal
	require.NotEqual(oldJob, oldJobRead)

	// NewJob the same when read back
	newJobRead := new(worker.OSBuildJob)
	err = server.OSBuildJob(newJobID, newJobRead)
	require.NoError(err)

	// Dequeue the jobs (via RequestJob) to get their tokens and update them to
	// test the result retrieval

	getJob := func() (uuid.UUID, uuid.UUID) {
		// don't block forever if the jobs weren't added or can't be retrieved
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		id, token, _, _, _, err := server.RequestJob(ctx, "x", []string{worker.JobTypeOSBuild}, []string{""})
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
		TargetErrors: []string{"Upload error"},
	}
	oldJobResultRaw, err := json.Marshal(oldJobResult)
	require.NoError(err)
	oldJobToken := jobTokens[oldJobID]
	err = server.FinishJob(oldJobToken, oldJobResultRaw)
	require.NoError(err)

	oldJobResultRead := new(worker.OSBuildJobResult)
	_, _, err = server.OSBuildJobStatus(oldJobID, oldJobResultRead)
	require.NoError(err)

	require.NotEqual(oldJobResult, oldJobResultRead)
	require.Equal(oldJobResult.Success, false)
	require.Equal(oldJobResultRead.Success, false)

	newJobResult := &worker.OSBuildJobResult{
		PipelineNames: &worker.PipelineNames{
			Build:   []string{"build-result"},
			Payload: []string{"result-test-payload", "result-test-assembler"},
		},
		JobResult: worker.JobResult{
			JobError: clienterrors.WorkerClientError(clienterrors.ErrorUploadingImage, "Error uploading image", nil),
		},
	}
	newJobResultRaw, err := json.Marshal(newJobResult)
	require.NoError(err)
	newJobToken := jobTokens[newJobID]
	err = server.FinishJob(newJobToken, newJobResultRaw)
	require.NoError(err)

	newJobResultRead := new(worker.OSBuildJobResult)
	_, _, err = server.OSBuildJobStatus(newJobID, newJobResultRead)
	require.NoError(err)
	require.Equal(newJobResult, newJobResultRead)
	require.Equal(newJobResult.Success, false)
	require.Equal(newJobResultRead.Success, false)
}

// Enquueue Koji jobs and save both kinds of
// error types (old & new) to the queue and ensure
// that both kinds of errors can be read back
func TestMixedOSBuildKojiJobErrors(t *testing.T) {
	require := require.New(t)

	emptyManifestV2 := distro.Manifest(`{"version":"2","pipelines":{}}`)
	server := newTestServer(t, t.TempDir(), time.Duration(0), "/api/worker/v1")

	enqueueKojiJob := func(job *worker.OSBuildKojiJob) uuid.UUID {
		initJob := new(worker.KojiInitJob)
		initJobID, err := server.EnqueueKojiInit(initJob, "")
		require.NoError(err)
		jobID, err := server.EnqueueOSBuildKoji("k", job, initJobID, "")
		require.NoError(err)
		return jobID
	}
	oldJob := worker.OSBuildKojiJob{
		Manifest:  emptyManifestV2,
		ImageName: "no-pipeline-names",
	}
	oldJobID := enqueueKojiJob(&oldJob)

	newJob := worker.OSBuildKojiJob{
		Manifest:  emptyManifestV2,
		ImageName: "with-pipeline-names",
		PipelineNames: &worker.PipelineNames{
			Build:   []string{"build"},
			Payload: []string{"other", "pipelines"},
		},
	}
	newJobID := enqueueKojiJob(&newJob)

	oldJobRead := new(worker.OSBuildKojiJob)
	err := server.OSBuildKojiJob(oldJobID, oldJobRead)
	require.NoError(err)
	// Not entirely equal
	require.NotEqual(oldJob, oldJobRead)

	// NewJob the same when read back
	newJobRead := new(worker.OSBuildKojiJob)
	err = server.OSBuildKojiJob(newJobID, newJobRead)
	require.NoError(err)

	// Dequeue the jobs (via RequestJob) to get their tokens and update them to
	// test the result retrieval

	// Finish init jobs
	for idx := uint(0); idx < 2; idx++ {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		_, token, _, _, _, err := server.RequestJob(ctx, "k", []string{worker.JobTypeKojiInit}, []string{""})
		require.NoError(err)
		require.NoError(server.FinishJob(token, nil))
	}

	getJob := func() (uuid.UUID, uuid.UUID) {
		// don't block forever if the jobs weren't added or can't be retrieved
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		id, token, _, _, _, err := server.RequestJob(ctx, "k", []string{worker.JobTypeOSBuildKoji}, []string{""})
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

	oldJobResult := &worker.OSBuildKojiJobResult{
		KojiError: "koji build error",
	}
	oldJobResultRaw, err := json.Marshal(oldJobResult)
	require.NoError(err)
	oldJobToken := jobTokens[oldJobID]
	err = server.FinishJob(oldJobToken, oldJobResultRaw)
	require.NoError(err)

	oldJobResultRead := new(worker.OSBuildKojiJobResult)
	_, _, err = server.OSBuildKojiJobStatus(oldJobID, oldJobResultRead)
	require.NoError(err)

	// oldJobResultRead should have PipelineNames now
	require.NotEqual(oldJobResult, oldJobResultRead)

	newJobResult := &worker.OSBuildKojiJobResult{
		PipelineNames: &worker.PipelineNames{
			Build:   []string{"build-result"},
			Payload: []string{"result-test-payload", "result-test-assembler"},
		},
		JobResult: worker.JobResult{
			JobError: clienterrors.WorkerClientError(clienterrors.ErrorKojiBuild, "Koji build error", nil),
		},
	}
	newJobResultRaw, err := json.Marshal(newJobResult)
	require.NoError(err)
	newJobToken := jobTokens[newJobID]
	err = server.FinishJob(newJobToken, newJobResultRaw)
	require.NoError(err)

	newJobResultRead := new(worker.OSBuildKojiJobResult)
	_, _, err = server.OSBuildKojiJobStatus(newJobID, newJobResultRead)
	require.NoError(err)
	require.Equal(newJobResult, newJobResultRead)
}

// old depsolve job format kept here to test compatibility with older workers
type oldDepsolveJob struct {
	PackageSetsChains map[string][]string           `json:"package_sets_chains"`
	PackageSets       map[string]rpmmd.PackageSet   `json:"package_sets"`
	Repos             []rpmmd.RepoConfig            `json:"repos"`
	ModulePlatformID  string                        `json:"module_platform_id"`
	Arch              string                        `json:"arch"`
	Releasever        string                        `json:"releasever"`
	PackageSetsRepos  map[string][]rpmmd.RepoConfig `json:"package_sets_repositories,omitempty"`
}

func TestDepsolveJobArgsCompat(t *testing.T) {
	// Test depsolve job argument transition compatibility
	//
	// NOTE: This test should be removed once all cloud workers are updated to
	// use the new structure.
	assert := assert.New(t)

	// common elements
	baseos := rpmmd.RepoConfig{
		Name:    "baseos",
		BaseURL: "https://example.com/baseos",
	}
	appstream := rpmmd.RepoConfig{
		Name:    "appstream",
		BaseURL: "https://example.com/appstream",
	}
	user1 := rpmmd.RepoConfig{
		Name:    "user1",
		BaseURL: "https://example.com/user/1",
	}
	user2 := rpmmd.RepoConfig{
		Name:    "user2",
		BaseURL: "https://example.com/user/2",
	}

	osIncludes := []string{"os1", "os2", "os3"}
	bpIncludes := []string{"bp1", "bp2"}
	buildIncludes := []string{"build1", "build2", "build3"}

	excludes := []string{"nope1", "nope2"}

	newJob := worker.DepsolveJob{
		PackageSets: map[string][]rpmmd.PackageSet{
			"os": {
				{
					Include:      osIncludes,
					Exclude:      excludes,
					Repositories: []rpmmd.RepoConfig{baseos, appstream},
				},
				{
					Include:      bpIncludes,
					Repositories: []rpmmd.RepoConfig{baseos, appstream, user1, user2},
				},
			},
			"build": {{
				Include:      buildIncludes,
				Exclude:      excludes,
				Repositories: []rpmmd.RepoConfig{baseos, appstream},
			}},
		},
		ModulePlatformID: "el9",
		Arch:             "x86_64",
		Releasever:       "9",
	}

	oldJob := oldDepsolveJob{
		PackageSetsChains: map[string][]string{
			"os": {"os-0", "os-1"},
		},
		PackageSets: map[string]rpmmd.PackageSet{
			"os-0": {
				Include: osIncludes,
				Exclude: excludes,
			},
			"os-1": {
				Include: bpIncludes,
			},
			"build": {
				Include: buildIncludes,
				Exclude: excludes,
			},
		},
		ModulePlatformID: "el9",
		Arch:             "x86_64",
		Releasever:       "9",
		PackageSetsRepos: map[string][]rpmmd.RepoConfig{
			"os-0":  {baseos, appstream},
			"os-1":  {baseos, appstream, user1, user2},
			"build": {baseos, appstream},
		},
	}

	{ // old in, old out (not really useful, but let's cover all bases)
		oldArgs, err := json.Marshal(oldJob)
		assert.NoError(err)
		var oldJobW oldDepsolveJob
		assert.NoError(json.Unmarshal(oldArgs, &oldJobW))
		assert.Equal(oldJob, oldJobW)
	}

	{ // new in, old out (the important scenario)
		newArgs, err := json.Marshal(newJob)
		assert.NoError(err)
		var oldJobW oldDepsolveJob
		assert.NoError(json.Unmarshal(newArgs, &oldJobW))
		assert.Equal(oldJob, oldJobW)
	}

	{ // new in, new out (check if the serialised format also unmarshals back into the new format)
		newArgs, err := json.Marshal(newJob)
		assert.NoError(err)
		var newJobW worker.DepsolveJob
		assert.NoError(json.Unmarshal(newArgs, &newJobW))
		assert.Equal(newJob, newJobW)
	}
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
			id, err = s.EnqueueOSBuildAsDependency(distro.X86_64ArchName, job, depUUIDs, "")
			if err != nil {
				return nil, err
			}

		case *worker.ManifestJobByID:
			job := dep.main.(*worker.ManifestJobByID)
			if len(depUUIDs) != 1 {
				return nil, fmt.Errorf("exactly one dependency is expected for ManifestJobByID, got: %d", len(depUUIDs))
			}
			id, err = s.EnqueueManifestJobByID(job, depUUIDs[0], "")
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

		case *worker.OSBuildKojiJob:
			job := dep.main.(*worker.OSBuildKojiJob)
			if len(depUUIDs) != 2 {
				return nil, fmt.Errorf("exactly two dependency is expected for OSBuildKojiJob, got: %d", len(depUUIDs))
			}
			id, err = s.EnqueueOSBuildKojiAsDependency(distro.X86_64ArchName, job, depUUIDs[1], depUUIDs[0], "")
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

		default:
			return nil, fmt.Errorf("unexpected job type")
		}

		// request the previously added Job
		_, token, _, _, _, err := s.RequestJobById(context.Background(), distro.X86_64ArchName, id)
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
		// koji-init + osbuild-koji + manifest + depsolve
		// failed depsolve
		{
			job: testJob{
				main: &worker.OSBuildKojiJob{},
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
				result: &worker.OSBuildKojiJobResult{
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
		// koji-init + (osbuild-koji + manifest + depsolve) + (osbuild-koji + manifest + depsolve) + koji-finalize
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
						main: &worker.OSBuildKojiJob{},
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
						result: &worker.OSBuildKojiJobResult{
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
						main: &worker.OSBuildKojiJob{},
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
						result: &worker.OSBuildKojiJobResult{
							OSBuildOutput: &osbuild2.Result{},
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
		// koji-init + (osbuild-koji + manifest + depsolve) + (osbuild-koji + manifest + depsolve) + koji-finalize
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
						main: &worker.OSBuildKojiJob{},
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
						result: &worker.OSBuildKojiJobResult{
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
						main: &worker.OSBuildKojiJob{},
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
						result: &worker.OSBuildKojiJobResult{
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
		// koji-init + (osbuild-koji + manifest + depsolve) + (osbuild-koji + manifest + depsolve) + koji-finalize
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
						main: &worker.OSBuildKojiJob{},
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
						result: &worker.OSBuildKojiJobResult{
							OSBuildOutput: &osbuild2.Result{},
						},
					},
					// passed build
					{
						main: &worker.OSBuildKojiJob{},
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
						result: &worker.OSBuildKojiJobResult{
							OSBuildOutput: &osbuild2.Result{},
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
		// koji-init + (osbuild-koji + manifest + depsolve) + (osbuild-koji + manifest + depsolve) + koji-finalize
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
						main: &worker.OSBuildKojiJob{},
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
						result: &worker.OSBuildKojiJobResult{
							OSBuildOutput: &osbuild2.Result{},
						},
					},
					// passed build
					{
						main: &worker.OSBuildKojiJob{},
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
						result: &worker.OSBuildKojiJobResult{
							OSBuildOutput: &osbuild2.Result{},
						},
					},
				},
				result: &worker.KojiFinalizeJobResult{
					JobResult: worker.JobResult{},
				},
			},
			expectedError: nil,
		},
	}

	for idx, c := range cases {
		t.Logf("Test case #%d", idx)
		server := newTestServer(t, t.TempDir(), time.Duration(0), "/api/worker/v1")
		ids, err := enqueueAndFinishTestJobDependencies(server, []testJob{c.job})
		require.Nil(t, err)
		require.Len(t, ids, 1)

		mainJobID := ids[0]
		errors, err := server.JobDependencyChainErrors(mainJobID)
		require.Nil(t, err)
		assert.EqualValues(t, c.expectedError, errors)
	}
}
