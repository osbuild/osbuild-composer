package v2_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/openshift-online/ocm-sdk-go/authentication"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/pkg/jobqueue"

	"github.com/osbuild/images/pkg/distro/test_distro"
	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/api"
)

func kojiRequest() string {
	return fmt.Sprintf(`
		{
			"distribution":"%[1]s",
			"image_requests": [
				{
					"architecture": "%[2]s",
					"image_type": "%[3]s",
					"repositories": [
						{
							"baseurl": "https://repo.example.com/"
						}
					]
				}
			],
			"koji": {
				"server": "koji.example.com",
				"task_id": 1,
				"name":"foo",
				"version":"1",
				"release":"2"
			}
		}`, test_distro.TestDistro1Name, test_distro.TestArch3Name, string(v2.ImageTypesGuestImage))
}

func s3Request() string {
	return fmt.Sprintf(`
		{
			"distribution":"%[1]s",
			"image_requests": [
				{
					"architecture": "%[2]s",
					"image_type": "%[3]s",
					"repositories": [
						{
							"baseurl": "https://repo.example.com/"
						}
					],
					"upload_options": {
						"region": "us-east-1"
					}
				}
			]
		}`, test_distro.TestDistro1Name, test_distro.TestArch3Name, string(v2.ImageTypesGuestImage))
}

func bootcRequest() string {
	return fmt.Sprintf(`
		{
			"bootc": {
				"reference": "registry.org/centos-bootc:tag"
			},
			"image_request": {
				"architecture": "%s",
				"repositories": [],
				"image_type": "guest-image",
				"upload_options": {}
			}
		}`, test_distro.TestArch3Name)
}

var reqContextCallCount = 0

func reqContext(orgID string) context.Context {
	// Alternate between rh-org-id and account_id so we verify that the APIs understand
	// both fields.
	tenantFields := []string{"rh-org-id", "account_id"}
	tenantField := tenantFields[reqContextCallCount%2]
	reqContextCallCount++
	return authentication.ContextWithToken(context.Background(), &jwt.Token{
		Claims: jwt.MapClaims{
			tenantField: orgID,
		},
	})
}

func scheduleRequest(t *testing.T, handler http.Handler, orgID, request string) uuid.UUID {
	result := test.APICall{
		Handler:        handler,
		Context:        reqContext(orgID),
		Method:         http.MethodPost,
		Path:           "/api/image-builder-composer/v2/compose",
		RequestBody:    test.JSONRequestBody(request),
		ExpectedStatus: http.StatusCreated,
	}.Do(t)

	// Parse ID
	var id v2.ComposeId
	require.NoError(t, json.Unmarshal(result.Body, &id))
	return id.Id
}

func getAllJobsOfCompose(t *testing.T, q jobqueue.JobQueue, finalJob uuid.UUID) []uuid.UUID {
	// This is basically a BFS on job dependencies
	// It may return duplicates (job dependencies are not a tree) but we don't care for our purposes

	jobsToQuery := []uuid.UUID{finalJob}
	discovered := []uuid.UUID{finalJob}

	for len(jobsToQuery) > 0 {
		current := jobsToQuery[0]
		jobsToQuery = jobsToQuery[1:]

		_, _, deps, _, err := q.Job(current)
		require.NoError(t, err)
		discovered = append(discovered, deps...)
		jobsToQuery = append(jobsToQuery, deps...)
	}

	return discovered
}

func jobRequest() string {
	return fmt.Sprintf(`
		{
			"types": [
				%q,
				%q,
				%q,
				%q
			],
			"arch": %q
		}`,
		worker.JobTypeKojiInit,
		worker.JobTypeOSBuild,
		worker.JobTypeKojiFinalize,
		worker.JobTypeDepsolve,
		test_distro.TestArch3Name)
}

