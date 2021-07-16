//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=cloudapi --generate types,spec,chi-server,client -o openapi.gen.go openapi.yml

package cloudapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
	"github.com/osbuild/osbuild-composer/internal/osbuild1"
	"github.com/osbuild/osbuild-composer/internal/ostree"
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
	r := chi.NewRouter()

	if len(identityFilter) > 0 {
		server.identityFilter = identityFilter
		r.Use(server.VerifyIdentityHeader)
	}
	r.Route(path, func(r chi.Router) {
		HandlerFromMux(server, r)
	})

	return r
}

func (server *Server) VerifyIdentityHeader(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		const identityHeaderKey contextKey = iota
		type identityHeader struct {
			Identity struct {
				AccountNumber string `json:"account_number"`
			} `json:"identity"`
		}

		idHeaderB64 := r.Header["X-Rh-Identity"]
		if len(idHeaderB64) != 1 {
			http.Error(w, "Auth header is not present", http.StatusNotFound)
			return
		}

		b64Result, err := base64.StdEncoding.DecodeString(idHeaderB64[0])
		if err != nil {
			http.Error(w, "Auth header has incorrect format", http.StatusNotFound)
			return
		}

		var idHeader identityHeader
		err = json.Unmarshal([]byte(strings.TrimSuffix(fmt.Sprintf("%s", b64Result), "\n")), &idHeader)
		if err != nil {
			http.Error(w, "Auth header has incorrect format", http.StatusNotFound)
			return
		}

		for _, i := range server.identityFilter {
			if idHeader.Identity.AccountNumber == i {
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityHeaderKey, idHeader)))
				return
			}
		}
		http.Error(w, "Account not allowed", http.StatusNotFound)
	})
}

// Compose handles a new /compose POST request
func (server *Server) Compose(w http.ResponseWriter, r *http.Request) {
	contentType := r.Header["Content-Type"]
	if len(contentType) != 1 || contentType[0] != "application/json" {
		http.Error(w, "Only 'application/json' content type is supported", http.StatusUnsupportedMediaType)
		return
	}

	var request ComposeRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "Could not parse JSON body", http.StatusBadRequest)
		return
	}

	distribution := server.distros.GetDistro(request.Distribution)
	if distribution == nil {
		http.Error(w, fmt.Sprintf("Unsupported distribution: %s", request.Distribution), http.StatusBadRequest)
		return
	}

	var bp = blueprint.Blueprint{}
	err = bp.Initialize()
	if err != nil {
		http.Error(w, "Unable to initialize blueprint", http.StatusInternalServerError)
		return
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
			http.Error(w, fmt.Sprintf("Unsupported architecture '%s' for distribution '%s'", ir.Architecture, request.Distribution), http.StatusBadRequest)
			return
		}
		imageType, err := arch.GetImageType(ir.ImageType)
		if err != nil {
			http.Error(w, fmt.Sprintf("Unsupported image type '%s' for %s/%s", ir.ImageType, ir.Architecture, request.Distribution), http.StatusBadRequest)
			return
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
				http.Error(w, "Must specify baseurl, mirrorlist, or metalink", http.StatusBadRequest)
				return
			}
		}

		packageSets := imageType.PackageSets(bp)
		pkgSpecSets := make(map[string][]rpmmd.PackageSpec)
		for name, packages := range packageSets {
			pkgs, _, err := server.rpmMetadata.Depsolve(packages, repositories, distribution.ModulePlatformID(), arch.Name())
			if err != nil {
				var error_type int
				switch err.(type) {
				// Known DNF errors falls under BadRequest
				case *rpmmd.DNFError:
					error_type = http.StatusBadRequest
				// All other kind of errors are internal server Errors.
				// (json marshalling issues for instance)
				case error:
					error_type = http.StatusInternalServerError
				}
				http.Error(w, fmt.Sprintf("Failed to depsolve base packages for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err), error_type)
				return
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

		// set default ostree ref, if one not provided
		ostreeOptions := ir.Ostree
		if ostreeOptions == nil || ostreeOptions.Ref == nil {
			imageOptions.OSTree = distro.OSTreeImageOptions{Ref: imageType.OSTreeRef()}
		} else if !ostree.VerifyRef(*ostreeOptions.Ref) {
			http.Error(w, fmt.Sprintf("Invalid OSTree ref: %s", *ostreeOptions.Ref), http.StatusBadRequest)
			return
		} else {
			imageOptions.OSTree = distro.OSTreeImageOptions{Ref: *ostreeOptions.Ref}
		}

		var parent string
		if ostreeOptions != nil && ostreeOptions.Url != nil {
			imageOptions.OSTree.URL = *ostreeOptions.Url
			parent, err = ostree.ResolveRef(imageOptions.OSTree.URL, imageOptions.OSTree.Ref)
			if err != nil {
				http.Error(w, fmt.Sprintf("Error resolving OSTree repo %s: %s", imageOptions.OSTree.URL, err), http.StatusBadRequest)
				return
			}
			imageOptions.OSTree.Parent = parent
		}

		manifest, err := imageType.Manifest(nil, imageOptions, repositories, pkgSpecSets, manifestSeed)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get manifest for for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err), http.StatusBadRequest)
			return
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
				http.Error(w, "Unable to marshal aws upload request", http.StatusInternalServerError)
				return
			}
			err = json.Unmarshal(jsonUploadOptions, &awsUploadOptions)
			if err != nil {
				http.Error(w, "Unable to unmarshal aws upload request", http.StatusInternalServerError)
				return
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
				http.Error(w, "Unable to marshal aws upload request", http.StatusInternalServerError)
				return
			}
			err = json.Unmarshal(jsonUploadOptions, &awsS3UploadOptions)
			if err != nil {
				http.Error(w, "Unable to unmarshal aws upload request", http.StatusInternalServerError)
				return
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
				http.Error(w, "Unable to marshal gcp upload request", http.StatusInternalServerError)
				return
			}
			err = json.Unmarshal(jsonUploadOptions, &gcpUploadOptions)
			if err != nil {
				http.Error(w, "Unable to unmarshal gcp upload request", http.StatusInternalServerError)
				return
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
				http.Error(w, "Unable to marshal azure upload request", http.StatusInternalServerError)
				return
			}
			err = json.Unmarshal(jsonUploadOptions, &azureUploadOptions)
			if err != nil {
				http.Error(w, "Unable to unmarshal azure upload request", http.StatusInternalServerError)
				return
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
			http.Error(w, "Unknown upload request type, only 'aws', 'azure' and 'gcp' are supported", http.StatusBadRequest)
			return
		}
	}

	var ir imageRequest
	if len(imageRequests) == 1 {
		// NOTE: the store currently does not support multi-image composes
		ir = imageRequests[0]
	} else {
		http.Error(w, "Only single-image composes are currently supported", http.StatusBadRequest)
		return
	}

	id, err := server.workers.EnqueueOSBuild(ir.arch, &worker.OSBuildJob{
		Manifest: ir.manifest,
		Targets:  targets,
		Exports:  ir.exports,
	})
	if err != nil {
		http.Error(w, "Failed to enqueue manifest", http.StatusInternalServerError)
		return
	}

	var response ComposeResult
	response.Id = id.String()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic("Failed to write response")
	}
}

