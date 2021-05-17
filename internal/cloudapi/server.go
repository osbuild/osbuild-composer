//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=cloudapi --generate types,spec,chi-server,client -o openapi.gen.go openapi.yml

package cloudapi

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"net/http"

	"github.com/go-chi/chi"
	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailure = "failure"
)

// Server represents the state of the cloud Server
type Server struct {
	workers     *worker.Server
	rpmMetadata rpmmd.RPMMD
	distros     *distro.Registry
}

// NewServer creates a new cloud server
func NewServer(workers *worker.Server, rpmMetadata rpmmd.RPMMD, distros *distro.Registry) *Server {
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
	r := chi.NewRouter()

	r.Route(path, func(r chi.Router) {
		HandlerFromMux(server, r)
	})

	return r
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
			pkgs, _, err := server.rpmMetadata.Depsolve(packages, repositories, distribution.ModulePlatformID(), arch.Name(), distribution.Releasever())
			if err != nil {
				http.Error(w, fmt.Sprintf("Failed to depsolve base packages for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err), http.StatusInternalServerError)
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

		manifest, err := imageType.Manifest(nil, imageOptions, repositories, pkgSpecSets, manifestSeed)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get manifest for for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err), http.StatusBadRequest)
			return
		}

		imageRequests[i].manifest = manifest
		imageRequests[i].arch = arch.Name()

		if len(ir.UploadRequests) != 1 {
			http.Error(w, "Only compose requests with a single upload target are currently supported", http.StatusBadRequest)
			return
		}
		uploadRequest := (ir.UploadRequests)[0]
		/* oneOf is not supported by the openapi generator so marshal and unmarshal the uploadrequest based on the type */
		if uploadRequest.Type == "aws" {
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
		} else {
			http.Error(w, "Unknown upload request type, only aws is supported", http.StatusBadRequest)
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

	response := ComposeStatus{
		ImageStatus: ImageStatus{
			Status: composeStatusFromJobStatus(status, &result),
			UploadStatus: &UploadStatus{
				Status: result.UploadStatus,
				Type:   "aws",
			},
		},
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic("Failed to write response")
	}
}

func composeStatusFromJobStatus(js *worker.JobStatus, result *worker.OSBuildJobResult) string {
	if js.Canceled {
		return StatusFailure
	}

	if js.Started.IsZero() {
		return StatusPending
	}

	if js.Finished.IsZero() {
		return StatusRunning
	}

	if result.Success {
		return StatusSuccess
	}

	return StatusFailure
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
