package worker_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type RunFunc func(context.Context, *worker.Job) (interface{}, error)
type TestJobImpl struct {
	testRun RunFunc
}

func (impl *TestJobImpl) Run(ctx context.Context, job *worker.Job) (interface{}, error) {
	if impl.testRun == nil {
		panic("no testRun() function defined")
	}
	return impl.testRun(ctx, job)
}

func newTestWorkerServerClientPair(t *testing.T, heartbeat time.Duration, proxy *proxy) (*worker.Server, *worker.Client) {
	tempdir := t.TempDir()

	q, err := fsjobqueue.New(tempdir)
	require.NoError(t, err)
	config := worker.Config{
		ArtifactsDir: tempdir,
		BasePath:     "/api/image-builder-worker/v1",
	}
	workerServer := worker.NewServer(nil, q, config)

	handler := workerServer.Handler()

	workSrv := httptest.NewServer(handler)
	t.Cleanup(workSrv.Close)

	offlineToken := "someOfflineToken"
	accessToken := "accessToken!"

	/* Check that the worker supplies the access token  */
	calls := 0
	gatewaySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls == 0 {
			require.Equal(t, "Bearer", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer "+accessToken, r.Header.Get("Authorization"))
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(gatewaySrv.Close)

	/* Start oauth srv supplying the bearer token */
	oauthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls += 1
		require.Equal(t, "POST", r.Method)
		err := r.ParseForm()
		require.NoError(t, err)

		require.Equal(t, "refresh_token", r.FormValue("grant_type"))
		require.Equal(t, "rhsm-api", r.FormValue("client_id"))
		require.Equal(t, offlineToken, r.FormValue("refresh_token"))

		bt := struct {
			AccessToken string `json:"access_token"`
		}{
			accessToken,
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(bt)
		require.NoError(t, err)
	}))
	t.Cleanup(oauthSrv.Close)

	var proxyURL string
	if proxy != nil {
		// initialize a test proxy server
		proxySrv := httptest.NewServer(proxy)
		t.Cleanup(proxySrv.Close)
		proxyURL = proxySrv.URL
	}

	workerClient, err := worker.NewClient(worker.ClientConfig{
		BaseURL:      gatewaySrv.URL,
		Heartbeat:    heartbeat,
		ClientId:     "rhsm-api",
		OfflineToken: offlineToken,
		OAuthURL:     oauthSrv.URL,
		BasePath:     "/api/image-builder-worker/v1",
		ProxyURL:     proxyURL,
	})
	require.NoError(t, err)

	return workerServer, workerClient
}

func TestClient(t *testing.T) {
	server, client := newTestWorkerServerClientPair(t, 0, nil)

	barrier := make(chan bool)
	testRun := func(ctx context.Context, job *worker.Job) (interface{}, error) {
		if <-barrier {
			return struct{}{}, nil
		} else {
			return nil, nil
		}
	}
	jobImpls := map[string]worker.JobImplementation{
		worker.JobTypeOSBuild: &TestJobImpl{testRun},
	}
	requestCtx, requestCtxCancel := context.WithCancel(context.Background())
	go client.Start(requestCtx, time.Duration(10*time.Microsecond), nil, "arch", jobImpls)
	t.Cleanup(requestCtxCancel)

	// schedule two jobs
	// we only really care about the first one, and use the second one
	// only as a barrier: once it is being processed we know the first one
	// has been finished.
	jobID, err := server.EnqueueOSBuild("arch", &worker.OSBuildJob{}, "")
	require.NoError(t, err)
	_, err = server.EnqueueOSBuild("arch", &worker.OSBuildJob{}, "")
	require.NoError(t, err)

	barrier <- true  // processing first job - run the test
	barrier <- false // processing second job - only a barrier, don't run the test

	var result worker.OSBuildJobResult
	info, err := server.OSBuildJobInfo(jobID, &result)
	require.NoError(t, err)
	require.NotZero(t, info.JobStatus.Finished)
}

func TestClientCancel(t *testing.T) {
	barrier := make(chan bool)
	testRun := func(ctx context.Context, job *worker.Job) (interface{}, error) {
		if <-barrier {
			<-ctx.Done() // wait to be cancelled
			return struct{}{}, nil
		} else {
			return nil, nil
		}
	}
	jobImpls := map[string]worker.JobImplementation{
		worker.JobTypeOSBuild: &TestJobImpl{testRun},
	}

	server, client := newTestWorkerServerClientPair(t, 2*time.Millisecond, nil)

	requestCtx, requestCtxCancel := context.WithCancel(context.Background())
	go client.Start(requestCtx, time.Duration(10*time.Microsecond), nil, "arch", jobImpls)
	t.Cleanup(requestCtxCancel)

	// schedule two jobs
	// we only really care about the first one, and use the second one
	// only as a barrier: once it is being processed we know the first one
	// has been finished.
	jobID, err := server.EnqueueOSBuild("arch", &worker.OSBuildJob{}, "")
	require.NoError(t, err)
	_, err = server.EnqueueOSBuild("arch", &worker.OSBuildJob{}, "")
	require.NoError(t, err)

	barrier <- true // processing first job - run the test
	err = server.Cancel(jobID)
	require.NoError(t, err)
	barrier <- false // processing second job - only a barrier, don't run the test
}