// ComposeStatus handles a /compose/{id} GET request
func (server *Server) ComposeStatus(w http.ResponseWriter, r *http.Request, id string) {
	jobId, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid format for parameter id: %s", err), http.StatusBadRequest)
		return
	}

	var result worker.OSBuildJobResult
	status, _, err := server.workers.JobStatus(jobId, &result)
	if err != nil {
		http.Error(w, fmt.Sprintf("Job %s not found: %s", id, err), http.StatusNotFound)
		return
	}

	var us *UploadStatus
	if result.TargetResults != nil {
		// Only single upload target is allowed, therefore only a single upload target result is allowed as well
		if len(result.TargetResults) != 1 {
			http.Error(w, fmt.Sprintf("Job %s returned more upload target results than allowed", id), http.StatusInternalServerError)
			return
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
			http.Error(w, fmt.Sprintf("Job %s returned unknown upload target results %s", id, tr.Name), http.StatusInternalServerError)
			return
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
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic("Failed to write response")
	}
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
func (server *Server) GetOpenapiJson(w http.ResponseWriter, r *http.Request) {
	spec, err := GetSwagger()
	if err != nil {
		http.Error(w, "Could not load openapi spec", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(spec)
	if err != nil {
		panic("Failed to write response")
	}
}

// GetVersion handles a /version GET request
func (server *Server) GetVersion(w http.ResponseWriter, r *http.Request) {
	spec, err := GetSwagger()
	if err != nil {
		http.Error(w, "Could not load version", http.StatusInternalServerError)
		return
	}
	version := Version{spec.Info.Version}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(version)
	if err != nil {
		panic("Failed to write response")
	}
}

// ComposeMetadata handles a /compose/{id}/metadata GET request
func (server *Server) ComposeMetadata(w http.ResponseWriter, r *http.Request, id string) {
	jobId, err := uuid.Parse(id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid format for parameter id: %s", err), http.StatusBadRequest)
		return
	}

	var result worker.OSBuildJobResult
	status, _, err := server.workers.JobStatus(jobId, &result)
	if err != nil {
		http.Error(w, fmt.Sprintf("Job %s not found: %s", id, err), http.StatusNotFound)
		return
	}

	var job worker.OSBuildJob
	if _, _, _, err = server.workers.Job(jobId, &job); err != nil {
		http.Error(w, fmt.Sprintf("Job %s not found: %s", id, err), http.StatusNotFound)
		return
	}

	if status.Finished.IsZero() {
		// job still running: empty response
		if err := json.NewEncoder(w).Encode(new(ComposeMetadata)); err != nil {
			panic("Failed to write response: " + err.Error())
		}
		return
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

	if err := json.NewEncoder(w).Encode(resp); err != nil {
		panic("Failed to write response: " + err.Error())
	}
}
