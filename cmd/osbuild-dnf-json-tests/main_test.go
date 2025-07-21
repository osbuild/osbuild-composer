// This package contains tests related to dnf-json and rpmmd package.

//go:build integration

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
)

// This test loads all the repositories available in /repositories directory
// and tries to run depsolve for each architecture. With N architectures available
// this should run cross-arch dependency solving N-1 times.
func TestCrossArchDepsolve(t *testing.T) {
	// Load repositories from the definition we provide in the RPM package
	repoDir := "/usr/share/tests/osbuild-composer/repositories"

	// NOTE: we can add RHEL, but don't make it hard requirement because it will fail outside of VPN
	cs9 := distrofactory.NewDefault().GetDistro("centos-9")
	require.NotNil(t, cs9)

	// Set up temporary directory for rpm/dnf cache
	dir := t.TempDir()
	baseSolver := dnfjson.NewBaseSolver(dir)

	repos, err := reporegistry.LoadRepositories([]string{repoDir}, cs9.Name())
	require.NoErrorf(t, err, "Failed to LoadRepositories %v", cs9.Name())

	for _, archStr := range cs9.ListArches() {
		t.Run(archStr, func(t *testing.T) {
			arch, err := cs9.GetArch(archStr)
			require.NoError(t, err)
			solver := baseSolver.NewWithConfig(cs9.ModulePlatformID(), cs9.Releasever(), archStr, cs9.Name())
			for _, imgTypeStr := range arch.ListImageTypes() {
				t.Run(imgTypeStr, func(t *testing.T) {
					imgType, err := arch.GetImageType(imgTypeStr)
					require.NoError(t, err)
					// set up bare minimum args for image type
					var customizations *blueprint.Customizations
					if imgTypeStr == "edge-simplified-installer" {
						customizations = &blueprint.Customizations{
							InstallationDevice: "/dev/null",
						}
					}
					var options distro.ImageOptions
					if imgType.OSTreeRef() != "" {
						options.OSTree = &ostree.ImageOptions{
							URL: "https://example.com",
						}
					}
					manifest, _, err := imgType.Manifest(
						&blueprint.Blueprint{
							Customizations: customizations,
						},
						options,
						repos[archStr], nil)
					assert.NoError(t, err)

					for _, set := range manifest.GetPackageSetChains() {
						_, err = solver.Depsolve(set, sbom.StandardTypeNone)
						assert.NoError(t, err)
					}
				})
			}
		})
	}
}

// This test loads all the repositories available in /repositories directory
// and tries to depsolve all package sets of one image type for one architecture.
func TestDepsolvePackageSets(t *testing.T) {
	// Load repositories from the definition we provide in the RPM package
	repoDir := "/usr/share/tests/osbuild-composer/repositories"

	// NOTE: we can add RHEL, but don't make it hard requirement because it will fail outside of VPN
	cs9 := distrofactory.NewDefault().GetDistro("centos-9")
	require.NotNil(t, cs9)

	// Set up temporary directory for rpm/dnf cache
	dir := t.TempDir()
	solver := dnfjson.NewSolver(cs9.ModulePlatformID(), cs9.Releasever(), arch.ARCH_X86_64.String(), cs9.Name(), dir)

	repos, err := reporegistry.LoadRepositories([]string{repoDir}, cs9.Name())
	require.NoErrorf(t, err, "Failed to LoadRepositories %v", cs9.Name())
	x86Repos, ok := repos[arch.ARCH_X86_64.String()]
	require.Truef(t, ok, "failed to get %q repos for %q", arch.ARCH_X86_64.String(), cs9.Name())

	x86Arch, err := cs9.GetArch(arch.ARCH_X86_64.String())
	require.Nilf(t, err, "failed to get %q arch of %q distro", arch.ARCH_X86_64.String(), cs9.Name())

	qcow2ImageTypeName := "qcow2"
	qcow2Image, err := x86Arch.GetImageType(qcow2ImageTypeName)
	require.Nilf(t, err, "failed to get %q image type of %q/%q distro/arch", qcow2ImageTypeName, cs9.Name(), arch.ARCH_X86_64.String())

	manifestSource, _, err := qcow2Image.Manifest(&blueprint.Blueprint{Packages: []blueprint.Package{{Name: "bind"}}}, distro.ImageOptions{}, x86Repos, nil)
	require.Nilf(t, err, "failed to initialise manifest for %q image type of %q/%q distro/arch", qcow2ImageTypeName, cs9.Name(), arch.ARCH_X86_64.String())
	imagePkgSets := manifestSource.GetPackageSetChains()

	gotPackageSpecsSets := make(map[string][]rpmmd.PackageSpec, len(imagePkgSets))
	gotRepoConfigs := make(map[string][]rpmmd.RepoConfig, len(imagePkgSets))
	for name, pkgSet := range imagePkgSets {
		res, err := solver.Depsolve(pkgSet, sbom.StandardTypeNone)
		if err != nil {
			require.Nil(t, err)
		}
		gotPackageSpecsSets[name] = res.Packages
		gotRepoConfigs[name] = res.Repos
	}
	expectedPackageSpecsSetNames := []string{"build", "os"}
	require.EqualValues(t, len(expectedPackageSpecsSetNames), len(gotPackageSpecsSets))
	for _, name := range expectedPackageSpecsSetNames {
		_, ok := gotPackageSpecsSets[name]
		assert.True(t, ok)
		_, ok = gotRepoConfigs[name]
		assert.True(t, ok)
	}
}
