package depsolvednf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
)

// v1 API structs

type v1RepoConfig struct {
	ID             string   `json:"id"`
	Name           string   `json:"name,omitempty"`
	BaseURLs       []string `json:"baseurl,omitempty"`
	Metalink       string   `json:"metalink,omitempty"`
	MirrorList     string   `json:"mirrorlist,omitempty"`
	GPGKeys        []string `json:"gpgkeys,omitempty"`
	GPGCheck       bool     `json:"gpgcheck"`
	RepoGPGCheck   bool     `json:"repo_gpgcheck"`
	SSLVerify      *bool    `json:"sslverify,omitempty"`
	SSLCACert      string   `json:"sslcacert,omitempty"`
	SSLClientKey   string   `json:"sslclientkey,omitempty"`
	SSLClientCert  string   `json:"sslclientcert,omitempty"`
	MetadataExpire string   `json:"metadata_expire,omitempty"`
	ModuleHotfixes *bool    `json:"module_hotfixes,omitempty"`
}

// Request command and arguments for osbuild-depsolve-dnf
type v1Request struct {
	// Command should be either "depsolve" or "dump"
	Command string `json:"command"`

	// Platform ID, e.g., "platform:el8"
	ModulePlatformID string `json:"module_platform_id"`

	// Distro Releasever, e.g., "8"
	Releasever string `json:"releasever"`

	// System architecture
	Arch string `json:"arch"`

	// Cache directory for the DNF metadata
	CacheDir string `json:"cachedir"`

	// Proxy to use
	Proxy string `json:"proxy"`

	// Arguments for the action defined by Command
	Arguments v1Arguments `json:"arguments"`
}

type v1SbomRequest struct {
	Type string `json:"type"`
}

// arguments for a osbuild-depsolve-dnf request
type v1Arguments struct {
	// Repositories to use for depsolving
	Repos []v1RepoConfig `json:"repos"`

	// Search terms to use with search command
	Search v1SearchArgs `json:"search"`

	// Depsolve package sets and repository mappings for this request
	Transactions []v1TransactionArgs `json:"transactions"`

	// Load repository configurations, gpg keys, and vars from an os-root-like
	// tree.
	RootDir string `json:"root_dir"`

	// Optional metadata to download for the repositories
	OptionalMetadata []string `json:"optional-metadata,omitempty"`

	// Optionally request an SBOM from depsolving
	Sbom *v1SbomRequest `json:"sbom,omitempty"`
}

type v1SearchArgs struct {
	// Only include latest NEVRA when true
	Latest bool `json:"latest"`

	// List of package name globs to search for
	// If it has '*' it is passed to dnf glob search, if it has *name* it is passed
	// to substr matching, and if it has neither an exact match is expected.
	Packages []string `json:"packages"`
}

type v1TransactionArgs struct {
	// Packages to depsolve
	PackageSpecs []string `json:"package-specs"`

	// Packages to exclude from results
	ExcludeSpecs []string `json:"exclude-specs"`

	// Modules to enable during depsolve
	ModuleEnableSpecs []string `json:"module-enable-specs,omitempty"`

	// IDs of repositories to use for this depsolve
	RepoIDs []string `json:"repo-ids"`

	// If we want weak deps for this depsolve
	InstallWeakDeps bool `json:"install_weak_deps"`
}

type v1PackageSpecs []v1PackageSpec
type v1ModuleSpecs map[string]v1ModuleSpec

type v1DepsolveResult struct {
	Packages v1PackageSpecs          `json:"packages"`
	Repos    map[string]v1RepoConfig `json:"repos"`
	Modules  v1ModuleSpecs           `json:"modules"`

	// (optional) contains the solver used, e.g. "dnf5"
	Solver string `json:"solver,omitempty"`

	// (optional) contains the SBOM for the depsolved transaction
	SBOM json.RawMessage `json:"sbom,omitempty"`
}

