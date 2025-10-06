package v2_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/pkg/jobqueue"

	"github.com/osbuild/images/pkg/distro/test_distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/manifest"
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

// mockDepsolve starts a routine which just completes depsolve jobs
// It requires some of the test framework to operate
// And the optional fail parameter will cause it to return an error as if the depsolve failed
func mockDepsolve(t *testing.T, workerServer *worker.Server, wg *sync.WaitGroup, fail bool) func() {
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, token, _, _, _, err := workerServer.RequestJob(ctx, test_distro.TestArchName, []string{worker.JobTypeDepsolve}, []string{""}, uuid.Nil)
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err != nil {
				continue
			}
			dummyPackage := rpmmd.PackageSpec{
				Name:           "pkg1",
				Version:        "1.33",
				Release:        "2.fc30",
				Arch:           "x86_64",
				Checksum:       "sha256:e50ddb78a37f5851d1a5c37a4c77d59123153c156e628e064b9daa378f45a2fe",
				RemoteLocation: "https://pkg1.example.com/1.33-2.fc30.x86_64.rpm",
			}
			dJR := &worker.DepsolveJobResult{
				PackageSpecs: map[string][]rpmmd.PackageSpec{
					// Used when depsolving a manifest
					"build": {dummyPackage},
					"os":    {dummyPackage},
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
			}

			if fail {
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
	return cancel
}

// mockOSTreeResolve starts a routine which completes a dummy ostree job
// It requires some of the test framework to operate
// And the optional fail parameter will cause it to return an error as if the ostree job failed
func mockOSTreeResolve(t *testing.T, workerServer *worker.Server, wg *sync.WaitGroup, fail bool) func() {
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, token, _, _, _, err := workerServer.RequestJob(ctx, test_distro.TestDistro1Name, []string{worker.JobTypeOSTreeResolve}, []string{""}, uuid.Nil)
			select {
			case <-ctx.Done():
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

			if fail {
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

	return cancel
}

// mockSearch starts a routine which just completes search jobs
// It requires some of the test framework to operate
// And the optional fail parameter will cause it to return an error as if the search failed
func mockSearch(t *testing.T, workerServer *worker.Server, wg *sync.WaitGroup, fail bool) func() {
	ctx, cancel := context.WithCancel(context.Background())
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, token, _, _, _, err := workerServer.RequestJob(ctx, test_distro.TestArchName, []string{worker.JobTypeSearchPackages}, []string{""}, uuid.Nil)
			select {
			case <-ctx.Done():
				return
			default:
			}
			if err != nil {
				continue
			}
			result := &worker.SearchPackagesJobResult{
				Packages: rpmmd.PackageList{
					{
						Name:        "package1",
						Summary:     "The package you searched for",
						Description: "A verbose paragraph about the package",
						Version:     "1.33",
						Release:     "2.fc42",
						Arch:        "x86_64",
						URL:         "https://example.com/package1",
						License:     "GPLv3",
						BuildTime:   time.Date(1985, time.October, 26, 9, 24, 0, 0, time.UTC),
					},
				},
			}

			// fail returns an empty list of matches
			if fail {
				result.Packages = nil
			}

			rawMsg, err := json.Marshal(result)
			require.NoError(t, err)
			err = workerServer.FinishJob(token, rawMsg)
			if err != nil {
				return
			}

		}
	}()
	return cancel
}

func newV2Server(t *testing.T, dir string, enableJWT bool, fail bool) (*v2.Server, *worker.Server, jobqueue.JobQueue, context.CancelFunc) {
	jobsDir := filepath.Join(dir, "jobs")
	err := os.Mkdir(jobsDir, 0755)
	require.NoError(t, err)
	q, err := fsjobqueue.New(jobsDir)
	require.NoError(t, err)

	artifactsDir := filepath.Join(dir, "artifacts")
	err = os.Mkdir(artifactsDir, 0755)
	require.NoError(t, err)

	workerServer := worker.NewServer(nil, q,
		worker.Config{
			ArtifactsDir:         artifactsDir,
			BasePath:             "/api/worker/v1",
			JWTEnabled:           enableJWT,
			TenantProviderFields: []string{"rh-org-id", "account_id"},
		})

	distros := distrofactory.NewTestDefault()
	require.NotNil(t, distros)

	repos, err := reporegistry.New([]string{"../../../test/data/repositories"}, nil)
	require.Nil(t, err)
	require.NotNil(t, repos)
	require.Greater(t, len(repos.ListDistros()), 0)

	config := v2.ServerConfig{
		JWTEnabled:           enableJWT,
		TenantProviderFields: []string{"rh-org-id", "account_id"},
	}
	v2Server := v2.NewServer(workerServer, distros, repos, config)
	require.NotNil(t, v2Server)
	t.Cleanup(v2Server.Shutdown)

	// Setup the depsolve and ostree resolve job handlers
	// These are mocked functions that return a static set of results for testing
	var wg sync.WaitGroup
	var cancelFuncs []context.CancelFunc

	cancelFuncs = append(cancelFuncs, mockDepsolve(t, workerServer, &wg, fail))
	cancelFuncs = append(cancelFuncs, mockOSTreeResolve(t, workerServer, &wg, fail))
	cancelFuncs = append(cancelFuncs, mockSearch(t, workerServer, &wg, fail))

	cancelWithWait := func() {
		for _, cancel := range cancelFuncs {
			cancel()
		}
		wg.Wait()
	}

	return v2Server, workerServer, q, cancelWithWait
}

func TestUnknownRoute(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
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
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
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
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
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

func TestGetDistributionList(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET",
		"/api/image-builder-composer/v2/distributions", ``, http.StatusOK, `
	{
		"test-distro-1":{
			"test_arch":{
				"test_ostree_type":[
					{"baseurls":["https://rpmrepo.osbuild.org/v2/mirror/public/f40/f40-x86_64-rawhide-20240101"], "check_gpg":true, "name":"test-distro"}
				],
				"test_type":[{"baseurls":["https://rpmrepo.osbuild.org/v2/mirror/public/f40/f40-x86_64-rawhide-20240101"], "check_gpg":true, "name":"test-distro"}]
			},
			"test_arch2":{
				"test_type":[{"baseurls":["https://rpmrepo.osbuild.org/v2/mirror/public/f40/f40-aarch64-rawhide-20240101"], "check_gpg":true, "name":"test-distro"}],
				"test_type2":[{"baseurls":["https://rpmrepo.osbuild.org/v2/mirror/public/f40/f40-aarch64-rawhide-20240101"], "check_gpg":true, "name":"test-distro"}]},
			"test_arch3":{
				"ami":[{"baseurls":["https://rpmrepo.osbuild.org/v2/mirror/public/f40/f40-ppc64le-rawhide-20240101"], "check_gpg":true, "name":"test-distro"}],
				"gce":[{"baseurls":["https://rpmrepo.osbuild.org/v2/mirror/public/f40/f40-ppc64le-rawhide-20240101"], "check_gpg":true, "name":"test-distro"}],
				"image-installer":[{"baseurls":["https://rpmrepo.osbuild.org/v2/mirror/public/f40/f40-ppc64le-rawhide-20240101"], "check_gpg":true, "name":"test-distro"}],
				"qcow2":[{"baseurls":["https://rpmrepo.osbuild.org/v2/mirror/public/f40/f40-ppc64le-rawhide-20240101"], "check_gpg":true, "name":"test-distro"}],
				"rhel-edge-commit":[{"baseurls":["https://rpmrepo.osbuild.org/v2/mirror/public/f40/f40-ppc64le-rawhide-20240101"], "check_gpg":true, "name":"test-distro"}],
				"rhel-edge-installer":[{"baseurls":["https://rpmrepo.osbuild.org/v2/mirror/public/f40/f40-ppc64le-rawhide-20240101"], "check_gpg":true, "name":"test-distro"}],
				"vhd":[{"baseurls":["https://rpmrepo.osbuild.org/v2/mirror/public/f40/f40-ppc64le-rawhide-20240101"], "check_gpg":true, "name":"test-distro"}],
				"vmdk":[{"baseurls":["https://rpmrepo.osbuild.org/v2/mirror/public/f40/f40-ppc64le-rawhide-20240101"], "check_gpg":true, "name":"test-distro"}]
			}
		}
	}`, "gpgkeys", "baseurl")
}

func TestCompose(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
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
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), false, false)
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
		PipelineNames: &worker.PipelineNames{
			Build:   []string{"build"},
			Payload: []string{"os"},
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
						"os"
					]
				},
				"success": true,
				"upload_status": ""
			}
		]
	}`, jobId, jobId))

	emptyManifest := `{"version":"2","pipelines":[{"name":"build"},{"name":"os"}],"sources":{"org.osbuild.curl":{"items":{"sha256:e50ddb78a37f5851d1a5c37a4c77d59123153c156e628e064b9daa378f45a2fe":{"url":"https://pkg1.example.com/1.33-2.fc30.x86_64.rpm"}}}}}`
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
	}`, jobId, jobId, sbomDoc, v2.ImageSBOMSbomType(v2.Spdx)), "details")
}

