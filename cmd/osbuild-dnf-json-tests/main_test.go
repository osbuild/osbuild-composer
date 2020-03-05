// This package contains tests related to dnf-json and rpmmd package.

// +build integration

package main

import (
	"fmt"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
	"path"
)

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

func TestFetchChecksum(t *testing.T) {
	dir, err := setUpTemporaryRepository()
	defer func(dir string) {
		err := tearDownTemporaryRepository(dir)
		if err != nil {
			t.Errorf("Warning: failed to clean up temporary repository.")
		}
	}(dir)
	if err != nil {
		t.Fatalf("Failed to set up temporary repository: %v", err)
	}

	repoCfg := rpmmd.RepoConfig{
		Id:        "repo",
		Name:      "repo",
		BaseURL:   fmt.Sprintf("file://%s", dir),
		IgnoreSSL: true,
	}
	rpmMetadata := rpmmd.NewRPMMD(path.Join(dir, "rpmmd"))
	_, c, err := rpmMetadata.FetchMetadata([]rpmmd.RepoConfig{repoCfg}, "platform:f31")
	if err != nil {
		t.Fatalf("Failed to fetch checksum: %v", err)
	}
	if c["repo"] == "" {
		t.Errorf("The checksum is empty")
	}
}
