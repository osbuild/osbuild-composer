//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=v2 --generate types,spec,server -o openapi.v2.gen.go openapi.v2.yml
package v2

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker"
	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type apiHandlers struct {
	server *Server
}

type binder struct{}

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

// splitExtension returns the extension of the given file. If there's
// a multipart extension (e.g. file.tar.gz), it returns all parts (e.g.
// .tar.gz). If there's no extension in the input, it returns an empty
// string. If the filename starts with dot, the part before the second dot
// is not considered as an extension.
func splitExtension(filename string) string {
	filenameParts := strings.Split(filename, ".")

	if len(filenameParts) > 0 && filenameParts[0] == "" {
		filenameParts = filenameParts[1:]
	}

	if len(filenameParts) <= 1 {
		return ""
	}

	return "." + strings.Join(filenameParts[1:], ".")
}

type imageRequest struct {
	imageType    distro.ImageType
	arch         distro.Arch
	repositories []rpmmd.RepoConfig
	imageOptions distro.ImageOptions
	target       *target.Target
}

func (h *apiHandlers) PostCompose(ctx echo.Context) error {
	var request ComposeRequest
	err := ctx.Bind(&request)
	if err != nil {
		return err
	}

	// channel is empty if JWT is not enabled
	channel, err := h.server.getTenantChannel(ctx)
	if err != nil {
		return HTTPErrorWithInternal(ErrorTenantNotFound, err)
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

	// Set the blueprint customisation to take care of the user
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
		if bp.Customizations == nil {
			bp.Customizations = &blueprint.Customizations{
				User: userCustomizations,
			}
		} else {
			bp.Customizations.User = userCustomizations
		}
	}

	if request.Customizations != nil && request.Customizations.Packages != nil {
		for _, p := range *request.Customizations.Packages {
			bp.Packages = append(bp.Packages, blueprint.Package{
				Name: p,
			})
		}
	}

	if request.Customizations != nil && request.Customizations.Filesystem != nil {
		var fsCustomizations []blueprint.FilesystemCustomization
		for _, f := range *request.Customizations.Filesystem {

			fsCustomizations = append(fsCustomizations,
				blueprint.FilesystemCustomization{
					Mountpoint: f.Mountpoint,
					MinSize:    f.MinSize,
				},
			)
		}
		if bp.Customizations == nil {
			bp.Customizations = &blueprint.Customizations{
				Filesystem: fsCustomizations,
			}
		} else {
			bp.Customizations.Filesystem = fsCustomizations
		}
	}

	// add the user-defined repositories only to the depsolve job for the
	// payload (the packages for the final image)
	var payloadRepositories []Repository
	if request.Customizations != nil && request.Customizations.PayloadRepositories != nil {
		payloadRepositories = *request.Customizations.PayloadRepositories
	}

	// use the same seed for all images so we get the same IDs
	bigSeed, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return HTTPError(ErrorFailedToGenerateManifestSeed)
	}
	manifestSeed := bigSeed.Int64()

	// For backwards compatibility, we support both a single image request
	// as well as an array of requests in the API. Exactly one must be
	// specified.
	if request.ImageRequest != nil {
		if request.ImageRequests != nil {
			// we should really be using oneOf in the spec
			return HTTPError(ErrorInvalidNumberOfImageBuilds)
		}
		request.ImageRequests = &[]ImageRequest{*request.ImageRequest}
	}
	if request.ImageRequests == nil {
		return HTTPError(ErrorInvalidNumberOfImageBuilds)
	}
	var irs []imageRequest
	for _, ir := range *request.ImageRequests {
		arch, err := distribution.GetArch(ir.Architecture)
		if err != nil {
			return HTTPError(ErrorUnsupportedArchitecture)
		}
		imageType, err := arch.GetImageType(imageTypeFromApiImageType(ir.ImageType, arch))
		if err != nil {
			return HTTPError(ErrorUnsupportedImageType)
		}

		repos, err := convertRepos(ir.Repositories, payloadRepositories, imageType.PayloadPackageSets())
		if err != nil {
			return err
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

		ostreeOptions := ostree.RequestParams{}
		if ir.Ostree != nil {
			if ir.Ostree.Ref != nil {
				ostreeOptions.Ref = *ir.Ostree.Ref
			}
			if ir.Ostree.Url != nil {
				ostreeOptions.URL = *ir.Ostree.Url
			}
			if ir.Ostree.Parent != nil {
				ostreeOptions.Parent = *ir.Ostree.Parent
			}
		}
		if imageOptions.OSTree, err = ostree.ResolveParams(ostreeOptions, imageType.OSTreeRef()); err != nil {
			switch v := err.(type) {
			case ostree.RefError:
				return HTTPError(ErrorInvalidOSTreeRef)
			case ostree.ResolveRefError:
				return HTTPErrorWithInternal(ErrorInvalidOSTreeRepo, v)
			case ostree.ParameterComboError:
				return HTTPError(ErrorInvalidOSTreeParams)
			default:
				// general case
				return HTTPError(ErrorInvalidOSTreeParams)
			}
		}

		var irTarget *target.Target
		if ir.UploadOptions == nil {
			// nowhere to put the image, this is a user error
			if request.Koji == nil {
				return HTTPError(ErrorJSONUnMarshallingError)
			}
		} else {
			// TODO: support uploads also for koji
			if request.Koji != nil {
				return HTTPError(ErrorJSONUnMarshallingError)
			}
			/* oneOf is not supported by the openapi generator so marshal and unmarshal the uploadrequest based on the type */
			switch ir.ImageType {
			case ImageTypesAws:
				fallthrough
			case ImageTypesAwsRhui:
				fallthrough
			case ImageTypesAwsHaRhui:
				fallthrough
			case ImageTypesAwsSapRhui:
				var awsUploadOptions AWSEC2UploadOptions
				jsonUploadOptions, err := json.Marshal(*ir.UploadOptions)
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
					Key:               key,
					ShareWithAccounts: awsUploadOptions.ShareWithAccounts,
				})
				if awsUploadOptions.SnapshotName != nil {
					t.ImageName = *awsUploadOptions.SnapshotName
				} else {
					t.ImageName = key
				}

				irTarget = t
			case ImageTypesGuestImage:
				fallthrough
			case ImageTypesVsphere:
				fallthrough
			case ImageTypesImageInstaller:
				fallthrough
			case ImageTypesEdgeInstaller:
				fallthrough
			case ImageTypesEdgeContainer:
				fallthrough
			case ImageTypesEdgeCommit:
				var awsS3UploadOptions AWSS3UploadOptions
				jsonUploadOptions, err := json.Marshal(*ir.UploadOptions)
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
					Key:      key,
				})
				t.ImageName = key

				irTarget = t
			case ImageTypesGcp:
				var gcpUploadOptions GCPUploadOptions
				jsonUploadOptions, err := json.Marshal(*ir.UploadOptions)
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

				imageName := fmt.Sprintf("composer-api-%s", uuid.New().String())
				t := target.NewGCPTarget(&target.GCPTargetOptions{
					Filename: imageType.Filename(),
					Region:   gcpUploadOptions.Region,
					Os:       imageType.Arch().Distro().Name(), // not exposed in cloudapi
					Bucket:   gcpUploadOptions.Bucket,
					// the uploaded object must have a valid extension
					Object:            fmt.Sprintf("%s.tar.gz", imageName),
					ShareWithAccounts: share,
				})
				// Import will fail if an image with this name already exists
				if gcpUploadOptions.ImageName != nil {
					t.ImageName = *gcpUploadOptions.ImageName
				} else {
					t.ImageName = imageName
				}

				irTarget = t
			case ImageTypesAzure:
				fallthrough
			case ImageTypesAzureRhui:
				var azureUploadOptions AzureUploadOptions
				jsonUploadOptions, err := json.Marshal(*ir.UploadOptions)
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
		}

		irs = append(irs, imageRequest{
			imageType:    imageType,
			arch:         arch,
			repositories: repos,
			imageOptions: imageOptions,
			target:       irTarget,
		})
	}

	var id uuid.UUID
	if request.Koji != nil {
		id, err = h.server.enqueueKojiCompose(uint64(request.Koji.TaskId), request.Koji.Server, request.Koji.Name, request.Koji.Version, request.Koji.Release, distribution, bp, manifestSeed, irs, channel)
		if err != nil {
			return err
		}
	} else {
		id, err = h.server.enqueueCompose(distribution, bp, manifestSeed, irs, channel)
		if err != nil {
			return err
		}
	}

	ctx.Logger().Infof("Job ID %s enqueued for operationID %s", id, ctx.Get(common.OperationIDKey))

	return ctx.JSON(http.StatusCreated, &ComposeId{
		ObjectReference: ObjectReference{
			Href: "/api/image-builder-composer/v2/compose",
			Id:   id.String(),
			Kind: "ComposeId",
		},
		Id: id.String(),
	})
}

