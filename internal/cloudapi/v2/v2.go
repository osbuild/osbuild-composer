//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=v2 --generate types,spec,server -o openapi.v2.gen.go openapi.v2.yml
package v2

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/sirupsen/logrus"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/jobqueue"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
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
	awsBucket   string
}

type apiHandlers struct {
	server *Server
}

type binder struct{}

func NewServer(workers *worker.Server, rpmMetadata rpmmd.RPMMD, distros *distroregistry.Registry, bucket string) *Server {
	server := &Server{
		workers:     workers,
		rpmMetadata: rpmMetadata,
		distros:     distros,
		awsBucket:   bucket,
	}
	return server
}

func (server *Server) Handler(path string) http.Handler {
	e := echo.New()
	e.Binder = binder{}
	e.HTTPErrorHandler = server.HTTPErrorHandler
	e.Pre(common.OperationIDMiddleware)
	e.Use(middleware.Recover())
	e.Logger = common.Logger()

	handler := apiHandlers{
		server: server,
	}
	RegisterHandlers(e.Group(path, prometheus.MetricsMiddleware), &handler)

	return e
}

func (b binder) Bind(i interface{}, ctx echo.Context) error {
	contentType := ctx.Request().Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		return HTTPError(ErrorUnsupportedMediaType)
	}

	err := json.NewDecoder(ctx.Request().Body).Decode(i)
	if err != nil {
		return HTTPErrorWithInternal(ErrorBodyDecodingError, err)
	}
	return nil
}

func (h *apiHandlers) GetOpenapi(ctx echo.Context) error {
	spec, err := GetSwagger()
	if err != nil {
		return HTTPError(ErrorFailedToLoadOpenAPISpec)
	}
	return ctx.JSON(http.StatusOK, spec)
}

func (h *apiHandlers) GetErrorList(ctx echo.Context, params GetErrorListParams) error {
	page := 0
	var err error
	if params.Page != nil {
		page, err = strconv.Atoi(string(*params.Page))
		if err != nil {
			return HTTPError(ErrorInvalidPageParam)
		}
	}

	size := 100
	if params.Size != nil {
		size, err = strconv.Atoi(string(*params.Size))
		if err != nil {
			return HTTPError(ErrorInvalidSizeParam)
		}
	}

	return ctx.JSON(http.StatusOK, APIErrorList(page, size, ctx))
}

func (h *apiHandlers) GetError(ctx echo.Context, id string) error {
	errorId, err := strconv.Atoi(id)
	if err != nil {
		return HTTPError(ErrorInvalidErrorId)
	}

	apiError := APIError(ServiceErrorCode(errorId), nil, ctx)
	// If the service error wasn't found, it's a 404 in this instance
	if apiError.Id == fmt.Sprintf("%d", ErrorServiceErrorNotFound) {
		return HTTPError(ErrorErrorNotFound)
	}
	return ctx.JSON(http.StatusOK, apiError)
}

