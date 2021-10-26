//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=v1 --generate types,spec,client,server -o openapi.v1.gen.go openapi.v1.yml

package v1

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/osbuild1"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/prometheus"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

// Server represents the state of the cloud Server
type Server struct {
	workers     *worker.Server
	rpmMetadata rpmmd.RPMMD
	distros     *distroregistry.Registry
}

type apiHandlers struct {
	server *Server
}

type binder struct{}

// NewServer creates a new cloud server
func NewServer(workers *worker.Server, rpmMetadata rpmmd.RPMMD, distros *distroregistry.Registry) *Server {
	server := &Server{
		workers:     workers,
		rpmMetadata: rpmMetadata,
		distros:     distros,
	}
	return server
}

// Create an http.Handler() for this server, that provides the composer API at
// the given path.
func (server *Server) Handler(path string) http.Handler {
	e := echo.New()
	e.Binder = binder{}

	handler := apiHandlers{
		server: server,
	}
	RegisterHandlers(e.Group(path, server.IncRequests), &handler)

	return e
}

func (b binder) Bind(i interface{}, ctx echo.Context) error {
	contentType := ctx.Request().Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		return echo.NewHTTPError(http.StatusUnsupportedMediaType, "Only 'application/json' content type is supported")
	}

	err := json.NewDecoder(ctx.Request().Body).Decode(i)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("Cannot parse request body: %v", err))
	}
	return nil
}

func (s *Server) IncRequests(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		prometheus.TotalRequests.Inc()
		if strings.HasSuffix(c.Path(), "/compose") {
			prometheus.ComposeRequests.Inc()
		}
		return next(c)
	}
}