func imageTypeFromApiImageType(it ImageTypes, arch distro.Arch) string {
	switch it {
	case ImageTypesAws:
		return "ami"
	case ImageTypesAwsRhui:
		return "ec2"
	case ImageTypesAwsHaRhui:
		return "ec2-ha"
	case ImageTypesAwsSapRhui:
		return "ec2-sap"
	case ImageTypesGcp:
		return "gce"
	case ImageTypesAzure:
		return "vhd"
	case ImageTypesAzureRhui:
		return "azure-rhui"
	case ImageTypesGuestImage:
		return "qcow2"
	case ImageTypesVsphere:
		return "vmdk"
	case ImageTypesImageInstaller:
		return "image-installer"
	case ImageTypesEdgeCommit:
		// Fedora doesn't define "rhel-edge-commit", or "edge-commit" yet.
		// Assume that if the distro contains "fedora-iot-commit", it's Fedora
		// and translate ImageTypesEdgeCommit to fedora-iot-commit in this case.
		// This should definitely be removed in the near future.
		if _, err := arch.GetImageType("fedora-iot-commit"); err == nil {
			return "fedora-iot-commit"
		}
		return "rhel-edge-commit"
	case ImageTypesEdgeContainer:
		return "rhel-edge-container"
	case ImageTypesEdgeInstaller:
		return "rhel-edge-installer"
	}
	return ""
}

