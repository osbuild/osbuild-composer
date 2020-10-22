package kojiapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/jobqueue/testjobqueue"
	"github.com/osbuild/osbuild-composer/internal/kojiapi"
	"github.com/osbuild/osbuild-composer/internal/kojiapi/api"
	distro_mock "github.com/osbuild/osbuild-composer/internal/mocks/distro"
	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/upload/koji"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/stretchr/testify/require"
)

func newTestKojiServer(t *testing.T) *kojiapi.Server {
	rpm_fixture := rpmmd_mock.BaseFixture()
	rpm := rpmmd_mock.NewRPMMDMock(rpm_fixture)
	require.NotNil(t, rpm)

	distros, err := distro_mock.NewDefaultRegistry()
	require.NoError(t, err)
	require.NotNil(t, distros)

	workers := worker.NewServer(nil, testjobqueue.New(), "")
	require.NotNil(t, workers)

	server := kojiapi.NewServer(nil, workers, rpm, distros, map[string]koji.GSSAPICredentials{})
	require.NotNil(t, server)

	return server
}

func TestStatus(t *testing.T) {
	server := newTestKojiServer(t)
	handler := server.Handler("/api/composer-koji/v1")

	req := httptest.NewRequest("GET", "/api/composer-koji/v1/status", nil)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp := rec.Result()

	require.Equal(t, 200, resp.StatusCode)

	var status api.Status
	err := json.NewDecoder(resp.Body).Decode(&status)
	require.NoError(t, err)
	require.Equal(t, "OK", status.Status)
}

func TestRequest(t *testing.T) {
	server := newTestKojiServer(t)
	handler := server.Handler("/api/composer-koji/v1")

	// Make request to an invalid route
	req := httptest.NewRequest("GET", "/invalidroute", nil)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp := rec.Result()

	var status api.Status
	err := json.NewDecoder(resp.Body).Decode(&status)
	require.NoError(t, err)
	require.Equal(t, http.StatusNotFound, resp.StatusCode)

	// Trigger an error 400 code
	req = httptest.NewRequest("GET", "/api/composer-koji/v1/compose/badid", nil)

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	resp = rec.Result()

	err = json.NewDecoder(resp.Body).Decode(&status)
	require.NoError(t, err)
	require.Equal(t, http.StatusBadRequest, resp.StatusCode)
}