func TestClientUpload(t *testing.T) {
	server, client := newTestWorkerServerClientPair(t, 0, nil)

	barrier := make(chan bool)
	artifactName := "some-artifact"
	artifactContents := "artifact contents"
	testRun := func(ctx context.Context, job *worker.Job) (interface{}, error) {
		if <-barrier {
			r := strings.NewReader(artifactContents)
			require.NoError(t, job.UploadArtifact(ctx, artifactName, r))
			return struct{}{}, nil
		} else {
			return nil, nil
		}
	}
	jobImpls := map[string]worker.JobImplementation{
		worker.JobTypeOSBuild: &TestJobImpl{testRun},
	}
	requestCtx, requestCtxCancel := context.WithCancel(context.Background())
	go client.Start(requestCtx, time.Duration(10*time.Microsecond), nil, "arch", jobImpls)
	t.Cleanup(requestCtxCancel)

	// schedule two jobs
	jobID, err := server.EnqueueOSBuild("arch", &worker.OSBuildJob{}, "")
	require.NoError(t, err)
	_, err = server.EnqueueOSBuild("arch", &worker.OSBuildJob{}, "")
	require.NoError(t, err)

	barrier <- true  // processing first job - run the test
	barrier <- false // processing second job - only a barrier, don't run the test

	var result worker.OSBuildJobResult
	info, err := server.OSBuildJobInfo(jobID, &result)
	require.NoError(t, err)
	require.NotZero(t, info.JobStatus.Finished)
	reader, size, err := server.JobArtifact(jobID, artifactName)
	require.NoError(t, err)
	require.Equal(t, int64(len(artifactContents)), size)
	artifact, err := ioutil.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, artifactContents, string(artifact))
}

func TestClientProxy(t *testing.T) {
	proxy := proxy{}
	server, client := newTestWorkerServerClientPair(t, 0, &proxy)

	barrier := make(chan bool)
	testRun := func(ctx context.Context, job *worker.Job) (interface{}, error) {
		if <-barrier {
			return struct{}{}, nil
		} else {
			return nil, nil
		}
	}
	jobImpls := map[string]worker.JobImplementation{
		worker.JobTypeOSBuild: &TestJobImpl{testRun},
	}
	requestCtx, requestCtxCancel := context.WithCancel(context.Background())
	go client.Start(requestCtx, time.Duration(10*time.Microsecond), nil, "arch", jobImpls)
	t.Cleanup(requestCtxCancel)

	// schedule two jobs
	_, err := server.EnqueueOSBuild("arch", &worker.OSBuildJob{}, "")
	require.NoError(t, err)
	_, err = server.EnqueueOSBuild("arch", &worker.OSBuildJob{}, "")
	require.NoError(t, err)

	barrier <- true  // processing first job - run the test
	barrier <- false // processing second job - only a barrier, don't run the test

	// we expect 5 calls to go through the proxy:
	// - request job (fails, no oauth token)
	// - oauth call
	// - request job (succeeds)
	// - finish
	// - request next job (hangs)
	require.Equal(t, 5, proxy.calls)
}

func TestClientInterrupt(t *testing.T) {
	server, client := newTestWorkerServerClientPair(t, 0, nil)

	_, err := server.EnqueueOSBuild("arch", &worker.OSBuildJob{}, "")
	require.NoError(t, err)

	barrier1 := make(chan struct{})
	testRun1 := func(ctx context.Context, job *worker.Job) (interface{}, error) {
		barrier1 <- struct{}{}
		<-ctx.Done()
		return struct{}{}, nil
	}
	jobImpls1 := map[string]worker.JobImplementation{
		worker.JobTypeOSBuild: &TestJobImpl{testRun1},
	}

	clientCtx1, clientCtxCancel1 := context.WithCancel(context.Background())
	go client.Start(clientCtx1, time.Duration(10*time.Microsecond), nil, "arch", jobImpls1)

	<-barrier1         // processing first job - will hang to wait for cancellation
	clientCtxCancel1() // cancel the client - this will cause the job to get rescheduled

	// start a second client to pick up the job
	barrier2 := make(chan struct{})
	testRun2 := func(ctx context.Context, job *worker.Job) (interface{}, error) {
		barrier2 <- struct{}{}
		return struct{}{}, nil
	}
	jobImpls2 := map[string]worker.JobImplementation{
		worker.JobTypeOSBuild: &TestJobImpl{testRun2},
	}

	clientCtx2, clientCtxCancel2 := context.WithCancel(context.Background())
	go client.Start(clientCtx2, time.Duration(10*time.Microsecond), nil, "arch", jobImpls2)
	t.Cleanup(clientCtxCancel2)

	<-barrier2 // the job got picked up again - interruption worked as expected
}
