package depsolvednf

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
)

// v2 API structs

// v2Checksum represents a checksum with algorithm and value.
type v2Checksum struct {
	Algorithm string `json:"algorithm"`
	Value     string `json:"value"`
}

// v2Dependency represents an RPM dependency or provided capability.
type v2Dependency struct {
	Name     string `json:"name"`
	Relation string `json:"relation,omitempty"`
	Version  string `json:"version,omitempty"`
}

// v2Repository represents a DNF/YUM repository configuration.
// Used for both request and response (response is a superset of request fields).
// In request, at least one of 'baseurl' or 'metalink' or 'mirrorlist' must be
// present.
type v2Repository struct {
	ID             string   `json:"id"`
	Name           string   `json:"name,omitempty"`
	BaseURLs       []string `json:"baseurl,omitempty"`
	Metalink       string   `json:"metalink,omitempty"`
	MirrorList     string   `json:"mirrorlist,omitempty"`
	GPGKeys        []string `json:"gpgkey,omitempty"`
	GPGCheck       *bool    `json:"gpgcheck,omitempty"`
	RepoGPGCheck   *bool    `json:"repo_gpgcheck,omitempty"`
	SSLVerify      *bool    `json:"sslverify,omitempty"`
	SSLCACert      string   `json:"sslcacert,omitempty"`
	SSLClientKey   string   `json:"sslclientkey,omitempty"`
	SSLClientCert  string   `json:"sslclientcert,omitempty"`
	MetadataExpire string   `json:"metadata_expire,omitempty"`
	ModuleHotfixes *bool    `json:"module_hotfixes,omitempty"`
	RHSM           bool     `json:"rhsm,omitempty"`
	RHUI           bool     `json:"rhui,omitempty"`
}

// v2Package represents an RPM package with full metadata.
// This is a unified representation used for depsolve, dump, and search responses.
type v2Package struct {
	// Core fields (always expected to have values)
	Name            string      `json:"name"`
	Epoch           int         `json:"epoch"`
	Version         string      `json:"version"`
	Release         string      `json:"release"`
	Arch            string      `json:"arch"`
	RepoID          string      `json:"repo_id"`
	Location        string      `json:"location"`
	RemoteLocations []string    `json:"remote_locations"`
	Checksum        *v2Checksum `json:"checksum"`

	// Metadata fields (may be nil/empty)
	HeaderChecksum *v2Checksum `json:"header_checksum"`
	License        string      `json:"license"`
	Summary        string      `json:"summary"`
	Description    string      `json:"description"`
	URL            string      `json:"url"`
	Vendor         string      `json:"vendor"`
	Packager       string      `json:"packager"`
	BuildTime      string      `json:"build_time"` // RFC3339 format or empty
	DownloadSize   int64       `json:"download_size"`
	InstallSize    int64       `json:"install_size"`
	Group          string      `json:"group"`
	SourceRPM      string      `json:"source_rpm"`
	Reason         string      `json:"reason"`

	// Dependencies (always arrays, may be empty)
	Provides        []v2Dependency `json:"provides"`
	Requires        []v2Dependency `json:"requires"`
	RequiresPre     []v2Dependency `json:"requires_pre"`
	Conflicts       []v2Dependency `json:"conflicts"`
	Obsoletes       []v2Dependency `json:"obsoletes"`
	RegularRequires []v2Dependency `json:"regular_requires"`
	Recommends      []v2Dependency `json:"recommends"`
	Suggests        []v2Dependency `json:"suggests"`
	Enhances        []v2Dependency `json:"enhances"`
	Supplements     []v2Dependency `json:"supplements"`
	Files           []string       `json:"files"`
}

// v2ModuleConfigData contains module configuration data.
type v2ModuleConfigData struct {
	Name     string   `json:"name"`
	Stream   string   `json:"stream"`
	Profiles []string `json:"profiles"`
	State    string   `json:"state"`
}

// v2ModuleConfigFile represents a module configuration file.
type v2ModuleConfigFile struct {
	Path string             `json:"path"`
	Data v2ModuleConfigData `json:"data"`
}

// v2ModuleFailsafeFile represents a module failsafe file.
type v2ModuleFailsafeFile struct {
	Path string `json:"path"`
	Data string `json:"data"`
}

// v2ModuleSpec represents a module specification.
type v2ModuleSpec struct {
	ModuleConfigFile v2ModuleConfigFile   `json:"module-file"`
	FailsafeFile     v2ModuleFailsafeFile `json:"failsafe-file"`
}