func (h *apiHandlers) GetComposeStatus(ctx echo.Context, id string) error {
	return h.server.EnsureJobChannel(h.getComposeStatusImpl)(ctx, id)
}

func (h *apiHandlers) getComposeStatusImpl(ctx echo.Context, id string) error {
	jobId, err := uuid.Parse(id)
	if err != nil {
		return HTTPError(ErrorInvalidComposeId)
	}

	jobType, err := h.server.workers.JobType(jobId)
	if err != nil {
		return HTTPError(ErrorComposeNotFound)
	}

	if jobType == worker.JobTypeOSBuild {
		var result worker.OSBuildJobResult
		status, _, err := h.server.workers.OSBuildJobStatus(jobId, &result)
		if err != nil {
			return HTTPError(ErrorMalformedOSBuildJobResult)
		}

		jobError, err := h.server.workers.JobDependencyChainErrors(jobId)
		if err != nil {
			return HTTPError(ErrorGettingBuildDependencyStatus)
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
				uploadType = UploadTypesAws
				awsOptions := tr.Options.(*target.AWSTargetResultOptions)
				uploadOptions = AWSEC2UploadStatus{
					Ami:    awsOptions.Ami,
					Region: awsOptions.Region,
				}
			case "org.osbuild.aws.s3":
				uploadType = UploadTypesAwsS3
				awsOptions := tr.Options.(*target.AWSS3TargetResultOptions)
				uploadOptions = AWSS3UploadStatus{
					Url: awsOptions.URL,
				}
			case "org.osbuild.gcp":
				uploadType = UploadTypesGcp
				gcpOptions := tr.Options.(*target.GCPTargetResultOptions)
				uploadOptions = GCPUploadStatus{
					ImageName: gcpOptions.ImageName,
					ProjectId: gcpOptions.ProjectID,
				}
			case "org.osbuild.azure.image":
				uploadType = UploadTypesAzure
				gcpOptions := tr.Options.(*target.AzureImageTargetResultOptions)
				uploadOptions = AzureUploadStatus{
					ImageName: gcpOptions.ImageName,
				}
			default:
				return HTTPError(ErrorUnknownUploadTarget)
			}

			us = &UploadStatus{
				Status:  UploadStatusValue(result.UploadStatus),
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
			Status: composeStatusFromOSBuildJobStatus(status, &result),
			ImageStatus: ImageStatus{
				Status:       imageStatusFromOSBuildJobStatus(status, &result),
				Error:        composeStatusErrorFromJobError(jobError),
				UploadStatus: us,
			},
		})
	} else if jobType == worker.JobTypeKojiFinalize {
		var result worker.KojiFinalizeJobResult
		finalizeStatus, deps, err := h.server.workers.KojiFinalizeJobStatus(jobId, &result)
		if err != nil {
			return HTTPError(ErrorMalformedOSBuildJobResult)
		}
		if len(deps) < 2 {
			return HTTPError(ErrorUnexpectedNumberOfImageBuilds)
		}
		var initResult worker.KojiInitJobResult
		_, _, err = h.server.workers.KojiInitJobStatus(deps[0], &initResult)
		if err != nil {
			return HTTPError(ErrorMalformedOSBuildJobResult)
		}
		var buildJobResults []worker.OSBuildJobResult
		var buildJobStatuses []ImageStatus
		for i := 1; i < len(deps); i++ {
			var buildJobResult worker.OSBuildJobResult
			buildJobStatus, _, err := h.server.workers.OSBuildJobStatus(deps[i], &buildJobResult)
			if err != nil {
				return HTTPError(ErrorMalformedOSBuildJobResult)
			}
			buildJobError, err := h.server.workers.JobDependencyChainErrors(deps[i])
			if err != nil {
				return HTTPError(ErrorGettingBuildDependencyStatus)
			}
			buildJobResults = append(buildJobResults, buildJobResult)
			buildJobStatuses = append(buildJobStatuses, ImageStatus{
				Status: imageStatusFromKojiJobStatus(buildJobStatus, &initResult, &buildJobResult),
				Error:  composeStatusErrorFromJobError(buildJobError),
			})
		}
		response := ComposeStatus{
			ObjectReference: ObjectReference{
				Href: fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId),
				Id:   jobId.String(),
				Kind: "ComposeStatus",
			},
			Status:        composeStatusFromKojiJobStatus(finalizeStatus, &initResult, buildJobResults, &result),
			ImageStatus:   buildJobStatuses[0], // backwards compatibility
			ImageStatuses: &buildJobStatuses,
			KojiStatus:    &KojiStatus{},
		}
		buildID := int(initResult.BuildID)
		if buildID != 0 {
			response.KojiStatus.BuildId = &buildID
		}
		return ctx.JSON(http.StatusOK, response)
	} else {
		return HTTPError(ErrorInvalidJobType)
	}
}

