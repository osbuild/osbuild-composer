package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/osbuild/osbuild-composer/internal/upload/pulp"
)

func check(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func main() {
	var filename, apiURL, repository, basePath, credsFile string
	flag.StringVar(&filename, "archive", "", "ostree archive to upload")
	flag.StringVar(&apiURL, "url", "", "server URL")
	flag.StringVar(&repository, "repository", "", "repository name")
	flag.StringVar(&basePath, "base-path", "", "base path for distribution (if the repository does not already exist)")
	flag.StringVar(&credsFile, "credentials", "", `file containing credentials (format: {"username": "...", "password": "..."})`)
	flag.Parse()

	client, err := pulp.NewClientFromFile(apiURL, credsFile)
	check(err)

	repoURL, err := client.UploadAndDistributeCommit(filename, repository, basePath)
	check(err)

	fmt.Printf("The commit will be available in the repository at %s\n", repoURL)
}
