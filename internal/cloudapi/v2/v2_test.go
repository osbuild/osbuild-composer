package v2_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/pkg/jobqueue"

	"github.com/osbuild/images/pkg/distro/test_distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree/mock_ostree_repo"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
	"github.com/osbuild/osbuild-composer/internal/jobqueue/fsjobqueue"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

var sbomDoc = json.RawMessage(`{
  "SPDXID": "SPDXRef-DOCUMENT",
  "creationInfo": {
    "created": "2024-09-06T08:12:28Z",
    "creators": [
      "Tool: osbuild-128"
    ]
  },
  "dataLicense": "CC0-1.0",
  "name": "sbom-by-osbuild-128",
  "spdxVersion": "SPDX-2.3",
  "documentNamespace": "https://osbuild.org/spdxdocs/sbom-by-osbuild-128-6bb434da-523a-463f-bc4c-69460af00040",
  "packages": [
    {
      "SPDXID": "SPDXRef-356f4620-c03e-3830-b013-ddbfee6665aa",
      "name": "pkg1",
      "downloadLocation": "http://example.org/pkg1",
      "filesAnalyzed": false,
      "versionInfo": "1",
      "checksums": [
        {
          "algorithm": "SHA256",
          "checksumValue": "e50ddb78a37f5851d1a5c37a4c77d59123153c156e628e064b9daa378f45a2fe"
        }
      ],
      "sourceInfo": "pkg1.src",
      "licenseDeclared": "MIT",
      "summary": "pkg1 summary",
      "description": "pkg1 description",
      "builtDate": "2024-01-23T00:11:15Z"
    }
  ],
  "relationships": [
    {
      "spdxElementId": "SPDXRef-DOCUMENT",
      "relationshipType": "DESCRIBES",
      "relatedSpdxElement": "SPDXRef-356f4620-c03e-3830-b013-ddbfee6665aa"
    }
  ]
}`)

