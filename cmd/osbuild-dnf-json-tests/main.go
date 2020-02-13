// This package contains tests related to dnf-json and rpmmd package.
package main

import (
	"fmt"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
)

func main() {
	// Tests that the package wrapping dnf-json works as expected
	dir, err := setUpTemporaryRepository()
	if err != nil {
		log.Fatal("Failed to set up temporary repository:", err)
	}
	TestFetchChecksum(false, dir)
	err = tearDownTemporaryRepository(dir)
	if err != nil {
		log.Print("Warning: failed to clean up temporary repository.")
	}
}

func setUpTemporaryRepository() (string, error) {
	dir, err := ioutil.TempDir("/tmp", "osbuild-composer-test-")
	if err != nil {
		return "", err
	}
	cmd := exec.Command("createrepo_c", path.Join(dir))
	err = cmd.Start()
	if err != nil {
		return "", err
	}
	err = cmd.Wait()
	if err != nil {
		return "", err
	}
	return dir, nil
}

func tearDownTemporaryRepository(dir string) error {
	return os.RemoveAll(dir)
}

func TestFetchChecksum(quiet bool, dir string) {
	repoCfg := rpmmd.RepoConfig{
		Id:        "repo",
		Name:      "repo",
		BaseURL:   fmt.Sprintf("file://%s", dir),
		IgnoreSSL: true,
	}
	if !quiet {
		log.Println("Running TestFetchChecksum on:", dir)
	}
	c, err := repoCfg.FetchChecksum()
	if err != nil {
		log.Fatal("Failed to fetch checksum:", err)
	}
	if c == "" {
		log.Fatal("The checksum is empty")
	}
	log.Println("SUCCESS")
}