func TestComposeManifests(t *testing.T) {
	testCases := []struct {
		name          string
		jobResult     worker.ManifestJobByIDResult
		expectedError *clienterrors.Error
	}{
		{
			name: "success",
			jobResult: worker.ManifestJobByIDResult{
				Manifest: manifest.OSBuildManifest([]byte(`{"version":"2","pipelines":[{"name":"build"},{"name":"os"}],"sources":{"org.osbuild.curl":{"items":{"sha256:e50ddb78a37f5851d1a5c37a4c77d59123153c156e628e064b9daa378f45a2fe":{"url":"https://pkg1.example.com/1.33-2.fc30.x86_64.rpm"}}}}}`)),
			}},
		// TODO: this case should actually fail, but it doesn't
		{
			name: "failure",
			jobResult: worker.ManifestJobByIDResult{
				Manifest: manifest.OSBuildManifest([]byte(`null`)),
				JobResult: worker.JobResult{
					JobError: clienterrors.New(clienterrors.ErrorManifestDependency, "Manifest generation test error", "Package XYZ does not have a RemoteLocation"),
				},
			},
			//expectedError: clienterrors.New(clienterrors.ErrorManifestDependency, "Manifest generation test error", "Package XYZ does not have a RemoteLocation"),
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {

			// Override the serialize manifest func to allow simulating various job states
			// This is the only way to do it, because of the way the manifest job is handled.
			serializeManifestFunc := func(ctx context.Context, manifestSource *manifest.Manifest, workers *worker.Server, depsolveJobID, containerResolveJobID, ostreeResolveJobID, manifestJobID uuid.UUID, seed int64) {
				var token uuid.UUID
				var err error
				// wait until job is in a pending state
				for {
					_, token, _, _, _, err = workers.RequestJobById(ctx, "", manifestJobID)
					if errors.Is(err, jobqueue.ErrNotPending) {
						time.Sleep(time.Millisecond * 50)
						select {
						case <-ctx.Done():
							t.Fatalf("Context done")
						default:
							continue
						}
					}
					if err != nil {
						t.Fatalf("Error requesting manifest job: %v", err)
						return
					}
					break
				}

				result, err := json.Marshal(testCase.jobResult)
				require.NoError(t, err)
				err = workers.FinishJob(token, result)
				require.NoError(t, err)
			}
			defer v2.OverrideSerializeManifestFunc(serializeManifestFunc)()

			srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), false, false)
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

			// Handle osbuild job
			jobId, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
			require.NoError(t, err)
			require.Equal(t, worker.JobTypeOSBuild, jobType)

			osbuildJobResult, err := json.Marshal(worker.OSBuildJobResult{
				Success:       true,
				OSBuildOutput: &osbuild.Result{Success: true},
				PipelineNames: &worker.PipelineNames{
					Build:   []string{"build"},
					Payload: []string{"os"},
				},
			})
			require.NoError(t, err)
			err = wrksrv.FinishJob(token, osbuildJobResult)
			require.NoError(t, err)

			// Verify the compose status
			test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId), ``, http.StatusOK, fmt.Sprintf(`
			{
				"href": "/api/image-builder-composer/v2/composes/%v",
				"kind": "ComposeStatus",
				"id": "%v",
				"image_status": {
					"status": "success"
				},
				"status": "success"
			}`, jobId, jobId))

			// Verify the compose manifests
			var expectedManifestsResponse string
			var expectedStatusCode int
			if testCase.expectedError != nil {
				expectedManifestsResponse = fmt.Sprintf(`
				{
					"code": "IMAGE-BUILDER-COMPOSER-11",
					"details": "job \"%s\": %s",
					"href": "/api/image-builder-composer/v2/errors/11",
					"id": "11",
					"kind": "Error",
					"reason": "Failed to get manifest"
				}`, jobId, testCase.expectedError)
				expectedStatusCode = http.StatusBadRequest
			} else {
				expectedManifestsResponse = fmt.Sprintf(`
				{
					"href": "/api/image-builder-composer/v2/composes/%v/manifests",
					"id": "%v",
					"kind": "ComposeManifests",
					"manifests": [%s]
				}`, jobId, jobId, testCase.jobResult.Manifest)
				expectedStatusCode = http.StatusOK
			}
			test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/manifests", jobId), ``, expectedStatusCode, expectedManifestsResponse, "operation_id")
		})
	}
}

