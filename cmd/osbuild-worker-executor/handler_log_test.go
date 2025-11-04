package main_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func simulateBuildResult(t *testing.T, buildBaseDir, result, log string) {
	err := os.MkdirAll(filepath.Join(buildBaseDir, "build"), 0755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(buildBaseDir, result), nil, 0644)
	assert.NoError(t, err)

	path := filepath.Join(buildBaseDir, "build/osbuild-result.json")
	if result == "result.good" {
		err := os.MkdirAll(filepath.Join(buildBaseDir, "build/output"), 0755)
		assert.NoError(t, err)
		path = filepath.Join(buildBaseDir, "build/output/osbuild-result.json")
	}
	err = os.WriteFile(path, []byte(log), 0644)
	assert.NoError(t, err)
}

func TestLogTooEarly(t *testing.T) {
	baseURL, _, _ := runTestServer(t)
	endpoint := baseURL + "api/v1/log"

	rsp, err := http.Get(endpoint)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooEarly, rsp.StatusCode)
}

func TestLogBad(t *testing.T) {
	baseURL, buildBaseDir, _ := runTestServer(t)
	endpoint := baseURL + "api/v1/log"

	simulateBuildResult(t, buildBaseDir, "result.bad", "failure log")
	rsp, err := http.Get(endpoint)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rsp.StatusCode)
	body, err := io.ReadAll(rsp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "failure log", string(body))
}

func TestLogGood(t *testing.T) {
	baseURL, buildBaseDir, _ := runTestServer(t)
	endpoint := baseURL + "api/v1/log"

	simulateBuildResult(t, buildBaseDir, "result.good", "fake-build-result")
	rsp, err := http.Get(endpoint)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, rsp.StatusCode)
	body, err := io.ReadAll(rsp.Body)
	assert.NoError(t, err)
	assert.Equal(t, "fake-build-result", string(body))
}