// v2Request is the top-level request structure for the V2 API.
type v2Request struct {
	// API version, must be 2
	APIVersion int `json:"api_version"`

	// Command should be "depsolve", "dump", or "search"
	Command string `json:"command"`

	// Platform ID, e.g., "platform:el9"
	ModulePlatformID string `json:"module_platform_id,omitempty"`

	// Distro Releasever, e.g., "9"
	Releasever string `json:"releasever"`

	// System architecture
	Arch string `json:"arch"`

	// Cache directory for the DNF metadata
	CacheDir string `json:"cachedir"`

	// Proxy to use
	Proxy string `json:"proxy,omitempty"`

	// Arguments for the action defined by Command
	Arguments v2Arguments `json:"arguments"`
}

// v2Arguments contains command arguments for V2 API requests.
type v2Arguments struct {
	// Repositories to use for depsolving
	Repos []v2Repository `json:"repos"`

	// Search terms to use with search command
	Search *v2SearchArgs `json:"search,omitempty"`

	// Depsolve package sets and repository mappings for this request
	Transactions []v2TransactionArgs `json:"transactions,omitempty"`

	// Load repository configurations, gpg keys, and vars from an os-root-like tree.
	RootDir string `json:"root_dir,omitempty"`

	// Optional metadata to download for the repositories
	OptionalMetadata []string `json:"optional-metadata,omitempty"`

	// Optionally request an SBOM from depsolving
	Sbom *v2SbomRequest `json:"sbom,omitempty"`
}

// v2TransactionArgs contains arguments for a single depsolve transaction.
type v2TransactionArgs struct {
	// Packages to depsolve
	PackageSpecs []string `json:"package-specs"`

	// Packages to exclude from results
	ExcludeSpecs []string `json:"exclude-specs,omitempty"`

	// Modules to enable during depsolve
	ModuleEnableSpecs []string `json:"module-enable-specs,omitempty"`

	// IDs of repositories to use for this depsolve.
	// If empty, all repositories will be used.
	RepoIDs []string `json:"repo-ids,omitempty"`

	// If we want weak deps for this depsolve
	InstallWeakDeps bool `json:"install_weak_deps"`
}

// v2SearchArgs contains arguments for search command.
type v2SearchArgs struct {
	// Only include latest NEVRA when true
	Latest bool `json:"latest"`

	// List of package name globs to search for
	Packages []string `json:"packages"`
}

// v2SbomRequest contains SBOM generation request.
type v2SbomRequest struct {
	Type string `json:"type"`
}

// v2DepsolveResult represents the V2 depsolve command response.
type v2DepsolveResult struct {
	Solver       string                  `json:"solver"`
	Transactions [][]v2Package           `json:"transactions"`
	Repos        map[string]v2Repository `json:"repos"`
	Modules      map[string]v2ModuleSpec `json:"modules"`
	SBOM         json.RawMessage         `json:"sbom,omitempty"`
}

// v2PackageListResult is the common response structure for dump and search.
type v2PackageListResult struct {
	Solver   string                  `json:"solver"`
	Packages []v2Package             `json:"packages"`
	Repos    map[string]v2Repository `json:"repos"`
}

// V2 API Handler Implementation

// v2Handler implements the apiHandler interface for API version 2.
type v2Handler struct{}

func newV2Handler() *v2Handler {
	return &v2Handler{}
}

var _ apiHandler = newV2Handler()

