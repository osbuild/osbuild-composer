package osbuildexecutor_test

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder/pkg/osbuild"

	"github.com/osbuild/osbuild-composer/internal/cloud/awscloud"
	"github.com/osbuild/osbuild-composer/internal/osbuildexecutor"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

func TestIsSecureInstanceGone(t *testing.T) {
	refused := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ECONNREFUSED}
	reset := &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}
	unreachable := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.ENETUNREACH}
	noRoute := &net.OpError{Op: "dial", Net: "tcp", Err: syscall.EHOSTUNREACH}
	timeout := &net.OpError{Op: "read", Net: "tcp", Err: os.ErrDeadlineExceeded}
	wrappedRefused := fmt.Errorf("unable to request build from executor instance: %w", refused)
	truncated := fmt.Errorf("error parsing osbuild status, please report a bug: %w", io.ErrUnexpectedEOF)
	truncatedJSON := fmt.Errorf(`error parsing osbuild status, please report a bug: cannot scan line: unexpected end of JSON input`)
	garbageJSON := fmt.Errorf(`error parsing osbuild status, please report a bug: cannot scan line "bad non-json text": invalid character 'b' looking for beginning of value`)
	liveLog := errors.New("osbuild log is available")

	tests := []struct {
		name     string
		buildErr error
		fetchErr error
		want     bool
	}{
		{name: "post refused", buildErr: refused, want: true},
		{name: "post reset", buildErr: reset, want: true},
		{name: "post unreachable", buildErr: unreachable, want: true},
		{name: "post no route", buildErr: noRoute, want: true},
		{name: "wrapped refused", buildErr: wrappedRefused, want: true},
		{name: "truncated and log refused", buildErr: truncated, fetchErr: refused, want: true},
		{name: "truncated json and log refused", buildErr: truncatedJSON, fetchErr: refused, want: true},
		{name: "output fetch refused", fetchErr: refused, want: true},
		{name: "truncated but live log", buildErr: truncated, fetchErr: nil, want: false},
		{name: "garbage json live log", buildErr: garbageJSON, fetchErr: nil, want: false},
		{name: "garbage json and log refused", buildErr: garbageJSON, fetchErr: refused, want: false},
		{name: "fetch timeout only", fetchErr: timeout, want: false},
		{name: "truncated and fetch timeout", buildErr: truncated, fetchErr: timeout, want: false},
		{name: "real osbuild log", buildErr: errors.New("stage failed"), fetchErr: liveLog, want: false},
		{name: "nil errors", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, osbuildexecutor.IsSecureInstanceGone(tt.buildErr, tt.fetchErr))
		})
	}
}

func TestHandleBuildTruncatedThenFetchLogRefused(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/build", r.URL.Path)
		w.WriteHeader(http.StatusCreated)
		_, err := w.Write([]byte(`{"message": "starting pipeline", "context": {"origin": "osbuild.monitor", "pipeline": {"name": "source", "id": "pipeline-id", "stage": {}}, "id": "context-id"}, "progress": {"name": "pipelines/sources", "total": 1, "done": 0}}
{"message": "truncated`))
		require.NoError(t, err)
	}))

	cacheDir := t.TempDir()
	inputArchive := filepath.Join(cacheDir, "test.tar")
	require.NoError(t, os.WriteFile(inputArchive, []byte("test"), 0600))

	entry, _ := makeMockEntry()
	buildErr := osbuildexecutor.HandleBuild(inputArchive, server.URL, entry, nil)
	require.Error(t, buildErr)
	require.ErrorContains(t, buildErr, "unexpected end of JSON input")

	server.Close()
	_, fetchErr := osbuildexecutor.FetchLog(server.URL)
	require.Error(t, fetchErr)
	require.True(t, osbuildexecutor.IsSecureInstanceGone(buildErr, fetchErr))
}

func TestHandleBuildInvalidJSONLiveLogNotGone(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/build":
			w.WriteHeader(http.StatusCreated)
			_, err := w.Write([]byte("bad non-json text"))
			require.NoError(t, err)
		case "/api/v1/log":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("osbuild still running"))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	cacheDir := t.TempDir()
	inputArchive := filepath.Join(cacheDir, "test.tar")
	require.NoError(t, os.WriteFile(inputArchive, []byte("test"), 0600))

	entry, _ := makeMockEntry()
	buildErr := osbuildexecutor.HandleBuild(inputArchive, server.URL, entry, nil)
	require.ErrorContains(t, buildErr, "invalid character")

	logBody, fetchErr := osbuildexecutor.FetchLog(server.URL)
	require.NoError(t, fetchErr)
	require.Equal(t, "osbuild still running", logBody)
	require.False(t, osbuildexecutor.IsSecureInstanceGone(buildErr, fetchErr))
}

func hostPortFromURL(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	require.NoError(t, err)
	return u.Host
}

func writeExecutorOutputTar(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	outputDir := filepath.Join(dir, "output")
	require.NoError(t, os.Mkdir(outputDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(outputDir, osbuildexecutor.OSBuildResultFilename), []byte(`{"success": true}`), 0600))
	tarPath := filepath.Join(dir, "output.tar")
	cmd := exec.Command("tar", "-C", dir, "-cf", tarPath, filepath.Base(outputDir))
	require.NoError(t, cmd.Run())
	return tarPath
}

