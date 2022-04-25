package rpmmd_mock

import (
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
	"github.com/osbuild/osbuild-composer/internal/worker"
)

type fetchPackageList struct {
	ret       rpmmd.PackageList
	checksums map[string]string
	err       error
}
type depsolve struct {
	ret       []rpmmd.PackageSpec
	retSets   map[string][]rpmmd.PackageSpec
	checksums map[string]string
	err       error
}

type Fixture struct {
	fetchPackageList
	depsolve
	*store.Store
	Workers *worker.Server
}

type rpmmdMock struct {
	Fixture Fixture
}

func NewRPMMDMock(fixture Fixture) rpmmd.RPMMD {
	return &rpmmdMock{Fixture: fixture}
}

func (r *rpmmdMock) FetchMetadata(repos []rpmmd.RepoConfig, modulePlatformID, arch, releasever string) (rpmmd.PackageList, map[string]string, error) {
	return r.Fixture.fetchPackageList.ret, r.Fixture.fetchPackageList.checksums, r.Fixture.fetchPackageList.err
}

func (r *rpmmdMock) Depsolve(packageSet rpmmd.PackageSet, repos []rpmmd.RepoConfig, modulePlatformID, arch, releasever string) ([]rpmmd.PackageSpec, map[string]string, error) {
	return r.Fixture.depsolve.ret, r.Fixture.fetchPackageList.checksums, r.Fixture.depsolve.err
}

func (r *rpmmdMock) DepsolvePackageSets(packageSetsChains map[string][]string, packageSets map[string]rpmmd.PackageSet, repos []rpmmd.RepoConfig, packageSetsRepos map[string][]rpmmd.RepoConfig, modulePlatformID, arch, releasever string) (map[string][]rpmmd.PackageSpec, error) {
	return r.Fixture.depsolve.retSets, r.Fixture.depsolve.err
}
