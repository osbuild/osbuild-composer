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
	assert.Equal(t, rsp.StatusCode, http.StatusTooEarly)
}

func TestResultBad(t *testing.T) {
	baseURL, buildBaseDir, _ := runTestServer(t)
	endpoint := baseURL + "api/v1/result/disk.img"

	// simulate build failure
	// todo: make a nice helper method
	err := os.MkdirAll(filepath.Join(buildBaseDir, "build"), 0755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(buildBaseDir, "result.bad"), nil, 0644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(buildBaseDir, "build/build.log"), []byte("failure log"), 0644)
	assert.NoError(t, err)

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

	// simulate build failure
	// todo: make a nice helper method
	err := os.MkdirAll(filepath.Join(buildBaseDir, "build/output"), 0755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(buildBaseDir, "result.good"), nil, 0644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(buildBaseDir, "build/output/disk.img"), []byte("fake-build-result"), 0644)
	assert.NoError(t, err)

	rsp, err := http.Get(endpoint)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rsp.StatusCode)
	body, err := io.ReadAll(rsp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "fake-build-result", string(body))
}
