package worker_test

import (
	"encoding/json"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Header.Get("Authorization") == "Bearer" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		require.Equal(t, "Bearer "+accessToken, req.Header.Get("Authorization"))
		handler.ServeHTTP(w, req)
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

	testCases := []struct {
		name                string
		resetAuthentication bool
		expectedCalls       int
	}{
		{"Test normal startup", false, 5},
		{"Test loosing authentication", true, 7},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workerURL, oauthURL, offlineToken := newTestWorkerServer(t)

			// initialize a test proxy server
			proxy := &proxy{registrationSuccessful: false}
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

			if tc.resetAuthentication {
				client.InvalidateAccessToken()
			}

			require.NoError(t, err)
			job, err := client.RequestJob([]string{worker.JobTypeOSBuild}, "arch")
			require.NoError(t, err)
			r := strings.NewReader("artifact contents")
			require.NoError(t, job.UploadArtifact("some-artifact", r))
			c, err := job.Canceled()
			require.False(t, c)
			require.NoError(t, err)

			// we expect 5 or 7 calls to go through the proxy:
			// depending, if we "loose" authentication or not
			// * oauth call
			// * register worker
			// --- second testcase SNIP
			// * request job (401 unauthorized)
			// * oauth call
			// ---
			// * request job
			// * upload artifact
			// * cancel
			require.Equal(t, tc.expectedCalls, proxy.calls, "We got those:\n"+strings.Join(proxy.paths, "\n"))

			require.True(t, proxy.registrationSuccessful)
		})
	}
}

type LogCheckerHook struct {
	checkMsgs []string
	t         *testing.T
	counter   int
}

func (h *LogCheckerHook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.DebugLevel,
		logrus.InfoLevel,
		logrus.WarnLevel,
		logrus.ErrorLevel,
		logrus.FatalLevel,
		logrus.PanicLevel,
	}
}

func (h *LogCheckerHook) Fire(e *logrus.Entry) error {
	require.Less(h.t, h.counter, len(h.checkMsgs), "Got %s, probably unexpected", e.Message)
	require.Contains(h.t, e.Message, h.checkMsgs[h.counter])
	h.counter += 1
	return nil
}

func muteLogrusDuringTest(t *testing.T) {
	// avoid having errors on stdout - as this is expected
	oldOut := logrus.StandardLogger().Out
	logrus.SetOutput(io.Discard)
	t.Cleanup(func() {
		logrus.SetOutput(oldOut)
	})

	newHooks := make(logrus.LevelHooks)
	originalHooks := logrus.StandardLogger().ReplaceHooks(newHooks)
	t.Cleanup(func() {
		logrus.StandardLogger().ReplaceHooks(originalHooks)
	})
}

func TestFailedInitialToken(t *testing.T) {
	muteLogrusDuringTest(t)

	workerURL, _, offlineToken := newTestWorkerServer(t)

	logrus.AddHook(&LogCheckerHook{
		t:         t,
		checkMsgs: []string{"Error getting access token on startup"},
	})

	_, err := worker.NewClient(worker.ClientConfig{
		BaseURL:      workerURL,
		TlsConfig:    nil,
		ClientId:     "rhsm-api",
		OfflineToken: offlineToken,
		OAuthURL:     "http://illegalHost:1234",
		BasePath:     "/api/image-builder-worker/v1",
	})
	require.NoError(t, err)

}

func TestFailedInitialRegisterWorker(t *testing.T) {
	muteLogrusDuringTest(t)

	_, oauthURL, offlineToken := newTestWorkerServer(t)

	logrus.AddHook(&LogCheckerHook{
		t: t,
		checkMsgs: []string{
			"Unable to register worker",
			"Error registering worker on startup",
		}})

	_, err := worker.NewClient(worker.ClientConfig{
		BaseURL:      "http://illegalHost:1234",
		TlsConfig:    nil,
		ClientId:     "rhsm-api",
		OfflineToken: offlineToken,
		OAuthURL:     oauthURL,
		BasePath:     "/api/image-builder-worker/v1",
	})
	require.NoError(t, err)
}
