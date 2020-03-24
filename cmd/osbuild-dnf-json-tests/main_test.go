// This package contains tests related to dnf-json and rpmmd package.

// +build integration

package main

import (
	"fmt"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora30"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"os"
	"path"
	"testing"
)

func TestFetchChecksum(t *testing.T) {
	dir, err := test.SetUpTemporaryRepository()
	defer func(dir string) {
		err := test.TearDownTemporaryRepository(dir)
		assert.Nil(t, err, "Failed to clean up temporary repository.")
	}(dir)
	assert.Nilf(t, err, "Failed to set up temporary repository: %v", err)

	repoCfg := rpmmd.RepoConfig{
		Id:        "repo",
		BaseURL:   fmt.Sprintf("file://%s", dir),
		IgnoreSSL: true,
	}
	rpmMetadata := rpmmd.NewRPMMD(path.Join(dir, "rpmmd"))
	_, c, err := rpmMetadata.FetchMetadata([]rpmmd.RepoConfig{repoCfg}, "platform:f31", "x86_64")
	assert.Nilf(t, err, "Failed to fetch checksum: %v", err)
	assert.NotEqual(t, "", c["repo"], "The checksum is empty")
}

// This test loads all the repositories available in /repositories directory
// and tries to run depsolve for each architecture. With N architectures available
// this should run cross-arch dependency solving N-1 times.
func TestCrossArchDepsolve(t *testing.T) {
	// Set up temporary directory for rpm/dnf cache
	dir, err := ioutil.TempDir("/tmp", "rpmmd-test-")
	require.Nilf(t, err, "Failed to create tmp dir for depsolve test: %v", err)
	defer os.RemoveAll(dir)
	rpm := rpmmd.NewRPMMD(dir)

	// Load repositories from the definition we provide in the RPM package
	repositories := "/usr/share/osbuild-composer"

	// NOTE: we can add RHEL, but don't make it hard requirement because it will fail outside of VPN
	for _, distroStruct := range []distro.Distro{fedora30.New(), fedora31.New(), fedora32.New()} {
		repoConfig, err := rpmmd.LoadRepositories([]string{repositories}, distroStruct.Name())
		assert.Nilf(t, err, "Failed to LoadRepositories from %v for %v: %v", repositories, distroStruct.Name(), err)
		if err != nil {
			// There is no point in running the tests without having repositories, but we can still run tests
			// for the remaining distros
			continue
		}
		for _, archStr := range distroStruct.ListArchs() {
			arch, err := distroStruct.GetArch(archStr)
			assert.Nilf(t, err, "Failed to GetArch from %v structure: %v", distroStruct.Name(), err)
			if err != nil {
				continue
			}
			for _, imgTypeStr := range arch.ListImageTypes() {
				imgType, err := arch.GetImageType(imgTypeStr)
				assert.Nilf(t, err, "Failed to GetImageType for %v on %v: %v", distroStruct.Name(), arch.Name(), err)
				if err != nil {
					continue
				}

				buildPackages := imgType.BuildPackages()
				_, _, err = rpm.Depsolve(buildPackages, []string{}, repoConfig[archStr], distroStruct.ModulePlatformID(), archStr)
				assert.Nilf(t, err, "Failed to Depsolve build packages for %v %v %v image: %v", distroStruct.Name(), imgType.Name(), arch.Name(), err)

				basePackagesInclude, basePackagesExclude := imgType.BasePackages()
				_, _, err = rpm.Depsolve(basePackagesInclude, basePackagesExclude, repoConfig[archStr], distroStruct.ModulePlatformID(), archStr)
				assert.Nilf(t, err, "Failed to Depsolve base packages for %v %v %v image: %v", distroStruct.Name(), imgType.Name(), arch.Name(), err)
			}
		}
	}
}
