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
	submit_body := `
		{
			"distribution": "fedora-31",
		 	"image_types": ["qcow2"], 
 		 	"architectures":["x86_64"], 
			"repositories": [
				{"url": "http://download.fedoraproject.org/pub/fedora/linux/releases/30/Everything/x86_64/os/"}
			]
		}
	`
	socket := "http://127.0.0.1:80/"
	endpoint := "v1/compose"

	// Case 1: POST request

	resp, err := http.Post(socket + endpoint, "application/json", strings.NewReader(submit_body))
	var reply struct {
		UUID uuid.UUID `json:"compose_id"`
	}
	if err != nil {
		log.Fatal("Failed to submit a compose: ", err.Error())
	}
	if resp.StatusCode != 200 {
		log.Print("Error: the ", endpoint, " returned non 200 status. Full response: ", resp)
		failed = true
	} else {
		err = json.NewDecoder(resp.Body).Decode(&reply)
		if err != nil {
			log.Fatal("Failed to decode JSON response from ", endpoint)
		}
		log.Print("Success: the ", endpoint, " returned compose UUID: ", reply.UUID)
	}

	// Case 2: GET status

	statusEndpoint := endpoint + "/" + reply.UUID.String()
	resp, err = http.Get(socket + statusEndpoint)
	var status struct {
		Status string `json:"status"`
	}
	if err != nil {
		log.Fatal("Failed to get a status: ", err.Error())
	}
	if resp.StatusCode != 200 {
		log.Print("Error: the ", endpoint, " returned non 200 status. Full response: ", resp)
		failed = true
	} else {
		err = json.NewDecoder(resp.Body).Decode(&status)
		if err != nil {
			log.Fatal("Failed to decode JSON response from ", endpoint)
		}
		log.Print("Success: the ", statusEndpoint, " returned status: ", status.Status)
	}

	if failed {
		os.Exit(1)
	}
}
