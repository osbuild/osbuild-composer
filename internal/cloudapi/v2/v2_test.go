package v2_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/pkg/jobqueue"

	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	distro_mock "github.com/osbuild/osbuild-composer/internal/mocks/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/ostree/mock_ostree_repo"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

func newV2Server(t *testing.T, dir string, depsolveChannels []string, enableJWT bool, failDepsolve bool) (*v2.Server, *worker.Server, jobqueue.JobQueue, context.CancelFunc) {
	q, err := fsjobqueue.New(dir)
	require.NoError(t, err)
	workerServer := worker.NewServer(nil, q, worker.Config{BasePath: "/api/worker/v1", JWTEnabled: enableJWT, TenantProviderFields: []string{"rh-org-id", "account_id"}})

	distros, err := distro_mock.NewDefaultRegistry()
	require.NoError(t, err)
	require.NotNil(t, distros)

	config := v2.ServerConfig{
		JWTEnabled:           enableJWT,
		TenantProviderFields: []string{"rh-org-id", "account_id"},
	}
	v2Server := v2.NewServer(workerServer, distros, config)
	require.NotNil(t, v2Server)
	t.Cleanup(v2Server.Shutdown)

	// start a routine which just completes depsolve jobs
	depsolveContext, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, token, _, _, _, err := workerServer.RequestJob(depsolveContext, test_distro.TestDistroName, []string{worker.JobTypeDepsolve}, depsolveChannels)
			select {
			case <-depsolveContext.Done():
				return
			default:
			}
			if err != nil {
				continue
			}
			dJR := &worker.DepsolveJobResult{
				PackageSpecs: map[string][]rpmmd.PackageSpec{"build": []rpmmd.PackageSpec{rpmmd.PackageSpec{Name: "pkg1"}}},
				Error:        "",
				ErrorType:    worker.ErrorType(""),
			}

			if failDepsolve {
				dJR.JobResult.JobError = clienterrors.WorkerClientError(clienterrors.ErrorDNFOtherError, "DNF Error", nil)
			}

			rawMsg, err := json.Marshal(dJR)
			require.NoError(t, err)
			err = workerServer.FinishJob(token, rawMsg)
			if err != nil {
				return
			}

		}
	}()

	cancelWithWait := func() {
		cancel()
		wg.Wait()
	}

	return v2Server, workerServer, q, cancelWithWait
}

func TestUnknownRoute(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, false)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/badroute", ``, http.StatusNotFound, `
	{
		"href": "/api/image-builder-composer/v2/errors/21",
		"id": "21",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-21",
		"reason": "Requested resource doesn't exist"
	}`, "operation_id", "details")
}

func TestGetError(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, false)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/errors/4", ``, http.StatusOK, `
	{
		"href": "/api/image-builder-composer/v2/errors/4",
		"id": "4",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-4",
		"reason": "Unsupported distribution"
	}`, "operation_id", "details")

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/errors/3000", ``, http.StatusNotFound, `
	{
		"href": "/api/image-builder-composer/v2/errors/17",
		"id": "17",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-17",
		"reason": "Error with given id not found"
	}`, "operation_id", "details")
}

func TestGetErrorList(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, false)
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
	}`, "operation_id", "total", "details")
}

