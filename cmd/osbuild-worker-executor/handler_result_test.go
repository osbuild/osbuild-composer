package main_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResultTooEarly(t *testing.T) {
	baseURL, _, _ := runTestServer(t)
	endpoint := baseURL + "api/v1/result"

	rsp, err := http.Get(endpoint)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooEarly, rsp.StatusCode)
}

func TestResultBad(t *testing.T) {
	baseURL, buildBaseDir, _ := runTestServer(t)
	endpoint := baseURL + "api/v1/result/disk.img"

	simulateBuildResult(t, buildBaseDir, "result.bad", "failure log")
	rsp, err := http.Get(endpoint)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, rsp.StatusCode)
	body, err := io.ReadAll(rsp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "build failed\nfailure log", string(body))
}

func TestResultGood(t *testing.T) {
	baseURL, buildBaseDir, _ := runTestServer(t)
	endpoint := baseURL + "api/v1/result/disk.img"

	simulateBuildResult(t, buildBaseDir, "result.good", "fake-build-log")
	err := os.WriteFile(filepath.Join(buildBaseDir, "build/output/disk.img"), []byte("fake-build-result"), 0644)
	assert.NoError(t, err)

	rsp, err := http.Get(endpoint)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rsp.StatusCode)
	body, err := io.ReadAll(rsp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "fake-build-result", string(body))
}
