package osbuildexecutor

import (
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
	"github.com/stretchr/testify/require"
)

func TestWaitForSI(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	require.False(t, waitForSI(ctx, server.URL))

	server.Start()
	ctx2, cancel2 := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel2()
	require.True(t, waitForSI(ctx2, server.URL))
}

func TestWriteInputArchive(t *testing.T) {
	cacheDir := t.TempDir()
	storeDir := filepath.Join(cacheDir, "store")
	require.NoError(t, os.Mkdir(storeDir, 0755))
	storeSubDir := filepath.Join(storeDir, "subdir")
	require.NoError(t, os.Mkdir(storeSubDir, 0755))

	require.NoError(t, os.WriteFile(filepath.Join(storeDir, "contents"), []byte("storedata"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(storeSubDir, "contents"), []byte("storedata"), 0600))

	archive, err := writeInputArchive(cacheDir, storeDir, []string{"image"}, []byte("{\"version\": 2}"))
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
}

func TestHandleBuild(t *testing.T) {
	buildServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	osbuildResult, err := handleBuild(inputArchive, buildServer.URL)
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
	archive, err := fetchOutputArchive(outputDir, resultServer.URL)
	require.NoError(t, err)

	extractDir := filepath.Join(outputDir, "extracted")
	require.NoError(t, os.Mkdir(extractDir, 0755))
	require.NoError(t, extractOutputArchive(extractDir, archive))

	content, err := os.ReadFile(filepath.Join(extractDir, "image", "disk.img"))
	require.NoError(t, err)
	require.Equal(t, []byte("image"), content)
}