func composeStatusErrorFromJobError(jobError *clienterrors.Error) *ComposeStatusError {
	if jobError == nil {
		return nil
	}
	return &ComposeStatusError{
		Id:      int(jobError.ID),
		Reason:  jobError.Reason,
		Details: &jobError.Details,
	}
}

func imageStatusFromOSBuildJobStatus(js *worker.JobStatus, result *worker.OSBuildJobResult) ImageStatusValue {
	if js.Canceled {
		return ImageStatusValueFailure
	}

	if js.Started.IsZero() {
		return ImageStatusValuePending
	}

	if js.Finished.IsZero() {
		// TODO: handle also ImageStatusValueUploading
		// TODO: handle also ImageStatusValueRegistering
		return ImageStatusValueBuilding
	}

	if result.Success {
		return ImageStatusValueSuccess
	}

	return ImageStatusValueFailure
}

func imageStatusFromKojiJobStatus(js *worker.JobStatus, initResult *worker.KojiInitJobResult, buildResult *worker.OSBuildJobResult) ImageStatusValue {
	if js.Canceled {
		return ImageStatusValueFailure
	}

	if initResult.JobError != nil {
		return ImageStatusValueFailure
	}

	if js.Started.IsZero() {
		return ImageStatusValuePending
	}

	if js.Finished.IsZero() {
		return ImageStatusValueBuilding
	}

	if buildResult.JobError != nil {
		return ImageStatusValueFailure
	}

	if buildResult.OSBuildOutput != nil && !buildResult.OSBuildOutput.Success {
		return ImageStatusValueFailure
	}

	return ImageStatusValueSuccess
}

func composeStatusFromOSBuildJobStatus(js *worker.JobStatus, result *worker.OSBuildJobResult) ComposeStatusValue {
	if js.Canceled {
		return ComposeStatusValueFailure
	}

	if js.Finished.IsZero() {
		return ComposeStatusValuePending
	}

	if result.Success {
		return ComposeStatusValueSuccess
	}

	return ComposeStatusValueFailure
}