func TestCompose(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, false)
	defer cancel()

	// create two ostree repos, one to serve the default test_distro ref (for fallback tests) and one to serve a custom ref
	ostreeRepoDefault := mock_ostree_repo.Setup(test_distro.New().OSTreeRef())
	defer ostreeRepoDefault.TearDown()
	ostreeRepoOther := mock_ostree_repo.Setup("some/other/ref")
	defer ostreeRepoOther.TearDown()

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
		"href": "/api/image-builder-composer/v2/errors/30",
		"id": "30",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-30",
		"reason": "Request could not be validated"
	}`, "operation_id", "details")

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
	}`, "operation_id", "details")

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
		"href": "/api/image-builder-composer/v2/errors/30",
		"id": "30",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-30",
		"reason": "Request could not be validated"
	}`, "operation_id", "details")

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
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"koji":{
			"name": "name",
			"version": "version",
			"release": "release",
			"server": "https://koji.example.com",
			"task_id": 42
		},
		"image_request":{
			"architecture": "%s",
			"image_type": "aws",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}]
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	// ostree parameters (success)

	// ref only
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "edge-commit",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			},
			"ostree": {
				"ref": "rhel/10/x86_64/edge"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	// url only (must use ostreeRepoDefault for default ref)
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "edge-commit",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			},
			"ostree": {
				"url": "%s"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, ostreeRepoDefault.Server.URL), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	// ref + url
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "edge-commit",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			},
			"ostree": {
				"ref": "%s",
				"url": "%s"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, ostreeRepoDefault.OSTreeRef, ostreeRepoDefault.Server.URL), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	// parent + url
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "edge-commit",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			},
			"ostree": {
				"parent": "%s",
				"url": "%s"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, ostreeRepoDefault.OSTreeRef, ostreeRepoDefault.Server.URL), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	// ref + parent + url
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "edge-commit",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			},
			"ostree": {
				"parent": "%s",
				"url": "%s",
				"ref": "a/new/ref"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, ostreeRepoOther.OSTreeRef, ostreeRepoOther.Server.URL), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	// ostree errors

	// bad url
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "edge-commit",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			},
			"ostree": {
				"ref": "rhel/10/x86_64/edge",
				"url": "not-a-URL"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name), http.StatusBadRequest, `
	{
		"href": "/api/image-builder-composer/v2/errors/10",
		"id": "10",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-10",
		"reason": "Error resolving OSTree repo"
	}`, "operation_id", "details")

	// bad ref
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "edge-commit",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			},
			"ostree": {
				"ref": "/bad/ref"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name), http.StatusBadRequest, `
	{
		"href": "/api/image-builder-composer/v2/errors/9",
		"id": "9",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-9",
		"reason": "Invalid OSTree ref"
	}`, "operation_id", "details")

	// bad parent ref
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "edge-commit",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			},
			"ostree": {
				"ref": "%s",
				"url": "%s",
				"parent": "/bad/ref/number/2"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, ostreeRepoDefault.OSTreeRef, ostreeRepoDefault.Server.URL), http.StatusBadRequest, `
	{
		"href": "/api/image-builder-composer/v2/errors/9",
		"id": "9",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-9",
		"reason": "Invalid OSTree ref"
	}`, "operation_id", "details")

	// incorrect ref for URL
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "edge-commit",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			},
			"ostree": {
				"url": "%s",
				"parent": "incorrect/ref"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, ostreeRepoOther.Server.URL), http.StatusBadRequest, `
	{
		"href": "/api/image-builder-composer/v2/errors/10",
		"id": "10",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-10",
		"reason": "Error resolving OSTree repo"
	}`, "operation_id", "details")

	// parent ref without URL
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_request":{
			"architecture": "%s",
			"image_type": "edge-commit",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			},
			"ostree": {
				"parent": "some/ref"
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name), http.StatusBadRequest, `
	{
		"href": "/api/image-builder-composer/v2/errors/27",
		"id": "27",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-27",
		"reason": "Invalid OSTree parameters or parameter combination"
	}`, "operation_id", "details")
}

func TestComposeStatusSuccess(t *testing.T) {
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, false)
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

	jobId, token, jobType, args, dynArgs, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""})
	require.NoError(t, err)
	require.Equal(t, worker.JobTypeOSBuild, jobType)

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
		"image_status": {"status": "building"},
		"status": "pending"
	}`, jobId, jobId))

	res, err := json.Marshal(&worker.OSBuildJobResult{
		Success:       true,
		OSBuildOutput: &osbuild.Result{Success: true},
	})
	require.NoError(t, err)

	err = wrksrv.FinishJob(token, res)
	require.NoError(t, err)
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "success"},
		"status": "success"
	}`, jobId, jobId))

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/metadata", jobId), ``, http.StatusInternalServerError, `
	{
		"href": "/api/image-builder-composer/v2/errors/1012",
		"id": "1012",
		"kind": "Error",
		"code": "IMAGE-BUILDER-COMPOSER-1012",
		"reason": "OSBuildJobResult does not have expected fields set"
	}`, "operation_id", "details")

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/logs", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v/logs",
		"id": "%v",
		"kind": "ComposeLogs",
		"image_builds": [
			{
				"arch": "",
				"host_os": "",
				"osbuild_output": {
					"log": null,
					"metadata": null,
					"success": true,
					"type": ""
				},
				"pipeline_names": {
					"build": [
						"build"
					],
					"payload": [
						"os",
						"assembler"
					]
				},
				"success": true,
				"upload_status": ""
			}
		]
	}`, jobId, jobId))

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/manifests", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v/manifests",
		"id": "%v",
		"kind": "ComposeManifests",
		"manifests": [
			{
				"version": "",
				"pipelines": [],
				"sources": {}
			}
		]
	}`, jobId, jobId), "details")
}

func TestComposeStatusFailure(t *testing.T) {
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, false)
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

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""})
	require.NoError(t, err)
	require.Equal(t, worker.JobTypeOSBuild, jobType)

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "building"},
		"status": "pending"
	}`, jobId, jobId))

	err = wrksrv.FinishJob(token, nil)
	require.NoError(t, err)
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {
			"error": {
				"id": 10,
				"reason": "osbuild build failed"
			},
			"status": "failure"
		},
		"status": "failure"
	}`, jobId, jobId))
}

func TestComposeStatusInvalidUUID(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, false)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/composes/abcdef", ``, http.StatusBadRequest, `
{
	"code": "IMAGE-BUILDER-COMPOSER-14",
	"details": "",
	"href": "/api/image-builder-composer/v2/errors/14",
	"id": "14",
	"kind": "Error",
	"reason": "Invalid format for compose id"
}
`, "operation_id")
}

func TestComposeJobError(t *testing.T) {
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, false)
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

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""})
	require.NoError(t, err)
	require.Equal(t, worker.JobTypeOSBuild, jobType)

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "building"},
		"status": "pending"
	}`, jobId, jobId))

	jobErr := worker.JobResult{
		JobError: clienterrors.WorkerClientError(clienterrors.ErrorBuildJob, "Error building image", nil),
	}
	jobResult, err := json.Marshal(worker.OSBuildJobResult{JobResult: jobErr})
	require.NoError(t, err)

	err = wrksrv.FinishJob(token, jobResult)
	require.NoError(t, err)
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {
			"error": {
				"id": 10,
				"reason": "Error building image"
			},
			"status": "failure"
		},
		"status": "failure"
	}`, jobId, jobId))
}

func TestComposeDependencyError(t *testing.T) {
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, true)
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

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""})
	require.NoError(t, err)
	require.Equal(t, worker.JobTypeOSBuild, jobType)

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "building"},
		"status": "pending"
	}`, jobId, jobId))

	jobErr := worker.JobResult{
		JobError: clienterrors.WorkerClientError(clienterrors.ErrorManifestDependency, "Manifest dependency failed", nil),
	}
	jobResult, err := json.Marshal(worker.OSBuildJobResult{JobResult: jobErr})
	require.NoError(t, err)

	err = wrksrv.FinishJob(token, jobResult)
	require.NoError(t, err)
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {
			"error": {
				"details": [{
					"id": 5,
					"reason": "Error in depsolve job dependency",
					"details": [{
						"id": 22,
						"reason": "DNF Error"
					}]
				}],
				"id": 9,
				"reason": "Manifest dependency failed"
			},
			"status": "failure"
		},
		"status": "failure"
	}`, jobId, jobId))
}

func TestComposeTargetErrors(t *testing.T) {
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, false)
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

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""})
	require.NoError(t, err)
	require.Equal(t, worker.JobTypeOSBuild, jobType)

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {"status": "building"},
		"status": "pending"
	}`, jobId, jobId))

	oJR := worker.OSBuildJobResult{
		TargetResults: []*target.TargetResult{
			&target.TargetResult{
				Name:        "org.osbuild.aws",
				Options:     target.AWSTargetResultOptions{Ami: "", Region: ""},
				TargetError: clienterrors.WorkerClientError(clienterrors.ErrorImportingImage, "error importing image", nil),
			},
		},
	}
	jobErr := worker.JobResult{
		JobError: clienterrors.WorkerClientError(clienterrors.ErrorTargetError, "at least one target failed", oJR.TargetErrors()),
	}
	oJR.JobResult = jobErr
	jobResult, err := json.Marshal(oJR)
	require.NoError(t, err)

	err = wrksrv.FinishJob(token, jobResult)
	require.NoError(t, err)
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"image_status": {
			"error": {
				"details": [{
					"id": 12,
					"reason": "error importing image",
					"details": "org.osbuild.aws"
				}],
				"id": 28,
				"reason": "at least one target failed"
			},
			"status": "failure",
			"upload_status": {
				"options": {
					"ami": "",
					"region": ""
				},
				"status": "",
				"type": "aws"
			}
		},
		"status": "failure"
	}`, jobId, jobId))
}

func TestComposeCustomizations(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, false)
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
			}],
			"services": {
				"enabled": [
					"nftables"
				],
				"disabled": [
					"firewalld"
				]
			}
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
	srv, _, _, cancel := newV2Server(t, t.TempDir(), []string{""}, false, false)
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
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypesAws)), http.StatusCreated, `
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
				"region": "eu-central-1",
				"snapshot_name": "name",
				"share_with_accounts": ["123456789012","234567890123"]
			}
		 }
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypesAws)), http.StatusCreated, `
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
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypesEdgeCommit)), http.StatusCreated, `
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
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypesEdgeInstaller)), http.StatusCreated, `
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
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypesAzure)), http.StatusCreated, `
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
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypesGcp)), http.StatusCreated, `
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
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypesImageInstaller)), http.StatusCreated, `
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
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypesGuestImage)), http.StatusCreated, `
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
	}`, test_distro.TestDistroName, test_distro.TestArch3Name, string(v2.ImageTypesVsphere)), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
}
