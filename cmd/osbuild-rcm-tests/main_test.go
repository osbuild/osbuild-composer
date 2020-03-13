// osbuild-rcm-tests run tests against running osbuild-composer instance that was spawned using the
// osbuild-rcm.socket unit. It defines the expected use cases of the RCM API.

// +build integration

package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"net/http"
	"strings"
	"testing"
)

func TestRCM(t *testing.T) {
	// This is the first request the user sends to osbuild.
	submitBody := `
		{
			"distribution": "fedora-31",
		 	"image_types": ["qcow2"], 
 		 	"architectures":["x86_64"], 
			"repositories": [
				{"url": "http://download.fedoraproject.org/pub/fedora/linux/releases/30/Everything/x86_64/os/"}
			]
		}
	`
	// This is what the user gets back.
	var submitResponse struct {
		UUID uuid.UUID `json:"compose_id"`
	}
	// Then it is possible to get the status on the /v1/compose/<UUID> endpoint.
	// And finally this is the response from getting the status.
	var statusResponse struct {
		Status string `json:"status"`
	}

	// osbuild instance running on localhost
	socket := "http://127.0.0.1:80/"
	endpoint := "v1/compose"

	// Case 1: POST request

	resp, err := http.Post(socket+endpoint, "application/json", strings.NewReader(submitBody))
	require.Nilf(t, err, "Failed to submit a compose: %v", err)
	require.Equalf(t, resp.StatusCode, 200, "Error: the %v returned non 200 status. Full response: %v", endpoint, resp)
	err = json.NewDecoder(resp.Body).Decode(&submitResponse)
	require.Nilf(t, err, "Failed to decode JSON response from %v", endpoint)

	// Case 2: GET status

	statusEndpoint := endpoint + "/" + submitResponse.UUID.String()
	resp, err = http.Get(socket + statusEndpoint)
	require.Nilf(t, err, "Failed to get a status: %v", err)
	require.Equalf(t, resp.StatusCode, 200, "Error: the %v returned non 200 status. Full response: %v", endpoint, resp)
	err = json.NewDecoder(resp.Body).Decode(&statusResponse)
	require.Nilf(t, err, "Failed to decode JSON response from %v", endpoint)

}