func newV2Server(t *testing.T, dir string, depsolveChannels []string, enableJWT bool, failDepsolve bool) (*v2.Server, *worker.Server, jobqueue.JobQueue, context.CancelFunc) {
	q, err := fsjobqueue.New(dir)
	require.NoError(t, err)
	workerServer := worker.NewServer(nil, q, worker.Config{BasePath: "/api/worker/v1", JWTEnabled: enableJWT, TenantProviderFields: []string{"rh-org-id", "account_id"}})

	distros := distrofactory.NewTestDefault()
	require.NotNil(t, distros)

	repos, err := reporegistry.New([]string{"../../../test/data"})
	require.Nil(t, err)
	require.NotNil(t, repos)

	solver := dnfjson.NewBaseSolver("") // test solver doesn't need a cache dir
	require.NotNil(t, solver)

	config := v2.ServerConfig{
		JWTEnabled:           enableJWT,
		TenantProviderFields: []string{"rh-org-id", "account_id"},
	}
	v2Server := v2.NewServer(workerServer, distros, repos, solver, config)
	require.NotNil(t, v2Server)
	t.Cleanup(v2Server.Shutdown)

	// start a routine which just completes depsolve jobs
	depsolveContext, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, token, _, _, _, err := workerServer.RequestJob(depsolveContext, test_distro.TestArchName, []string{worker.JobTypeDepsolve}, depsolveChannels, uuid.Nil)
			select {
			case <-depsolveContext.Done():
				return
			default:
			}
			if err != nil {
				continue
			}
			dJR := &worker.DepsolveJobResult{
				PackageSpecs: map[string][]rpmmd.PackageSpec{
					"build": {
						{
							Name:     "pkg1",
							Checksum: "sha256:e50ddb78a37f5851d1a5c37a4c77d59123153c156e628e064b9daa378f45a2fe",
						},
					},
					"os": {
						{
							Name:     "pkg1",
							Checksum: "sha256:e50ddb78a37f5851d1a5c37a4c77d59123153c156e628e064b9daa378f45a2fe",
						},
					},
				},
				SbomDocs: map[string]worker.SbomDoc{
					"build": {
						DocType:  sbom.StandardTypeSpdx,
						Document: sbomDoc,
					},
					"os": {
						DocType:  sbom.StandardTypeSpdx,
						Document: sbomDoc,
					},
				},
				Error:     "",
				ErrorType: worker.ErrorType(""),
			}

			if failDepsolve {
				dJR.JobResult.JobError = clienterrors.New(clienterrors.ErrorDNFOtherError, "DNF Error", nil)
			}

			rawMsg, err := json.Marshal(dJR)
			require.NoError(t, err)
			err = workerServer.FinishJob(token, rawMsg)
			if err != nil {
				return
			}

		}
	}()

	ostreeResolveContext, cancelOstree := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, token, _, _, _, err := workerServer.RequestJob(ostreeResolveContext, test_distro.TestDistro1Name, []string{worker.JobTypeOSTreeResolve}, depsolveChannels, uuid.Nil)
			select {
			case <-ostreeResolveContext.Done():
				return
			default:
			}

			if err != nil {
				continue
			}
			oJR := &worker.OSTreeResolveJobResult{
				Specs: []worker.OSTreeResolveResultSpec{
					{
						URL:      "",
						Ref:      "",
						Checksum: "",
					},
				},
			}

			if failDepsolve {
				oJR.JobResult.JobError = clienterrors.New(clienterrors.ErrorOSTreeParamsInvalid, "ostree error", nil)
			}

			rawMsg, err := json.Marshal(oJR)
			require.NoError(t, err)
			err = workerServer.FinishJob(token, rawMsg)
			if err != nil {
				return
			}
		}
	}()

	cancelWithWait := func() {
		cancel()
		cancelOstree()
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

	testDistro := test_distro.DistroFactory(test_distro.TestDistro1Name)
	require.NotNil(t, testDistro)
	// create two ostree repos, one to serve the default test_distro ref (for fallback tests) and one to serve a custom ref
	ostreeRepoDefault := mock_ostree_repo.Setup(testDistro.OSTreeRef())
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
	}`, testDistro.Name()), http.StatusBadRequest, `
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
	}`, testDistro.Name(), test_distro.TestArch3Name), http.StatusBadRequest, `
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
	// }`, testDistro.Name(), test_distro.TestArch3Name, test_distro.TestImageTypeName), http.StatusMethodNotAllowed, `
	// {
	//	"href": "/api/image-builder-composer/v2/errors/22",
	//	"id": "22",
	//	"kind": "Error",
	//	"code": "IMAGE-BUILDER-COMPOSER-22",
	//	"reason": "Requested method isn't supported for resource"
	// }`, "operation_id")

	// With upload options for default target
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
	}`, testDistro.Name(), test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	// With upload options for specific upload target
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
			"upload_targets": [{
				"type": "aws",
				"upload_options": {
					"region": "eu-central-1"
				}
			}]
		}
	}`, testDistro.Name(), test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	// With both upload options for default target and a specific target
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
			"upload_targets": [{
				"type": "aws",
				"upload_options": {
					"region": "eu-central-1"
				}
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		}
	}`, testDistro.Name(), test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	// Koji
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
	}`, testDistro.Name(), test_distro.TestArch3Name), http.StatusCreated, `
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
	}`, testDistro.Name(), test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	// ref only with secondary pulp upload target
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
			"upload_targets": [{
				"type": "pulp.ostree",
				"upload_options": {
					"basepath": "edge/rhel10"
				}
			}],
			"upload_options": {
				"region": "eu-central-1"
			},
			"ostree": {
				"ref": "rhel/10/x86_64/edge"
			}
		}
	}`, testDistro.Name(), test_distro.TestArch3Name), http.StatusCreated, `
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
	}`, testDistro.Name(), test_distro.TestArch3Name, ostreeRepoDefault.Server.URL), http.StatusCreated, `
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
	}`, testDistro.Name(), test_distro.TestArch3Name, ostreeRepoDefault.OSTreeRef, ostreeRepoDefault.Server.URL), http.StatusCreated, `
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
	}`, testDistro.Name(), test_distro.TestArch3Name, ostreeRepoDefault.OSTreeRef, ostreeRepoDefault.Server.URL), http.StatusCreated, `
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
	}`, testDistro.Name(), test_distro.TestArch3Name, ostreeRepoOther.OSTreeRef, ostreeRepoOther.Server.URL), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	// ref + parent + url + contenturl + rhsm
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
				"ref": "a/new/ref",
				"contenturl": "%s",
				"rhsm": true
			}
		}
	}`, testDistro.Name(), test_distro.TestArch3Name, ostreeRepoOther.OSTreeRef, ostreeRepoOther.Server.URL, fmt.Sprintf("%s/content", ostreeRepoOther.Server.URL)), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, token, jobType, args, dynArgs, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
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

	emptyManifest := `{"version":"2","pipelines":[{"name":"build"},{"name":"os"}],"sources":{"org.osbuild.curl":{"items":{"sha256:e50ddb78a37f5851d1a5c37a4c77d59123153c156e628e064b9daa378f45a2fe":{"url":""}}}}}`
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/manifests", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v/manifests",
		"id": "%v",
		"kind": "ComposeManifests",
		"manifests": [
			%s
		]
	}`, jobId, jobId, emptyManifest), "details")

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/sboms", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v/sboms",
		"id": "%v",
		"kind": "ComposeSBOMs",
		"items": [
			[
				{
					"pipeline_name": "build",
					"pipeline_purpose": "buildroot",
					"sbom": %[3]s,
					"sbom_type": %[4]q
				},
				{
					"pipeline_name": "os",
					"pipeline_purpose": "image",
					"sbom": %[3]s,
					"sbom_type": %[4]q
				}
			]
		]
	}`, jobId, jobId, sbomDoc, v2.ImageSBOMSbomTypeSpdx), "details")
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
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
		JobError: clienterrors.New(clienterrors.ErrorBuildJob, "Error building image", nil),
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
			"image_type": "edge-commit",
                        "ostree": {
                                "url": "somerepo.org",
                                "ref": "test"
                        },
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_options": {
				"region": "eu-central-1"
			}
		}
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
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
		JobError: clienterrors.New(clienterrors.ErrorManifestDependency, "Manifest dependency failed", nil),
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
					},
                                        {
                                                "id": 34,
                                                "reason": "ostree error"
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
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
			{
				Name:        "org.osbuild.aws",
				Options:     target.AWSTargetResultOptions{Ami: "", Region: ""},
				TargetError: clienterrors.New(clienterrors.ErrorImportingImage, "error importing image", nil),
			},
		},
	}
	jobErr := worker.JobResult{
		JobError: clienterrors.New(clienterrors.ErrorTargetError, "at least one target failed", oJR.TargetErrors()),
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
				"status": "failure",
				"type": "aws"
			},
			"upload_statuses": [{
				"options": {
					"ami": "",
					"region": ""
				},
				"status": "failure",
				"type": "aws"
			}]
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
			"custom_repositories": [{
				"name": "hello",
				"id": "hello",
				"baseurl": [ "http://hello.com" ],
				"gpg_key": [ "somekey" ],
				"check_gpg": true,
				"enabled": true
			}],
			"services": {
				"enabled": [
					"nftables"
				],
				"disabled": [
					"firewalld"
				]
			},
			"directories": [
				{
					"path": "/etc/my/dir",
					"mode": "0700"
				},
				{
					"path": "/etc/my/dir1",
					"mode": "0700",
					"user": "user1",
					"group": "user1",
					"ensure_parents": true
				},
				{
					"path": "/etc/my/dir2",
					"mode": "0700",
					"user": 1000,
					"group": 1000,
					"ensure_parents": true
				}
			],
			"files": [
				{
					"path": "/etc/my/dir/file",
					"mode": "0600",
					"data": "Hello world!"
				},
				{
					"path": "/etc/my/dir/file2",
					"mode": "0600",
					"user": "user1",
					"group": "user1",
					"data": "Hello world!"
				},
				{
					"path": "/etc/my/dir/file3",
					"mode": "0600",
					"user": 1000,
					"group": 1000,
					"data": "Hello world!"
				}
			],
			"openscap": {
				"profile_id": "test_profile"
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
}