func TestComposeStatusFailure(t *testing.T) {
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), false, false)
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
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/composes/abcdef", ``, http.StatusBadRequest, `
{
	"code": "IMAGE-BUILDER-COMPOSER-42",
	"details": "code=400, message=Invalid format for parameter id: error unmarshaling 'abcdef' text as *uuid.UUID: invalid UUID length: 6",
	"href": "/api/image-builder-composer/v2/errors/42",
	"id": "42",
	"kind": "Error",
	"reason": "Invalid request, see details for more information"
}
`, "operation_id")
}

func TestComposeJobError(t *testing.T) {
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), false, false)
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
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), false, true)
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
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), false, false)
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
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
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
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
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
				"rhc": true,
				"proxy": "http://proxy.example.com",
				"template_name": "template-name",
				"template_uuid": "template-uuid",
				"patch_url": "http://patch.example.com"
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
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
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
				"location": "westeurope",
                                "hyper_v_generation": "V2"
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
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), false, false)
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

func TestDepsolveBlueprint(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST",
		"/api/image-builder-composer/v2/depsolve/blueprint", fmt.Sprintf(`
		{
			"blueprint": {
				"name": "deptest1",
				"version": "0.0.1",
				"distro": "%[1]s",
				"enabled_modules": [{ "name": "deps", "stream": "1" }],
				"packages": [
					{ "name": "pkg1", "version": "*" }
			]},
			"distribution": "%[1]s",
			"architecture": "%[2]s"
		}`, test_distro.TestDistro1Name, test_distro.TestArchName),
		http.StatusOK,
		`{
			"packages": [
                {
                    "name": "pkg1",
					"type": "rpm",
                    "version": "1.33",
                    "release": "2.fc30",
                    "arch": "x86_64",
					"checksum": "sha256:e50ddb78a37f5851d1a5c37a4c77d59123153c156e628e064b9daa378f45a2fe"
				}
			]
		}`)
}

func TestDepsolveImageType(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST",
		"/api/image-builder-composer/v2/depsolve/blueprint", fmt.Sprintf(`
		{
			"blueprint": {
				"name": "deptest1",
				"version": "0.0.1",
				"distro": "%[1]s",
				"enabled_modules": [{ "name": "deps", "stream": "1" }],
				"packages": [
					{ "name": "pkg1", "version": "*" }
			]},
			"distribution": "%[1]s",
			"architecture": "%[2]s",
			"image_type": "%[3]s"
		}`, test_distro.TestDistro1Name, test_distro.TestArch3Name, test_distro.TestImageTypeImageInstaller),
		http.StatusOK,
		`{
			"packages": [
                {
                    "name": "pkg1",
					"type": "rpm",
                    "version": "1.33",
                    "release": "2.fc30",
                    "arch": "x86_64",
					"checksum": "sha256:e50ddb78a37f5851d1a5c37a4c77d59123153c156e628e064b9daa378f45a2fe"
				}
			]
		}`)
}

func TestDepsolveImageTypeError(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST",
		"/api/image-builder-composer/v2/depsolve/blueprint", fmt.Sprintf(`
		{
			"blueprint": {
				"name": "deptest1",
				"version": "0.0.1",
				"distro": "%[1]s",
				"enabled_modules": [{ "name": "deps", "stream": "1" }],
				"packages": [
					{ "name": "pkg1", "version": "*" }
			]},
			"distribution": "%[1]s",
			"architecture": "%[2]s",
			"image_type": "bad-image-type"
		}`, test_distro.TestDistro1Name, test_distro.TestArchName),
		http.StatusBadRequest, `
		{
			"href": "/api/image-builder-composer/v2/errors/30",
			"id": "30",
			"kind": "Error",
			"code": "IMAGE-BUILDER-COMPOSER-30",
			"reason":"Request could not be validated"
		}`, "operation_id", "details")
}

func TestDepsolveDistroErrors(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	// matching distros, but not supported
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST",
		"/api/image-builder-composer/v2/depsolve/blueprint", fmt.Sprintf(`
		{
			"blueprint": {
				"name": "deptest1",
				"version": "0.0.1",
				"distro": "bart",
				"packages": [
					{ "name": "dep-package", "version": "*" }
			]},
			"distribution": "bart",
			"architecture": "%[2]s"
		}`, test_distro.TestDistro1Name, test_distro.TestArchName),
		http.StatusBadRequest, `
		{
			"href": "/api/image-builder-composer/v2/errors/4",
			"id": "4",
			"kind": "Error",
			"code": "IMAGE-BUILDER-COMPOSER-4",
			"reason": "Unsupported distribution"
		}`, "operation_id", "details")

	// Mismatched distros
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST",
		"/api/image-builder-composer/v2/depsolve/blueprint", fmt.Sprintf(`
		{
			"blueprint": {
				"name": "deptest1",
				"version": "0.0.1",
				"distro": "bart",
				"packages": [
					{ "name": "dep-package", "version": "*" }
			]},
			"distribution": "%[1]s",
			"architecture": "%[2]s"
		}`, test_distro.TestDistro1Name, test_distro.TestArchName),
		http.StatusBadRequest, `
		{
			"href": "/api/image-builder-composer/v2/errors/40",
			"id": "40",
			"kind": "Error",
			"code": "IMAGE-BUILDER-COMPOSER-40",
			"reason": "Invalid request, Blueprint and Cloud API request Distribution must match"
		}`, "operation_id", "details")

	// Bad distro in request, none in blueprint
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST",
		"/api/image-builder-composer/v2/depsolve/blueprint", fmt.Sprintf(`
		{
			"blueprint": {
				"name": "deptest1",
				"version": "0.0.1",
				"packages": [
					{ "name": "dep-package", "version": "*" }
			]},
			"distribution": "bart",
			"architecture": "%[2]s"
		}`, test_distro.TestDistro1Name, test_distro.TestArchName),
		http.StatusBadRequest, `
		{
			"href": "/api/image-builder-composer/v2/errors/4",
			"id": "4",
			"kind": "Error",
			"code": "IMAGE-BUILDER-COMPOSER-4",
			"reason": "Unsupported distribution"
		}`, "operation_id", "details")
}

func TestDepsolveArchErrors(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	// Unsupported architecture
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST",
		"/api/image-builder-composer/v2/depsolve/blueprint", fmt.Sprintf(`
		{
			"blueprint": {
				"name": "deptest1",
				"version": "0.0.1",
				"packages": [
					{ "name": "dep-package", "version": "*" }
			]},
			"distribution": "%[1]s",
			"architecture": "MOS6502",
		}`, test_distro.TestDistro1Name),
		http.StatusBadRequest, `
		{
			"href": "/api/image-builder-composer/v2/errors/30",
			"id": "30",
			"kind": "Error",
			"code": "IMAGE-BUILDER-COMPOSER-30",
			"reason": "Request could not be validated"
		}`, "operation_id", "details")
}

func TestSearchPackages(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST",
		"/api/image-builder-composer/v2/search/packages", fmt.Sprintf(`
		{
			"packages": ["package1"],
			"distribution": "%[1]s",
			"architecture": "%[2]s"
		}`, test_distro.TestDistro1Name, test_distro.TestArchName),
		http.StatusOK,
		`{
			"packages": [
                {
                    "name": "package1",
					"summary": "The package you searched for",
					"description": "A verbose paragraph about the package",
                    "version": "1.33",
                    "release": "2.fc42",
                    "arch": "x86_64",
					"url": "https://example.com/package1",
					"license": "GPLv3",
					"buildtime": "1985-10-26T09:24:00Z"
				}
			]
		}`)
}

func TestSearchDistroErrors(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	// Bad distro in request
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST",
		"/api/image-builder-composer/v2/search/packages", fmt.Sprintf(`
		{
			"packages": ["package1"],
			"distribution": "bart",
			"architecture": "%[2]s"
		}`, test_distro.TestDistro1Name, test_distro.TestArchName),
		http.StatusBadRequest, `
		{
			"href": "/api/image-builder-composer/v2/errors/4",
			"id": "4",
			"kind": "Error",
			"code": "IMAGE-BUILDER-COMPOSER-4",
			"reason": "Unsupported distribution"
		}`, "operation_id", "details")
}

func TestSearchArchErrors(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	// Unsupported architecture
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST",
		"/api/image-builder-composer/v2/search/packages", fmt.Sprintf(`
		{
			"packages": ["package1"],
			"distribution": "%[1]s",
			"architecture": "MOS6502",
		}`, test_distro.TestDistro1Name),
		http.StatusBadRequest, `
		{
			"href": "/api/image-builder-composer/v2/errors/30",
			"id": "30",
			"kind": "Error",
			"code": "IMAGE-BUILDER-COMPOSER-30",
			"reason": "Request could not be validated"
		}`, "operation_id", "details")
}

func TestComposesRoute(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	// List empty root composes
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/composes/", ``,
		http.StatusOK, `[]`)

	// Make a compose so it has something to list
	reply := test.TestRouteWithReply(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
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

	// Extract the compose ID to use to test the list response
	var composeReply v2.ComposeId
	err := json.Unmarshal(reply, &composeReply)
	require.NoError(t, err)

	// List root composes
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/composes/", ``,
		http.StatusOK, fmt.Sprintf(`[{"href":"/api/image-builder-composer/v2/composes/%[1]s", "id":"%[1]s", "image_status":{"status":"pending"}, "kind":"ComposeStatus", "status":"pending"}]`,
			composeReply.Id.String()))
}

func TestDownload(t *testing.T) {
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_requests": [{
			"architecture": "%s",
			"image_type": "guest-image",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_targets": [{
				"type": "local",
				"upload_options": {}
			}]
		}]
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

	// Mock up a local target result
	tr := target.NewWorkerServerTargetResult(
		&target.WorkerServerTargetResultOptions{},
		&target.OsbuildArtifact{
			ExportFilename: "disk.qcow2",
			ExportName:     "qcow2",
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

	// Write a fake disk.qcow2 file to the artifact directory
	file, err := wrksrv.JobArtifactLocation(jobId, tr.OsbuildArtifact.ExportFilename)
	// Error is expected, file doesn't exist yet
	require.Error(t, err)
	// Yes, the dummy file is json to make TestRoute happy
	err = os.WriteFile(file, []byte("{\"msg\":\"This is the disk.qcow2 you are looking for\"}"), 0600)
	require.NoError(t, err)

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET",
		fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/download", jobId),
		``,
		http.StatusOK,
		`{
			"msg": "This is the disk.qcow2 you are looking for"
		}`)
}

func TestDownloadNotFinished(t *testing.T) {
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_requests": [{
			"architecture": "%s",
			"image_type": "guest-image",
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false
			}],
			"upload_targets": [{
				"type": "local",
				"upload_options": {}
			}]
		}]
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name), http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, _, jobType, args, dynArgs, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
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

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET",
		fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/download", jobId),
		``,
		http.StatusBadRequest,
		fmt.Sprintf(`{
			"href": "/api/image-builder-composer/v2/errors/1022",
			"id": "1022",
			"kind": "Error",
			"code": "IMAGE-BUILDER-COMPOSER-1022",
			"details": "Cannot access artifacts before job is finished: %s",
			"reason": "Artifact not found"
		}`, jobId), "operation_id")
}

func TestDownloadUnknown(t *testing.T) {
	srv, _, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET",
		"/api/image-builder-composer/v2/composes/80977a35-1b27-4604-b998-9cd331f9089e/download",
		``,
		http.StatusNotFound,
		`{
			"href": "/api/image-builder-composer/v2/errors/15",
			"id": "15",
			"kind": "Error",
			"code": "IMAGE-BUILDER-COMPOSER-15",
			"details": "job does not exist",
			"reason":"Compose with given id not found"
		}`, "operation_id")
}

// TestComposeRequestMetadata tests that the original ComposeRequest is included with the
// metadata response.
func TestComposeRequestMetadata(t *testing.T) {
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	request := fmt.Sprintf(`
	{
		"distribution": "%s",
		"image_requests":[{
			"architecture": "%s",
			"image_type": "aws",
			"size": 0,
			"repositories": [{
				"baseurl": "somerepo.org",
				"rhsm": false,
				"check_repo_gpg": false,
				"module_hotfixes": false
			}],
			"upload_options": {
				"region": "eu-central-1",
				"public": false
			}
		}]
	}`, test_distro.TestDistro1Name, test_distro.TestArch3Name)
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", request, http.StatusCreated, `
	{
		"href": "/api/image-builder-composer/v2/compose",
		"kind": "ComposeId"
	}`, "id")

	jobId, _, jobType, args, dynArgs, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
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

	// metadata response should include the ComposeRequest even when build is not done
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/metadata", jobId), ``, http.StatusOK,
		fmt.Sprintf(`{
		"href": "/api/image-builder-composer/v2/composes/%[1]v/metadata",
		"id": "%[1]v",
		"kind": "ComposeMetadata",
		"request": %s
	}`, jobId, request))
}

func TestComposesDeleteRoute(t *testing.T) {
	srv, wrksrv, _, cancel := newV2Server(t, t.TempDir(), false, false)
	defer cancel()

	// Make a compose so it has something to list and delete
	reply := test.TestRouteWithReply(t, srv.Handler("/api/image-builder-composer/v2"), false, "POST", "/api/image-builder-composer/v2/compose", fmt.Sprintf(`
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

	// Extract the compose ID to use to test the list response
	var composeReply v2.ComposeId
	err := json.Unmarshal(reply, &composeReply)
	require.NoError(t, err)
	jobID := composeReply.Id

	_, token, jobType, _, _, err := wrksrv.RequestJob(context.Background(), test_distro.TestArch3Name, []string{worker.JobTypeOSBuild}, []string{""}, uuid.Nil)
	require.NoError(t, err)
	require.Equal(t, worker.JobTypeOSBuild, jobType)
	res, err := json.Marshal(&worker.OSBuildJobResult{
		Success:       true,
		OSBuildOutput: &osbuild.Result{Success: true},
	})
	require.NoError(t, err)
	err = wrksrv.FinishJob(token, res)
	require.NoError(t, err)

	// List root composes
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/composes/", ``,
		http.StatusOK, fmt.Sprintf(`[{"href":"/api/image-builder-composer/v2/composes/%[1]s", "id":"%[1]s", "image_status":{"status":"success"}, "kind":"ComposeStatus", "status":"success"}]`,
			jobID.String()))

	// Delete the compose
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "DELETE", fmt.Sprintf("/api/image-builder-composer/v2/composes/%s", jobID.String()), ``,
		http.StatusOK, fmt.Sprintf(`
	{
		"id": "%s",
		"kind": "ComposeDeleteStatus"
	}`, jobID.String()), "href")

	// List root composes (should now be none)
	test.TestRoute(t, srv.Handler("/api/image-builder-composer/v2"), false, "GET", "/api/image-builder-composer/v2/composes/", ``,
		http.StatusOK, `[]`)
}