func newSuccessExecutorServer(t *testing.T) *httptest.Server {
	t.Helper()
	outputTar := writeExecutorOutputTar(t)
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/" || r.URL.Path == "/api/v1"):
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/build":
			_, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			w.WriteHeader(http.StatusCreated)
			_, err = w.Write([]byte(`{"message": "starting pipeline", "context": {"origin": "osbuild.monitor", "pipeline": {"name": "source", "id": "pipeline-id", "stage": {}}, "id": "context-id"}, "progress": {"name": "pipelines/sources", "total": 1, "done": 0}}
{"message": "finishing pipeline", "result": {"name": "source", "id": "pipeline-id", "success": true}, "context": {"id": "context-id"}, "progress": {"name": "source", "total": 1, "done": 1}}`))
			require.NoError(t, err)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/result/output.tar":
			w.WriteHeader(http.StatusOK)
			f, err := os.Open(outputTar)
			require.NoError(t, err)
			defer f.Close()
			_, err = io.Copy(w, f)
			require.NoError(t, err)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/log":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("ok"))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
}

func siForHost(hostPort string) *awscloud.SecureInstance {
	addr := hostPort
	return &awscloud.SecureInstance{
		Instance: &ec2types.Instance{
			PrivateIpAddress: &addr,
		},
	}
}

func TestRunOSBuildRetriesOnceThenSucceeds(t *testing.T) {
	restore := osbuildexecutor.MockRunPrepareSources(func([]byte, logrus.FieldLogger, *osbuild.OSBuildOptions) (*osbuild.Result, error) {
		return &osbuild.Result{Success: true}, nil
	})
	defer restore()

	success := newSuccessExecutorServer(t)
	defer success.Close()

	runs := 0
	terms := 0
	client := osbuildexecutor.SIClientFunc{
		Run: func(iamProfile, keyName, hostname string) (*awscloud.SecureInstance, error) {
			runs++
			if runs == 1 {
				return siForHost("127.0.0.1:1"), nil
			}
			return siForHost(hostPortFromURL(t, success.URL)), nil
		},
		Term: func(si *awscloud.SecureInstance) error {
			terms++
			return nil
		},
	}

	cacheDir := t.TempDir()
	storeDir := filepath.Join(cacheDir, "store")
	require.NoError(t, os.Mkdir(storeDir, 0755))
	outputDir := filepath.Join(cacheDir, "out")
	require.NoError(t, os.Mkdir(outputDir, 0755))

	job := &testJob{}
	entry, hook := makeMockEntry()
	exec := osbuildexecutor.NewAWSEC2ExecutorWithSIClient("profile", "key", "host", cacheDir, client, 2*time.Second)
	result, err := exec.RunOSBuild([]byte(`{"version": 2}`), entry, job, &osbuild.OSBuildOptions{
		Exports:   []string{"image"},
		StoreDir:  storeDir,
		OutputDir: outputDir,
	})
	require.NoError(t, err)
	require.True(t, result.Success)
	require.Equal(t, 2, runs)
	require.Equal(t, 2, terms)
	require.GreaterOrEqual(t, len(job.PartialUpdates), 1)
	partial, ok := job.PartialUpdates[0].(worker.JobResult)
	require.True(t, ok)
	require.Equal(t, "Retrying on a new secure instance", partial.Progress.Message)

	var sawWarn bool
	for _, e := range hook.AllEntries() {
		if e.Level == logrus.WarnLevel && e.Message == "Secure instance died, retrying osbuild" {
			sawWarn = true
		}
		require.NotEqual(t, logrus.ErrorLevel, e.Level, "first-attempt SI-gone must not Error: %s", e.Message)
	}
	require.True(t, sawWarn)
}

func TestRunOSBuildTwiceThenFail(t *testing.T) {
	restore := osbuildexecutor.MockRunPrepareSources(func([]byte, logrus.FieldLogger, *osbuild.OSBuildOptions) (*osbuild.Result, error) {
		return &osbuild.Result{Success: true}, nil
	})
	defer restore()

	runs := 0
	terms := 0
	client := osbuildexecutor.SIClientFunc{
		Run: func(iamProfile, keyName, hostname string) (*awscloud.SecureInstance, error) {
			runs++
			return siForHost("127.0.0.1:1"), nil
		},
		Term: func(si *awscloud.SecureInstance) error {
			terms++
			return nil
		},
	}

	cacheDir := t.TempDir()
	storeDir := filepath.Join(cacheDir, "store")
	require.NoError(t, os.Mkdir(storeDir, 0755))
	outputDir := filepath.Join(cacheDir, "out")
	require.NoError(t, os.Mkdir(outputDir, 0755))

	entry, hook := makeMockEntry()
	exec := osbuildexecutor.NewAWSEC2ExecutorWithSIClient("profile", "key", "host", cacheDir, client, 2*time.Second)
	_, err := exec.RunOSBuild([]byte(`{"version": 2}`), entry, nil, &osbuild.OSBuildOptions{
		Exports:   []string{"image"},
		StoreDir:  storeDir,
		OutputDir: outputDir,
	})
	require.ErrorIs(t, err, osbuildexecutor.ErrSecureInstanceGone)
	require.Equal(t, 2, runs)
	require.Equal(t, 2, terms)
	require.Equal(t, logrus.ErrorLevel, hook.LastEntry().Level)
}