func (h *v2Handler) makeDepsolveRequest(cfg *solverConfig, pkgSets []rpmmd.PackageSet, sbomType sbom.StandardType) ([]byte, error) {
	allRepos := collectRepos(pkgSets)

	transactions := make([]v2TransactionArgs, len(pkgSets))
	for dsIdx, pkgSet := range pkgSets {
		transactions[dsIdx] = v2TransactionArgs{
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

	args := v2Arguments{
		Repos:            dnfRepos,
		RootDir:          cfg.rootDir,
		Transactions:     transactions,
		OptionalMetadata: optionalMetadataForDistro(cfg.modulePlatformID),
	}

	req := v2Request{
		APIVersion:       2,
		Command:          "depsolve",
		ModulePlatformID: cfg.modulePlatformID,
		Arch:             cfg.arch,
		Releasever:       cfg.releaseVer,
		CacheDir:         cfg.cacheDir,
		Proxy:            cfg.proxy,
		Arguments:        args,
	}

	if sbomType != sbom.StandardTypeNone {
		req.Arguments.Sbom = &v2SbomRequest{Type: sbomType.String()}
	}

	return json.Marshal(req)
}

func (h *v2Handler) makeDumpRequest(cfg *solverConfig, repos []rpmmd.RepoConfig) ([]byte, error) {
	dnfRepos, err := h.reposFromRPMMD(cfg, repos)
	if err != nil {
		return nil, err
	}
	req := v2Request{
		APIVersion:       2,
		Command:          "dump",
		ModulePlatformID: cfg.modulePlatformID,
		Arch:             cfg.arch,
		Releasever:       cfg.releaseVer,
		CacheDir:         cfg.cacheDir,
		Proxy:            cfg.proxy,
		Arguments: v2Arguments{
			Repos: dnfRepos,
		},
	}
	return json.Marshal(req)
}

func (h *v2Handler) makeSearchRequest(cfg *solverConfig, repos []rpmmd.RepoConfig, packages []string) ([]byte, error) {
	dnfRepos, err := h.reposFromRPMMD(cfg, repos)
	if err != nil {
		return nil, err
	}
	req := v2Request{
		APIVersion:       2,
		Command:          "search",
		ModulePlatformID: cfg.modulePlatformID,
		Arch:             cfg.arch,
		CacheDir:         cfg.cacheDir,
		Releasever:       cfg.releaseVer,
		Proxy:            cfg.proxy,
		Arguments: v2Arguments{
			Repos: dnfRepos,
			Search: &v2SearchArgs{
				Packages: packages,
			},
		},
	}
	return json.Marshal(req)
}

func (h *v2Handler) parseDepsolveResult(output []byte) (*depsolveResultRaw, error) {
	var result v2DepsolveResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("decoding depsolve result failed: %w", err)
	}

	// Convert repos first, to enable adding a reference to the RepoConfig to each package
	repos, repoMap := h.toRPMMDRepoConfigs(result.Repos)

	// Build both per-transaction lists and a flattened list of all packages.
	// V2 returns disjoint sets per transaction
	transactions := make(TransactionList, len(result.Transactions))

	// Convert depsolved packages
	for transIdx, transaction := range result.Transactions {
		transPkgs := make(rpmmd.PackageList, 0, len(transaction))
		for _, pkg := range transaction {
			repo, ok := repoMap[pkg.RepoID]
			if !ok {
				return nil, fmt.Errorf("repo ID not found in repositories: %s", pkg.RepoID)
			}

			rpmPkg, err := h.toRPMMDPackage(pkg, repo)
			if err != nil {
				return nil, err
			}
			transPkgs = append(transPkgs, rpmPkg)
		}
		transactions[transIdx] = transPkgs
	}

	// Convert modules
	modules := make([]rpmmd.ModuleSpec, 0, len(result.Modules))
	for _, mod := range result.Modules {
		modules = append(modules, h.toRPMMDModuleSpec(mod))
	}

	return &depsolveResultRaw{
		Transactions: transactions,
		Modules:      modules,
		Repos:        repos,
		Solver:       result.Solver,
		SBOMRaw:      result.SBOM,
	}, nil
}

func (h *v2Handler) parseDumpResult(output []byte) (*DumpResult, error) {
	pkgs, repos, solver, err := h.parsePackageListResult(output, "dump")
	if err != nil {
		return nil, err
	}
	return &DumpResult{Packages: pkgs, Repos: repos, Solver: solver}, nil
}

func (h *v2Handler) parseSearchResult(output []byte) (*SearchResult, error) {
	pkgs, repos, solver, err := h.parsePackageListResult(output, "search")
	if err != nil {
		return nil, err
	}
	return &SearchResult{Packages: pkgs, Repos: repos, Solver: solver}, nil
}

// V2 API Helper Functions

// parsePackageListResult parses the common dump/search response structure.
func (h *v2Handler) parsePackageListResult(output []byte, operation string) (rpmmd.PackageList, []rpmmd.RepoConfig, string, error) {
	var result v2PackageListResult
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, nil, "", fmt.Errorf("decoding %s result failed: %w", operation, err)
	}

	repos, repoMap := h.toRPMMDRepoConfigs(result.Repos)

	// Convert packages
	pkgs := make(rpmmd.PackageList, 0, len(result.Packages))
	for _, pkg := range result.Packages {
		repo, ok := repoMap[pkg.RepoID]
		if !ok {
			return nil, nil, "", fmt.Errorf("repo ID not found in repositories: %s", pkg.RepoID)
		}
		rpmPkg, err := h.toRPMMDPackage(pkg, repo)
		if err != nil {
			return nil, nil, "", err
		}
		pkgs = append(pkgs, rpmPkg)
	}

	return pkgs, repos, result.Solver, nil
}

