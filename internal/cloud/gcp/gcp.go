package gcp

import (
	"context"
	"fmt"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

// GCP structure holds necessary information to authenticate and interact with GCP.
type GCP struct {
	creds *google.Credentials
}

// New returns an authenticated GCP instance, allowing to interact with GCP API.
func New(credentials []byte) (*GCP, error) {
	scopes := []string{
		compute.ComputeScope,   // permissions to image
		storage.ScopeReadWrite, // file upload
	}
	scopes = append(scopes, cloudbuild.DefaultAuthScopes()...) // image import

	var getCredsFunc func() (*google.Credentials, error)
	if credentials != nil {
		getCredsFunc = func() (*google.Credentials, error) {
			return google.CredentialsFromJSON(
				context.Background(),
				credentials,
				scopes...,
			)
		}
	} else {
		getCredsFunc = func() (*google.Credentials, error) {
			return google.FindDefaultCredentials(
				context.Background(),
				scopes...,
			)
		}
	}

	creds, err := getCredsFunc()
	if err != nil {
		return nil, fmt.Errorf("failed to get Google credentials: %v", err)
	}

	return &GCP{creds}, nil
}

// GetProjectID returns a string with the Project ID of the project, used for
// all GCP operations.
func (g *GCP) GetProjectID() string {
	return g.creds.ProjectID
}
