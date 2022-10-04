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
	"github.com/osbuild/osbuild-composer/internal/osbuild"
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

	apiError := APIError(ServiceErrorCode(errorId), nil, ctx, nil)
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

	var bp = blueprint.Blueprint{Name: "empty blueprint"}
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

	if request.Customizations != nil && request.Customizations.Containers != nil {
		for _, c := range *request.Customizations.Containers {
			bc := blueprint.Container{
				Source:    c.Source,
				TLSVerify: c.TlsVerify,
			}
			if c.Name != nil {
				bc.Name = *c.Name
			}
			bp.Containers = append(bp.Containers, bc)
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

	if request.Customizations != nil && request.Customizations.Services != nil {
		servicesCustomization := &blueprint.ServicesCustomization{}
		if request.Customizations.Services.Enabled != nil {
			servicesCustomization.Enabled = make([]string, len(*request.Customizations.Services.Enabled))
			copy(servicesCustomization.Enabled, *request.Customizations.Services.Enabled)
		}
		if request.Customizations.Services.Disabled != nil {
			servicesCustomization.Disabled = make([]string, len(*request.Customizations.Services.Disabled))
			copy(servicesCustomization.Disabled, *request.Customizations.Services.Disabled)
		}
		if bp.Customizations == nil {
			bp.Customizations = &blueprint.Customizations{
				Services: servicesCustomization,
			}
		} else {
			bp.Customizations.Services = servicesCustomization
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

		if request.Koji == nil {
			imageOptions.Facts = &distro.FactsImageOptions{
				ApiType: "cloudapi-v2",
			}
		}

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
					Region:            awsUploadOptions.Region,
					Key:               key,
					ShareWithAccounts: awsUploadOptions.ShareWithAccounts,
				})
				if awsUploadOptions.SnapshotName != nil {
					t.ImageName = *awsUploadOptions.SnapshotName
				} else {
					t.ImageName = key
				}
				t.OsbuildArtifact.ExportFilename = imageType.Filename()

				irTarget = t
			case ImageTypesGuestImage:
				fallthrough
			case ImageTypesVsphere:
				fallthrough
			case ImageTypesImageInstaller:
				fallthrough
			case ImageTypesEdgeInstaller:
				fallthrough
			case ImageTypesIotInstaller:
				fallthrough
			case ImageTypesEdgeCommit:
				fallthrough
			case ImageTypesIotCommit:
				fallthrough
			case ImageTypesIotRawImage:
				var awsS3UploadOptions AWSS3UploadOptions
				jsonUploadOptions, err := json.Marshal(*ir.UploadOptions)
				if err != nil {
					return HTTPError(ErrorJSONMarshallingError)
				}
				err = json.Unmarshal(jsonUploadOptions, &awsS3UploadOptions)
				if err != nil {
					return HTTPError(ErrorJSONUnMarshallingError)
				}

				public := false
				if awsS3UploadOptions.Public != nil && *awsS3UploadOptions.Public {
					public = true
				}

				key := fmt.Sprintf("composer-api-%s", uuid.New().String())
				t := target.NewAWSS3Target(&target.AWSS3TargetOptions{
					Region: awsS3UploadOptions.Region,
					Key:    key,
					Public: public,
				})
				t.ImageName = key
				t.OsbuildArtifact.ExportFilename = imageType.Filename()

				irTarget = t
			case ImageTypesEdgeContainer:
				fallthrough
			case ImageTypesIotContainer:
				var containerUploadOptions ContainerUploadOptions
				jsonUploadOptions, err := json.Marshal(*ir.UploadOptions)
				if err != nil {
					return HTTPError(ErrorJSONMarshallingError)
				}
				err = json.Unmarshal(jsonUploadOptions, &containerUploadOptions)
				if err != nil {
					return HTTPError(ErrorJSONUnMarshallingError)
				}

				var name = request.Distribution
				var tag = uuid.New().String()
				if containerUploadOptions.Name != nil {
					name = *containerUploadOptions.Name
					if containerUploadOptions.Tag != nil {
						tag = *containerUploadOptions.Tag
					}
				}

				t := target.NewContainerTarget(&target.ContainerTargetOptions{})
				t.ImageName = fmt.Sprintf("%s:%s", name, tag)
				t.OsbuildArtifact.ExportFilename = imageType.Filename()

				irTarget = t
			case ImageTypesGcp:
				fallthrough
			case ImageTypesGcpRhui:
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
					Region: gcpUploadOptions.Region,
					Os:     imageType.Arch().Distro().Name(), // not exposed in cloudapi
					Bucket: gcpUploadOptions.Bucket,
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
				t.OsbuildArtifact.ExportFilename = imageType.Filename()

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
				t.OsbuildArtifact.ExportFilename = imageType.Filename()

				irTarget = t
			default:
				return HTTPError(ErrorUnsupportedImageType)
			}

			irTarget.OsbuildArtifact.ExportName = imageType.Exports()[0]
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
	case ImageTypesGcpRhui:
		return "gce-rhui"
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
		return "rhel-edge-commit"
	case ImageTypesEdgeContainer:
		return "rhel-edge-container"
	case ImageTypesEdgeInstaller:
		return "rhel-edge-installer"
	case ImageTypesIotCommit:
		return "iot-commit"
	case ImageTypesIotContainer:
		return "iot-container"
	case ImageTypesIotInstaller:
		return "iot-installer"
	case ImageTypesIotRawImage:
		return "iot-raw-image"
	}
	return ""
}