func runNextJob(t *testing.T, jobs []uuid.UUID, workerServer *worker.Server, orgID string) {
	// test that a different tenant doesn't get any job
	// 100ms ought to be enough 🤞
	ctx, cancel := context.WithDeadline(reqContext("987"), time.Now().Add(time.Millisecond*100))
	defer cancel()
	test.APICall{
		Handler:        workerServer.Handler(),
		Method:         http.MethodPost,
		Path:           "/api/worker/v1/jobs",
		RequestBody:    test.JSONRequestBody(jobRequest()),
		Context:        ctx,
		ExpectedStatus: http.StatusNoContent,
	}.Do(t)

	// get a job using the right tenant
	resp := test.APICall{
		Handler:        workerServer.Handler(),
		Method:         http.MethodPost,
		Path:           "/api/worker/v1/jobs",
		RequestBody:    test.JSONRequestBody(jobRequest()),
		Context:        reqContext(orgID),
		ExpectedStatus: http.StatusCreated,
	}.Do(t)

	// get the job ID and test if it belongs to the list of jobs belonging to a particular compose (and thus tenant)
	var job struct {
		ID       string `json:"id"`
		Location string `json:"location"`
	}
	require.NoError(t, json.Unmarshal(resp.Body, &job))
	jobID := uuid.MustParse(job.ID)
	require.Contains(t, jobs, jobID)

	jobType, err := workerServer.JobType(jobID)
	require.NoError(t, err)

	var requestBody []byte
	switch jobType {
	// We need to set dummy values for the depsolve job result, because otherwise
	// the manifest generation job would fail on empty depsolved package list.
	// This would make the ComposeManifests endpoint return an error.
	case worker.JobTypeDepsolve:
		dummyRepoConfig := worker.DepsolvedRepoConfig{
			Id:       "test-repo",
			BaseURLs: []string{"https://example.com/repo"},
		}
		dummyPackage := worker.DepsolvedPackage{
			Name:    "pkg1",
			Version: "1.33",
			Release: "2.fc30",
			Arch:    "x86_64",
			Checksum: &worker.DepsolvedPackageChecksum{
				Type:  "sha256",
				Value: "e50ddb78a37f5851d1a5c37a4c77d59123153c156e628e064b9daa378f45a2fe",
			},
			RemoteLocations: []string{"https://pkg1.example.com/1.33-2.fc30.x86_64.rpm"},
			RepoID:          dummyRepoConfig.Id,
		}
		depsolveJobResult := &worker.DepsolveJobResult{
			PackageSpecs: map[string]worker.DepsolvedPackageList{
				// Used when depsolving a manifest
				"build": {dummyPackage},
				"os":    {dummyPackage},
			},
			RepoConfigs: map[string][]worker.DepsolvedRepoConfig{
				"build": {dummyRepoConfig},
				"os":    {dummyRepoConfig},
			},
		}
		rawDepsolveJobResult, err := json.Marshal(depsolveJobResult)
		require.NoError(t, err)
		result := api.UpdateJobResult{
			Result: json.RawMessage(rawDepsolveJobResult),
		}
		requestBody, err = json.Marshal(result)
		require.NoError(t, err)
	// For the purpose of the test, other job types results are not important
	default:
		requestBody = []byte(`{"result": {"job_result":{}}}`)
	}

	// finish the job
	test.APICall{
		Handler:        workerServer.Handler(),
		Method:         http.MethodPatch,
		Path:           job.Location,
		RequestBody:    test.JSONRequestBody(requestBody),
		Context:        reqContext(orgID),
		ExpectedStatus: http.StatusOK,
	}.Do(t)
}

// TestMultitenancy tests that the cloud API is securely multi-tenant.
//
// It creates 4 composes (mixed s3 and koji ones) and then simulates workers of
// 5 different tenants and makes sure that the workers only pick jobs that
// belong to their tenant.
//
// The test is not written in a parallel way but since our queue is FIFO and
// the test is running the job in a LIFO way, it should cover everything.
//
// It's important to acknowledge that this test is not E2E. We don't pass raw
// JWT here but an already parsed one inside a request context. A proper E2E
// also exists to test the full setup. Unfortunately, it cannot properly test
// that all jobs are assigned to the correct channel, therefore we need also
// this test.
func TestMultitenancy(t *testing.T) {
	apiServer, workerServer, q, cancel := newV2Server(t, t.TempDir(), &v2ServerOpts{enableJWT: true})
	handler := apiServer.Handler("/api/image-builder-composer/v2")
	defer cancel()

	// define 4 composes
	composes := []*struct {
		koji   bool
		orgID  string
		id     uuid.UUID
		jobIDs []uuid.UUID
	}{
		{
			koji:  true,
			orgID: "42",
		},
		{
			koji:  false,
			orgID: "123",
		},
		{
			koji:  true,
			orgID: "2022",
		},
		{
			koji:  true,
			orgID: "1995",
		},
	}

	// schedule all composes and retrieve some information about them
	for _, c := range composes {
		var request string
		if c.koji {
			request = kojiRequest()
		} else {
			request = s3Request()
		}
		id := scheduleRequest(t, handler, c.orgID, request)
		c.id = id

		// make sure that the channel is prefixed with "org-"
		_, _, _, channel, err := q.Job(id)
		require.NoError(t, err)
		require.Equal(t, "org-"+c.orgID, channel)

		// get all jobs belonging to this compose
		c.jobIDs = getAllJobsOfCompose(t, q, id)
	}

	// Run the composes in a LIFO way
	for i := len(composes) - 1; i >= 0; i -= 1 {
		c := composes[i]

		// We have to run 2 jobs for S3 composes (depsolve, osbuild)
		// 4 jobs for koji composes (depsolve, koji-init, osbuild, koji-finalize)
		numjobs := 2
		if c.koji {
			numjobs = 4
		}

		// Run all jobs
		for j := 0; j < numjobs; j++ {
			runNextJob(t, c.jobIDs, workerServer, c.orgID)
		}

		// Make sure that the compose is not pending (i.e. all jobs did run)
		resp := test.APICall{
			Handler:        handler,
			Method:         http.MethodGet,
			Context:        reqContext(c.orgID),
			Path:           "/api/image-builder-composer/v2/composes/" + c.id.String(),
			ExpectedStatus: http.StatusOK,
		}.Do(t)
		var result struct {
			Status string `json:"status"`
		}
		require.NoError(t, json.Unmarshal(resp.Body, &result))
		require.NotEqual(t, "pending", result.Status)

		composeEndpoints := []string{"", "logs", "manifests", "metadata"}
		// Verify that all compose endpoints work with the appropriate orgID
		for _, endpoint := range composeEndpoints {
			// TODO: "metadata" endpoint is not supported for Koji composes
			jobType, err := workerServer.JobType(c.id)
			require.NoError(t, err)
			if jobType == worker.JobTypeKojiFinalize && endpoint == "metadata" {
				continue
			}

			path := "/api/image-builder-composer/v2/composes/" + c.id.String()
			if endpoint != "" {
				path = path + "/" + endpoint
			}

			_ = test.APICall{
				Handler:        handler,
				Method:         http.MethodGet,
				Context:        reqContext(c.orgID),
				Path:           path,
				ExpectedStatus: http.StatusOK,
			}.Do(t)
		}

		// Verify that no compose endpoints are accessible with wrong orgID
		for _, endpoint := range composeEndpoints {
			path := "/api/image-builder-composer/v2/composes/" + c.id.String()
			if endpoint != "" {
				path = path + "/" + endpoint
			}

			_ = test.APICall{
				Handler:        handler,
				Method:         http.MethodGet,
				Context:        reqContext("bad-org"),
				Path:           path,
				ExpectedStatus: http.StatusNotFound,
			}.Do(t)
		}
	}
}