func (h *v2Handler) reposFromRPMMD(cfg *solverConfig, rpmRepos []rpmmd.RepoConfig) ([]v2Repository, error) {
	dnfRepos := make([]v2Repository, len(rpmRepos))
	for idx, rr := range rpmRepos {
		dr := v2Repository{
			ID:             rr.Hash(),
			Name:           rr.Name,
			BaseURLs:       slices.Clone(rr.BaseURLs),
			Metalink:       rr.Metalink,
			MirrorList:     rr.MirrorList,
			GPGKeys:        slices.Clone(rr.GPGKeys),
			GPGCheck:       common.ClonePtr(rr.CheckGPG),
			RepoGPGCheck:   common.ClonePtr(rr.CheckRepoGPG),
			MetadataExpire: rr.MetadataExpire,
			SSLCACert:      rr.SSLCACert,
			SSLClientKey:   rr.SSLClientKey,
			SSLClientCert:  rr.SSLClientCert,
			ModuleHotfixes: common.ClonePtr(rr.ModuleHotfixes),
		}

		if rr.IgnoreSSL != nil {
			dr.SSLVerify = common.ToPtr(!*rr.IgnoreSSL)
		}

		if rr.RHUI {
			// RHUI repos delegate secret discovery to osbuild-depsolve-dnf.
			// The Python solver reads the host RHUI repo files and discovers
			// SSL certs from /etc/pki/rhui/ directly.
			dr.RHUI = true
		} else if rr.RHSM {
			// TODO: Enable V2 RHSM secrets discovery by setting dr.RHSM = true
			// and removing the client-side secrets resolution below.
			// This requires functional testing to ensure RHSM secrets discovery
			// works correctly in the solver.
			// See: https://github.com/osbuild/images/issues/2055

			// NOTE: It is assumed that the s.subscriptions are not nil if the repo needs RHSM secrets
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

func (h *v2Handler) toRPMMDPackage(pkg v2Package, repo *rpmmd.RepoConfig) (rpmmd.Package, error) {
	if repo == nil {
		return rpmmd.Package{}, fmt.Errorf("repository configuration is nil for package %s", pkg.Name)
	}

	rpmPkg := rpmmd.Package{
		Name:            pkg.Name,
		Version:         pkg.Version,
		Release:         pkg.Release,
		Arch:            pkg.Arch,
		RepoID:          pkg.RepoID,
		Location:        pkg.Location,
		RemoteLocations: pkg.RemoteLocations,
		License:         pkg.License,
		Summary:         pkg.Summary,
		Description:     pkg.Description,
		URL:             pkg.URL,
		Vendor:          pkg.Vendor,
		Packager:        pkg.Packager,
		Group:           pkg.Group,
		SourceRpm:       pkg.SourceRPM,
		Reason:          pkg.Reason,
		Files:           pkg.Files,
		Repo:            repo,
	}

	if pkg.Epoch < 0 {
		return rpmmd.Package{}, fmt.Errorf("invalid negative epoch for package %s", pkg.Name)
	}
	rpmPkg.Epoch = uint(pkg.Epoch)

	// Parse checksum
	if pkg.Checksum != nil {
		rpmPkg.Checksum = rpmmd.Checksum{
			Type:  pkg.Checksum.Algorithm,
			Value: pkg.Checksum.Value,
		}
	}

	// Parse header checksum
	if pkg.HeaderChecksum != nil {
		rpmPkg.HeaderChecksum = rpmmd.Checksum{
			Type:  pkg.HeaderChecksum.Algorithm,
			Value: pkg.HeaderChecksum.Value,
		}
	}

	// Parse build time (RFC3339 format)
	if pkg.BuildTime != "" {
		buildTime, err := time.Parse(time.RFC3339, pkg.BuildTime)
		if err != nil {
			return rpmmd.Package{}, fmt.Errorf("parsing build_time %q for package %s failed: %w", pkg.BuildTime, pkg.Name, err)
		}
		rpmPkg.BuildTime = buildTime
	}

	// Parse sizes
	if pkg.DownloadSize < 0 {
		return rpmmd.Package{}, fmt.Errorf("invalid negative download size for package %s", pkg.Name)
	}
	rpmPkg.DownloadSize = uint64(pkg.DownloadSize)
	if pkg.InstallSize < 0 {
		return rpmmd.Package{}, fmt.Errorf("invalid negative install size for package %s", pkg.Name)
	}
	rpmPkg.InstallSize = uint64(pkg.InstallSize)

	// Convert dependencies
	rpmPkg.Provides = h.toRPMMDRelDepList(pkg.Provides)
	rpmPkg.Requires = h.toRPMMDRelDepList(pkg.Requires)
	rpmPkg.RequiresPre = h.toRPMMDRelDepList(pkg.RequiresPre)
	rpmPkg.Conflicts = h.toRPMMDRelDepList(pkg.Conflicts)
	rpmPkg.Obsoletes = h.toRPMMDRelDepList(pkg.Obsoletes)
	rpmPkg.RegularRequires = h.toRPMMDRelDepList(pkg.RegularRequires)
	rpmPkg.Recommends = h.toRPMMDRelDepList(pkg.Recommends)
	rpmPkg.Suggests = h.toRPMMDRelDepList(pkg.Suggests)
	rpmPkg.Enhances = h.toRPMMDRelDepList(pkg.Enhances)
	rpmPkg.Supplements = h.toRPMMDRelDepList(pkg.Supplements)

	// Assign convenience values from the repository configuration
	if repo.CheckGPG != nil {
		rpmPkg.CheckGPG = *repo.CheckGPG
	}

	if repo.IgnoreSSL != nil {
		rpmPkg.IgnoreSSL = *repo.IgnoreSSL
	}

	// Set mTLS secrets if SSLClientKey is set.
	// The Solver will override secrets to 'org.osbuild.rhsm' if the repo needs RHSM secrets.
	if repo.SSLClientKey != "" {
		rpmPkg.Secrets = "org.osbuild.mtls"
	}

	return rpmPkg, nil
}

func (h *v2Handler) toRPMMDRelDepList(deps []v2Dependency) rpmmd.RelDepList {
	if len(deps) == 0 {
		return nil
	}
	result := make(rpmmd.RelDepList, len(deps))
	for i, dep := range deps {
		result[i] = rpmmd.RelDep{
			Name:         dep.Name,
			Relationship: dep.Relation,
			Version:      dep.Version,
		}
	}
	return result
}

func (h *v2Handler) toRPMMDModuleSpec(mod v2ModuleSpec) rpmmd.ModuleSpec {
	return rpmmd.ModuleSpec{
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
	}
}

func (h *v2Handler) toRPMMDRepoConfig(repo v2Repository) rpmmd.RepoConfig {
	var ignoreSSL bool
	if sslVerify := repo.SSLVerify; sslVerify != nil {
		ignoreSSL = !*sslVerify
	}

	return rpmmd.RepoConfig{
		Id:             repo.ID,
		Name:           repo.Name,
		BaseURLs:       slices.Clone(repo.BaseURLs),
		Metalink:       repo.Metalink,
		MirrorList:     repo.MirrorList,
		GPGKeys:        slices.Clone(repo.GPGKeys),
		CheckGPG:       common.ClonePtr(repo.GPGCheck),
		CheckRepoGPG:   common.ClonePtr(repo.RepoGPGCheck),
		IgnoreSSL:      &ignoreSSL,
		MetadataExpire: repo.MetadataExpire,
		ModuleHotfixes: common.ClonePtr(repo.ModuleHotfixes),
		Enabled:        common.ToPtr(true),
		SSLCACert:      repo.SSLCACert,
		SSLClientKey:   repo.SSLClientKey,
		SSLClientCert:  repo.SSLClientCert,
		RHSM:           repo.RHSM,
	}
}

func (h *v2Handler) toRPMMDRepoConfigs(v2Repos map[string]v2Repository) ([]rpmmd.RepoConfig, map[string]*rpmmd.RepoConfig) {
	repos := make([]rpmmd.RepoConfig, 0, len(v2Repos))
	for repoID := range v2Repos {
		v2Repo := v2Repos[repoID]
		repo := h.toRPMMDRepoConfig(v2Repo)
		repos = append(repos, repo)
	}
	// Sort repos by ID for deterministic ordering
	slices.SortFunc(repos, func(a, b rpmmd.RepoConfig) int {
		return cmp.Compare(a.Id, b.Id)
	})
	// Build repoMap after sorting so pointers are correct
	repoMap := make(map[string]*rpmmd.RepoConfig, len(repos))
	for i := range repos {
		repoMap[repos[i].Id] = &repos[i]
	}
	return repos, repoMap
}
