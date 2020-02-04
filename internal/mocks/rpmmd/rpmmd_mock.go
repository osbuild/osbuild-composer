package rpmmd_mock

import (
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
)

type fetchPackageList struct {
	ret       rpmmd.PackageList
	checksums map[string]string
	err       error
}
type depsolve struct {
	ret       []rpmmd.PackageSpec
	checksums map[string]string
	err       error
}

type Fixture struct {
	fetchPackageList
	depsolve
	*store.Store
}

type rpmmdMock struct {
	Fixture Fixture
}

func NewRPMMDMock(fixture Fixture) rpmmd.RPMMD {
	return &rpmmdMock{Fixture: fixture}
}

func (r *rpmmdMock) FetchPackageList(repos []rpmmd.RepoConfig) (rpmmd.PackageList, map[string]string, error) {
	return r.Fixture.fetchPackageList.ret, r.Fixture.fetchPackageList.checksums, r.Fixture.fetchPackageList.err
}

func (r *rpmmdMock) Depsolve(specs, excludeSpecs []string, repos []rpmmd.RepoConfig, clean bool) ([]rpmmd.PackageSpec, map[string]string, error) {
	return r.Fixture.depsolve.ret, r.Fixture.fetchPackageList.checksums, r.Fixture.depsolve.err
}