func composeStatusFromKojiJobStatus(js *worker.JobStatus, initResult *worker.KojiInitJobResult, buildResults []worker.OSBuildJobResult, result *worker.KojiFinalizeJobResult) ComposeStatusValue {
	if js.Canceled {
		return ComposeStatusValueFailure
	}

	if js.Finished.IsZero() {
		return ComposeStatusValuePending
	}

	if initResult.JobError != nil {
		return ComposeStatusValueFailure
	}

	for _, buildResult := range buildResults {
		if buildResult.JobError != nil {
			return ComposeStatusValueFailure
		}

		if buildResult.OSBuildOutput != nil && !buildResult.OSBuildOutput.Success {
			return ComposeStatusValueFailure
		}
	}

	if result.JobError != nil {
		return ComposeStatusValueFailure
	}

	return ComposeStatusValueSuccess
}

// ComposeMetadata handles a /composes/{id}/metadata GET request
func (h *apiHandlers) GetComposeMetadata(ctx echo.Context, id string) error {
	return h.server.EnsureJobChannel(h.getComposeMetadataImpl)(ctx, id)
}

func (h *apiHandlers) getComposeMetadataImpl(ctx echo.Context, id string) error {
	jobId, err := uuid.Parse(id)
	if err != nil {
		return HTTPError(ErrorInvalidComposeId)
	}

	jobType, err := h.server.workers.JobType(jobId)
	if err != nil {
		return HTTPError(ErrorComposeNotFound)
	}

	// TODO: support koji builds
	if jobType != worker.JobTypeOSBuild {
		return HTTPError(ErrorInvalidJobType)
	}

	var result worker.OSBuildJobResult
	status, _, err := h.server.workers.OSBuildJobStatus(jobId, &result)
	if err != nil {
		return HTTPErrorWithInternal(ErrorComposeNotFound, err)
	}

	var job worker.OSBuildJob
	if err = h.server.workers.OSBuildJob(jobId, &job); err != nil {
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
					Signature: osbuild.RPMPackageMetadataToSignature(rpm),
				},
			)
		}
	}
	return packages
}

// Get logs for a compose
func (h *apiHandlers) GetComposeLogs(ctx echo.Context, id string) error {
	return h.server.EnsureJobChannel(h.getComposeLogsImpl)(ctx, id)
}

func (h *apiHandlers) getComposeLogsImpl(ctx echo.Context, id string) error {

	jobId, err := uuid.Parse(id)
	if err != nil {
		return HTTPError(ErrorInvalidComposeId)
	}

	jobType, err := h.server.workers.JobType(jobId)
	if err != nil {
		return HTTPError(ErrorComposeNotFound)
	}

	var buildResultBlobs []interface{}

	resp := &ComposeLogs{
		ObjectReference: ObjectReference{
			Href: fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/logs", jobId),
			Id:   jobId.String(),
			Kind: "ComposeLogs",
		},
	}

	switch jobType {
	case worker.JobTypeKojiFinalize:
		var finalizeResult worker.KojiFinalizeJobResult
		_, deps, err := h.server.workers.KojiFinalizeJobStatus(jobId, &finalizeResult)
		if err != nil {
			return HTTPErrorWithInternal(ErrorComposeNotFound, err)
		}

		var initResult worker.KojiInitJobResult
		_, _, err = h.server.workers.KojiInitJobStatus(deps[0], &initResult)
		if err != nil {
			return HTTPErrorWithInternal(ErrorComposeNotFound, err)
		}

		for i := 1; i < len(deps); i++ {
			buildJobType, err := h.server.workers.JobType(deps[i])
			if err != nil {
				return HTTPErrorWithInternal(ErrorComposeNotFound, err)
			}

			switch buildJobType {
			// TODO: remove eventually. Kept for backward compatibility
			case worker.JobTypeOSBuildKoji:
				var buildResult worker.OSBuildKojiJobResult
				_, _, err = h.server.workers.OSBuildKojiJobStatus(deps[i], &buildResult)
				if err != nil {
					return HTTPErrorWithInternal(ErrorComposeNotFound, err)
				}
				buildResultBlobs = append(buildResultBlobs, buildResult)

			case worker.JobTypeOSBuild:
				var buildResult worker.OSBuildJobResult
				_, _, err = h.server.workers.OSBuildJobStatus(deps[i], &buildResult)
				if err != nil {
					return HTTPErrorWithInternal(ErrorComposeNotFound, err)
				}
				buildResultBlobs = append(buildResultBlobs, buildResult)

			default:
				return HTTPErrorWithInternal(ErrorInvalidJobType,
					fmt.Errorf("unexpected job type in koji compose dependencies: %q", buildJobType))
			}
		}

		resp.Koji = &KojiLogs{
			Init:   initResult,
			Import: finalizeResult,
		}

	case worker.JobTypeOSBuild:
		var buildResult worker.OSBuildJobResult
		_, _, err = h.server.workers.OSBuildJobStatus(jobId, &buildResult)
		if err != nil {
			return HTTPErrorWithInternal(ErrorComposeNotFound, err)
		}
		buildResultBlobs = append(buildResultBlobs, buildResult)

	default:
		return HTTPError(ErrorInvalidJobType)
	}

	// Return the OSBuildJobResults as-is for now. The contents of ImageBuilds
	// is not part of the API. It's meant for a human to be able to access
	// the logs, which just happen to be in JSON.
	resp.ImageBuilds = buildResultBlobs
	return ctx.JSON(http.StatusOK, resp)
}

