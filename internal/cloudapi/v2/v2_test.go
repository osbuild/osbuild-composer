package v2_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	distro_mock "github.com/osbuild/osbuild-composer/internal/mocks/distro"
	rpmmd_mock "github.com/osbuild/osbuild-composer/internal/mocks/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

func newV2Server(t *testing.T, dir string) (*v2.Server, *worker.Server) {
	rpmFixture := rpmmd_mock.BaseFixture(dir)
	rpm := rpmmd_mock.NewRPMMDMock(rpmFixture)
	require.NotNil(t, rpm)

	distros, err := distro_mock.NewDefaultRegistry()
	require.NoError(t, err)
	require.NotNil(t, distros)

	v2Server := v2.NewServer(rpmFixture.Workers, rpm, distros)
	require.NotNil(t, v2Server)

	return v2Server, rpmFixture.Workers
}

func TestUnknownRoute(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, _ := newV2Server(t, dir)

	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "GET", "/api/composer/v2/badroute", ``, http.StatusNotFound, `
	{
		"href": "/api/composer/v2/errors/21",
		"id": "21",
		"kind": "Error",
		"code": "COMPOSER-21",
		"reason": "Requested resource doesn't exist"
	}`, "operation_id")
}

func TestGetError(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, _ := newV2Server(t, dir)

	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "GET", "/api/composer/v2/errors/4", ``, http.StatusOK, `
	{
		"href": "/api/composer/v2/errors/4",
		"id": "4",
		"kind": "Error",
		"code": "COMPOSER-4",
		"reason": "Unsupported distribution"
	}`, "operation_id")

	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "GET", "/api/composer/v2/errors/3000", ``, http.StatusNotFound, `
	{
		"href": "/api/composer/v2/errors/17",
		"id": "17",
		"kind": "Error",
		"code": "COMPOSER-17",
		"reason": "Error with given id not found"
	}`, "operation_id")
}

func TestGetErrorList(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, _ := newV2Server(t, dir)

	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "GET", "/api/composer/v2/errors?page=3&size=1", ``, http.StatusOK, `
	{
		"kind": "ErrorList",
		"page": 3,
		"size": 1,
		"items": [{
			"href": "/api/composer/v2/errors/4",
			"id": "4",
			"kind": "Error",
			"code": "COMPOSER-4",
			"reason": "Unsupported distribution"
		 }]
	}`, "operation_id", "total")
}

func TestCompose(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, _ := newV2Server(t, dir)

	// unsupported distribution
	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "POST", "/api/composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "unsupported_distro",
		"image_requests":[{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_request": {
				"type": "aws.s3",
				"options": {
					"access_key_id": "somekey",
					"secret_access_key": "somesecretkey",
					"bucket": "somebucket"
				}
			}
		 }]
	}`, test_distro.TestArchName, test_distro.TestImageTypeName), http.StatusBadRequest, `
	{
		"href": "/api/composer/v2/errors/4",
		"id": "4",
		"kind": "Error",
		"code": "COMPOSER-4",
		"reason": "Unsupported distribution"
	}`, "operation_id")

	// unsupported architecture
	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "POST", "/api/composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_requests":[{
			"architecture": "unsupported_arch",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_request": {
				"type": "aws.s3",
				"options": {
					"access_key_id": "somekey",
					"secret_access_key": "somesecretkey",
					"bucket": "somebucket"
				}
			}
		 }]
	}`, test_distro.TestDistroName, test_distro.TestImageTypeName), http.StatusBadRequest, `
	{
		"href": "/api/composer/v2/errors/5",
		"id": "5",
		"kind": "Error",
		"code": "COMPOSER-5",
		"reason": "Unsupported architecture"
	}`, "operation_id")

	// unsupported imagetype
	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "POST", "/api/composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_requests":[{
			"architecture": "%s",
			"image_type": "unsupported_image_type",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_request": {
				"type": "aws.s3",
				"options": {
					"access_key_id": "somekey",
					"secret_access_key": "somesecretkey",
					"bucket": "somebucket"
				}
			}
		 }]
	}`, test_distro.TestDistroName, test_distro.TestArchName), http.StatusBadRequest, `
	{
		"href": "/api/composer/v2/errors/6",
		"id": "6",
		"kind": "Error",
		"code": "COMPOSER-6",
		"reason": "Unsupported image type"
	}`, "operation_id")

	// Returns 404, but should be 405; see https://github.com/labstack/echo/issues/1981
	// test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "GET", "/api/composer/v2/compose", fmt.Sprintf(`
	// {
	//	"distribution": "%s",
	//	"image_requests":[{
	//		"architecture": "%s",
	//		"image_type": "%s",
	//		"repositories": [{
	//			"baseurl": "somerepo.org",
	//			"rhsm": false
	//		}],
	//		"upload_request": {
	//			"type": "aws.s3",
	//			"options": {
	//				"access_key_id": "somekey",
	//				"secret_access_key": "somesecretkey",
	//				"bucket": "somebucket"
	//			}
	//		}
	//          }]
	// }`, test_distro.TestDistroName, test_distro.TestArchName, test_distro.TestImageTypeName), http.StatusMethodNotAllowed, `
	// {
	//	"href": "/api/composer/v2/errors/22",
	//	"id": "22",
	//	"kind": "Error",
	//	"code": "COMPOSER-22",
	//	"reason": "Requested method isn't supported for resource"
	// }`, "operation_id")

	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "POST", "/api/composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_requests":[{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_request": {
				"type": "aws.s3",
				"options": {
					"access_key_id": "somekey",
					"secret_access_key": "somesecretkey",
					"bucket": "somebucket"
				}
			}
		 }]
	}`, test_distro.TestDistroName, test_distro.TestArchName, test_distro.TestImageTypeName), http.StatusCreated, `
	{
		"href": "/api/composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
}

