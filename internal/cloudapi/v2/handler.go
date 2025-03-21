//go:generate go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen --package=v2 --generate types,spec,server -o openapi.v2.gen.go openapi.v2.yml
package v2

import (
	"cmp"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/jsondb"
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

	apiError := APIError(find(ServiceErrorCode(errorId)), ctx, nil)
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
	repositories []rpmmd.RepoConfig
	imageOptions distro.ImageOptions
	targets      []*target.Target
	blueprint    blueprint.Blueprint
	manifestSeed int64
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

	irs, err := request.GetImageRequests(h.server.distros, h.server.repos)
	if err != nil {
		return err
	}

	var id uuid.UUID
	if request.Koji != nil {
		id, err = h.server.enqueueKojiCompose(uint64(request.Koji.TaskId), request.Koji.Server, request.Koji.Name, request.Koji.Version, request.Koji.Release, irs, channel)
		if err != nil {
			return err
		}
	} else {
		id, err = h.server.enqueueCompose(irs, channel)
		if err != nil {
			return err
		}
	}

	ctx.Logger().Infof("Job ID %s enqueued for operationID %s", id, ctx.Get(common.OperationIDKey))

	// Save the request in the artifacts directory, log errors but continue
	if err := saveComposeRequest(h.server.workers.ArtifactsDir(), id, request); err != nil {
		ctx.Logger().Warnf("Failed to save compose request: %v", err)
	}

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
	case ImageTypesAzureEap7Rhui:
		return "azure-eap7-rhui"
	case ImageTypesAzureSapRhui:
		return "azure-sap-rhui"
	case ImageTypesGuestImage:
		return "qcow2"
	case ImageTypesVsphere:
		return "vmdk"
	case ImageTypesVsphereOva:
		return "ova"
	case ImageTypesImageInstaller:
		return "image-installer"
	case ImageTypesEdgeCommit:
		return "rhel-edge-commit"
	case ImageTypesEdgeContainer:
		return "rhel-edge-container"
	case ImageTypesEdgeInstaller:
		return "rhel-edge-installer"
	case ImageTypesIotBootableContainer:
		return "iot-bootable-container"
	case ImageTypesIotCommit:
		return "iot-commit"
	case ImageTypesIotContainer:
		return "iot-container"
	case ImageTypesIotInstaller:
		return "iot-installer"
	case ImageTypesIotSimplifiedInstaller:
		return "iot-simplified-installer"
	case ImageTypesIotRawImage:
		return "iot-raw-image"
	case ImageTypesLiveInstaller:
		return "live-installer"
	case ImageTypesMinimalRaw:
		return "minimal-raw"
	case ImageTypesOci:
		return "oci"
	case ImageTypesWsl:
		return "wsl"
	}
	return ""
}