func TestRunOSBuildDoesNotRetryStartFailure(t *testing.T) {
	restore := osbuildexecutor.MockRunPrepareSources(func([]byte, logrus.FieldLogger, *osbuild.OSBuildOptions) (*osbuild.Result, error) {
		return &osbuild.Result{Success: true}, nil
	})
	defer restore()

	runs := 0
	client := osbuildexecutor.SIClientFunc{
		Run: func(iamProfile, keyName, hostname string) (*awscloud.SecureInstance, error) {
			runs++
			return nil, errors.New("CreateFleet failed")
		},
	}

	cacheDir := t.TempDir()
	storeDir := filepath.Join(cacheDir, "store")
	require.NoError(t, os.Mkdir(storeDir, 0755))

	entry, _ := makeMockEntry()
	exec := osbuildexecutor.NewAWSEC2ExecutorWithSIClient("profile", "key", "host", cacheDir, client, time.Second)
	_, err := exec.RunOSBuild([]byte(`{"version": 2}`), entry, nil, &osbuild.OSBuildOptions{
		Exports:   []string{"image"},
		StoreDir:  storeDir,
		OutputDir: t.TempDir(),
	})
	require.ErrorContains(t, err, "Unable to start secure instance")
	require.False(t, errors.Is(err, osbuildexecutor.ErrSecureInstanceGone))
	require.Equal(t, 1, runs)
}

type cancelJob struct {
	testJob
	canceled bool
}

func (j *cancelJob) Canceled() (bool, error) {
	return j.canceled, nil
}

func TestRunOSBuildCanceledDoesNotRetry(t *testing.T) {
	restore := osbuildexecutor.MockRunPrepareSources(func([]byte, logrus.FieldLogger, *osbuild.OSBuildOptions) (*osbuild.Result, error) {
		return &osbuild.Result{Success: true}, nil
	})
	defer restore()

	runs := 0
	client := osbuildexecutor.SIClientFunc{
		Run: func(iamProfile, keyName, hostname string) (*awscloud.SecureInstance, error) {
			runs++
			return siForHost("127.0.0.1:1"), nil
		},
	}

	cacheDir := t.TempDir()
	storeDir := filepath.Join(cacheDir, "store")
	require.NoError(t, os.Mkdir(storeDir, 0755))

	job := &cancelJob{canceled: true}
	entry, _ := makeMockEntry()
	exec := osbuildexecutor.NewAWSEC2ExecutorWithSIClient("profile", "key", "host", cacheDir, client, 2*time.Second)
	_, err := exec.RunOSBuild([]byte(`{"version": 2}`), entry, job, &osbuild.OSBuildOptions{
		Exports:   []string{"image"},
		StoreDir:  storeDir,
		OutputDir: t.TempDir(),
	})
	require.EqualError(t, err, "job was canceled")
	require.Equal(t, 1, runs)
}

func TestRunOSBuildDoesNotRetryWhenLogAvailable(t *testing.T) {
	restore := osbuildexecutor.MockRunPrepareSources(func([]byte, logrus.FieldLogger, *osbuild.OSBuildOptions) (*osbuild.Result, error) {
		return &osbuild.Result{Success: true}, nil
	})
	defer restore()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && (r.URL.Path == "/api/v1/" || r.URL.Path == "/api/v1"):
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/build":
			w.WriteHeader(http.StatusCreated)
			_, err := w.Write([]byte("bad non-json text"))
			require.NoError(t, err)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/log":
			w.WriteHeader(http.StatusOK)
			_, err := w.Write([]byte("stage exploded"))
			require.NoError(t, err)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	runs := 0
	client := osbuildexecutor.SIClientFunc{
		Run: func(iamProfile, keyName, hostname string) (*awscloud.SecureInstance, error) {
			runs++
			return siForHost(hostPortFromURL(t, server.URL)), nil
		},
	}

	cacheDir := t.TempDir()
	storeDir := filepath.Join(cacheDir, "store")
	require.NoError(t, os.Mkdir(storeDir, 0755))

	entry, _ := makeMockEntry()
	exec := osbuildexecutor.NewAWSEC2ExecutorWithSIClient("profile", "key", "host", cacheDir, client, 2*time.Second)
	_, err := exec.RunOSBuild([]byte(`{"version": 2}`), entry, nil, &osbuild.OSBuildOptions{
		Exports:   []string{"image"},
		StoreDir:  storeDir,
		OutputDir: t.TempDir(),
	})
	require.ErrorContains(t, err, "osbuild failed: stage exploded")
	require.False(t, errors.Is(err, osbuildexecutor.ErrSecureInstanceGone))
	require.Equal(t, 1, runs)
}
