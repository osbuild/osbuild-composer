package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/osbuild/osbuild-composer/internal/container"
)

func main() {
	var filename string
	var destination string
	var username string
	var password string
	var tag string
	var ignoreTls bool

	flag.StringVar(&filename, "container", "", "path to the oci-archive to upload (required)")
	flag.StringVar(&destination, "destination", "", "destination to upload to (required)")
	flag.StringVar(&tag, "tag", "", "destination tag to use for the container")
	flag.StringVar(&username, "username", "", "username to use for registry")
	flag.StringVar(&password, "password", "", "password to use for registry")
	flag.BoolVar(&ignoreTls, "ignore-tls", false, "ignore tls verification for destination")
	flag.Parse()

	if filename == "" || destination == "" {
		flag.Usage()
		os.Exit(1)
	}

	absPath, err := filepath.Abs(filename)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}

	fmt.Println("Container to upload is:", filename)

	client, err := container.NewClient(destination)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating the upload client: %v\n", err)
		os.Exit(1)
	}

	if password != "" {
		if username == "" {
			u, err := user.Current()
			if err != nil {
				fmt.Fprintf(os.Stderr, "error looking up current user: %v\n", err)
				os.Exit(1)
			}
			username = u.Username
		}
		client.SetCredentials(username, password)
	}

	client.TlsVerify = !ignoreTls

	ctx := context.Background()

	from := fmt.Sprintf("oci-archive://%s", absPath)

	digest, err := client.UploadImage(ctx, from, tag)

	if err != nil {
		fmt.Fprintf(os.Stderr, "error uploading: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("upload done; destination manifest: %s\n", digest.String())
}
