package gcp

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	cloudbuild "cloud.google.com/go/cloudbuild/apiv1"
	"cloud.google.com/go/storage"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/compute/v1"
)

// GCPCredentialsEnvName contains name of the environment variable used
// to specify the path to file with CGP service account credentials
const (
	GCPCredentialsEnvName string = "GOOGLE_APPLICATION_CREDENTIALS"
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

// GetCredentialsFromEnv reads the service account credentials JSON file from
// the path pointed to by the environment variable name stored in
// 'GCPCredentialsEnvName'. If the content of the JSON file was read successfully,
// its content is returned as []byte, otherwise nil is returned with proper error.
func GetCredentialsFromEnv() ([]byte, error) {
	credsPath, exists := os.LookupEnv(GCPCredentialsEnvName)

	if !exists {
		return nil, fmt.Errorf("'%s' env variable is not set", GCPCredentialsEnvName)
	}
	if credsPath == "" {
		return nil, fmt.Errorf("'%s' env variable is empty", GCPCredentialsEnvName)
	}

	var err error
	credentials, err := ioutil.ReadFile(credsPath)
	if err != nil {
		return nil, fmt.Errorf("Error while reading credentials file: %s", err)
	}

	return credentials, nil
}

// GetProjectID returns a string with the Project ID of the project, used for
// all GCP operations.
func (g *GCP) GetProjectID() string {
	return g.creds.ProjectID
}
