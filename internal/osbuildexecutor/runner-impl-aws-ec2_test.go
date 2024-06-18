package osbuildexecutor_test

import (
	"archive/tar"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/osbuild/images/pkg/osbuild"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/osbuildexecutor"
)

func TestWaitForSI(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	require.False(t, osbuildexecutor.WaitForSI(ctx, server.URL))

	server.Start()
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel2()
	require.True(t, osbuildexecutor.WaitForSI(ctx2, server.URL))
}

func TestWriteInputArchive(t *testing.T) {
	cacheDir := t.TempDir()
	storeDir := filepath.Join(cacheDir, "store")
	require.NoError(t, os.Mkdir(storeDir, 0755))
	storeSubDir := filepath.Join(storeDir, "subdir")
	require.NoError(t, os.Mkdir(storeSubDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(storeDir, "contents"), []byte("storedata"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(storeSubDir, "contents"), []byte("storedata"), 0600))

	archive, err := osbuildexecutor.WriteInputArchive(cacheDir, storeDir, "some-job-id", []string{"image"}, []byte("{\"version\": 2}"))
	require.NoError(t, err)

	cmd := exec.Command("tar",
		"-tf",
		archive,
	)
	out, err := cmd.CombinedOutput()
	require.NoError(t, err)
	require.ElementsMatch(t, []string{
		"control.json",
		"manifest.json",
		"store/",
		"store/subdir/",
		"store/subdir/contents",
		"store/contents",
		"",
	}, strings.Split(string(out), "\n"))

	output, err := exec.Command("tar", "xOf", archive, "control.json").CombinedOutput()
	require.NoError(t, err)
	require.Equal(t, `{"exports":["image"],"job-id":"some-job-id"}`, string(output))
}

func TestHandleBuild(t *testing.T) {
	buildServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		input, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.Equal(t, []byte("test"), input)

		w.WriteHeader(http.StatusCreated)
		osbuildResult := osbuild.Result{
			Success: true,
		}
		data, err := json.Marshal(osbuildResult)
		require.NoError(t, err)
		_, err = w.Write(data)
		require.NoError(t, err)
	}))

	cacheDir := t.TempDir()
	inputArchive := filepath.Join(cacheDir, "test.tar")
	require.NoError(t, os.WriteFile(inputArchive, []byte("test"), 0600))

	osbuildResult, err := osbuildexecutor.HandleBuild(inputArchive, buildServer.URL)
	require.NoError(t, err)
	require.True(t, osbuildResult.Success)
}

func TestHandleOutputArchive(t *testing.T) {
	serverDir := t.TempDir()
	serverOutputDir := filepath.Join(serverDir, "output")
	require.NoError(t, os.Mkdir(serverOutputDir, 0755))
	serverImageDir := filepath.Join(serverOutputDir, "image")
	require.NoError(t, os.Mkdir(serverImageDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(serverImageDir, "disk.img"), []byte("image"), 0600))

	serverOutput := filepath.Join(serverDir, "server-output.tar")
	cmd := exec.Command("tar",
		"-C",
		serverDir,
		"-cf",
		serverOutput,
		filepath.Base(serverOutputDir),
	)
	require.NoError(t, cmd.Run())

	resultServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		file, err := os.Open(serverOutput)
		if err != nil {
			require.NoError(t, err)
		}
		defer file.Close()
		_, err = io.Copy(w, file)
		require.NoError(t, err)
	}))

	outputDir := t.TempDir()
	archive, err := osbuildexecutor.FetchOutputArchive(outputDir, resultServer.URL)
	require.NoError(t, err)

	extractDir := filepath.Join(outputDir, "extracted")
	require.NoError(t, os.Mkdir(extractDir, 0755))
	require.NoError(t, osbuildexecutor.ExtractOutputArchive(extractDir, archive))

	content, err := os.ReadFile(filepath.Join(extractDir, "image", "disk.img"))
	require.NoError(t, err)
	require.Equal(t, []byte("image"), content)
}

func makeTestTarfile(t *testing.T, content map[*tar.Header]string) string {
	tmpdir := t.TempDir()
	testTarPath := filepath.Join(tmpdir, "test.tar")
	f, err := os.Create(testTarPath)
	assert.NoError(t, err)
	defer f.Close()

	atar := tar.NewWriter(f)
	for hdr, fcnt := range content {
		if hdr.Mode == 0 {
			hdr.Mode = 0644
		}
		hdr.Size = int64(len(fcnt))

		err := atar.WriteHeader(hdr)
		assert.NoError(t, err)
		_, err = atar.Write([]byte(fcnt))
		assert.NoError(t, err)
	}

	return testTarPath
}

func TestValidateOutputArchiveHappy(t *testing.T) {
	testTarPath := makeTestTarfile(t, map[*tar.Header]string{
		&tar.Header{Name: "file1"}:        "some content",
		&tar.Header{Name: "path/to/file"}: "other content",
		&tar.Header{
			Name:     "path/to/dir",
			Typeflag: tar.TypeDir,
		}: "",
	})
	err := osbuildexecutor.ValidateOutputArchive(testTarPath)
	assert.NoError(t, err)
}

func makeSparseFile(t *testing.T, path string) {
	output, err := exec.Command("truncate", "-s", "10M", path).CombinedOutput()
	assert.Equal(t, "", string(output))
	assert.NoError(t, err)
}

func TestValidateOutputArchiveHappySparseFile(t *testing.T) {
	// go tar makes support for sparse files very hard, see also
	// https://github.com/golang/go/issues/22735
	tmpdir := t.TempDir()
	makeSparseFile(t, filepath.Join(tmpdir, "big.img"))
	testTarPath := filepath.Join(t.TempDir(), "test.tar")
	output, err := exec.Command("tar", "--strip-components=1", "-C", tmpdir, "-c", "-S", "-f", testTarPath, "big.img").CombinedOutput()
	assert.Equal(t, "", string(output))
	assert.NoError(t, err)

	err = osbuildexecutor.ValidateOutputArchive(testTarPath)
	assert.NoError(t, err)
}

func TestValidateOutputArchiveSadDotDot(t *testing.T) {
	testTarPath := makeTestTarfile(t, map[*tar.Header]string{
		&tar.Header{Name: "file1/.."}: "some content",
	})
	err := osbuildexecutor.ValidateOutputArchive(testTarPath)
	assert.EqualError(t, err, `name "file1/.." not clean, got "." after cleaning`)
}

func TestValidateOutputArchiveSadAbsolutePath(t *testing.T) {
	testTarPath := makeTestTarfile(t, map[*tar.Header]string{
		&tar.Header{Name: "/file1"}: "some content",
	})
	err := osbuildexecutor.ValidateOutputArchive(testTarPath)
	assert.EqualError(t, err, `name "/file1" must not start with an absolute path`)
}

func TestValidateOutputArchiveSadBadType(t *testing.T) {
	testTarPath := makeTestTarfile(t, map[*tar.Header]string{
		&tar.Header{Name: "dev/sda", Typeflag: tar.TypeBlock}: "",
	})
	err := osbuildexecutor.ValidateOutputArchive(testTarPath)
	assert.EqualError(t, err, `name "dev/sda" must be a file/dir, is header type '4'`)
}

func TestValidateOutputArchiveSadExecutable(t *testing.T) {
	testTarPath := makeTestTarfile(t, map[*tar.Header]string{
		&tar.Header{Name: "exe", Mode: 0755}: "#!/bin/sh p0wned",
	})
	err := osbuildexecutor.ValidateOutputArchive(testTarPath)
	assert.EqualError(t, err, `name "exe" must not be executable (is mode 0755)`)
}
