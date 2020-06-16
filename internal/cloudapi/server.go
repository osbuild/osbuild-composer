//go:generate go run github.com/deepmap/oapi-codegen/cmd/oapi-codegen --package=cloudapi --generate types,chi-server,client -o openapi.gen.go openapi.yml

package cloudapi

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/target"
	"github.com/osbuild/osbuild-composer/internal/worker"
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

// Serve serves the cloud API over the provided listener socket
func (server *Server) Serve(listener net.Listener) error {
	s := http.Server{Handler: Handler(server)}

	err := s.Serve(listener)
	if err != nil && err != http.ErrServerClosed {
		return err
	}

	return nil
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

	type imageRequest struct {
		manifest distro.Manifest
	}
	imageRequests := make([]imageRequest, len(request.ImageRequests))
	var targets []*target.Target

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
			repositories[j].BaseURL = repo.Baseurl
			repositories[j].RHSM = true
		}

		var bp = blueprint.Blueprint{}
		err = bp.Initialize()
		if err != nil {
			http.Error(w, "Unable to initialize blueprint", http.StatusInternalServerError)
			return
		}

		packageSpecs, _ := imageType.Packages(bp)
		packages, _, err := server.rpmMetadata.Depsolve(packageSpecs, nil, repositories, distribution.ModulePlatformID(), arch.Name())
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to depsolve base packages for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err), http.StatusInternalServerError)
			return
		}
		buildPackageSpecs := imageType.BuildPackages()
		buildPackages, _, err := server.rpmMetadata.Depsolve(buildPackageSpecs, nil, repositories, distribution.ModulePlatformID(), arch.Name())
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to depsolve build packages for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err), http.StatusInternalServerError)
			return
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

		manifest, err := imageType.Manifest(nil, imageOptions, repositories, packages, buildPackages)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get manifest for for %s/%s/%s: %s", ir.ImageType, ir.Architecture, request.Distribution, err), http.StatusBadRequest)
			return
		}

		imageRequests[i].manifest = manifest

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

			key := fmt.Sprintf("composer-cloudapi-%s", uuid.New().String())
			t := target.NewAWSTarget(&target.AWSTargetOptions{
				Filename:        imageType.Filename(),
				Region:          awsUploadOptions.Region,
				AccessKeyID:     awsUploadOptions.S3.AccessKeyId,
				SecretAccessKey: awsUploadOptions.S3.SecretAccessKey,
				Bucket:          awsUploadOptions.S3.Bucket,
				Key:             key,
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

	id, err := server.workers.Enqueue(ir.manifest, targets)
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
	composeId, err := uuid.Parse(id)

	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid format for parameter id: %s", err), http.StatusBadRequest)
		return
	}

	status, err := server.workers.JobStatus(composeId)
	if err != nil {
		http.Error(w, fmt.Sprintf("Job %s not found: %s", id, err), http.StatusNotFound)
		return
	}

	response := ComposeStatus{
		Status: status.State.ToString(), // TODO: map the status correctly
		ImageStatuses: &[]ImageStatus{
			{
				Status: status.State.ToString(), // TODO: map the status correctly
			},
		},
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		panic("Failed to write response")
	}
}
