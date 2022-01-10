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
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

func newV2Server(t *testing.T, dir string) (*v2.Server, *worker.Server, context.CancelFunc) {
	rpmFixture := rpmmd_mock.BaseFixture(dir)
	rpm := rpmmd_mock.NewRPMMDMock(rpmFixture)
	require.NotNil(t, rpm)

	distros, err := distro_mock.NewDefaultRegistry()
	require.NoError(t, err)
	require.NotNil(t, distros)

	v2Server := v2.NewServer(rpmFixture.Workers, rpm, distros, "image-builder.service")
	require.NotNil(t, v2Server)

	// start a routine which just completes depsolve jobs
	depsolveContext, cancel := context.WithCancel(context.Background())
	go func() {
		for {
			_, token, _, _, _, err := rpmFixture.Workers.RequestJob(context.Background(), test_distro.TestDistroName, []string{"depsolve"})
			if err != nil {
				continue
			}
			rawMsg, err := json.Marshal(&worker.DepsolveJobResult{PackageSpecs: map[string][]rpmmd.PackageSpec{"build": []rpmmd.PackageSpec{rpmmd.PackageSpec{Name: "pkg1"}}}, Error: "", ErrorType: worker.ErrorType("")})
			require.NoError(t, err)
			err = rpmFixture.Workers.FinishJob(token, rawMsg)
			if err != nil {
				return
			}

			select {
			case <-depsolveContext.Done():
				return
			default:
				continue
			}
		}
	}()

	return v2Server, rpmFixture.Workers, cancel
}

func TestUnknownRoute(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, _, cancel := newV2Server(t, dir)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/badroute", ``, http.StatusNotFound, `
	{
		"href": "/api/image-builder-composer/v2/errors/21",
		"id": "21",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-21",
		"reason": "Requested resource doesn't exist"
	}`, "operation_id")
}

func TestGetError(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, _, cancel := newV2Server(t, dir)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/errors/4", ``, http.StatusOK, `
	{
		"href": "/api/image-builder-composer/v2/errors/4",
		"id": "4",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-4",
		"reason": "Unsupported distribution"
	}`, "operation_id")

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/errors/3000", ``, http.StatusNotFound, `
	{
		"href": "/api/image-builder-composer/v2/errors/17",
		"id": "17",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-17",
		"reason": "Error with given id not found"
	}`, "operation_id")
}

func TestGetErrorList(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, _, cancel := newV2Server(t, dir)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/errors?page=3&size=1", ``, http.StatusOK, `
	{
		"kind": "ErrorList",
		"page": 3,
		"size": 1,
		"items": [{
			"href": "/api/image-builder-composer/v2/errors/4",
			"id": "4",
			"kind": "Error",
			"code": "IMAGE-BUILDER-COMPOSER-4",
			"reason": "Unsupported distribution"
		 }]
	}`, "operation_id", "total")
}

func TestCompose(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, _, cancel := newV2Server(t, dir)
	defer cancel()

	// unsupported distribution
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "unsupported_distro",
		"image_request":{
			"architecture": "%s",
			"image_type": "aws.ec2",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1",
				"share_with_accounts": ["123456789012"]
			}
		 }
	}`, test_distro.TestArch3Name), http.StatusBadRequest, `
	{
		"href": "/api/image-builder-composer/v2/errors/4",
		"id": "4",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-4",
		"reason": "Unsupported distribution"
	}`, "operation_id")

	// unsupported architecture
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "unsupported_arch",
			"image_type": "aws",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName), http.StatusBadRequest, `
	{
		"href": "/api/image-builder-composer/v2/errors/5",
		"id": "5",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-5",
		"reason": "Unsupported architecture"
	}`, "operation_id")

	// unsupported imagetype
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "unsupported_image_type",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name), http.StatusBadRequest, `
	{
		"href": "/api/image-builder-composer/v2/errors/6",
		"id": "6",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-6",
		"reason": "Unsupported image type"
	}`, "operation_id")

	// Returns 404, but should be 405; see https://github.com/labstack/echo/issues/1981
	// test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	// {
	//	"distribution": "%s",
	//	"image_requests":[{
	//		"architecture": "%s",
	//		"image_type": "%s",
	//		"repositories": [{
	//			"baseurl": "somerepo.org",
	//			"rhsm": false
	//		}],
	//		"upload_options": {
	//			"region": "eu-central-1"
	//		}
	//          }]
	// }`, test_distro.TestDistroName, test_distro.TestArch3Name, test_distro.TestImageTypeName), http.StatusMethodNotAllowed, `
	// {
	//	"href": "/api/image-builder-composer/v2/errors/22",
	//	"id": "22",
	//	"kind": "Error",
	//	"code": "IMAGE-BUILDER-COMPOSER-22",
	//	"reason": "Requested method isn't supported for resource"
	// }`, "operation_id")

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "aws",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
}