func TestComposeStatusSuccess(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, wrksrv := newV2Server(t, dir)

	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "POST", "/api/composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_requests":[{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_request": {
				"type": "aws.s3",
				"options": {
					"access_key_id": "somekey",
					"secret_access_key": "somesecretkey",
					"bucket": "somebucket"
				}
			}
		 }]
	}`, test_distro.TestDistroName, test_distro.TestArchName, test_distro.TestImageTypeName), http.StatusCreated, `
	{
		"href": "/api/composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArchName, []string{"osbuild"})
	require.NoError(t, err)
	require.Equal(t, "osbuild", jobType)

	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "GET", fmt.Sprintf("/api/composer/v2/compose/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/composer/v2/compose/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "building"}
	}`, jobId, jobId))

	// todo make it an osbuildjobresult
	res, err := json.Marshal(&worker.OSBuildJobResult{
		Success: true,
	})
	require.NoError(t, err)

	err = wrksrv.FinishJob(token, res)
	require.NoError(t, err)
	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "GET", fmt.Sprintf("/api/composer/v2/compose/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/composer/v2/compose/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "success"}
	}`, jobId, jobId))

	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "GET", fmt.Sprintf("/api/composer/v2/compose/%v/metadata", jobId), ``, http.StatusInternalServerError, `
	{
		"href": "/api/composer/v2/errors/1012",
		"id": "1012",
		"kind": "Error",
		"code": "COMPOSER-1012",
		"reason": "OSBuildJobResult does not have expected fields set"
	}`, "operation_id")

}

func TestComposeStatusFailure(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, wrksrv := newV2Server(t, dir)

	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "POST", "/api/composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_requests":[{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_request": {
				"type": "aws.s3",
				"options": {
					"access_key_id": "somekey",
					"secret_access_key": "somesecretkey",
					"bucket": "somebucket"
				}
			}
		 }]
	}`, test_distro.TestDistroName, test_distro.TestArchName, test_distro.TestImageTypeName), http.StatusCreated, `
	{
		"href": "/api/composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArchName, []string{"osbuild"})
	require.NoError(t, err)
	require.Equal(t, "osbuild", jobType)

	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "GET", fmt.Sprintf("/api/composer/v2/compose/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/composer/v2/compose/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "building"}
	}`, jobId, jobId))

	err = wrksrv.FinishJob(token, nil)
	require.NoError(t, err)
	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "GET", fmt.Sprintf("/api/composer/v2/compose/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/composer/v2/compose/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "failure"}
	}`, jobId, jobId))
}

func TestComposeCustomizations(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, _ := newV2Server(t, dir)

	test.TestRoute(t, srv.Handler("/api/composer/v2"), false, "POST", "/api/composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"customizations": {
			"subscription": {
				"organization": "2040324",
				"activation_key": "my-secret-key",
				"server_url": "subscription.rhsm.redhat.com",
				"base_url": "http://cdn.redhat.com/",
				"insights": true
			},
			"packages": [ "pkg1", "pkg2" ],
			"users": [{
				"name": "user1",
				"groups": [ "wheel" ],
				"key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAINrGKErMYi+MMUwuHaRAJmRLoIzRf2qD2dD5z0BTx/6x"
			}]
		},
		"image_requests":[{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_request": {
				"type": "aws.s3",
				"options": {
					"access_key_id": "somekey",
					"secret_access_key": "somesecretkey",
					"bucket": "somebucket"
				}
			}
		 }]
	}`, test_distro.TestDistroName, test_distro.TestArchName, test_distro.TestImageTypeName), http.StatusCreated, `
	{
		"href": "/api/composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
}
