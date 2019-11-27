package rpmmd_mock

import (
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
)

type fetchPackageList struct {
	ret rpmmd.PackageList
	err error
}
type depsolve struct {
	ret []rpmmd.PackageSpec
	err error
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

func (r *rpmmdMock) FetchPackageList(repos []rpmmd.RepoConfig) (rpmmd.PackageList, error) {
	return r.Fixture.fetchPackageList.ret, r.Fixture.fetchPackageList.err
}

func (r *rpmmdMock) Depsolve(specs []string, repos []rpmmd.RepoConfig) ([]rpmmd.PackageSpec, error) {
	return r.Fixture.depsolve.ret, r.Fixture.depsolve.err
}