func manifestJobResultsFromJobDeps(w *worker.Server, deps []uuid.UUID) (*worker.ManifestJobByIDResult, error) {
	var manifestResult worker.ManifestJobByIDResult

	for i := 0; i < len(deps); i++ {
		depType, err := w.JobType(deps[i])
		if err != nil {
			return nil, err
		}
		if depType == worker.JobTypeManifestIDOnly {
			_, _, err = w.ManifestJobStatus(deps[i], &manifestResult)
			if err != nil {
				return nil, err
			}
			return &manifestResult, nil
		}
	}

	return nil, fmt.Errorf("no %q job found in the dependencies", worker.JobTypeManifestIDOnly)
}

// GetComposeIdManifests returns the Manifests for a given Compose (one for each image).
func (h *apiHandlers) GetComposeManifests(ctx echo.Context, id string) error {
	return h.server.EnsureJobChannel(h.getComposeManifestsImpl)(ctx, id)
}

func (h *apiHandlers) getComposeManifestsImpl(ctx echo.Context, id string) error {
	jobId, err := uuid.Parse(id)
	if err != nil {
		return HTTPError(ErrorInvalidComposeId)
	}

	jobType, err := h.server.workers.JobType(jobId)
	if err != nil {
		return HTTPError(ErrorComposeNotFound)
	}

	var manifestBlobs []interface{}

	switch jobType {
	case worker.JobTypeKojiFinalize:
		var finalizeResult worker.KojiFinalizeJobResult
		_, deps, err := h.server.workers.KojiFinalizeJobStatus(jobId, &finalizeResult)
		if err != nil {
			return HTTPErrorWithInternal(ErrorComposeNotFound, err)
		}

		for i := 1; i < len(deps); i++ {
			buildJobType, err := h.server.workers.JobType(deps[i])
			if err != nil {
				return HTTPErrorWithInternal(ErrorComposeNotFound, err)
			}

			var manifest distro.Manifest

			switch buildJobType {
			// TODO: remove eventually. Kept for backward compatibility
			case worker.JobTypeOSBuildKoji:
				var buildJob worker.OSBuildKojiJob
				err = h.server.workers.OSBuildKojiJob(deps[i], &buildJob)
				if err != nil {
					return HTTPErrorWithInternal(ErrorComposeNotFound, err)
				}

				if len(buildJob.Manifest) != 0 {
					manifest = buildJob.Manifest
				} else {
					_, buildDeps, err := h.server.workers.OSBuildKojiJobStatus(deps[i], &worker.OSBuildKojiJobResult{})
					if err != nil {
						return HTTPErrorWithInternal(ErrorComposeNotFound, err)
					}
					manifestResult, err := manifestJobResultsFromJobDeps(h.server.workers, buildDeps)
					if err != nil {
						return HTTPErrorWithInternal(ErrorComposeNotFound, fmt.Errorf("job %q: %v", jobId, err))
					}
					manifest = manifestResult.Manifest
				}

			case worker.JobTypeOSBuild:
				var buildJob worker.OSBuildJob
				err = h.server.workers.OSBuildJob(deps[i], &buildJob)
				if err != nil {
					return HTTPErrorWithInternal(ErrorComposeNotFound, err)
				}

				if len(buildJob.Manifest) != 0 {
					manifest = buildJob.Manifest
				} else {
					_, buildDeps, err := h.server.workers.OSBuildJobStatus(deps[i], &worker.OSBuildJobResult{})
					if err != nil {
						return HTTPErrorWithInternal(ErrorComposeNotFound, err)
					}
					manifestResult, err := manifestJobResultsFromJobDeps(h.server.workers, buildDeps)
					if err != nil {
						return HTTPErrorWithInternal(ErrorComposeNotFound, fmt.Errorf("job %q: %v", jobId, err))
					}
					manifest = manifestResult.Manifest
				}

			default:
				return HTTPErrorWithInternal(ErrorInvalidJobType,
					fmt.Errorf("unexpected job type in koji compose dependencies: %q", buildJobType))
			}
			manifestBlobs = append(manifestBlobs, manifest)
		}

	case worker.JobTypeOSBuild:
		var buildJob worker.OSBuildJob
		err = h.server.workers.OSBuildJob(jobId, &buildJob)
		if err != nil {
			return HTTPErrorWithInternal(ErrorComposeNotFound, err)
		}

		var manifest distro.Manifest
		if len(buildJob.Manifest) != 0 {
			manifest = buildJob.Manifest
		} else {
			_, deps, err := h.server.workers.OSBuildJobStatus(jobId, &worker.OSBuildJobResult{})
			if err != nil {
				return HTTPErrorWithInternal(ErrorComposeNotFound, err)
			}
			manifestResult, err := manifestJobResultsFromJobDeps(h.server.workers, deps)
			if err != nil {
				return HTTPErrorWithInternal(ErrorComposeNotFound, fmt.Errorf("job %q: %v", jobId, err))
			}
			manifest = manifestResult.Manifest
		}
		manifestBlobs = append(manifestBlobs, manifest)

	default:
		return HTTPError(ErrorInvalidJobType)
	}

	resp := &ComposeManifests{
		ObjectReference: ObjectReference{
			Href: fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/manifests", jobId),
			Id:   jobId.String(),
			Kind: "ComposeManifests",
		},
		Manifests: manifestBlobs,
	}

	return ctx.JSON(http.StatusOK, resp)
}

