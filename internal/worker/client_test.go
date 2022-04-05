package worker_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

func TestOAuth(t *testing.T) {
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
	defer workSrv.Close()

	/* Check that the worker supplies the access token  */
	calls := 0
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls == 0 {
			require.Equal(t, "Bearer", r.Header.Get("Authorization"))
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer accessToken!", r.Header.Get("Authorization"))
		handler.ServeHTTP(w, r)
	}))
	defer proxySrv.Close()

	offlineToken := "someOfflineToken"
	/* Start oauth srv supplying the bearer token */
	oauthSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls += 1
		require.Equal(t, "POST", r.Method)
		err = r.ParseForm()
		require.NoError(t, err)

		require.Equal(t, "refresh_token", r.FormValue("grant_type"))
		require.Equal(t, "rhsm-api", r.FormValue("client_id"))
		require.Equal(t, offlineToken, r.FormValue("refresh_token"))

		bt := struct {
			AccessToken string `json:"access_token"`
		}{
			"accessToken!",
		}

		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(bt)
		require.NoError(t, err)
	}))
	defer oauthSrv.Close()

	_, err = workerServer.EnqueueOSBuild("arch", &worker.OSBuildJob{}, "")
	require.NoError(t, err)

	client, err := worker.NewClient(worker.ClientConfig{
		BaseURL:      proxySrv.URL,
		TlsConfig:    nil,
		ClientId:     "rhsm-api",
		OfflineToken: offlineToken,
		OAuthURL:     oauthSrv.URL,
		BasePath:     "/api/image-builder-worker/v1",
	})
	require.NoError(t, err)
	job, err := client.RequestJob([]string{"osbuild"}, "arch")
	require.NoError(t, err)
	r := strings.NewReader("artifact contents")
	require.NoError(t, job.UploadArtifact("some-artifact", r))
	c, err := job.Canceled()
	require.False(t, c)
}