func (h *apiHandlers) PostCompose(ctx echo.Context) error {
	var request ComposeRequest
	err := ctx.Bind(&request)
	if err != nil {
		return err
	}

	distribution := h.server.distros.GetDistro(request.Distribution)
	if distribution == nil {
		return HTTPError(ErrorUnsupportedDistribution)
	}

	var bp = blueprint.Blueprint{}
	err = bp.Initialize()
	if err != nil {
		return HTTPErrorWithInternal(ErrorFailedToInitializeBlueprint, err)
	}
	if request.Customizations != nil && request.Customizations.Packages != nil {
		for _, p := range *request.Customizations.Packages {
			bp.Packages = append(bp.Packages, blueprint.Package{
				Name: p,
			})
		}
	}

	// use the same seed for all images so we get the same IDs
	bigSeed, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return HTTPError(ErrorFailedToGenerateManifestSeed)
	}
	manifestSeed := bigSeed.Int64()

	ir := request.ImageRequest
	arch, err := distribution.GetArch(ir.Architecture)
	if err != nil {
		return HTTPError(ErrorUnsupportedArchitecture)
	}
	imageType, err := arch.GetImageType(imageTypeFromApiImageType(ir.ImageType))
	if err != nil {
		return HTTPError(ErrorUnsupportedImageType)
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
			return HTTPError(ErrorInvalidRepository)
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
		return HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	imageOptions := distro.ImageOptions{Size: imageType.Size(0)}
	if request.Customizations != nil && request.Customizations.Subscription != nil {
		imageOptions.Subscription = &distro.SubscriptionImageOptions{
			Organization:  request.Customizations.Subscription.Organization,
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
		return HTTPError(ErrorInvalidOSTreeRef)
	} else {
		imageOptions.OSTree = distro.OSTreeImageOptions{Ref: *ostreeOptions.Ref}
	}

	var parent string
	if ostreeOptions != nil && ostreeOptions.Url != nil {
		imageOptions.OSTree.URL = *ostreeOptions.Url
		parent, err = ostree.ResolveRef(imageOptions.OSTree.URL, imageOptions.OSTree.Ref)
		if err != nil {
			return HTTPErrorWithInternal(ErrorInvalidOSTreeRepo, err)
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

	var irTarget *target.Target
	/* oneOf is not supported by the openapi generator so marshal and unmarshal the uploadrequest based on the type */
	switch ir.ImageType {
	case ImageTypes_aws:
		var awsUploadOptions AWSEC2UploadOptions
		jsonUploadOptions, err := json.Marshal(ir.UploadOptions)
		if err != nil {
			return HTTPError(ErrorJSONMarshallingError)
		}
		err = json.Unmarshal(jsonUploadOptions, &awsUploadOptions)
		if err != nil {
			return HTTPError(ErrorJSONUnMarshallingError)
		}

		// For service maintenance, images are discovered by the "Name:composer-api-*"
		// tag filter. Currently all image names in the service are generated, so they're
		// guaranteed to be unique as well. If users are ever allowed to name their images,
		// an extra tag should be added.
		key := fmt.Sprintf("composer-api-%s", uuid.New().String())
		t := target.NewAWSTarget(&target.AWSTargetOptions{
			Filename:          imageType.Filename(),
			Region:            awsUploadOptions.Region,
			Bucket:            h.server.awsBucket,
			Key:               key,
			ShareWithAccounts: awsUploadOptions.ShareWithAccounts,
		})
		if awsUploadOptions.SnapshotName != nil {
			t.ImageName = *awsUploadOptions.SnapshotName
		} else {
			t.ImageName = key
		}

		irTarget = t
	case ImageTypes_guest_image:
		fallthrough
	case ImageTypes_vsphere:
		fallthrough
	case ImageTypes_image_installer:
		fallthrough
	case ImageTypes_edge_installer:
		fallthrough
	case ImageTypes_edge_container:
		fallthrough
	case ImageTypes_edge_commit:
		var awsS3UploadOptions AWSS3UploadOptions
		jsonUploadOptions, err := json.Marshal(ir.UploadOptions)
		if err != nil {
			return HTTPError(ErrorJSONMarshallingError)
		}
		err = json.Unmarshal(jsonUploadOptions, &awsS3UploadOptions)
		if err != nil {
			return HTTPError(ErrorJSONUnMarshallingError)
		}

		key := fmt.Sprintf("composer-api-%s", uuid.New().String())
		t := target.NewAWSS3Target(&target.AWSS3TargetOptions{
			Filename: imageType.Filename(),
			Region:   awsS3UploadOptions.Region,
			Bucket:   h.server.awsBucket,
			Key:      key,
		})
		t.ImageName = key

		irTarget = t
	case ImageTypes_gcp:
		var gcpUploadOptions GCPUploadOptions
		jsonUploadOptions, err := json.Marshal(ir.UploadOptions)
		if err != nil {
			return HTTPError(ErrorJSONMarshallingError)
		}
		err = json.Unmarshal(jsonUploadOptions, &gcpUploadOptions)
		if err != nil {
			return HTTPError(ErrorJSONUnMarshallingError)
		}

		var share []string
		if gcpUploadOptions.ShareWithAccounts != nil {
			share = *gcpUploadOptions.ShareWithAccounts
		}

		object := fmt.Sprintf("composer-api-%s", uuid.New().String())
		t := target.NewGCPTarget(&target.GCPTargetOptions{
			Filename:          imageType.Filename(),
			Region:            gcpUploadOptions.Region,
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

		irTarget = t
	case ImageTypes_azure:
		var azureUploadOptions AzureUploadOptions
		jsonUploadOptions, err := json.Marshal(ir.UploadOptions)
		if err != nil {
			return HTTPError(ErrorJSONMarshallingError)
		}
		err = json.Unmarshal(jsonUploadOptions, &azureUploadOptions)
		if err != nil {
			return HTTPError(ErrorJSONUnMarshallingError)
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

		irTarget = t
	default:
		return HTTPError(ErrorUnsupportedImageType)
	}

	manifestJobID, err := h.server.workers.EnqueueManifestJobByID(&worker.ManifestJobByID{}, depsolveJobID)
	if err != nil {
		return HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	id, err := h.server.workers.EnqueueOSBuildAsDependency(arch.Name(), &worker.OSBuildJob{
		Targets: []*target.Target{irTarget},
		Exports: imageType.Exports(),
		PipelineNames: &worker.PipelineNames{
			Build:   imageType.BuildPipelines(),
			Payload: imageType.PayloadPipelines(),
		},
	}, manifestJobID)
	if err != nil {
		return HTTPErrorWithInternal(ErrorEnqueueingJob, err)
	}

	ctx.Logger().Infof("Job ID %s enqueued for operationID %s", id, ctx.Get("operationID"))
	manifestJobContext, manifestCancel := context.WithTimeout(context.Background(), time.Minute*5)

	// start 1 goroutine which requests datajob type
	go func(workers *worker.Server, manifestJobID uuid.UUID, b *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, seed int64) {
		defer manifestCancel()
		// wait until job is in a pending state
		var token uuid.UUID
		var dynArgs []json.RawMessage
		var err error
		for {
			_, token, _, _, dynArgs, err = workers.RequestJobById(context.Background(), "", manifestJobID)
			if err == jobqueue.ErrNotPending {
				logrus.Debugf("Manifest job %v not pending, waiting for depsolve job to finish", manifestJobID)
				time.Sleep(time.Millisecond * 50)
				select {
				case <-manifestJobContext.Done():
					logrus.Warnf("Manifest job %v's dependencies took longer than 5 minutes to finish, returning to avoid dangling routines", manifestJobID)
					return
				default:
					continue
				}
			}
			if err != nil {
				logrus.Errorf("Error requesting manifest job: %v", err)
				return
			}
			break
		}

		var jobResult *worker.ManifestJobByIDResult = &worker.ManifestJobByIDResult{
			Manifest:   nil,
			Error:      "",
			ResultCode: worker.JobSuccess,
		}

		defer func() {
			if jobResult.Error != "" {
				logrus.Errorf("Error in manifest job %v: %v", manifestJobID, jobResult.Error)
				jobResult.ResultCode = worker.ManifestByIDError
			}

			result, err := json.Marshal(jobResult)
			if err != nil {
				logrus.Errorf("Error marshalling manifest job %v results: %v", manifestJobID, err)
			}

			err = workers.FinishJob(token, result)
			if err != nil {
				logrus.Errorf("Error finishing manifest job: %v", err)
			}
		}()

		if len(dynArgs) == 0 {
			jobResult.Error = "No dynamic arguments"
			return
		}

		var depsolveResults worker.DepsolveJobResult
		err = json.Unmarshal(dynArgs[0], &depsolveResults)
		if err != nil {
			jobResult.Error = "Error parsing dynamic arguments"
			return
		}

		if depsolveResults.Error != "" {
			if depsolveResults.ErrorType == worker.DepsolveErrorType {
				jobResult.Error = "Error in depsolve job dependency input, bad request"
				return
			}
			jobResult.Error = "Error in depsolve job dependency"
			return
		}

		manifest, err := imageType.Manifest(b, options, repos, depsolveResults.PackageSpecs, seed)
		if err != nil {
			jobResult.Error = "Error generating manifest"
			return
		}

		jobResult.Manifest = manifest
	}(h.server.workers, manifestJobID, blueprintCustoms, imageOptions, repositories, manifestSeed)

	return ctx.JSON(http.StatusCreated, &ComposeId{
		ObjectReference: ObjectReference{
			Href: "/api/image-builder-composer/v2/compose",
			Id:   id.String(),
			Kind: "ComposeId",
		},
		Id: id.String(),
	})
}

func imageTypeFromApiImageType(it ImageTypes) string {
	switch it {
	case ImageTypes_aws:
		return "ami"
	case ImageTypes_gcp:
		return "vhd"
	case ImageTypes_azure:
		return "vhd"
	case ImageTypes_guest_image:
		return "qcow2"
	case ImageTypes_vsphere:
		return "vmdk"
	case ImageTypes_image_installer:
		return "image-installer"
	case ImageTypes_edge_commit:
		return "rhel-edge-commit"
	case ImageTypes_edge_container:
		return "rhel-edge-container"
	case ImageTypes_edge_installer:
		return "rhel-edge-installer"
	}
	return ""
}

func (h *apiHandlers) GetComposeStatus(ctx echo.Context, id string) error {
	jobId, err := uuid.Parse(id)
	if err != nil {
		return HTTPError(ErrorInvalidComposeId)
	}

	var result worker.OSBuildJobResult
	status, _, err := h.server.workers.JobStatus(jobId, &result)
	if err != nil {
		return HTTPError(ErrorComposeNotFound)
	}

	var us *UploadStatus
	if result.TargetResults != nil {
		// Only single upload target is allowed, therefore only a single upload target result is allowed as well
		if len(result.TargetResults) != 1 {
			return HTTPError(ErrorSeveralUploadTargets)
		}
		tr := *result.TargetResults[0]

		var uploadType UploadTypes
		var uploadOptions interface{}

		switch tr.Name {
		case "org.osbuild.aws":
			uploadType = UploadTypes_aws
			awsOptions := tr.Options.(*target.AWSTargetResultOptions)
			uploadOptions = AWSEC2UploadStatus{
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
			return HTTPError(ErrorUnknownUploadTarget)
		}

		us = &UploadStatus{
			Status:  result.UploadStatus,
			Type:    uploadType,
			Options: uploadOptions,
		}
	}

	return ctx.JSON(http.StatusOK, ComposeStatus{
		ObjectReference: ObjectReference{
			Href: fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId),
			Id:   jobId.String(),
			Kind: "ComposeStatus",
		},
		ImageStatus: ImageStatus{
			Status:       composeStatusFromJobStatus(status, &result),
			UploadStatus: us,
		},
	})
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

// ComposeMetadata handles a /composes/{id}/metadata GET request
func (h *apiHandlers) GetComposeMetadata(ctx echo.Context, id string) error {
	jobId, err := uuid.Parse(id)
	if err != nil {
		return HTTPError(ErrorInvalidComposeId)
	}

	var result worker.OSBuildJobResult
	status, _, err := h.server.workers.JobStatus(jobId, &result)
	if err != nil {
		return HTTPErrorWithInternal(ErrorComposeNotFound, err)
	}

	var job worker.OSBuildJob
	if _, _, _, err = h.server.workers.Job(jobId, &job); err != nil {
		return HTTPErrorWithInternal(ErrorComposeNotFound, err)
	}

	if status.Finished.IsZero() {
		// job still running: empty response
		return ctx.JSON(200, ComposeMetadata{
			ObjectReference: ObjectReference{
				Href: fmt.Sprintf("/api/image-builder-composer/v2/%v/metadata", jobId),
				Id:   jobId.String(),
				Kind: "ComposeMetadata",
			},
		})
	}

	if status.Canceled || !result.Success {
		// job canceled or failed, empty response
		return ctx.JSON(200, ComposeMetadata{
			ObjectReference: ObjectReference{
				Href: fmt.Sprintf("/api/image-builder-composer/v2/%v/metadata", jobId),
				Id:   jobId.String(),
				Kind: "ComposeMetadata",
			},
		})
	}

	if result.OSBuildOutput == nil || len(result.OSBuildOutput.Log) == 0 {
		// no osbuild output recorded for job, error
		return HTTPError(ErrorMalformedOSBuildJobResult)
	}

	var ostreeCommitMetadata *osbuild.OSTreeCommitStageMetadata
	var rpmStagesMd []osbuild.RPMStageMetadata // collect rpm stage metadata from payload pipelines
	for _, plName := range job.PipelineNames.Payload {
		plMd, hasMd := result.OSBuildOutput.Metadata[plName]
		if !hasMd {
			continue
		}
		for _, stageMd := range plMd {
			switch md := stageMd.(type) {
			case *osbuild.RPMStageMetadata:
				rpmStagesMd = append(rpmStagesMd, *md)
			case *osbuild.OSTreeCommitStageMetadata:
				ostreeCommitMetadata = md
			}
		}
	}

	packages := stagesToPackageMetadata(rpmStagesMd)

	resp := &ComposeMetadata{
		ObjectReference: ObjectReference{
			Href: fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/metadata", jobId),
			Id:   jobId.String(),
			Kind: "ComposeMetadata",
		},
		Packages: &packages,
	}

	if ostreeCommitMetadata != nil {
		resp.OstreeCommit = &ostreeCommitMetadata.Compose.OSTreeCommit
	}

	return ctx.JSON(200, resp)
}

func stagesToPackageMetadata(stages []osbuild.RPMStageMetadata) []PackageMetadata {
	packages := make([]PackageMetadata, 0)
	for _, md := range stages {
		for _, rpm := range md.Packages {
			packages = append(packages,
				PackageMetadata{
					Type:      "rpm",
					Name:      rpm.Name,
					Version:   rpm.Version,
					Release:   rpm.Release,
					Epoch:     rpm.Epoch,
					Arch:      rpm.Arch,
					Sigmd5:    rpm.SigMD5,
					Signature: rpmmd.PackageMetadataToSignature(rpm),
				},
			)
		}
	}
	return packages
}