// Converts repositories in the request to the internal rpmmd.RepoConfig representation
func convertRepos(irRepos, payloadRepositories []Repository, payloadPackageSets []string) ([]rpmmd.RepoConfig, error) {
	repos := make([]rpmmd.RepoConfig, 0, len(irRepos)+len(payloadRepositories))

	for idx := range irRepos {
		r, err := genRepoConfig(irRepos[idx])
		if err != nil {
			return nil, err
		}
		repos = append(repos, *r)
	}

	for idx := range payloadRepositories {
		// the PackageSets (package_sets) field for these repositories is
		// ignored (see openapi.v2.yml description for payload_repositories)
		// and we replace any value in it with the names of the payload package
		// sets
		r, err := genRepoConfig(payloadRepositories[idx])
		if err != nil {
			return nil, err
		}
		r.PackageSets = payloadPackageSets
		repos = append(repos, *r)
	}

	return repos, nil
}

func genRepoConfig(repo Repository) (*rpmmd.RepoConfig, error) {
	repoConfig := new(rpmmd.RepoConfig)

	repoConfig.RHSM = repo.Rhsm != nil && *repo.Rhsm

	if repo.Baseurl != nil {
		repoConfig.BaseURL = *repo.Baseurl
	} else if repo.Mirrorlist != nil {
		repoConfig.MirrorList = *repo.Mirrorlist
	} else if repo.Metalink != nil {
		repoConfig.Metalink = *repo.Metalink
	} else {
		return nil, HTTPError(ErrorInvalidRepository)
	}

	if repo.CheckGpg != nil {
		repoConfig.CheckGPG = *repo.CheckGpg
	}
	if repo.Gpgkey != nil {
		repoConfig.GPGKey = *repo.Gpgkey
	}
	if repo.IgnoreSsl != nil {
		repoConfig.IgnoreSSL = *repo.IgnoreSsl
	}

	if repoConfig.CheckGPG && repoConfig.GPGKey == "" {
		return nil, HTTPError(ErrorNoGPGKey)
	}

	if repo.PackageSets != nil {
		repoConfig.PackageSets = *repo.PackageSets
	}

	return repoConfig, nil
}