// Compose handles a new /compose POST request
func (h *apiHandlers) Compose(ctx echo.Context) error {
	contentType := ctx.Request().Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		return echo.NewHTTPError(http.StatusUnsupportedMediaType, "Only 'application/json' content type is supported")
	}

	var request ComposeRequest
	err := ctx.Bind(&request)
	if err != nil {
		return err
	}

	distribution := h.server.distros.GetDistro(request.Distribution)
	if distribution == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Unsupported distribution: %s", request.Distribution)
	}

	var bp = blueprint.Blueprint{}
	err = bp.Initialize()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Unable to initialize blueprint")
	}
	if request.Customizations != nil && request.Customizations.Packages != nil {
		for _, p := range *request.Customizations.Packages {
			bp.Packages = append(bp.Packages, blueprint.Package{
				Name: p,
			})
		}
	}

	// imagerequest
	type imageRequest struct {
		manifest distro.Manifest
		arch     string
		exports  []string
	}
	imageRequests := make([]imageRequest, len(request.ImageRequests))
	var targets []*target.Target

	// use the same seed for all images so we get the same IDs
	bigSeed, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		panic("cannot generate a manifest seed: " + err.Error())
	}
	manifestSeed := bigSeed.Int64()

	for i, ir := range request.ImageRequests {
		arch, err := distribution.GetArch(ir.Architecture)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Unsupported architecture '%s' for distribution '%s'", ir.Architecture, request.Distribution)
		}
		imageType, err := arch.GetImageType(ir.ImageType)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Unsupported image type '%s' for %s/%s", ir.ImageType, ir.Architecture, request.Distribution)
		}
		repositories := make([]rpmmd.RepoConfig, len(ir.Repositories))
		for j, repo := range ir.Repositories {
			repositories[j].RHSM = repo.Rhsm

			if repo.Baseurl != nil {
				repositories[j].BaseURL = *repo.Baseurl
			} else if repo.Mirrorlist != nil {
				repositories[j].MirrorList = *repo.Mirrorlist
			} else if repo.Metalink != nil {
				repositories[j].Metalink = *repo.Metalink
			} else {
				return echo.NewHTTPError(http.StatusBadRequest, "Must specify baseurl, mirrorlist, or metalink")
			}
		}

		packageSets := imageType.PackageSets(bp)
		depsolveJobID, err := h.server.workers.EnqueueDepsolve(&worker.DepsolveJob{
			PackageSets:      packageSets,
			Repos:            repositories,
			ModulePlatformID: distribution.ModulePlatformID(),
			Arch:             arch.Name(),
			Releasever:       distribution.Releasever(),
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Unable to enqueue depsolve job")
		}

		var depsolveResults worker.DepsolveJobResult
		for {
			status, _, err := h.server.workers.JobStatus(depsolveJobID, &depsolveResults)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Unable to get depsolve results")
			}
			if status.Canceled {
				return echo.NewHTTPError(http.StatusInternalServerError, "Depsolving job canceled unexpectedly")
			}
			if !status.Finished.IsZero() {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		if depsolveResults.Error != "" {
			if depsolveResults.ErrorType == worker.DepsolveErrorType {
				return echo.NewHTTPError(http.StatusBadRequest, "Failed to depsolve requested package set: %s", depsolveResults.Error)
			}
			return echo.NewHTTPError(http.StatusInternalServerError, "Error while depsolving: %s", depsolveResults.Error)
		}
		pkgSpecSets := depsolveResults.PackageSpecs

		imageOptions := distro.ImageOptions{Size: imageType.Size(0)}
		if request.Customizations != nil && request.Customizations.Subscription != nil {
			imageOptions.Subscription = &distro.SubscriptionImageOptions{
				Organization:  fmt.Sprintf("%d", request.Customizations.Subscription.Organization),
				ActivationKey: request.Customizations.Subscription.ActivationKey,
				ServerUrl:     request.Customizations.Subscription.ServerUrl,
				BaseUrl:       request.Customizations.Subscription.BaseUrl,
				Insights:      request.Customizations.Subscription.Insights,
			}
		}

		// set default ostree ref, if one not provided
		ostreeOptions := ir.Ostree
		if ostreeOptions == nil || ostreeOptions.Ref == nil {
			imageOptions.OSTree = distro.OSTreeImageOptions{Ref: imageType.OSTreeRef()}
		} else if !ostree.VerifyRef(*ostreeOptions.Ref) {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid OSTree ref: %s", *ostreeOptions.Ref)
		} else {
			imageOptions.OSTree = distro.OSTreeImageOptions{Ref: *ostreeOptions.Ref}
		}

		var parent string
		if ostreeOptions != nil && ostreeOptions.Url != nil {
			imageOptions.OSTree.URL = *ostreeOptions.Url
			parent, err = ostree.ResolveRef(imageOptions.OSTree.URL, imageOptions.OSTree.Ref)
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "Error resolving OSTree repo %s: %s", imageOptions.OSTree.URL, err)
			}
			imageOptions.OSTree.Parent = parent
		}

		// Set the blueprint customisation to take care of the user
		var blueprintCustoms *blueprint.Customizations
		if request.Customizations != nil && request.Customizations.Users != nil {
			var userCustomizations []blueprint.UserCustomization
			for _, user := range *request.Customizations.Users {
				var groups []string
				if user.Groups != nil {
					groups = *user.Groups
				} else {
					groups = nil
				}
				userCustomizations = append(userCustomizations,
					blueprint.UserCustomization{
						Name:   user.Name,
						Key:    user.Key,
						Groups: groups,
					},
				)
			}
			blueprintCustoms = &blueprint.Customizations{
				User: userCustomizations,
			}
		}

		manifest, err := imageType.Manifest(blueprintCustoms, imageOptions, repositories, pkgSpecSets, manifestSeed)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Failed to get manifest for for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err)
		}

		imageRequests[i].manifest = manifest
		imageRequests[i].arch = arch.Name()
		imageRequests[i].exports = imageType.Exports()

		uploadRequest := ir.UploadRequest
		/* oneOf is not supported by the openapi generator so marshal and unmarshal the uploadrequest based on the type */
		if uploadRequest.Type == UploadTypes_aws {
			var awsUploadOptions AWSUploadRequestOptions
			jsonUploadOptions, err := json.Marshal(uploadRequest.Options)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Unable to marshal aws upload request")
			}
			err = json.Unmarshal(jsonUploadOptions, &awsUploadOptions)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Unable to unmarshal aws upload request")
			}

			var share []string
			if awsUploadOptions.Ec2.ShareWithAccounts != nil {
				share = *awsUploadOptions.Ec2.ShareWithAccounts
			}
			key := fmt.Sprintf("composer-api-%s", uuid.New().String())
			t := target.NewAWSTarget(&target.AWSTargetOptions{
				Filename:          imageType.Filename(),
				Region:            awsUploadOptions.Region,
				AccessKeyID:       awsUploadOptions.S3.AccessKeyId,
				SecretAccessKey:   awsUploadOptions.S3.SecretAccessKey,
				Bucket:            awsUploadOptions.S3.Bucket,
				Key:               key,
				ShareWithAccounts: share,
			})
			if awsUploadOptions.Ec2.SnapshotName != nil {
				t.ImageName = *awsUploadOptions.Ec2.SnapshotName
			} else {
				t.ImageName = key
			}

			targets = append(targets, t)
		} else if uploadRequest.Type == UploadTypes_aws_s3 {
			var awsS3UploadOptions AWSS3UploadRequestOptions
			jsonUploadOptions, err := json.Marshal(uploadRequest.Options)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Unable to unmarshal aws upload request")
			}
			err = json.Unmarshal(jsonUploadOptions, &awsS3UploadOptions)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Unable to unmarshal aws upload request")
			}

			key := fmt.Sprintf("composer-api-%s", uuid.New().String())
			t := target.NewAWSS3Target(&target.AWSS3TargetOptions{
				Filename:        imageType.Filename(),
				Region:          awsS3UploadOptions.Region,
				AccessKeyID:     awsS3UploadOptions.S3.AccessKeyId,
				SecretAccessKey: awsS3UploadOptions.S3.SecretAccessKey,
				Bucket:          awsS3UploadOptions.S3.Bucket,
				Key:             key,
			})
			t.ImageName = key

			targets = append(targets, t)
		} else if uploadRequest.Type == UploadTypes_gcp {
			var gcpUploadOptions GCPUploadRequestOptions
			jsonUploadOptions, err := json.Marshal(uploadRequest.Options)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Unable to marshal gcp upload request")
			}
			err = json.Unmarshal(jsonUploadOptions, &gcpUploadOptions)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Unable to unmarshal gcp upload request")
			}

			var share []string
			if gcpUploadOptions.ShareWithAccounts != nil {
				share = *gcpUploadOptions.ShareWithAccounts
			}
			var region string
			if gcpUploadOptions.Region != nil {
				region = *gcpUploadOptions.Region
			}
			object := fmt.Sprintf("composer-api-%s", uuid.New().String())
			t := target.NewGCPTarget(&target.GCPTargetOptions{
				Filename:          imageType.Filename(),
				Region:            region,
				Os:                "", // not exposed in cloudapi for now
				Bucket:            gcpUploadOptions.Bucket,
				Object:            object,
				ShareWithAccounts: share,
			})
			// Import will fail if an image with this name already exists
			if gcpUploadOptions.ImageName != nil {
				t.ImageName = *gcpUploadOptions.ImageName
			} else {
				t.ImageName = object
			}

			targets = append(targets, t)
		} else if uploadRequest.Type == UploadTypes_azure {
			var azureUploadOptions AzureUploadRequestOptions
			jsonUploadOptions, err := json.Marshal(uploadRequest.Options)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Unable to marshal azure upload request")
			}
			err = json.Unmarshal(jsonUploadOptions, &azureUploadOptions)
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Unable to unmarshal azure upload request")
			}
			t := target.NewAzureImageTarget(&target.AzureImageTargetOptions{
				Filename:       imageType.Filename(),
				TenantID:       azureUploadOptions.TenantId,
				Location:       azureUploadOptions.Location,
				SubscriptionID: azureUploadOptions.SubscriptionId,
				ResourceGroup:  azureUploadOptions.ResourceGroup,
			})

			if azureUploadOptions.ImageName != nil {
				t.ImageName = *azureUploadOptions.ImageName
			} else {
				// if ImageName wasn't given, generate a random one
				t.ImageName = fmt.Sprintf("composer-api-%s", uuid.New().String())
			}

			targets = append(targets, t)
		} else {
			return echo.NewHTTPError(http.StatusBadRequest, "Unknown upload request type, only 'aws', 'azure' and 'gcp' are supported")
		}
	}

	var ir imageRequest
	if len(imageRequests) == 1 {
		// NOTE: the store currently does not support multi-image composes
		ir = imageRequests[0]
	} else {
		return echo.NewHTTPError(http.StatusBadRequest, "Only single-image composes are currently supported")
	}

	id, err := h.server.workers.EnqueueOSBuild(ir.arch, &worker.OSBuildJob{
		Manifest: ir.manifest,
		Targets:  targets,
		Exports:  ir.exports,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to enqueue manifest")
	}

	var response ComposeResult
	response.Id = id.String()

	return ctx.JSON(http.StatusCreated, response)
}