func targetResultToUploadStatus(t *target.TargetResult) (*UploadStatus, error) {
	var us *UploadStatus
	var uploadType UploadTypes
	var uploadOptions interface{}

	switch t.Name {
	case target.TargetNameAWS:
		uploadType = UploadTypesAws
		awsOptions := t.Options.(*target.AWSTargetResultOptions)
		uploadOptions = AWSEC2UploadStatus{
			Ami:    awsOptions.Ami,
			Region: awsOptions.Region,
		}
	case target.TargetNameAWSS3:
		uploadType = UploadTypesAwsS3
		awsOptions := t.Options.(*target.AWSS3TargetResultOptions)
		uploadOptions = AWSS3UploadStatus{
			Url: awsOptions.URL,
		}
	case target.TargetNameGCP:
		uploadType = UploadTypesGcp
		gcpOptions := t.Options.(*target.GCPTargetResultOptions)
		uploadOptions = GCPUploadStatus{
			ImageName: gcpOptions.ImageName,
			ProjectId: gcpOptions.ProjectID,
		}
	case target.TargetNameAzureImage:
		uploadType = UploadTypesAzure
		gcpOptions := t.Options.(*target.AzureImageTargetResultOptions)
		uploadOptions = AzureUploadStatus{
			ImageName: gcpOptions.ImageName,
		}
	case target.TargetNameContainer:
		uploadType = UploadTypesContainer
		containerOptions := t.Options.(*target.ContainerTargetResultOptions)
		uploadOptions = ContainerUploadStatus{
			Url:    containerOptions.URL,
			Digest: containerOptions.Digest,
		}

	default:
		return nil, fmt.Errorf("unknown upload target: %s", t.Name)
	}

	us = &UploadStatus{
		// TODO: determine upload status based on the target results, not job results
		// Don't set the status here for now, but let it be set by the caller.
		//Status:  UploadStatusValue(result.UploadStatus),
		Type:    uploadType,
		Options: uploadOptions,
	}

	return us, nil
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
		jobInfo, err := h.server.workers.OSBuildJobInfo(jobId, &result)
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
			tr := result.TargetResults[0]
			us, err = targetResultToUploadStatus(tr)
			if err != nil {
				return HTTPError(ErrorUnknownUploadTarget)
			}
			us.Status = uploadStatusFromJobStatus(jobInfo.JobStatus, result.JobError)
		}

		return ctx.JSON(http.StatusOK, ComposeStatus{
			ObjectReference: ObjectReference{
				Href: fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId),
				Id:   jobId.String(),
				Kind: "ComposeStatus",
			},
			Status: composeStatusFromOSBuildJobStatus(jobInfo.JobStatus, &result),
			ImageStatus: ImageStatus{
				Status:       imageStatusFromOSBuildJobStatus(jobInfo.JobStatus, &result),
				Error:        composeStatusErrorFromJobError(jobError),
				UploadStatus: us,
			},
		})
	} else if jobType == worker.JobTypeKojiFinalize {
		var result worker.KojiFinalizeJobResult
		finalizeInfo, err := h.server.workers.KojiFinalizeJobInfo(jobId, &result)
		if err != nil {
			return HTTPError(ErrorMalformedOSBuildJobResult)
		}
		if len(finalizeInfo.Deps) < 2 {
			return HTTPError(ErrorUnexpectedNumberOfImageBuilds)
		}
		var initResult worker.KojiInitJobResult
		_, err = h.server.workers.KojiInitJobInfo(finalizeInfo.Deps[0], &initResult)
		if err != nil {
			return HTTPError(ErrorMalformedOSBuildJobResult)
		}
		var buildJobResults []worker.OSBuildJobResult
		var buildJobStatuses []ImageStatus
		for i := 1; i < len(finalizeInfo.Deps); i++ {
			var buildJobResult worker.OSBuildJobResult
			buildInfo, err := h.server.workers.OSBuildJobInfo(finalizeInfo.Deps[i], &buildJobResult)
			if err != nil {
				return HTTPError(ErrorMalformedOSBuildJobResult)
			}
			buildJobError, err := h.server.workers.JobDependencyChainErrors(finalizeInfo.Deps[i])
			if err != nil {
				return HTTPError(ErrorGettingBuildDependencyStatus)
			}

			var us *UploadStatus
			// Only a single upload target in addition to Koji is allowed.
			// Koji target is always added to osbuild jobs for Koji compose
			// by the enqueueKojiCompose() function.
			if len(buildJobResult.TargetResults) > 2 {
				return HTTPError(ErrorSeveralUploadTargets)
			}
			for _, tr := range buildJobResult.TargetResults {
				if tr.Name != target.TargetNameKoji {
					us, err = targetResultToUploadStatus(tr)
					if err != nil {
						return HTTPError(ErrorUnknownUploadTarget)
					}
					us.Status = uploadStatusFromJobStatus(buildInfo.JobStatus, result.JobError)
				}
			}

			buildJobResults = append(buildJobResults, buildJobResult)
			buildJobStatuses = append(buildJobStatuses, ImageStatus{
				Status:       imageStatusFromKojiJobStatus(buildInfo.JobStatus, &initResult, &buildJobResult),
				Error:        composeStatusErrorFromJobError(buildJobError),
				UploadStatus: us,
			})
		}
		response := ComposeStatus{
			ObjectReference: ObjectReference{
				Href: fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId),
				Id:   jobId.String(),
				Kind: "ComposeStatus",
			},
			Status:        composeStatusFromKojiJobStatus(finalizeInfo.JobStatus, &initResult, buildJobResults, &result),
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
	err := &ComposeStatusError{
		Id:     int(jobError.ID),
		Reason: jobError.Reason,
	}
	if jobError.Details != nil {
		err.Details = &jobError.Details
	}
	return err
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
	buildInfo, err := h.server.workers.OSBuildJobInfo(jobId, &result)
	if err != nil {
		return HTTPErrorWithInternal(ErrorComposeNotFound, err)
	}

	var job worker.OSBuildJob
	if err = h.server.workers.OSBuildJob(jobId, &job); err != nil {
		return HTTPErrorWithInternal(ErrorComposeNotFound, err)
	}

	if buildInfo.JobStatus.Finished.IsZero() {
		// job still running: empty response
		return ctx.JSON(200, ComposeMetadata{
			ObjectReference: ObjectReference{
				Href: fmt.Sprintf("/api/image-builder-composer/v2/%v/metadata", jobId),
				Id:   jobId.String(),
				Kind: "ComposeMetadata",
			},
		})
	}

	if buildInfo.JobStatus.Canceled || !result.Success {
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
		finalizeInfo, err := h.server.workers.KojiFinalizeJobInfo(jobId, &finalizeResult)
		if err != nil {
			return HTTPErrorWithInternal(ErrorComposeNotFound, err)
		}

		var initResult worker.KojiInitJobResult
		_, err = h.server.workers.KojiInitJobInfo(finalizeInfo.Deps[0], &initResult)
		if err != nil {
			return HTTPErrorWithInternal(ErrorComposeNotFound, err)
		}

		for i := 1; i < len(finalizeInfo.Deps); i++ {
			buildJobType, err := h.server.workers.JobType(finalizeInfo.Deps[i])
			if err != nil {
				return HTTPErrorWithInternal(ErrorComposeNotFound, err)
			}

			switch buildJobType {
			case worker.JobTypeOSBuild:
				var buildResult worker.OSBuildJobResult
				_, err = h.server.workers.OSBuildJobInfo(finalizeInfo.Deps[i], &buildResult)
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
		_, err = h.server.workers.OSBuildJobInfo(jobId, &buildResult)
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
			_, err = w.ManifestJobInfo(deps[i], &manifestResult)
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
		finalizeInfo, err := h.server.workers.KojiFinalizeJobInfo(jobId, &finalizeResult)
		if err != nil {
			return HTTPErrorWithInternal(ErrorComposeNotFound, err)
		}

		for i := 1; i < len(finalizeInfo.Deps); i++ {
			buildJobType, err := h.server.workers.JobType(finalizeInfo.Deps[i])
			if err != nil {
				return HTTPErrorWithInternal(ErrorComposeNotFound, err)
			}

			var manifest distro.Manifest

			switch buildJobType {
			case worker.JobTypeOSBuild:
				var buildJob worker.OSBuildJob
				err = h.server.workers.OSBuildJob(finalizeInfo.Deps[i], &buildJob)
				if err != nil {
					return HTTPErrorWithInternal(ErrorComposeNotFound, err)
				}

				if len(buildJob.Manifest) != 0 {
					manifest = buildJob.Manifest
				} else {
					buildInfo, err := h.server.workers.OSBuildJobInfo(finalizeInfo.Deps[i], &worker.OSBuildJobResult{})
					if err != nil {
						return HTTPErrorWithInternal(ErrorComposeNotFound, err)
					}
					manifestResult, err := manifestJobResultsFromJobDeps(h.server.workers, buildInfo.Deps)
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
			buildInfo, err := h.server.workers.OSBuildJobInfo(jobId, &worker.OSBuildJobResult{})
			if err != nil {
				return HTTPErrorWithInternal(ErrorComposeNotFound, err)
			}
			manifestResult, err := manifestJobResultsFromJobDeps(h.server.workers, buildInfo.Deps)
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

func (h *apiHandlers) PostCloneCompose(ctx echo.Context, id string) error {
	return h.server.EnsureJobChannel(h.postCloneComposeImpl)(ctx, id)
}

func (h *apiHandlers) postCloneComposeImpl(ctx echo.Context, id string) error {
	channel, err := h.server.getTenantChannel(ctx)
	if err != nil {
		return HTTPErrorWithInternal(ErrorTenantNotFound, err)
	}

	jobId, err := uuid.Parse(id)
	if err != nil {
		return HTTPError(ErrorInvalidComposeId)
	}

	jobType, err := h.server.workers.JobType(jobId)
	if err != nil {
		return HTTPError(ErrorComposeNotFound)
	}

	if jobType != worker.JobTypeOSBuild {
		return HTTPError(ErrorInvalidJobType)
	}

	var osbuildResult worker.OSBuildJobResult
	osbuildInfo, err := h.server.workers.OSBuildJobInfo(jobId, &osbuildResult)
	if err != nil {
		return HTTPErrorWithInternal(ErrorGettingOSBuildJobStatus, err)
	}

	if osbuildInfo.JobStatus.Finished.IsZero() || !osbuildResult.Success {
		return HTTPError(ErrorComposeBadState)
	}

	if osbuildResult.TargetResults == nil {
		return HTTPError(ErrorMalformedOSBuildJobResult)
	}
	// Only single upload target is allowed, therefore only a single upload target result is allowed as well
	if len(osbuildResult.TargetResults) != 1 {
		return HTTPError(ErrorSeveralUploadTargets)
	}
	var us *UploadStatus
	us, err = targetResultToUploadStatus(osbuildResult.TargetResults[0])
	if err != nil {
		return HTTPError(ErrorUnknownUploadTarget)
	}

	var osbuildJob worker.OSBuildJob
	err = h.server.workers.OSBuildJob(jobId, &osbuildJob)
	if err != nil {
		return HTTPErrorWithInternal(ErrorComposeNotFound, err)
	}

	if len(osbuildJob.Targets) != 1 {
		return HTTPError(ErrorSeveralUploadTargets)
	}

	// the id of the last job in the dependency chain which users should wait on
	finalJob := jobId
	// look at the upload status of the osbuild dependency to decide what to do
	if us.Type == UploadTypesAws {
		options := us.Options.(AWSEC2UploadStatus)
		var img AWSEC2CloneCompose
		err := ctx.Bind(&img)
		if err != nil {
			return err
		}

		shareAmi := options.Ami
		shareRegion := img.Region
		if img.Region != options.Region {
			// Let the share job use dynArgs
			shareAmi = ""
			shareRegion = ""

			// Check dependents if we need to do a copyjob
			foundDep := false
			for _, d := range osbuildInfo.Dependents {
				jt, err := h.server.workers.JobType(d)
				if err != nil {
					return HTTPErrorWithInternal(ErrorGettingJobType, err)
				}
				if jt == worker.JobTypeAWSEC2Copy {
					var cjResult worker.AWSEC2CopyJobResult
					_, err := h.server.workers.AWSEC2CopyJobInfo(d, &cjResult)
					if err != nil {
						return HTTPErrorWithInternal(ErrorGettingAWSEC2JobStatus, err)
					}

					if cjResult.JobError == nil && options.Region == cjResult.Region {
						finalJob = d
						foundDep = true
						break
					}
				}
			}

			if !foundDep {
				copyJob := &worker.AWSEC2CopyJob{
					Ami:          options.Ami,
					SourceRegion: options.Region,
					TargetRegion: img.Region,
					TargetName:   fmt.Sprintf("composer-api-%s", uuid.New().String()),
				}
				finalJob, err = h.server.workers.EnqueueAWSEC2CopyJob(copyJob, finalJob, channel)
				if err != nil {
					return HTTPErrorWithInternal(ErrorEnqueueingJob, err)
				}
			}
		}

		var shares []string
		awsT, ok := (osbuildJob.Targets[0].Options).(*target.AWSTargetOptions)
		if !ok {
			return HTTPError(ErrorUnknownUploadTarget)
		}
		if len(awsT.ShareWithAccounts) > 0 {
			shares = append(shares, awsT.ShareWithAccounts...)
		}
		if img.ShareWithAccounts != nil && len(*img.ShareWithAccounts) > 0 {
			shares = append(shares, (*img.ShareWithAccounts)...)
		}
		if len(shares) > 0 {
			shareJob := &worker.AWSEC2ShareJob{
				Ami:               shareAmi,
				Region:            shareRegion,
				ShareWithAccounts: shares,
			}
			finalJob, err = h.server.workers.EnqueueAWSEC2ShareJob(shareJob, finalJob, channel)
			if err != nil {
				return HTTPErrorWithInternal(ErrorEnqueueingJob, err)
			}
		}
	} else {
		return HTTPError(ErrorUnsupportedImage)
	}

	return ctx.JSON(http.StatusCreated, CloneComposeResponse{
		ObjectReference: ObjectReference{
			Href: fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/clone", jobId),
			Id:   finalJob.String(),
			Kind: "CloneComposeId",
		},
		Id: finalJob.String(),
	})
}

func (h *apiHandlers) GetCloneStatus(ctx echo.Context, id string) error {
	return h.server.EnsureJobChannel(h.getCloneStatus)(ctx, id)
}

func (h *apiHandlers) getCloneStatus(ctx echo.Context, id string) error {
	jobId, err := uuid.Parse(id)
	if err != nil {
		return HTTPError(ErrorInvalidComposeId)
	}

	jobType, err := h.server.workers.JobType(jobId)
	if err != nil {
		return HTTPError(ErrorComposeNotFound)
	}

	var us UploadStatus
	switch jobType {
	case worker.JobTypeAWSEC2Copy:
		var result worker.AWSEC2CopyJobResult
		info, err := h.server.workers.AWSEC2CopyJobInfo(jobId, &result)
		if err != nil {
			return HTTPError(ErrorGettingAWSEC2JobStatus)
		}

		us = UploadStatus{
			Status: uploadStatusFromJobStatus(info.JobStatus, result.JobError),
			Type:   UploadTypesAws,
			Options: AWSEC2UploadStatus{
				Ami:    result.Ami,
				Region: result.Region,
			},
		}
	case worker.JobTypeAWSEC2Share:
		var result worker.AWSEC2ShareJobResult
		info, err := h.server.workers.AWSEC2ShareJobInfo(jobId, &result)
		if err != nil {
			return HTTPError(ErrorGettingAWSEC2JobStatus)
		}

		us = UploadStatus{
			Status: uploadStatusFromJobStatus(info.JobStatus, result.JobError),
			Type:   UploadTypesAws,
			Options: AWSEC2UploadStatus{
				Ami:    result.Ami,
				Region: result.Region,
			},
		}
	default:
		return HTTPError(ErrorInvalidJobType)
	}

	return ctx.JSON(http.StatusOK, CloneStatus{
		ObjectReference: ObjectReference{
			Href: fmt.Sprintf("/api/image-builder-composer/v2/clones/%v", jobId),
			Id:   jobId.String(),
			Kind: "CloneComposeStatus",
		},
		UploadStatus: us,
	})
}

// TODO: determine upload status based on the target results, not job results
func uploadStatusFromJobStatus(js *worker.JobStatus, je *clienterrors.Error) UploadStatusValue {
	if je != nil || js.Canceled {
		return UploadStatusValueFailure
	}

	if js.Started.IsZero() {
		return UploadStatusValuePending
	}

	if js.Finished.IsZero() {
		return UploadStatusValueRunning
	}
	return UploadStatusValueSuccess
}
