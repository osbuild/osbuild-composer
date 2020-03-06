// osbuild-rcm-tests run tests against running osbuild-composer instance that was spawned using the
// osbuild-rcm.socket unit. It defines the expected use cases of the RCM API.
package main

import (
	"bytes"
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
	resp, err := http.Post(socket + endpoint, "application/json", strings.NewReader(submit_body))
	if err != nil {
		log.Fatal("Failed to submit a compose")
	}
	if resp.StatusCode != 200 {
		log.Print("Error: the ", endpoint, " returned non 200 status. Full response: ", resp)
		failed = true
	} else {
		buf := new(bytes.Buffer)
		_, _ = buf.ReadFrom(resp.Body)
		log.Print("Success: the ", endpoint, " returned: ", buf.String())
	}
	if failed {
		os.Exit(1)
	}
}
