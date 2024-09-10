package worker_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

// newTestWorkerServer returns 3 strings:
// - base URL of the worker server
// - base URL of the OAuth server
// - offline token for OAuth
//
// The worker server has one pending osbuild job of arch "arch" ready to be dequeued.
func newTestWorkerServer(t *testing.T) (string, string, string) {
	tempdir := t.TempDir()

	q, err := fsjobqueue.New(tempdir)
	require.NoError(t, err)
	config := worker.Config{
		ArtifactsDir: tempdir,
		BasePath:     "/api/image-builder-worker/v1",
	}
	workerServer := worker.NewServer(nil, q, config)
	_, err = workerServer.EnqueueOSBuild("arch", &worker.OSBuildJob{}, "")
	require.NoError(t, err)

	handler := workerServer.Handler()

	workSrv := httptest.NewServer(handler)
	t.Cleanup(workSrv.Close)

	offlineToken := "someOfflineToken"
	accessToken := "accessToken!"

	/* Check that the worker supplies the access token  */
	calls := 0
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls == 0 {
			require.Equal(t, "Bearer", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer "+accessToken, r.Header.Get("Authorization"))
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(proxySrv.Close)

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

	return proxySrv.URL, oauthSrv.URL, offlineToken
}

func TestOAuth(t *testing.T) {
	workerURL, oauthURL, offlineToken := newTestWorkerServer(t)

	client, err := worker.NewClient(worker.ClientConfig{
		BaseURL:      workerURL,
		TlsConfig:    nil,
		ClientId:     "rhsm-api",
		OfflineToken: offlineToken,
		OAuthURL:     oauthURL,
		BasePath:     "/api/image-builder-worker/v1",
	})
	require.NoError(t, err)
	job, err := client.RequestJob([]string{worker.JobTypeOSBuild}, "arch")
	require.NoError(t, err)
	r := strings.NewReader("artifact contents")
	require.NoError(t, job.UploadArtifact("some-artifact", r))
	c, err := job.Canceled()
	require.False(t, c)
	require.NoError(t, err)
}

func TestProxy(t *testing.T) {
	workerURL, oauthURL, offlineToken := newTestWorkerServer(t)

	// initialize a test proxy server
	proxy := &proxy{}
	proxySrv := httptest.NewServer(proxy)
	t.Cleanup(proxySrv.Close)

	client, err := worker.NewClient(worker.ClientConfig{
		BaseURL:      workerURL,
		TlsConfig:    nil,
		ClientId:     "rhsm-api",
		OfflineToken: offlineToken,
		OAuthURL:     oauthURL,
		BasePath:     "/api/image-builder-worker/v1",
		ProxyURL:     proxySrv.URL,
	})

	require.NoError(t, err)
	job, err := client.RequestJob([]string{worker.JobTypeOSBuild}, "arch")
	require.NoError(t, err)
	r := strings.NewReader("artifact contents")
	require.NoError(t, job.UploadArtifact("some-artifact", r))
	c, err := job.Canceled()
	require.False(t, c)
	require.NoError(t, err)

	// we expect 6 calls to go through the proxy:
	// - register worker
	// - request job (fails, no oauth token)
	// - oauth call
	// - request job (succeeds)
	// - upload artifact
	// - cancel
	require.Equal(t, 6, proxy.calls)
}

func TestNewClientWorkerNoErrorOnRegisterWorkerFailure(t *testing.T) {
	logrusOutput := bytes.NewBuffer(nil)
	logrus.SetOutput(logrusOutput)

	apiCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		apiCalls++

		require.Equal(t, "/api/image-builder-worker/v1/workers", req.URL.Path)
		w.WriteHeader(400)
		_, err := w.Write([]byte(`{"reason":"reason", "details": "details"}`))
		require.NoError(t, err)
	}))
	defer srv.Close()

	client, err := worker.NewClient(worker.ClientConfig{
		BaseURL:  srv.URL,
		BasePath: "/api/image-builder-worker/v1",
	})
	require.NoError(t, err)
	require.NotNil(t, client)
	require.Equal(t, 1, apiCalls)
	require.Contains(t, logrusOutput.String(), `Error registering worker on startup, error registering worker: 400`)
}
