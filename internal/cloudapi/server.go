//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=cloudapi --generate types,spec,client,server -o openapi.gen.go openapi.yml

package cloudapi

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/prometheus"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

// Server represents the state of the cloud Server
type Server struct {
	workers        *worker.Server
	rpmMetadata    rpmmd.RPMMD
	distros        *distroregistry.Registry
	identityFilter []string
}

type contextKey int

const (
	identityHeaderKey contextKey = iota
)

type IdentityHeader struct {
	Identity struct {
		AccountNumber string `json:"account_number"`
	} `json:"identity"`
}

type apiHandlers struct {
	server *Server
}

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
func (server *Server) Handler(path string, identityFilter []string) http.Handler {
	e := echo.New()

	if len(identityFilter) > 0 {
		server.identityFilter = identityFilter
		e.Use(server.VerifyIdentityHeader)
	}
	e.Use(server.IncRequests)
	handler := apiHandlers{
		server: server,
	}
	RegisterHandlers(e.Group(path), &handler)

	return e
}

// translate to echo
func (server *Server) VerifyIdentityHeader(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		idHeaderB64 := c.Request().Header.Get("X-Rh-Identity") //r.Header["X-Rh-Identity"]
		if len(idHeaderB64) != 1 {
			return echo.NewHTTPError(http.StatusNotFound, "Auth header is not present")
		}

		b64Result, err := base64.StdEncoding.DecodeString(idHeaderB64)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "Auth header has incorrect format")
		}

		var idHeader IdentityHeader
		err = json.Unmarshal([]byte(strings.TrimSuffix(fmt.Sprintf("%s", b64Result), "\n")), &idHeader)
		if err != nil {
			return echo.NewHTTPError(http.StatusNotFound, "Auth header has incorrect format")
		}

		for _, i := range server.identityFilter {
			if idHeader.Identity.AccountNumber == i {
				c.Set("IdentityHeader", idHeader)
				c.Set("IdentityHeaderKey", identityHeaderKey)
				return next(c)
			}
		}
		return echo.NewHTTPError(http.StatusNotFound, "Account not allowed")
	}
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
	err := json.NewDecoder(ctx.Request().Body).Decode(&request)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Could not parse request body")
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

	type imageRequest struct {
		manifest distro.Manifest
		arch     string
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
		pkgSpecSets := make(map[string][]rpmmd.PackageSpec)
		for name, packages := range packageSets {
			pkgs, _, err := h.server.rpmMetadata.Depsolve(packages, repositories, distribution.ModulePlatformID(), arch.Name())
			if err != nil {
				return echo.NewHTTPError(http.StatusInternalServerError, "Failed to depsolve base packages for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err)
			}
			pkgSpecSets[name] = pkgs
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

		manifest, err := imageType.Manifest(nil, imageOptions, repositories, pkgSpecSets, manifestSeed)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Failed to get manifest for for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err)
		}

		imageRequests[i].manifest = manifest
		imageRequests[i].arch = arch.Name()

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
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to enqueue manifest")
	}

	var response ComposeResult
	response.Id = id.String()

	ctx.Response().Header().Set("Content-Type", "application/json; charset=utf-8")
	ctx.Response().WriteHeader(http.StatusOK)
	return json.NewEncoder(ctx.Response()).Encode(response)
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
	ctx.Response().Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(ctx.Response()).Encode(response)
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
	ctx.Response().Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(ctx.Response()).Encode(spec)
}

// GetVersion handles a /version GET request
func (h *apiHandlers) GetVersion(ctx echo.Context) error {
	spec, err := GetSwagger()
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Could not load version")
	}
	version := Version{spec.Info.Version}
	ctx.Response().Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(ctx.Response()).Encode(version)
}
