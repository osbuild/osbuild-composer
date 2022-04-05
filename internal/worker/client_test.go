package worker_test

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

func TestOAuth(t *testing.T) {
	tempdir, err := ioutil.TempDir("", "worker-tests-")
	require.NoError(t, err)
	defer os.RemoveAll(tempdir)

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

	_, err = workerServer.EnqueueOSBuild(arch.Name(), &worker.OSBuildJob{Manifest: manifest}, "")
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
	job, err := client.RequestJob([]string{"osbuild"}, arch.Name())
	require.NoError(t, err)
	r := strings.NewReader("artifact contents")
	require.NoError(t, job.UploadArtifact("some-artifact", r))
	c, err := job.Canceled()
	require.False(t, c)
}
