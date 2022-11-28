package gcp

import (
	"context"
	"fmt"
	"io/ioutil"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
)

// GCPCredentialsEnvName contains name of the environment variable used
// to specify the path to file with CGP service account credentials
const (
	//nolint:gosec
	GCPCredentialsEnvName string = "GOOGLE_APPLICATION_CREDENTIALS"
)

// GCP structure holds necessary information to authenticate and interact with GCP.
type GCP struct {
	creds *google.Credentials
}

// New returns an authenticated GCP instance, allowing to interact with GCP API.
func New(ctx context.Context, credentials []byte) (*GCP, error) {
	scopes := []string{storage.ScopeReadWrite}                 // file upload
	scopes = append(scopes, compute.DefaultAuthScopes()...)    // permissions to image
	scopes = append(scopes, cloudbuild.DefaultAuthScopes()...) // image import

	var getCredsFunc func() (*google.Credentials, error)
	if credentials != nil {
		getCredsFunc = func() (*google.Credentials, error) {
			return google.CredentialsFromJSON(
				ctx,
				credentials,
				scopes...,
			)
		}
	} else {
		getCredsFunc = func() (*google.Credentials, error) {
			return google.FindDefaultCredentials(
				ctx,
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

// NewFromFile loads the credentials from a file and returns an authenticated
// *GCP object instance.
func NewFromFile(ctx context.Context, path string) (*GCP, error) {
	gcpCredentials, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot load GCP credentials from file %q: %v", path, err)
	}
	return New(ctx, gcpCredentials)
}

// GetProjectID returns a string with the Project ID of the project, used for
// all GCP operations.
func (g *GCP) GetProjectID() string {
	return g.creds.ProjectID
}
