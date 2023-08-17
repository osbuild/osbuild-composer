package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/osbuild/osbuild-composer/internal/upload/pulp"
)

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func readCredentials(credPath string) *pulp.Credentials {
	fp, err := os.Open(credPath)
	check(err)
	data, err := io.ReadAll(fp)
	check(err)
	var creds pulp.Credentials
	check(json.Unmarshal(data, &creds))
	return &creds
}

func main() {
	var filename, apiURL, repository, basePath, credsFile string
	flag.StringVar(&filename, "archive", "", "ostree archive to upload")
	flag.StringVar(&apiURL, "url", "", "server URL")
	flag.StringVar(&repository, "repository", "", "repository name")
	flag.StringVar(&basePath, "base-path", "", "base path for distribution (if the repository does not already exist)")
	flag.StringVar(&credsFile, "credentials", "", `file containing credentials (format: {"username": "...", "password": "..."})`)
	flag.Parse()

	client := pulp.NewClient(apiURL, readCredentials(credsFile))

	repoURL, err := client.UploadAndDistributeCommit(filename, repository, basePath)
	check(err)
	fmt.Printf("The commit will be available in the repository at %s\n", repoURL)
}
