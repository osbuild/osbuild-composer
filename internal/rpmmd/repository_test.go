package rpmmd_test

import (
	"os"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

var f30 = rpmmd.RepoConfig{
	Id:       "fedora",
	Name:     "Fedora",
	Metalink: "https://mirrors.fedoraproject.org/metalink?repo=fedora-30&arch=x86_64",
}

func TestMain(m *testing.M) {
	// insert the root directory into PATH to find dnf-json, becaue GO runs
	// tests from the package directory
	path := os.Getenv("PATH")
	os.Setenv("PATH", "../..:" + path)

	os.Exit(m.Run())
}

func TestFetchPackageList(t *testing.T) {
	pkgs, err := rpmmd.FetchPackageList([]rpmmd.RepoConfig{f30})
	if err != nil {
		t.Fatalf("error fetching package list: %v", err)
	}

	// We're testing that dnf-json works, not that dnf returns the right
	// packages for a url. Verifying that the number of returned packages
	// is correct is enough.
	if len(pkgs) != 56697 {
		t.Fatal("received unexpected amount of packages")
	}
}

func TestDepsolve(t *testing.T) {
	pkgs, err := rpmmd.Depsolve([]string{"@Core", "grub2-pc"}, []rpmmd.RepoConfig{f30})
	if err != nil {
		t.Fatalf("error depsolving: %v", err)
	}

	if len(pkgs) != 300 {
		t.Fatal("received unexpected amount of packages")
	}
}