// legacyPackageList represents the old 'PackageList' structure, which
// was used for both dump and search results. It is kept here for unmarshaling
// the results.
type v1LegacyPackageList []struct {
	Name        string    `json:"name"`
	Summary     string    `json:"summary"`
	Description string    `json:"description"`
	URL         string    `json:"url"`
	Epoch       uint      `json:"epoch"`
	Version     string    `json:"version"`
	Release     string    `json:"release"`
	Arch        string    `json:"arch"`
	BuildTime   time.Time `json:"build_time"`
	License     string    `json:"license"`
}

type v1DumpResult = v1LegacyPackageList
type v1SearchResult = v1LegacyPackageList

// Package specification
type v1PackageSpec struct {
	Name           string `json:"name"`
	Epoch          uint   `json:"epoch"`
	Version        string `json:"version,omitempty"`
	Release        string `json:"release,omitempty"`
	Arch           string `json:"arch,omitempty"`
	RepoID         string `json:"repo_id,omitempty"`
	Path           string `json:"path,omitempty"`
	RemoteLocation string `json:"remote_location,omitempty"`
	Checksum       string `json:"checksum,omitempty"`
}

// Module specification
type v1ModuleSpec struct {
	ModuleConfigFile v1ModuleConfigFile   `json:"module-file"`
	FailsafeFile     v1ModuleFailsafeFile `json:"failsafe-file"`
}

type v1ModuleConfigFile struct {
	Path string             `json:"path"`
	Data v1ModuleConfigData `json:"data"`
}

type v1ModuleConfigData struct {
	Name     string   `json:"name"`
	Stream   string   `json:"stream"`
	Profiles []string `json:"profiles"`
	State    string   `json:"state"`
}

type v1ModuleFailsafeFile struct {
	Path string `json:"path"`
	Data string `json:"data"`
}

// V1 API handler implementation

// v1Handler implements the apiHandler interface for API version 1.
type v1Handler struct {
}

func newV1Handler() *v1Handler {
	return &v1Handler{}
}

var _ apiHandler = newV1Handler()

func (h *v1Handler) makeDepsolveRequest(cfg *solverConfig, pkgSets []rpmmd.PackageSet, sbomType sbom.StandardType) ([]byte, error) {
	// NB: we could have the allRepos be passed in as a parameter from
	// Depsolve() instead of collecting it here. However, it feels weird
	// to depend on pre-processed data, when the pkgSets are the supposed
	// source of truth.
	allRepos := collectRepos(pkgSets)

	transactions := make([]v1TransactionArgs, len(pkgSets))
	for dsIdx, pkgSet := range pkgSets {
		transactions[dsIdx] = v1TransactionArgs{
			PackageSpecs:      pkgSet.Include,
			ExcludeSpecs:      pkgSet.Exclude,
			ModuleEnableSpecs: pkgSet.EnabledModules,
			InstallWeakDeps:   pkgSet.InstallWeakDeps,
		}

		for _, jobRepo := range pkgSet.Repositories {
			transactions[dsIdx].RepoIDs = append(transactions[dsIdx].RepoIDs, jobRepo.Hash())
		}
	}

	dnfRepos, err := h.reposFromRPMMD(cfg, allRepos)
	if err != nil {
		return nil, err
	}

	args := v1Arguments{
		Repos:            dnfRepos,
		RootDir:          cfg.rootDir,
		Transactions:     transactions,
		OptionalMetadata: optionalMetadataForDistro(cfg.modulePlatformID),
	}

	req := v1Request{
		Command:          "depsolve",
		ModulePlatformID: cfg.modulePlatformID,
		Arch:             cfg.arch,
		Releasever:       cfg.releaseVer,
		CacheDir:         cfg.cacheDir,
		Proxy:            cfg.proxy,
		Arguments:        args,
	}

	if sbomType != sbom.StandardTypeNone {
		req.Arguments.Sbom = &v1SbomRequest{Type: sbomType.String()}
	}

	return json.Marshal(req)
}

func (h *v1Handler) makeDumpRequest(cfg *solverConfig, repos []rpmmd.RepoConfig) ([]byte, error) {
	dnfRepos, err := h.reposFromRPMMD(cfg, repos)
	if err != nil {
		return nil, err
	}
	req := v1Request{
		Command:          "dump",
		ModulePlatformID: cfg.modulePlatformID,
		Arch:             cfg.arch,
		Releasever:       cfg.releaseVer,
		CacheDir:         cfg.cacheDir,
		Proxy:            cfg.proxy,
		Arguments: v1Arguments{
			Repos: dnfRepos,
		},
	}
	return json.Marshal(req)
}