func TestComposeRhcSubscription(t *testing.T) {
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
				"insights": false,
				"rhc": true
			},
			"packages": [ "pkg1", "pkg2" ]
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name), http.StatusCreated, `
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name, string(v2.ImageTypesAws)), http.StatusCreated, `
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name, string(v2.ImageTypesAws)), http.StatusCreated, `
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name, string(v2.ImageTypesEdgeCommit)), http.StatusCreated, `
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name, string(v2.ImageTypesEdgeInstaller)), http.StatusCreated, `
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name, string(v2.ImageTypesAzure)), http.StatusCreated, `
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name, string(v2.ImageTypesGcp)), http.StatusCreated, `
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name, string(v2.ImageTypesImageInstaller)), http.StatusCreated, `
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name, string(v2.ImageTypesGuestImage)), http.StatusCreated, `
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name, string(v2.ImageTypesVsphere)), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")
}

func TestImageFromCompose(t *testing.T) {
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
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
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

	tr := target.NewAWSTargetResult(&target.AWSTargetResultOptions{
		Ami:    "ami-abc123",
		Region: "eu-central-1",
	}, &target.OsbuildArtifact{
		ExportFilename: "image.raw",
		ExportName:     "image",
	})
	res, err := json.Marshal(&worker.OSBuildJobResult{
		Success:       true,
		OSBuildOutput: &osbuild.Result{Success: true},
		TargetResults: []*target.TargetResult{
			tr,
		},
	})
	require.NoError(t, err)

	err = wrksrv.FinishJob(token, res)
	require.NoError(t, err)
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v",
		"kind": "ComposeStatus",
		"id": "%v",
		"status": "success",
		"image_status": {
			"status": "success",
			"upload_status": {
				"type": "aws",
				"status": "success",
				"options": {
					"ami": "ami-abc123",
					"region": "eu-central-1"
				}
			},
			"upload_statuses": [{
				"type": "aws",
				"status": "success",
				"options": {
					"ami": "ami-abc123",
					"region": "eu-central-1"
				}
			}]
		}
	}`, jobId, jobId))

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST",
		fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/clone", jobId), `
	{
		"region": "eu-central-2",
                "share_with_accounts": ["123456789012"]
	}`, http.StatusCreated, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/composes/%v/clone",
		"kind": "CloneComposeId"
	}`, jobId), "id")

	_, token, jobType, _, _, err = wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeAWSEC2Copy}, []string{""}, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, worker.JobTypeAWSEC2Copy, jobType)

	res, err = json.Marshal(&worker.AWSEC2CopyJobResult{
		Ami:    "ami-def456",
		Region: "eu-central-2",
	})
	require.NoError(t, err)
	err = wrksrv.FinishJob(token, res)
	require.NoError(t, err)

	imgJobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeAWSEC2Share}, []string{""}, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, worker.JobTypeAWSEC2Share, jobType)

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/clones/%v", imgJobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/clones/%v",
		"kind": "CloneComposeStatus",
		"id": "%v",
		"status": "running",
		"type": "aws"
	}`, imgJobId, imgJobId), "options")

	res, err = json.Marshal(&worker.AWSEC2ShareJobResult{
		Ami:    "ami-def456",
		Region: "eu-central-2",
	})
	require.NoError(t, err)
	err = wrksrv.FinishJob(token, res)
	require.NoError(t, err)

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/clones/%v", imgJobId), ``, http.StatusOK, fmt.Sprintf(`
	{
		"href": "/api/image-builder-composer/v2/clones/%v",
		"kind": "CloneComposeStatus",
		"id": "%v",
		"status": "success",
		"type": "aws",
		"options": {
			"ami": "ami-def456",
			"region": "eu-central-2"
		}
	}`, imgJobId, imgJobId))
}