// ComposeStatus handles a /compose/{id} GET request
func (h *apiHandlers) ComposeStatus(ctx echo.Context, id string) error {
	jobId, err := uuid.Parse(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid format for parameter id: %s", err)
	}

	var result worker.OSBuildJobResult
	status, _, err := h.server.workers.JobStatus(jobId, &result)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Job %s not found: %s", id, err)
	}

	var us *UploadStatus
	if result.TargetResults != nil {
		// Only single upload target is allowed, therefore only a single upload target result is allowed as well
		if len(result.TargetResults) != 1 {
			return echo.NewHTTPError(http.StatusInternalServerError, "Job %s returned more upload target results than allowed", id)
		}
		tr := *result.TargetResults[0]

		var uploadType UploadTypes
		var uploadOptions interface{}

		switch tr.Name {
		case "org.osbuild.aws":
			uploadType = UploadTypes_aws
			awsOptions := tr.Options.(*target.AWSTargetResultOptions)
			uploadOptions = AWSUploadStatus{
				Ami:    awsOptions.Ami,
				Region: awsOptions.Region,
			}
		case "org.osbuild.aws.s3":
			uploadType = UploadTypes_aws_s3
			awsOptions := tr.Options.(*target.AWSS3TargetResultOptions)
			uploadOptions = AWSS3UploadStatus{
				Url: awsOptions.URL,
			}
		case "org.osbuild.gcp":
			uploadType = UploadTypes_gcp
			gcpOptions := tr.Options.(*target.GCPTargetResultOptions)
			uploadOptions = GCPUploadStatus{
				ImageName: gcpOptions.ImageName,
				ProjectId: gcpOptions.ProjectID,
			}
		case "org.osbuild.azure.image":
			uploadType = UploadTypes_azure
			gcpOptions := tr.Options.(*target.AzureImageTargetResultOptions)
			uploadOptions = AzureUploadStatus{
				ImageName: gcpOptions.ImageName,
			}
		default:
			return echo.NewHTTPError(http.StatusInternalServerError, "Job %s returned unknown upload target results %s", id, tr.Name)
		}

		us = &UploadStatus{
			Status:  result.UploadStatus,
			Type:    uploadType,
			Options: uploadOptions,
		}
	}

	response := ComposeStatus{
		ImageStatus: ImageStatus{
			Status:       composeStatusFromJobStatus(status, &result),
			UploadStatus: us,
		},
	}
	return ctx.JSON(http.StatusOK, response)
}