func TestComposeStatusSuccess(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, wrksrv, cancel := newV2Server(t, dir)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "aws",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, token, jobType, args, dynArgs, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{"osbuild"})
	require.NoError(t, err)
	require.Equal(t, "osbuild", jobType)

	var osbuildJob worker.OSBuildJob
	err = json.Unmarshal(args, &osbuildJob)
	require.NoError(t, err)
	require.Equal(t, 0, len(osbuildJob.Manifest))
	require.NotEqual(t, 0, len(dynArgs[0]))

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "building"}
	}`, jobId, jobId))

	res, err := json.Marshal(&worker.OSBuildJobResult{
		Success: true,
	})
	require.NoError(t, err)

	err = wrksrv.FinishJob(token, res)
	require.NoError(t, err)
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "success"}
	}`, jobId, jobId))

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/metadata", jobId), ``, http.StatusInternalServerError, `
	{
		"href": "/api/image-builder-composer/v2/errors/1012",
		"id": "1012",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-1012",
		"reason": "OSBuildJobResult does not have expected fields set"
	}`, "operation_id")
}

func TestComposeStatusFailure(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, wrksrv, cancel := newV2Server(t, dir)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "aws",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{"osbuild"})
	require.NoError(t, err)
	require.Equal(t, "osbuild", jobType)

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "building"}
	}`, jobId, jobId))

	err = wrksrv.FinishJob(token, nil)
	require.NoError(t, err)
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "failure"}
	}`, jobId, jobId))
}

func TestComposeLegacyError(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, wrksrv, cancel := newV2Server(t, dir)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "aws",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{"osbuild"})
	require.NoError(t, err)
	require.Equal(t, "osbuild", jobType)

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "building"}
	}`, jobId, jobId))

	jobResult, err := json.Marshal(worker.OSBuildJobResult{TargetErrors: []string{"Osbuild failed"}})
	require.NoError(t, err)

	err = wrksrv.FinishJob(token, jobResult)
	require.NoError(t, err)
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "failure"}
	}`, jobId, jobId))
}

func TestComposeJobError(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, wrksrv, cancel := newV2Server(t, dir)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "aws",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{"osbuild"})
	require.NoError(t, err)
	require.Equal(t, "osbuild", jobType)

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "building"}
	}`, jobId, jobId))

	jobErr := clienterrors.WorkerClientError(clienterrors.ErrorBuildJob, "Error building image")
	jobResult, err := json.Marshal(worker.OSBuildJobResult{JobError: jobErr})
	require.NoError(t, err)

	err = wrksrv.FinishJob(token, jobResult)
	require.NoError(t, err)
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "failure"}
	}`, jobId, jobId))
}

func TestComposeCustomizations(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, _, cancel := newV2Server(t, dir)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
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
			}],
			"payload_repositories": [{
				"baseurl": "some-custom-repo.org",
				"check_gpg": false,
				"ignore_ssl": false,
				"gpg_key": "some-gpg-key"
			}]
		},
		"image_request":{
			"architecture": "%s",
			"image_type": "aws",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
}

func TestImageTypes(t *testing.T) {
	dir, err := ioutil.TempDir("", "osbuild-composer-test-api-v2-")
	require.NoError(t, err)
	defer os.RemoveAll(dir)
	srv, _, cancel := newV2Server(t, dir)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1",
				"snapshot_name": "name",
				"share_with_accounts": ["123456789012","234567890123"]
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypes_aws)), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypes_aws)), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypes_edge_commit)), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypes_edge_installer)), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"subscription_id": "4e5d8b2c-ab24-4413-90c5-612306e809e2",
				"tenant_id": "5c7ef5b6-1c3f-4da0-a622-0b060239d7d7",
				"resource_group": "ToucanResourceGroup",
				"location": "westeurope"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypes_azure)), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu",
				"bucket": "some-eu-bucket",
				"share_with_accounts": ["user:alice@example.com"]
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypes_gcp)), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypes_image_installer)), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypes_guest_image)), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "%s",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypes_vsphere)), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
}
