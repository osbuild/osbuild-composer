// osbuild-rcm-tests run tests against running osbuild-composer instance that was spawned using the
// osbuild-rcm.socket unit. It defines the expected use cases of the RCM API.
package main

import (
	"encoding/json"
	"github.com/google/uuid"
	"log"
	"net/http"
	"os"
	"strings"
)

func main() {
	failed := false
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

	resp, err := http.Post(socket + endpoint, "application/json", strings.NewReader(submitBody))
	if err != nil {
		log.Fatal("Failed to submit a compose: ", err.Error())
	}
	if resp.StatusCode != 200 {
		log.Print("Error: the ", endpoint, " returned non 200 status. Full response: ", resp)
		failed = true
	} else {
		err = json.NewDecoder(resp.Body).Decode(&submitResponse)
		if err != nil {
			log.Fatal("Failed to decode JSON response from ", endpoint)
		}
		log.Print("Success: the ", endpoint, " returned compose UUID: ", submitResponse.UUID)
	}

	// Case 2: GET status

	statusEndpoint := endpoint + "/" + submitResponse.UUID.String()
	resp, err = http.Get(socket + statusEndpoint)
	if err != nil {
		log.Fatal("Failed to get a status: ", err.Error())
	}
	if resp.StatusCode != 200 {
		log.Print("Error: the ", endpoint, " returned non 200 status. Full response: ", resp)
		failed = true
	} else {
		err = json.NewDecoder(resp.Body).Decode(&statusResponse)
		if err != nil {
			log.Fatal("Failed to decode JSON response from ", endpoint)
		}
		log.Print("Success: the ", statusEndpoint, " returned status: ", statusResponse.Status)
	}

	// If anything failed return non-zero exit code.
	if failed {
		os.Exit(1)
	}
}