func (h *apiHandlers) targetResultToUploadStatus(jobId uuid.UUID, t *target.TargetResult) (*UploadStatus, error) {
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
	case target.TargetNameOCIObjectStorage:
		uploadType = UploadTypesOciObjectstorage
		ociOptions := t.Options.(*target.OCIObjectStorageTargetResultOptions)
		uploadOptions = OCIUploadStatus{
			Url: ociOptions.URL,
		}
	case target.TargetNamePulpOSTree:
		uploadType = UploadTypesPulpOstree
		pulpOSTreeOptions := t.Options.(*target.PulpOSTreeTargetResultOptions)
		uploadOptions = PulpOSTreeUploadStatus{
			RepoUrl: pulpOSTreeOptions.RepoURL,
		}
	case target.TargetNameWorkerServer:
		uploadType = UploadTypesLocal
		workerServerOptions := t.Options.(*target.WorkerServerTargetResultOptions)
		absPath, err := h.server.workers.JobArtifactLocation(jobId, workerServerOptions.ArtifactRelPath)
		if err != nil {
			return nil, fmt.Errorf("unable to find job artifact: %w", err)
		}
		uploadOptions = LocalUploadStatus{
			ArtifactPath: absPath,
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

// GetComposeList returns a list of the root job UUIDs
func (h *apiHandlers) GetComposeList(ctx echo.Context) error {
	jobs, err := h.server.workers.AllRootJobIDs()
	if err != nil {
		return HTTPErrorWithInternal(ErrorGettingComposeList, err)
	}

	// Gather up the details of each job
	var stats []ComposeStatus
	for _, jid := range jobs {
		s, err := h.getJobIDComposeStatus(jid)
		if err != nil {
			// TODO log this error?
			continue
		}
		stats = append(stats, s)
	}
	slices.SortFunc(stats, func(a, b ComposeStatus) int {
		return cmp.Compare(a.Id, b.Id)
	})

	return ctx.JSON(http.StatusOK, stats)
}

func (h *apiHandlers) GetComposeStatus(ctx echo.Context, id string) error {
	return h.server.EnsureJobChannel(h.getComposeStatusImpl)(ctx, id)
}

func (h *apiHandlers) getComposeStatusImpl(ctx echo.Context, id string) error {
	jobId, err := uuid.Parse(id)
	if err != nil {
		return HTTPError(ErrorInvalidComposeId)
	}

	response, err := h.getJobIDComposeStatus(jobId)
	if err != nil {
		return err
	}
	return ctx.JSON(http.StatusOK, response)
}

// getJobIDComposeStatus returns the ComposeStatus for the job
// or an HTTPError
func (h *apiHandlers) getJobIDComposeStatus(jobId uuid.UUID) (ComposeStatus, error) {
	jobType, err := h.server.workers.JobType(jobId)
	if err != nil {
		return ComposeStatus{}, HTTPError(ErrorComposeNotFound)
	}

	if jobType == worker.JobTypeOSBuild {
		var result worker.OSBuildJobResult
		jobInfo, err := h.server.workers.OSBuildJobInfo(jobId, &result)
		if err != nil {
			return ComposeStatus{}, HTTPError(ErrorMalformedOSBuildJobResult)
		}

		jobError, err := h.server.workers.JobDependencyChainErrors(jobId)
		if err != nil {
			return ComposeStatus{}, HTTPError(ErrorGettingBuildDependencyStatus)
		}

		var uploadStatuses *[]UploadStatus
		var us0 *UploadStatus
		if result.TargetResults != nil {
			statuses := make([]UploadStatus, len(result.TargetResults))
			for idx := range result.TargetResults {
				tr := result.TargetResults[idx]
				us, err := h.targetResultToUploadStatus(jobId, tr)
				if err != nil {
					return ComposeStatus{}, HTTPErrorWithInternal(ErrorUnknownUploadTarget, err)
				}
				us.Status = uploadStatusFromJobStatus(jobInfo.JobStatus, result.JobError)
				statuses[idx] = *us
			}

			if len(statuses) > 0 {
				// make sure uploadStatuses remains nil if the array is empty but not nill
				uploadStatuses = &statuses
				// get first upload status if there's at least one
				us0 = &statuses[0]
			}
		}

		return ComposeStatus{
			ObjectReference: ObjectReference{
				Href: fmt.Sprintf("/api/image-builder-composer/v2/composes/%v", jobId),
				Id:   jobId.String(),
				Kind: "ComposeStatus",
			},
			Status: composeStatusFromOSBuildJobStatus(jobInfo.JobStatus, &result),
			ImageStatus: ImageStatus{
				Status:         imageStatusFromOSBuildJobStatus(jobInfo.JobStatus, &result),
				Error:          composeStatusErrorFromJobError(jobError),
				UploadStatus:   us0, // add the first upload status to the old top-level field
				UploadStatuses: uploadStatuses,
			},
		}, nil
	} else if jobType == worker.JobTypeKojiFinalize {
		var result worker.KojiFinalizeJobResult
		finalizeInfo, err := h.server.workers.KojiFinalizeJobInfo(jobId, &result)
		if err != nil {
			return ComposeStatus{}, HTTPError(ErrorMalformedOSBuildJobResult)
		}
		if len(finalizeInfo.Deps) < 2 {
			return ComposeStatus{}, HTTPError(ErrorUnexpectedNumberOfImageBuilds)
		}
		var initResult worker.KojiInitJobResult
		_, err = h.server.workers.KojiInitJobInfo(finalizeInfo.Deps[0], &initResult)
		if err != nil {
			return ComposeStatus{}, HTTPError(ErrorMalformedOSBuildJobResult)
		}
		var buildJobResults []worker.OSBuildJobResult
		var buildJobStatuses []ImageStatus
		for i := 1; i < len(finalizeInfo.Deps); i++ {
			var buildJobResult worker.OSBuildJobResult
			buildInfo, err := h.server.workers.OSBuildJobInfo(finalizeInfo.Deps[i], &buildJobResult)
			if err != nil {
				return ComposeStatus{}, HTTPError(ErrorMalformedOSBuildJobResult)
			}
			buildJobError, err := h.server.workers.JobDependencyChainErrors(finalizeInfo.Deps[i])
			if err != nil {
				return ComposeStatus{}, HTTPError(ErrorGettingBuildDependencyStatus)
			}

			var uploadStatuses *[]UploadStatus
			var us0 *UploadStatus
			if buildJobResult.TargetResults != nil {
				// can't set the array size because koji targets wont be counted
				statuses := make([]UploadStatus, 0, len(buildJobResult.TargetResults))
				for idx := range buildJobResult.TargetResults {
					tr := buildJobResult.TargetResults[idx]
					if tr.Name != target.TargetNameKoji {
						us, err := h.targetResultToUploadStatus(jobId, tr)
						if err != nil {
							return ComposeStatus{}, HTTPErrorWithInternal(ErrorUnknownUploadTarget, err)
						}
						us.Status = uploadStatusFromJobStatus(buildInfo.JobStatus, result.JobError)
						statuses = append(statuses, *us)
					}
				}

				if len(statuses) > 0 {
					// make sure uploadStatuses remains nil if the array is empty but not nill
					uploadStatuses = &statuses
					// get first upload status if there's at least one
					us0 = &statuses[0]
				}
			}

			buildJobResults = append(buildJobResults, buildJobResult)
			buildJobStatuses = append(buildJobStatuses, ImageStatus{
				Status:         imageStatusFromKojiJobStatus(buildInfo.JobStatus, &initResult, &buildJobResult),
				Error:          composeStatusErrorFromJobError(buildJobError),
				UploadStatus:   us0, // add the first upload status to the old top-level field
				UploadStatuses: uploadStatuses,
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
		/* #nosec G115 */
		buildID := int(initResult.BuildID)
		// Make sure signed integer conversion didn't underflow
		if buildID < 0 {
			err := fmt.Errorf("BuildID integer underflow: %d", initResult.BuildID)
			return ComposeStatus{}, HTTPErrorWithInternal(ErrorMalformedOSBuildJobResult, err)
		}
		if buildID != 0 {
			response.KojiStatus.BuildId = &buildID
		}
		return response, nil
	} else {
		return ComposeStatus{}, HTTPError(ErrorInvalidJobType)
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

	// Get the original compose request, if present
	request, err := readComposeRequest(h.server.workers.ArtifactsDir(), jobId)
	if err != nil {
		ctx.Logger().Warnf("Failed to read compose request: %v", err)
	}

	if buildInfo.JobStatus.Finished.IsZero() {
		// job still running: empty response
		return ctx.JSON(200, ComposeMetadata{
			ObjectReference: ObjectReference{
				Href: fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/metadata", jobId),
				Id:   jobId.String(),
				Kind: "ComposeMetadata",
			},
			Request: request,
		})
	}

	if buildInfo.JobStatus.Canceled || !result.Success {
		// job canceled or failed, empty response
		return ctx.JSON(200, ComposeMetadata{
			ObjectReference: ObjectReference{
				Href: fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/metadata", jobId),
				Id:   jobId.String(),
				Kind: "ComposeMetadata",
			},
			Request: request,
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
		Request:  request,
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
					PackageMetadataCommon: PackageMetadataCommon{
						Type:      "rpm",
						Name:      rpm.Name,
						Version:   rpm.Version,
						Release:   rpm.Release,
						Epoch:     rpm.Epoch,
						Arch:      rpm.Arch,
						Signature: osbuild.RPMPackageMetadataToSignature(rpm),
					},
					Sigmd5: rpm.SigMD5,
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

func manifestJobResultsFromJobDeps(w *worker.Server, deps []uuid.UUID) (*worker.JobInfo, *worker.ManifestJobByIDResult, error) {
	var manifestResult worker.ManifestJobByIDResult

	for i := 0; i < len(deps); i++ {
		depType, err := w.JobType(deps[i])
		if err != nil {
			return nil, nil, err
		}
		if depType == worker.JobTypeManifestIDOnly {
			manifestJobInfo, err := w.ManifestJobInfo(deps[i], &manifestResult)
			if err != nil {
				return nil, nil, err
			}
			return manifestJobInfo, &manifestResult, nil
		}
	}

	return nil, nil, fmt.Errorf("no %q job found in the dependencies", worker.JobTypeManifestIDOnly)
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

			var mf manifest.OSBuildManifest

			switch buildJobType {
			case worker.JobTypeOSBuild:
				var buildJob worker.OSBuildJob
				err = h.server.workers.OSBuildJob(finalizeInfo.Deps[i], &buildJob)
				if err != nil {
					return HTTPErrorWithInternal(ErrorComposeNotFound, err)
				}

				if len(buildJob.Manifest) != 0 {
					mf = buildJob.Manifest
				} else {
					buildInfo, err := h.server.workers.OSBuildJobInfo(finalizeInfo.Deps[i], &worker.OSBuildJobResult{})
					if err != nil {
						return HTTPErrorWithInternal(ErrorComposeNotFound, err)
					}
					_, manifestResult, err := manifestJobResultsFromJobDeps(h.server.workers, buildInfo.Deps)
					if err != nil {
						return HTTPErrorWithInternal(ErrorComposeNotFound, fmt.Errorf("job %q: %v", jobId, err))
					}
					mf = manifestResult.Manifest
				}

			default:
				return HTTPErrorWithInternal(ErrorInvalidJobType,
					fmt.Errorf("unexpected job type in koji compose dependencies: %q", buildJobType))
			}
			manifestBlobs = append(manifestBlobs, mf)
		}

	case worker.JobTypeOSBuild:
		var buildJob worker.OSBuildJob
		err = h.server.workers.OSBuildJob(jobId, &buildJob)
		if err != nil {
			return HTTPErrorWithInternal(ErrorComposeNotFound, err)
		}

		var mf manifest.OSBuildManifest
		if len(buildJob.Manifest) != 0 {
			mf = buildJob.Manifest
		} else {
			buildInfo, err := h.server.workers.OSBuildJobInfo(jobId, &worker.OSBuildJobResult{})
			if err != nil {
				return HTTPErrorWithInternal(ErrorComposeNotFound, err)
			}
			_, manifestResult, err := manifestJobResultsFromJobDeps(h.server.workers, buildInfo.Deps)
			if err != nil {
				return HTTPErrorWithInternal(ErrorComposeNotFound, fmt.Errorf("job %q: %v", jobId, err))
			}
			mf = manifestResult.Manifest
		}
		manifestBlobs = append(manifestBlobs, mf)

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

// sbomsFromOSBuildJob extracts SBOM documents from dependencies of an OSBuild job.
func sbomsFromOSBuildJob(w *worker.Server, osbuildJobUUID uuid.UUID) ([]ImageSBOM, error) {
	var osbuildJobResult worker.OSBuildJobResult
	osbuildJobInfo, err := w.OSBuildJobInfo(osbuildJobUUID, &osbuildJobResult)
	if err != nil {
		return nil, fmt.Errorf("Failed to get results for OSBuild job %q: %v", osbuildJobUUID, err)
	}

	pipelineNameToPurpose := func(pipelineName string) (ImageSBOMPipelinePurpose, error) {
		if slices.Contains(osbuildJobResult.PipelineNames.Payload, pipelineName) {
			return ImageSBOMPipelinePurposeImage, nil
		}
		if slices.Contains(osbuildJobResult.PipelineNames.Build, pipelineName) {
			return ImageSBOMPipelinePurposeBuildroot, nil
		}
		return "", fmt.Errorf("Pipeline %q is not listed as either a payload or build pipeline", pipelineName)
	}

	// SBOMs are attached to the depsolve job results.
	// Depsolve jobs are dependencies of Manifest job.
	// Manifest job is a dependency of OSBuild job.
	manifesJobInfo, _, err := manifestJobResultsFromJobDeps(w, osbuildJobInfo.Deps)
	if err != nil {
		return nil, fmt.Errorf("Failed to get manifest job info for OSBuild job %q: %v", osbuildJobUUID, err)
	}

	var imageSBOMs []ImageSBOM
	for _, manifestDepUUID := range manifesJobInfo.Deps {
		depJobType, err := w.JobType(manifestDepUUID)
		if err != nil {
			return nil, fmt.Errorf("Failed to get job type for dependency %q: %v", manifestDepUUID, err)
		}

		if depJobType != worker.JobTypeDepsolve {
			continue
		}

		var depsolveJobResult worker.DepsolveJobResult
		_, err = w.DepsolveJobInfo(manifestDepUUID, &depsolveJobResult)
		if err != nil {
			return nil, fmt.Errorf("Failed to get results for depsolve job %q: %v", manifestDepUUID, err)
		}

		if depsolveJobResult.SbomDocs == nil {
			return nil, fmt.Errorf("depsolve job %q: missing SBOMs", manifestDepUUID)
		}

		for pipelineName, sbomDoc := range depsolveJobResult.SbomDocs {
			purpose, err := pipelineNameToPurpose(pipelineName)
			if err != nil {
				return nil, fmt.Errorf("Failed to determine purpose for pipeline %q: %v", pipelineName, err)
			}

			var sbomType ImageSBOMSbomType
			switch sbomDoc.DocType {
			case sbom.StandardTypeSpdx:
				sbomType = ImageSBOMSbomTypeSpdx
			default:
				return nil, fmt.Errorf("Unknown SBOM type %q attached to depsolve job %q", sbomDoc.DocType, manifestDepUUID)
			}

			imageSBOMs = append(imageSBOMs, ImageSBOM{
				PipelineName:    pipelineName,
				PipelinePurpose: purpose,
				Sbom:            sbomDoc.Document,
				SbomType:        sbomType,
			})
		}

		// There should be only one depsolve job per OSBuild job
		break
	}

	if len(imageSBOMs) == 0 {
		return nil, fmt.Errorf("OSBuild job %q: manifest job dependency is missing depsolve job dependency", osbuildJobUUID)
	}

	// Sort the SBOMs by pipeline name to ensure consistent ordering.
	// The SBOM documents are attached to the depsolve job results, in a map where the key is the pipeline name.
	// The order of the keys in the map is not guaranteed to be consistent across different runs.
	sort.Slice(imageSBOMs, func(i, j int) bool {
		return imageSBOMs[i].PipelineName < imageSBOMs[j].PipelineName
	})

	return imageSBOMs, nil
}

// GetComposeSBOMs returns the SBOM documents for a given Compose (multiple SBOMs for each image).
func (h *apiHandlers) GetComposeSBOMs(ctx echo.Context, id string) error {
	return h.server.EnsureJobChannel(h.getComposeSBOMsImpl)(ctx, id)
}

func (h *apiHandlers) getComposeSBOMsImpl(ctx echo.Context, id string) error {
	jobId, err := uuid.Parse(id)
	if err != nil {
		return HTTPError(ErrorInvalidComposeId)
	}

	jobType, err := h.server.workers.JobType(jobId)
	if err != nil {
		return HTTPError(ErrorComposeNotFound)
	}

	var items [][]ImageSBOM

	switch jobType {
	// Koji compose
	case worker.JobTypeKojiFinalize:
		var finalizeResult worker.KojiFinalizeJobResult
		finalizeInfo, err := h.server.workers.KojiFinalizeJobInfo(jobId, &finalizeResult)
		if err != nil {
			return HTTPErrorWithInternal(ErrorComposeNotFound, err)
		}

		for _, kojiFinalizeDepUUID := range finalizeInfo.Deps {
			buildJobType, err := h.server.workers.JobType(kojiFinalizeDepUUID)
			if err != nil {
				return HTTPErrorWithInternal(ErrorComposeNotFound, err)
			}

			switch buildJobType {
			case worker.JobTypeKojiInit:
				continue

			case worker.JobTypeOSBuild:
				imageSBOMs, err := sbomsFromOSBuildJob(h.server.workers, kojiFinalizeDepUUID)
				if err != nil {
					return HTTPErrorWithInternal(ErrorComposeNotFound,
						fmt.Errorf("Failed to get SBOMs for OSBuild job %q: %v", kojiFinalizeDepUUID, err))
				}
				items = append(items, imageSBOMs)

			default:
				return HTTPErrorWithInternal(ErrorInvalidJobType,
					fmt.Errorf("unexpected job type in koji compose dependencies: %q", buildJobType))
			}
		}

	// non-Koji compose
	case worker.JobTypeOSBuild:
		imageSBOMs, err := sbomsFromOSBuildJob(h.server.workers, jobId)
		if err != nil {
			return HTTPErrorWithInternal(ErrorComposeNotFound,
				fmt.Errorf("Failed to get SBOMs for OSBuild job %q: %v", jobId, err))
		}
		items = append(items, imageSBOMs)

	default:
		return HTTPError(ErrorInvalidJobType)
	}

	resp := &ComposeSBOMs{
		ObjectReference: ObjectReference{
			Href: fmt.Sprintf("/api/image-builder-composer/v2/composes/%v/sboms", jobId),
			Id:   jobId.String(),
			Kind: "ComposeSBOMs",
		},
		Items: items,
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

	if repo.Baseurl != nil && *repo.Baseurl != "" {
		repoConfig.BaseURLs = []string{*repo.Baseurl}
	} else if repo.Mirrorlist != nil {
		repoConfig.MirrorList = *repo.Mirrorlist
	} else if repo.Metalink != nil {
		repoConfig.Metalink = *repo.Metalink
	} else {
		return nil, HTTPError(ErrorInvalidRepository)
	}

	if repo.Gpgkey != nil && *repo.Gpgkey != "" {
		repoConfig.GPGKeys = []string{*repo.Gpgkey}
	}
	if repo.IgnoreSsl != nil {
		repoConfig.IgnoreSSL = repo.IgnoreSsl
	}

	if repo.CheckGpg != nil {
		repoConfig.CheckGPG = repo.CheckGpg
	}
	if repo.Gpgkey != nil && *repo.Gpgkey != "" {
		repoConfig.GPGKeys = []string{*repo.Gpgkey}
	}
	if repo.IgnoreSsl != nil {
		repoConfig.IgnoreSSL = repo.IgnoreSsl
	}
	if repo.CheckRepoGpg != nil {
		repoConfig.CheckRepoGPG = repo.CheckRepoGpg
	}
	if repo.ModuleHotfixes != nil {
		repoConfig.ModuleHotfixes = repo.ModuleHotfixes
	}

	if repoConfig.CheckGPG != nil && *repoConfig.CheckGPG && len(repoConfig.GPGKeys) == 0 {
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
	us, err = h.targetResultToUploadStatus(jobId, osbuildResult.TargetResults[0])
	if err != nil {
		return HTTPErrorWithInternal(ErrorUnknownUploadTarget, err)
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

// PostDepsolveBlueprint depsolves the packages in a blueprint and returns
// the results as a list of rpmmd.PackageSpecs
func (h *apiHandlers) PostDepsolveBlueprint(ctx echo.Context) error {
	var request DepsolveRequest
	err := ctx.Bind(&request)
	if err != nil {
		return err
	}

	// Depsolve the requested blueprint
	// Any errors returned are suitable as a response
	deps, err := request.Depsolve(h.server.distros, h.server.repos, h.server.workers)
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK,
		DepsolveResponse{
			Packages: packageSpecToPackageMetadata(deps),
		})
}

// packageSpecToPackageMetadata converts the rpmmd.PackageSpec to PackageMetadata
// This is used to return package information from the blueprint depsolve request
// using the common PackageMetadata format from the openapi schema.
func packageSpecToPackageMetadata(pkgspecs []rpmmd.PackageSpec) []PackageMetadataCommon {
	packages := make([]PackageMetadataCommon, 0)
	for _, rpm := range pkgspecs {
		// Set epoch if it is not 0

		var epoch *string
		if rpm.Epoch > 0 {
			epoch = common.ToPtr(strconv.FormatUint(uint64(rpm.Epoch), 10))
		}
		packages = append(packages,
			PackageMetadataCommon{
				Type:     "rpm",
				Name:     rpm.Name,
				Version:  rpm.Version,
				Release:  rpm.Release,
				Epoch:    epoch,
				Arch:     rpm.Arch,
				Checksum: common.ToPtr(rpm.Checksum),
			},
		)
	}
	return packages
}

// packageListToPackageDetails converts the rpmmd.PackageList to PackageDetails
// This is used to return detailed package information from the package search
func packageListToPackageDetails(packages rpmmd.PackageList) []PackageDetails {
	details := make([]PackageDetails, 0)
	for _, rpm := range packages {
		d := PackageDetails{
			Name:    rpm.Name,
			Version: rpm.Version,
			Release: rpm.Release,
			Arch:    rpm.Arch,
		}

		// Set epoch if it is not 0
		if rpm.Epoch > 0 {
			d.Epoch = common.ToPtr(strconv.FormatUint(uint64(rpm.Epoch), 10))
		}

		// Set buildtime to a RFC3339 string
		d.Buildtime = common.ToPtr(rpm.BuildTime.Format(time.RFC3339))
		if len(rpm.Summary) > 0 {
			d.Summary = common.ToPtr(rpm.Summary)
		}
		if len(rpm.Description) > 0 {
			d.Description = common.ToPtr(rpm.Description)
		}
		if len(rpm.URL) > 0 {
			d.Url = common.ToPtr(rpm.URL)
		}
		if len(rpm.License) > 0 {
			d.License = common.ToPtr(rpm.License)
		}

		details = append(details, d)
	}

	return details
}

// PostSearchPackages searches for packages and returns detailed
// information about the matches.
func (h *apiHandlers) PostSearchPackages(ctx echo.Context) error {
	var request SearchPackagesRequest
	err := ctx.Bind(&request)
	if err != nil {
		return err
	}

	// Search for the listed packages
	// Any errors returned are suitable as a response
	packages, err := request.Search(h.server.distros, h.server.repos, h.server.workers)
	if err != nil {
		return err
	}

	return ctx.JSON(http.StatusOK,
		SearchPackagesResponse{
			Packages: packageListToPackageDetails(packages),
		})
}

// GetDistributionList returns the list of all supported distribution repositories
// It is arranged by distro name -> architecture -> image type
func (h *apiHandlers) GetDistributionList(ctx echo.Context) error {
	distros := make(map[string]map[string]map[string][]rpmmd.RepoConfig)
	distroNames := h.server.repos.ListDistros()
	sort.Strings(distroNames)
	for _, distroName := range distroNames {
		distro := h.server.distros.GetDistro(distroName)
		if distro == nil {
			continue
		}

		for _, archName := range distro.ListArches() {
			arch, _ := distro.GetArch(archName)
			for _, imageType := range arch.ListImageTypes() {
				repos, err := h.server.repos.ReposByImageTypeName(distroName, archName, imageType)
				if err != nil {
					continue
				}

				if _, ok := distros[distroName]; !ok {
					distros[distroName] = make(map[string]map[string][]rpmmd.RepoConfig)
				}
				if _, ok := distros[distroName][archName]; !ok {
					distros[distroName][archName] = make(map[string][]rpmmd.RepoConfig)
				}

				distros[distroName][archName][imageType] = repos
			}
		}
	}

	return ctx.JSON(http.StatusOK, distros)
}

// GetComposeDownload downloads a compose artifact
func (h *apiHandlers) GetComposeDownload(ctx echo.Context, id string) error {
	return h.server.EnsureJobChannel(h.getComposeDownloadImpl)(ctx, id)
}

func (h *apiHandlers) getComposeDownloadImpl(ctx echo.Context, id string) error {
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
	jobInfo, err := h.server.workers.OSBuildJobInfo(jobId, &osbuildResult)
	if err != nil {
		return HTTPErrorWithInternal(ErrorGettingOSBuildJobStatus, err)
	}

	// Is it finished?
	if jobInfo.JobStatus.Finished.IsZero() {
		err := fmt.Errorf("Cannot access artifacts before job is finished: %s", jobId)
		return HTTPErrorWithInternal(ErrorArtifactNotFound, err)
	}

	// Building only supports one target, but that may change, so make sure to check.
	// NOTE: TargetResults isn't populated until it is finished
	if len(osbuildResult.TargetResults) != 1 {
		msg := fmt.Errorf("%#v", osbuildResult.TargetResults)
		//return HTTPError(ErrorSeveralUploadTargets)
		return HTTPErrorWithInternal(ErrorSeveralUploadTargets, msg)
	}
	tr := osbuildResult.TargetResults[0]
	if tr.OsbuildArtifact == nil {
		return HTTPError(ErrorArtifactNotFound)
	}

	// NOTE: This also returns an error if the job isn't finished or it cannot find the file
	file, err := h.server.workers.JobArtifactLocation(jobId, tr.OsbuildArtifact.ExportFilename)
	if err != nil {
		return HTTPErrorWithInternal(ErrorArtifactNotFound, err)
	}
	return ctx.Attachment(file, fmt.Sprintf("%s-%s", jobId, tr.OsbuildArtifact.ExportFilename))
}

// saveComposeRequest stores the compose request's json on disk
// This is saved in the ComposeRequest directory of the artifacts directory
// If no artifacts directory has been configured it saves nothing and silently returns
func saveComposeRequest(artifactsDir string, id uuid.UUID, request ComposeRequest) error {
	if artifactsDir == "" {
		return nil
	}
	p := path.Join(artifactsDir, "ComposeRequest")
	err := os.MkdirAll(p, 0700)
	if err != nil {
		return err
	}
	db := jsondb.New(p, 0700)
	return db.Write(id.String(), request)
}

// readComposeRequest reads the compose request's json on disk
// This reads the original compose request json from the ComposeRequest directory of
// the artifacts directory.
// If no artifacts directory had been setup it silently returns nothing
func readComposeRequest(artifactsDir string, id uuid.UUID) (*ComposeRequest, error) {
	if artifactsDir == "" {
		return nil, nil
	}
	p := path.Join(artifactsDir, "ComposeRequest")
	err := os.MkdirAll(p, 0700)
	if err != nil {
		return nil, err
	}
	db := jsondb.New(p, 0700)
	var request ComposeRequest
	exists, err := db.Read(id.String(), &request)
	if !exists {
		return nil, err
	}
	return &request, err
}