// TestBootcMultitenancyPreManifestProcessed verifies that bootc composes
// submitted with a tenant channel have their BootcPreManifest job processed
// by the bootcPreManifestLoop, which uses RequestJobAnyChannel to dequeue
// jobs regardless of channel.
func TestBootcMultitenancyPreManifestProcessed(t *testing.T) {
	apiServer, workerServer, q, cancel := newV2Server(t, t.TempDir(), &v2ServerOpts{enableJWT: true})
	handler := apiServer.Handler("/api/image-builder-composer/v2")
	defer cancel()

	orgID := "bootc-org-42"

	// Submit a bootc compose with a tenant
	composeID := scheduleRequest(t, handler, orgID, bootcRequest())

	// Verify the compose's top-level job has the correct channel
	_, _, _, channel, err := q.Job(composeID)
	require.NoError(t, err)
	require.Equal(t, "org-"+orgID, channel)

	// Walk the job graph to find the BootcInfoResolve and BootcPreManifest jobs
	allJobs := getAllJobsOfCompose(t, q, composeID)

	var bootcInfoResolveJobID, preManifestJobID uuid.UUID
	for _, jobID := range allJobs {
		jobType, _, _, jobChannel, err := q.Job(jobID)
		require.NoError(t, err)
		// All jobs should be in the tenant channel
		require.Equal(t, "org-"+orgID, jobChannel, "job %s (type %s) should have tenant channel", jobID, jobType)

		switch {
		// NOTE: BootcInfoResolve is arch-suffixed (e.g. "bootc-info-resolve:x86_64"), hence we use HasPrefix.
		case strings.HasPrefix(jobType, worker.JobTypeBootcInfoResolve):
			bootcInfoResolveJobID = jobID
		case jobType == worker.JobTypeBootcPreManifest:
			preManifestJobID = jobID
		}
	}
	require.NotEqual(t, uuid.Nil, bootcInfoResolveJobID, "should have BootcInfoResolve job")
	require.NotEqual(t, uuid.Nil, preManifestJobID, "should have BootcPreManifest job")

	// Finish the BootcInfoResolve job so the BootcPreManifest dependency is met.
	// Dequeue it using the tenant channel via the worker server directly.
	_, infoToken, _, _, _, err := workerServer.RequestJob(
		context.Background(), test_distro.TestArch3Name,
		[]string{worker.JobTypeBootcInfoResolve}, []string{"org-" + orgID}, uuid.Nil,
	)
	require.NoError(t, err)
	err = workerServer.FinishJob(infoToken, rawValidBaseBootcInfoResult(t))
	require.NoError(t, err)

	// Wait for the bootcPreManifestLoop to pick up and process the job.
	// Poll with a timeout rather than a fixed sleep.
	require.Eventually(t, func() bool {
		_, _, _, _, started, _, _, _, _, err := q.JobStatus(preManifestJobID)
		// NOTE: we can't use require.NoError, because require.Eventually runs the condition in a goroutine.
		// require.NoError calls t.FailNow(), which must be called from the goroutine running the test or benchmark
		// function, not from other goroutines created during the test
		assert.NoError(t, err)
		// Job should have been started by the loop
		return !started.IsZero()
	}, 5*time.Second, 50*time.Millisecond,
		"BootcPreManifest job should have been started by bootcPreManifestLoop via RequestJobAnyChannel",
	)
}