func (h *v1Handler) makeSearchRequest(cfg *solverConfig, repos []rpmmd.RepoConfig, packages []string) ([]byte, error) {
	dnfRepos, err := h.reposFromRPMMD(cfg, repos)
	if err != nil {
		return nil, err
	}
	req := v1Request{
		Command:          "search",
		ModulePlatformID: cfg.modulePlatformID,
		Arch:             cfg.arch,
		CacheDir:         cfg.cacheDir,
		Releasever:       cfg.releaseVer,
		Proxy:            cfg.proxy,
		Arguments: v1Arguments{
			Repos: dnfRepos,
			Search: v1SearchArgs{
				Packages: packages,
			},
		},
	}
	return json.Marshal(req)
}

func (h *v1Handler) parseDepsolveResult(output []byte) (*depsolveResultRaw, error) {
	var result v1DepsolveResult
	dec := json.NewDecoder(bytes.NewReader(output))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding depsolve result failed: %w", err)
	}

	// Convert packages
	pkgs := make(rpmmd.PackageList, 0, len(result.Packages))
	for _, pkg := range result.Packages {
		repo, ok := result.Repos[pkg.RepoID]
		if !ok {
			return nil, fmt.Errorf("repo ID not found in repositories: %s", pkg.RepoID)
		}

		checksum := strings.Split(pkg.Checksum, ":")
		if len(checksum) != 2 {
			return nil, fmt.Errorf("invalid checksum format for package %s: %s", pkg.Name, pkg.Checksum)
		}

		pkg := rpmmd.Package{
			Name:            pkg.Name,
			Epoch:           pkg.Epoch,
			Version:         pkg.Version,
			Release:         pkg.Release,
			Arch:            pkg.Arch,
			RemoteLocations: []string{pkg.RemoteLocation},
			Checksum: rpmmd.Checksum{
				Type:  checksum[0],
				Value: checksum[1],
			},
			CheckGPG: repo.GPGCheck,
			RepoID:   pkg.RepoID,
			Location: pkg.Path,
		}

		if sslVerify := repo.SSLVerify; sslVerify != nil {
			pkg.IgnoreSSL = !*sslVerify
		}

		// Set mTLS secrets if SSLClientKey is set.
		// The Solver will override secrets to 'org.osbuild.rhsm' if the repo needs RHSM secrets.
		if repo.SSLClientKey != "" {
			pkg.Secrets = "org.osbuild.mtls"
		}

		pkgs = append(pkgs, pkg)
	}

	// Convert modules
	modules := make([]rpmmd.ModuleSpec, 0, len(result.Modules))
	for _, mod := range result.Modules {
		modules = append(modules, rpmmd.ModuleSpec{
			ModuleConfigFile: rpmmd.ModuleConfigFile{
				Path: mod.ModuleConfigFile.Path,
				Data: rpmmd.ModuleConfigData{
					Name:     mod.ModuleConfigFile.Data.Name,
					Stream:   mod.ModuleConfigFile.Data.Stream,
					State:    mod.ModuleConfigFile.Data.State,
					Profiles: mod.ModuleConfigFile.Data.Profiles,
				},
			},
			FailsafeFile: rpmmd.ModuleFailsafeFile{
				Path: mod.FailsafeFile.Path,
				Data: mod.FailsafeFile.Data,
			},
		})
	}

	// Convert repos
	repos := make([]rpmmd.RepoConfig, 0, len(result.Repos))
	for repoID := range result.Repos {
		repo := result.Repos[repoID]

		var ignoreSSL bool
		if sslVerify := repo.SSLVerify; sslVerify != nil {
			ignoreSSL = !*sslVerify
		}

		repos = append(repos, rpmmd.RepoConfig{
			Id:             repo.ID,
			Name:           repo.Name,
			BaseURLs:       repo.BaseURLs,
			Metalink:       repo.Metalink,
			MirrorList:     repo.MirrorList,
			GPGKeys:        repo.GPGKeys,
			CheckGPG:       common.ToPtr(repo.GPGCheck),
			CheckRepoGPG:   common.ToPtr(repo.RepoGPGCheck),
			IgnoreSSL:      &ignoreSSL,
			MetadataExpire: repo.MetadataExpire,
			ModuleHotfixes: repo.ModuleHotfixes,
			Enabled:        common.ToPtr(true),
			SSLCACert:      repo.SSLCACert,
			SSLClientKey:   repo.SSLClientKey,
			SSLClientCert:  repo.SSLClientCert,
		})
	}

	return &depsolveResultRaw{
		Packages: pkgs,
		Modules:  modules,
		Repos:    repos,
		Solver:   result.Solver,
		SBOMRaw:  result.SBOM,
	}, nil
}