func composeStatusFromJobStatus(js *worker.JobStatus, result *worker.OSBuildJobResult) ImageStatusValue {
	if js.Canceled {
		return ImageStatusValue_failure
	}

	if js.Started.IsZero() {
		return ImageStatusValue_pending
	}

	if js.Finished.IsZero() {
		// TODO: handle also ImageStatusValue_uploading
		// TODO: handle also ImageStatusValue_registering
		return ImageStatusValue_building
	}

	if result.Success {
		return ImageStatusValue_success
	}

	return ImageStatusValue_failure
}

// GetOpenapiJson handles a /openapi.json GET request
func (h *apiHandlers) GetOpenapiJson(ctx echo.Context) error {
	spec, err := GetSwagger()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Could not load openapi spec")
	}
	return ctx.JSON(http.StatusOK, spec)
}

// GetVersion handles a /version GET request
func (h *apiHandlers) GetVersion(ctx echo.Context) error {
	spec, err := GetSwagger()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Could not load version")
	}
	version := Version{spec.Info.Version}
	return ctx.JSON(http.StatusOK, version)
}

// ComposeMetadata handles a /compose/{id}/metadata GET request
func (h *apiHandlers) ComposeMetadata(ctx echo.Context, id string) error {
	jobId, err := uuid.Parse(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid format for parameter id: %s", err)
	}

	var result worker.OSBuildJobResult
	status, _, err := h.server.workers.JobStatus(jobId, &result)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Job %s not found: %s", id, err)
	}

	var job worker.OSBuildJob
	if _, _, _, err = h.server.workers.Job(jobId, &job); err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Job %s not found: %s", id, err)
	}

	if status.Finished.IsZero() {
		// job still running: empty response
		return ctx.JSON(200, ComposeMetadata{})
	}

	manifestVer, err := job.Manifest.Version()
	if err != nil {
		panic("Failed to parse manifest version: " + err.Error())
	}

	var rpms []rpmmd.RPM
	var ostreeCommitResult *osbuild1.StageResult
	var coreStages []osbuild1.StageResult
	switch manifestVer {
	case "1":
		coreStages = result.OSBuildOutput.Stages
		if assemblerResult := result.OSBuildOutput.Assembler; assemblerResult.Name == "org.osbuild.ostree.commit" {
			ostreeCommitResult = result.OSBuildOutput.Assembler
		}
	case "2":
		// v2 manifest results store all stage output in the main stages
		// here we filter out the build stages to collect only the RPMs for the
		// core stages
		// the filtering relies on two assumptions:
		// 1. the build pipeline is named "build"
		// 2. the stage results from v2 manifests when converted to v1 are
		// named by prefixing the pipeline name
		for _, stage := range result.OSBuildOutput.Stages {
			if !strings.HasPrefix(stage.Name, "build") {
				coreStages = append(coreStages, stage)
			}
		}
		// find the ostree.commit stage
		for idx, stage := range result.OSBuildOutput.Stages {
			if strings.HasSuffix(stage.Name, "org.osbuild.ostree.commit") {
				ostreeCommitResult = &result.OSBuildOutput.Stages[idx]
				break
			}
		}
	default:
		panic("Unknown manifest version: " + manifestVer)
	}

	rpms = rpmmd.OSBuildStagesToRPMs(coreStages)

	packages := make([]PackageMetadata, len(rpms))
	for idx, rpm := range rpms {
		packages[idx] = PackageMetadata{
			Type:      rpm.Type,
			Name:      rpm.Name,
			Version:   rpm.Version,
			Release:   rpm.Release,
			Epoch:     rpm.Epoch,
			Arch:      rpm.Arch,
			Sigmd5:    rpm.Sigmd5,
			Signature: rpm.Signature,
		}
	}

	resp := new(ComposeMetadata)
	resp.Packages = &packages

	if ostreeCommitResult != nil && ostreeCommitResult.Metadata != nil {
		commitMetadata, ok := ostreeCommitResult.Metadata.(*osbuild1.OSTreeCommitStageMetadata)
		if !ok {
			panic("Failed to convert ostree commit stage metadata")
		}
		resp.OstreeCommit = &commitMetadata.Compose.OSTreeCommit
	}

	return ctx.JSON(200, resp)
}
