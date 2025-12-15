package depsolvednf

import (
	"encoding/json"

	"github.com/osbuild/images/pkg/rhsm"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
)

// depsolveResultRaw is the internal result type returned by handlers.
// It contains packages with mtls secrets set (from response SSLClientKey),
// but NOT rhsm secrets (applied by Solver from rpmmd.RepoConfig.RHSM).
// It also contains raw SBOM data (Solver is responsible for creating
// sbom.Document).
type depsolveResultRaw struct {
	Packages rpmmd.PackageList
	Modules  []rpmmd.ModuleSpec
	Repos    []rpmmd.RepoConfig
	Solver   string
	SBOMRaw  json.RawMessage
}

// apiHandler defines the interface for API version implementations.
// Each API version implements this interface to handle request building
// and response parsing for the osbuild-depsolve-dnf API.
type apiHandler interface {
	// makeDepsolveRequest builds the depsolve request as serialized JSON.
	makeDepsolveRequest(cfg *solverConfig, pkgSets []rpmmd.PackageSet, sbomType sbom.StandardType) ([]byte, error)

	// makeDumpRequest builds the metadata dump request as serialized JSON.
	makeDumpRequest(cfg *solverConfig, repos []rpmmd.RepoConfig) ([]byte, error)

	// makeSearchRequest builds the package search request as serialized JSON.
	makeSearchRequest(cfg *solverConfig, repos []rpmmd.RepoConfig, packages []string) ([]byte, error)

	// parseDepsolveResult parses depsolve output into depsolveResultRaw.
	parseDepsolveResult(output []byte) (*depsolveResultRaw, error)

	// parseDumpResult parses dump output into a DumpResult.
	parseDumpResult(output []byte) (*DumpResult, error)

	// parseSearchResult parses search output into a SearchResult.
	parseSearchResult(output []byte) (*SearchResult, error)
}

// solverConfig contains solver configuration passed to API handlers.
// This provides handlers with necessary context without coupling them
// to the Solver type directly.
type solverConfig struct {
	modulePlatformID string
	arch             string
	releaseVer       string
	cacheDir         string
	rootDir          string
	proxy            string
	subscriptions    *rhsm.Subscriptions
}

// activeHandler is the currently active API handler implementation.
var activeHandler apiHandler

func init() {
	activeHandler = newV1Handler()
}