func (h *v1Handler) parseDumpResult(output []byte) (*DumpResult, error) {
	var res v1DumpResult
	if err := json.Unmarshal(output, &res); err != nil {
		return nil, err
	}

	pkgs := res.toRPMMD()
	return &DumpResult{
		Packages: pkgs,
	}, nil
}

func (h *v1Handler) parseSearchResult(output []byte) (*SearchResult, error) {
	var res v1SearchResult
	if err := json.Unmarshal(output, &res); err != nil {
		return nil, err
	}

	pkgs := res.toRPMMD()
	return &SearchResult{
		Packages: pkgs,
	}, nil
}

// V1 API helper functions

func (h *v1Handler) reposFromRPMMD(cfg *solverConfig, rpmRepos []rpmmd.RepoConfig) ([]v1RepoConfig, error) {
	dnfRepos := make([]v1RepoConfig, len(rpmRepos))
	for idx, rr := range rpmRepos {
		dr := v1RepoConfig{
			ID:             rr.Hash(),
			Name:           rr.Name,
			BaseURLs:       rr.BaseURLs,
			Metalink:       rr.Metalink,
			MirrorList:     rr.MirrorList,
			GPGKeys:        rr.GPGKeys,
			MetadataExpire: rr.MetadataExpire,
			SSLCACert:      rr.SSLCACert,
			SSLClientKey:   rr.SSLClientKey,
			SSLClientCert:  rr.SSLClientCert,
		}
		if rr.ModuleHotfixes != nil {
			val := *rr.ModuleHotfixes
			dr.ModuleHotfixes = &val
		}

		if rr.CheckGPG != nil {
			dr.GPGCheck = *rr.CheckGPG
		}

		if rr.CheckRepoGPG != nil {
			dr.RepoGPGCheck = *rr.CheckRepoGPG
		}

		if rr.IgnoreSSL != nil {
			dr.SSLVerify = common.ToPtr(!*rr.IgnoreSSL)
		}

		if rr.RHSM {
			// NB: it is assumed that the s.subscriptions are not nil if the repo needs RHSM secrets
			// because validateSubscriptionsForRepos() is called before makeDepsolveRequest().
			secrets, err := cfg.subscriptions.GetSecretsForBaseurl(rr.BaseURLs, cfg.arch, cfg.releaseVer)
			if err != nil {
				return nil, fmt.Errorf("getting RHSM secrets for baseurl %s failed: %w", rr.BaseURLs, err)
			}
			dr.SSLCACert = secrets.SSLCACert
			dr.SSLClientKey = secrets.SSLClientKey
			dr.SSLClientCert = secrets.SSLClientCert
		}

		dnfRepos[idx] = dr
	}
	return dnfRepos, nil
}

func (pl v1LegacyPackageList) toRPMMD() rpmmd.PackageList {
	rpmPkgs := make(rpmmd.PackageList, len(pl))
	for i, p := range pl {
		rpmPkgs[i] = rpmmd.Package{
			Name:        p.Name,
			Summary:     p.Summary,
			Description: p.Description,
			URL:         p.URL,
			Epoch:       p.Epoch,
			Version:     p.Version,
			Release:     p.Release,
			Arch:        p.Arch,
			BuildTime:   p.BuildTime,
			License:     p.License,
		}
	}
	return rpmPkgs
}
