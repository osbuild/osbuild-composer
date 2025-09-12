package depsolvednf_mock

import (
	"fmt"
	"slices"
	"strings"

	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
)

// MockDepsolveDNF is a mock implementation of the weldr.Solver interface
type MockDepsolveDNF struct {
	CleanCacheErr error

	DepsolveErr error
	DepsolveRes *dnfjson.DepsolveResult

	FetchErr error
	FetchRes rpmmd.PackageList

	SearchErr error
	// SearchResMap is a map of package names to package lists
	// The key is a comma-separated list of the packages requested.
	SearchResMap map[string]rpmmd.PackageList
}

func (m *MockDepsolveDNF) CleanCache() error {
	if m.CleanCacheErr != nil {
		return m.CleanCacheErr
	}
	return nil
}

func (m *MockDepsolveDNF) Depsolve(packages []rpmmd.PackageSet, sbomType sbom.StandardType) (*dnfjson.DepsolveResult, error) {
	if m.DepsolveErr != nil {
		return nil, fmt.Errorf("running osbuild-depsolve-dnf failed:\n%w", m.DepsolveErr)
	}
	if m.DepsolveRes != nil {
		return m.DepsolveRes, nil
	}
	return &dnfjson.DepsolveResult{}, nil
}

func (m *MockDepsolveDNF) FetchMetadata(repos []rpmmd.RepoConfig) (rpmmd.PackageList, error) {
	if m.FetchErr != nil {
		return nil, m.FetchErr
	}
	if m.FetchRes != nil {
		return lexSortedPackageList(m.FetchRes), nil
	}
	return rpmmd.PackageList{}, nil
}

// lexSortedPackageList returns a lexically sorted copy of the package list.
// osbuild-depsolve-dnf sorts the returned package list lexically.
func lexSortedPackageList(pkgList rpmmd.PackageList) rpmmd.PackageList {
	sorted := append(rpmmd.PackageList{}, pkgList...)
	slices.SortFunc(sorted, func(a, b rpmmd.Package) int {
		if a.Name == b.Name {
			return strings.Compare(a.Version, b.Version)
		}
		return strings.Compare(a.Name, b.Name)
	})
	return sorted
}

func (m *MockDepsolveDNF) SearchMetadata(repos []rpmmd.RepoConfig, packages []string) (rpmmd.PackageList, error) {
	if m.SearchErr != nil {
		return nil, m.SearchErr
	}

	if m.SearchResMap != nil {
		key := strings.Join(packages, ",")
		if res, ok := m.SearchResMap[key]; ok {
			return lexSortedPackageList(res), nil
		}
	}

	return rpmmd.PackageList{}, nil
}
