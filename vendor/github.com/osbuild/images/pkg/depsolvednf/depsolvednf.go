// Package depsolvednf is an interface to the osbuild-depsolve-dnf Python script
// that is packaged with the osbuild project. The core component of this
// package is the Solver type. The Solver can be configured with
// distribution-specific values (platform ID, architecture, and version
// information) and provides methods for dependency resolution (Depsolve) and
// retrieving a full list of repository package metadata (FetchMetadata).
//
// Alternatively, a BaseSolver can be created which represents an un-configured
// Solver. This type can't be used for depsolving, but can be used to create
// configured Solver instances sharing the same cache directory.
//
// This package relies on the types defined in rpmmd to describe RPM package
// metadata.
package depsolvednf

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/rhsm"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
)

// BaseSolver defines the basic solver configuration without platform
// information. It can be used to create configured Solver instances with the
// NewWithConfig() method. A BaseSolver maintains the global repository cache
// directory.
type BaseSolver struct {
	// Cache information
	cache *rpmCache

	// Path to the osbuild-depsolve-dnf binary and optional args (default: "/usr/libexec/osbuild-depsolve-dnf")
	depsolveDNFCmd []string

	resultCache *dnfCache
}

// Find the osbuild-depsolve-dnf script. This checks the default location in
// /usr/libexec but also /usr/lib in case it's used on a distribution that
// doesn't use libexec.
func findDepsolveDnf() string {
	locations := []string{"/usr/libexec/osbuild-depsolve-dnf", "/usr/lib/osbuild/osbuild-depsolve-dnf"}

	// Override the default location
	testLocation := os.Getenv("OSBUILD_DEPSOLVE_DNF")
	if len(testLocation) > 0 {
		locations = []string{testLocation}
	}
	for _, djPath := range locations {
		_, err := os.Stat(djPath)
		if !os.IsNotExist(err) {
			return djPath
		}
	}

	// if it's not found, return empty string; the run() function will fail if
	// it's used before setting.
	return ""
}

// Create a new unconfigured BaseSolver (without platform information). It can
// be used to create configured Solver instances with the NewWithConfig()
// method.
func NewBaseSolver(cacheDir string) *BaseSolver {
	return &BaseSolver{
		cache:       newRPMCache(cacheDir, 1024*1024*1024), // 1 GiB
		resultCache: NewDNFCache(60 * time.Second),
	}
}

// SetMaxCacheSize sets the maximum size for the global repository metadata
// cache. This is the maximum size of the cache after a CleanCache()
// call. Cache cleanup is never performed automatically.
func (s *BaseSolver) SetMaxCacheSize(size uint64) {
	s.cache.maxSize = size
}

// SetDepsolveDNFPath sets the path to the osbuild-depsolve-dnf binary and optionally any command line arguments.
func (s *BaseSolver) SetDepsolveDNFPath(cmd string, args ...string) {
	s.depsolveDNFCmd = append([]string{cmd}, args...)
}

// NewWithConfig initialises a Solver with the platform information and the
// BaseSolver's subscription info, cache directory, and osbuild-depsolve-dnf path.
// Also loads system subscription information.
func (bs *BaseSolver) NewWithConfig(modulePlatformID, releaseVer, arch, distro string) *Solver {
	s := new(Solver)
	s.BaseSolver = *bs
	s.modulePlatformID = modulePlatformID
	s.arch = arch
	s.releaseVer = releaseVer
	s.distro = distro
	s.subscriptions, s.subscriptionsErr = rhsm.LoadSystemSubscriptions()
	return s
}

// CleanCache deletes the least recently used repository metadata caches until
// the total size of the cache falls below the configured maximum size (see
// SetMaxCacheSize()).
func (bs *BaseSolver) CleanCache() error {
	bs.resultCache.CleanCache()
	return bs.cache.shrink()
}

// CleanupOldCacheDirs will remove cache directories for unsupported distros
// eg. Once support for a fedora release stops and it is removed, this will
// delete its directory under BaseSolver cache root.
//
// A happy side effect of this is that it will delete old cache directories
// and files from before the switch to per-distro cache directories.
//
// NOTE: This does not return any errors. This is because the most common one
// will be a nonexistant directory which will be created later, during initial
// cache creation. Any other errors like permission issues will be caught by
// later use of the cache. eg. touchRepo
func (bs *BaseSolver) CleanupOldCacheDirs(distros []string) {
	CleanupOldCacheDirs(bs.cache.root, distros)
}

// Solver is configured with system information in order to resolve
// dependencies for RPM packages using DNF.
type Solver struct {
	BaseSolver

	// Platform ID, e.g., "platform:el8"
	modulePlatformID string

	// System architecture
	arch string

	// Release version of the distro. This is used in repo files on the host
	// system and required for subscription support.
	releaseVer string

	// Full distribution string, eg. fedora-38, used to create separate dnf cache directories
	// for each distribution.
	distro string

	rootDir string

	// Proxy to use while depsolving. This is used in DNF's base configuration.
	proxy string

	subscriptions    *rhsm.Subscriptions
	subscriptionsErr error

	// Stderr is the stderr output from osbuild-depsolve-dnf, if unset os.Stderr
	// will be used.
	//
	// XXX: ideally this would not be public but just passed via
	// NewSolver() but it already has 5 args so ideally we would
	// add a SolverOptions struct here with "CacheDir" and "Stderr"?
	Stderr io.Writer
}

// DepsolveResult contains the results of a depsolve operation.
type DepsolveResult struct {
	Packages rpmmd.PackageList
	Modules  []rpmmd.ModuleSpec
	Repos    []rpmmd.RepoConfig
	SBOM     *sbom.Document
	Solver   string
}

// Create a new Solver with the given configuration. Initialising a Solver also loads system subscription information.
func NewSolver(modulePlatformID, releaseVer, arch, distro, cacheDir string) *Solver {
	s := NewBaseSolver(cacheDir)
	return s.NewWithConfig(modulePlatformID, releaseVer, arch, distro)
}

// SetRootDir sets a path from which repository configurations, gpg keys, and
// vars are loaded during depsolve, instead of (or in addition to) the
// repositories and keys included in each depsolve request.
func (s *Solver) SetRootDir(path string) {
	s.rootDir = path
}

// GetCacheDir returns a distro specific rpm cache directory
// It ensures that the distro name is below the root cache directory, and if there is
// a problem it returns the root cache instead of an error.
func (s *Solver) GetCacheDir() string {
	b := filepath.Base(strings.Join([]string{s.modulePlatformID, s.releaseVer, s.arch}, "-"))
	if b == "." || b == "/" {
		return s.cache.root
	}

	return filepath.Join(s.cache.root, b)
}

// Set the proxy to use while depsolving. The proxy will be set in DNF's base configuration.
func (s *Solver) SetProxy(proxy string) error {
	if _, err := url.ParseRequestURI(proxy); err != nil {
		return fmt.Errorf("proxy URL %q is invalid", proxy)
	}
	s.proxy = proxy
	return nil
}

// Depsolve the list of required package sets with explicit excludes using
// their associated repositories.  Each package set is depsolved as a separate
// transactions in a chain.  It returns a list of all packages (with solved
// dependencies) that will be installed into the system.
func (s *Solver) Depsolve(pkgSets []rpmmd.PackageSet, sbomType sbom.StandardType) (*DepsolveResult, error) {
	req, rhsmMap, err := s.makeDepsolveRequest(pkgSets, sbomType)
	if err != nil {
		return nil, fmt.Errorf("makeDepsolveRequest failed: %w", err)
	}

	// get non-exclusive read lock
	s.cache.locker.RLock()
	defer s.cache.locker.RUnlock()

	output, err := run(s.depsolveDNFCmd, req, s.Stderr)
	if err != nil {
		return nil, fmt.Errorf("running osbuild-depsolve-dnf failed:\n%w", err)
	}
	// touch repos to now
	now := time.Now().Local()
	for _, r := range req.Arguments.Repos {
		// ignore errors
		_ = s.cache.touchRepo(r.Hash(), now)
	}
	s.cache.updateInfo()

	var result depsolveResult
	dec := json.NewDecoder(bytes.NewReader(output))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding depsolve result failed: %w", err)
	}

	packages, modules, repos := result.toRPMMD(rhsmMap)

	var sbomDoc *sbom.Document
	if sbomType != sbom.StandardTypeNone {
		sbomDoc, err = sbom.NewDocument(sbomType, result.SBOM)
		if err != nil {
			return nil, fmt.Errorf("creating SBOM document failed: %w", err)
		}
	}

	return &DepsolveResult{
		Packages: packages,
		Modules:  modules,
		Repos:    repos,
		SBOM:     sbomDoc,
		Solver:   result.Solver,
	}, nil
}

// FetchMetadata returns the list of all the available packages in repos and
// their info.
func (s *Solver) FetchMetadata(repos []rpmmd.RepoConfig) (rpmmd.PackageList, error) {
	req, err := s.makeDumpRequest(repos)
	if err != nil {
		return nil, err
	}

	// get non-exclusive read lock
	s.cache.locker.RLock()
	defer s.cache.locker.RUnlock()

	// Is this cached?
	if pkgs, ok := s.resultCache.Get(req.Hash()); ok {
		return pkgs, nil
	}

	rawRes, err := run(s.depsolveDNFCmd, req, s.Stderr)
	if err != nil {
		return nil, err
	}

	// touch repos to now
	now := time.Now().Local()
	for _, r := range repos {
		// ignore errors
		_ = s.cache.touchRepo(r.Hash(), now)
	}
	s.cache.updateInfo()

	var res dumpResult
	if err := json.Unmarshal(rawRes, &res); err != nil {
		return nil, err
	}

	pkgs := res.toRPMMD()

	sortID := func(pkg rpmmd.Package) string {
		return fmt.Sprintf("%s-%s-%s", pkg.Name, pkg.Version, pkg.Release)
	}
	sort.Slice(pkgs, func(i, j int) bool {
		return sortID(pkgs[i]) < sortID(pkgs[j])
	})

	// Cache the results
	s.resultCache.Store(req.Hash(), pkgs)
	return pkgs, nil
}

// SearchMetadata searches for packages and returns a list of the info for matches.
func (s *Solver) SearchMetadata(repos []rpmmd.RepoConfig, packages []string) (rpmmd.PackageList, error) {
	req, err := s.makeSearchRequest(repos, packages)
	if err != nil {
		return nil, err
	}

	// get non-exclusive read lock
	s.cache.locker.RLock()
	defer s.cache.locker.RUnlock()

	// Is this cached?
	if pkgs, ok := s.resultCache.Get(req.Hash()); ok {
		return pkgs, nil
	}

	rawRes, err := run(s.depsolveDNFCmd, req, s.Stderr)
	if err != nil {
		return nil, err
	}

	// touch repos to now
	now := time.Now().Local()
	for _, r := range repos {
		// ignore errors
		_ = s.cache.touchRepo(r.Hash(), now)
	}
	s.cache.updateInfo()

	var res searchResult
	if err := json.Unmarshal(rawRes, &res); err != nil {
		return nil, err
	}

	pkgs := res.toRPMMD()

	sortID := func(pkg rpmmd.Package) string {
		return fmt.Sprintf("%s-%s-%s", pkg.Name, pkg.Version, pkg.Release)
	}
	sort.Slice(pkgs, func(i, j int) bool {
		return sortID(pkgs[i]) < sortID(pkgs[j])
	})

	// Cache the results
	s.resultCache.Store(req.Hash(), pkgs)
	return pkgs, nil
}

func (s *Solver) reposFromRPMMD(rpmRepos []rpmmd.RepoConfig) ([]repoConfig, error) {
	dnfRepos := make([]repoConfig, len(rpmRepos))
	for idx, rr := range rpmRepos {
		dr := repoConfig{
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
			repoHash:       rr.Hash(),
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
			if s.subscriptions == nil {
				return nil, fmt.Errorf("This system does not have any valid subscriptions. Subscribe it before specifying rhsm: true in sources (error details: %w)", s.subscriptionsErr)
			}
			secrets, err := s.subscriptions.GetSecretsForBaseurl(rr.BaseURLs, s.arch, s.releaseVer)
			if err != nil {
				return nil, fmt.Errorf("RHSM secrets not found on the host for this baseurl: %s", rr.BaseURLs)
			}
			dr.SSLCACert = secrets.SSLCACert
			dr.SSLClientKey = secrets.SSLClientKey
			dr.SSLClientCert = secrets.SSLClientCert
		}

		dnfRepos[idx] = dr
	}
	return dnfRepos, nil
}

// Repository configuration for resolving dependencies for a set of packages. A
// Solver needs at least one RPM repository configured to be able to depsolve.
type repoConfig struct {
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
	// set the repo hass from `rpmmd.RepoConfig.Hash()` function
	// rather than re-calculating it
	repoHash string
}

// use the hash calculated by the `rpmmd.RepoConfig.Hash()`
// function rather than re-implementing the same code
func (r *repoConfig) Hash() string {
	return r.repoHash
}

// Helper function for creating a depsolve request payload. The request defines
// a sequence of transactions, each depsolving one of the elements of `pkgSets`
// in the order they appear. The repositories are collected in the request
// arguments indexed by their ID, and each transaction lists the repositories
// it will use for depsolving.
//
// The second return value is a map of repository IDs that have RHSM enabled.
// The RHSM property is not part of the dnf repository configuration so it's
// returned separately for setting the value on each package that requires it.
//
// NOTE: Due to implementation limitations of DNF and osbuild-depsolve-dnf,
// each package set in the chain must use all of the repositories used by its
// predecessor. An error is returned if this requirement is not met.
func (s *Solver) makeDepsolveRequest(pkgSets []rpmmd.PackageSet, sbomType sbom.StandardType) (*Request, map[string]bool, error) {
	// dedupe repository configurations but maintain order
	// the order in which repositories are added to the request affects the
	// order of the dependencies in the result
	repos := make([]rpmmd.RepoConfig, 0)
	rhsmMap := make(map[string]bool)

	for _, ps := range pkgSets {
		for _, repo := range ps.Repositories {
			id := repo.Hash()
			if _, ok := rhsmMap[id]; !ok {
				rhsmMap[id] = repo.RHSM
				repos = append(repos, repo)
			}
		}
	}

	transactions := make([]transactionArgs, len(pkgSets))
	for dsIdx, pkgSet := range pkgSets {
		transactions[dsIdx] = transactionArgs{
			PackageSpecs:      pkgSet.Include,
			ExcludeSpecs:      pkgSet.Exclude,
			ModuleEnableSpecs: pkgSet.EnabledModules,
			InstallWeakDeps:   pkgSet.InstallWeakDeps,
		}

		for _, jobRepo := range pkgSet.Repositories {
			transactions[dsIdx].RepoIDs = append(transactions[dsIdx].RepoIDs, jobRepo.Hash())
		}

		// If more than one transaction, ensure that the transaction uses
		// all of the repos from its predecessor
		if dsIdx > 0 {
			prevRepoIDs := transactions[dsIdx-1].RepoIDs
			if len(transactions[dsIdx].RepoIDs) < len(prevRepoIDs) {
				return nil, nil, fmt.Errorf("chained packageSet %d does not use all of the repos used by its predecessor", dsIdx)
			}

			for idx, repoID := range prevRepoIDs {
				if repoID != transactions[dsIdx].RepoIDs[idx] {
					return nil, nil, fmt.Errorf("chained packageSet %d does not use all of the repos used by its predecessor", dsIdx)
				}
			}
		}
	}

	dnfRepoMap, err := s.reposFromRPMMD(repos)
	if err != nil {
		return nil, nil, err
	}

	args := arguments{
		Repos:            dnfRepoMap,
		RootDir:          s.rootDir,
		Transactions:     transactions,
		OptionalMetadata: s.optionalMetadataForDistro(),
	}

	req := Request{
		Command:          "depsolve",
		ModulePlatformID: s.modulePlatformID,
		Arch:             s.arch,
		Releasever:       s.releaseVer,
		CacheDir:         s.GetCacheDir(),
		Proxy:            s.proxy,
		Arguments:        args,
	}

	if sbomType != sbom.StandardTypeNone {
		req.Arguments.Sbom = &sbomRequest{Type: sbomType.String()}
	}

	return &req, rhsmMap, nil
}

func (s *Solver) optionalMetadataForDistro() []string {
	// filelist repo metadata is required when using newer versions of libdnf
	// with old repositories or packages that specify dependencies on files.
	// EL10+ and Fedora 40+ packaging guidelines prohibit depending on
	// filepaths so filelist downloads are disabled by default and are not
	// required when depsolving for those distros. Explicitly enable the option
	// for older distro versions in case we are using a newer libdnf.
	switch s.modulePlatformID {
	case "platform:f39", "platform:el7", "platform:el8", "platform:el9":
		return []string{"filelists"}
	}
	return nil
}

// Helper function for creating a dump request payload
func (s *Solver) makeDumpRequest(repos []rpmmd.RepoConfig) (*Request, error) {
	dnfRepos, err := s.reposFromRPMMD(repos)
	if err != nil {
		return nil, err
	}
	req := Request{
		Command:          "dump",
		ModulePlatformID: s.modulePlatformID,
		Arch:             s.arch,
		Releasever:       s.releaseVer,
		CacheDir:         s.GetCacheDir(),
		Proxy:            s.proxy,
		Arguments: arguments{
			Repos: dnfRepos,
		},
	}
	return &req, nil
}

// Helper function for creating a search request payload
func (s *Solver) makeSearchRequest(repos []rpmmd.RepoConfig, packages []string) (*Request, error) {
	dnfRepos, err := s.reposFromRPMMD(repos)
	if err != nil {
		return nil, err
	}
	req := Request{
		Command:          "search",
		ModulePlatformID: s.modulePlatformID,
		Arch:             s.arch,
		CacheDir:         s.GetCacheDir(),
		Releasever:       s.releaseVer,
		Proxy:            s.proxy,
		Arguments: arguments{
			Repos: dnfRepos,
			Search: searchArgs{
				Packages: packages,
			},
		},
	}
	return &req, nil
}

// convert internal a list of PackageSpecs and map of repoConfig to the rpmmd
// equivalents and attach key and subscription information based on the
// repository configs.
func (result depsolveResult) toRPMMD(rhsm map[string]bool) (rpmmd.PackageList, []rpmmd.ModuleSpec, []rpmmd.RepoConfig) {
	pkgs := result.Packages
	repos := result.Repos
	rpmDependencies := make(rpmmd.PackageList, len(pkgs))
	for i, dep := range pkgs {
		repo, ok := repos[dep.RepoID]
		if !ok {
			panic("dependency repo ID not found in repositories")
		}
		dep := pkgs[i]
		rpmDependencies[i].Name = dep.Name
		rpmDependencies[i].Epoch = dep.Epoch
		rpmDependencies[i].Version = dep.Version
		rpmDependencies[i].Release = dep.Release
		rpmDependencies[i].Arch = dep.Arch
		rpmDependencies[i].RemoteLocations = []string{dep.RemoteLocation}

		depChecksum := strings.Split(dep.Checksum, ":")
		if len(depChecksum) != 2 {
			panic(fmt.Sprintf("invalid checksum format for package %s: %s", dep.Name, dep.Checksum))
		}
		rpmDependencies[i].Checksum = rpmmd.Checksum{
			Type:  depChecksum[0],
			Value: depChecksum[1],
		}
		rpmDependencies[i].CheckGPG = repo.GPGCheck
		rpmDependencies[i].RepoID = dep.RepoID
		rpmDependencies[i].Location = dep.Path
		if verify := repo.SSLVerify; verify != nil {
			rpmDependencies[i].IgnoreSSL = !*verify
		}

		// The ssl secrets will also be set if rhsm is true,
		// which should take priority.
		if rhsm[dep.RepoID] {
			rpmDependencies[i].Secrets = "org.osbuild.rhsm"
		} else if repo.SSLClientKey != "" {
			rpmDependencies[i].Secrets = "org.osbuild.mtls"
		}
	}

	mods := result.Modules
	moduleSpecs := make([]rpmmd.ModuleSpec, len(mods))

	i := 0
	for _, mod := range mods {
		moduleSpecs[i].ModuleConfigFile.Data.Name = mod.ModuleConfigFile.Data.Name
		moduleSpecs[i].ModuleConfigFile.Data.Stream = mod.ModuleConfigFile.Data.Stream
		moduleSpecs[i].ModuleConfigFile.Data.State = mod.ModuleConfigFile.Data.State
		moduleSpecs[i].ModuleConfigFile.Data.Profiles = mod.ModuleConfigFile.Data.Profiles

		moduleSpecs[i].FailsafeFile.Path = mod.FailsafeFile.Path
		moduleSpecs[i].FailsafeFile.Data = mod.FailsafeFile.Data

		i++
	}

	repoConfigs := make([]rpmmd.RepoConfig, 0, len(repos))
	for repoID := range repos {
		repo := repos[repoID]
		var ignoreSSL bool
		if sslVerify := repo.SSLVerify; sslVerify != nil {
			ignoreSSL = !*sslVerify
		}
		repoConfigs = append(repoConfigs, rpmmd.RepoConfig{
			Id:             repo.ID,
			Name:           repo.Name,
			BaseURLs:       repo.BaseURLs,
			Metalink:       repo.Metalink,
			MirrorList:     repo.MirrorList,
			GPGKeys:        repo.GPGKeys,
			CheckGPG:       &repo.GPGCheck,
			CheckRepoGPG:   &repo.RepoGPGCheck,
			IgnoreSSL:      &ignoreSSL,
			MetadataExpire: repo.MetadataExpire,
			ModuleHotfixes: repo.ModuleHotfixes,
			Enabled:        common.ToPtr(true),
			SSLCACert:      repo.SSLCACert,
			SSLClientKey:   repo.SSLClientKey,
			SSLClientCert:  repo.SSLClientCert,
		})
	}
	return rpmDependencies, moduleSpecs, repoConfigs
}

// Request command and arguments for osbuild-depsolve-dnf
type Request struct {
	// Command should be either "depsolve" or "dump"
	Command string `json:"command"`

	// Platform ID, e.g., "platform:el8"
	ModulePlatformID string `json:"module_platform_id"`

	// Distro Releasever, e.e., "8"
	Releasever string `json:"releasever"`

	// System architecture
	Arch string `json:"arch"`

	// Cache directory for the DNF metadata
	CacheDir string `json:"cachedir"`

	// Proxy to use
	Proxy string `json:"proxy"`

	// Arguments for the action defined by Command
	Arguments arguments `json:"arguments"`
}

// Hash returns a hash of the unique aspects of the Request
//
//nolint:errcheck
func (r *Request) Hash() string {
	h := sha256.New()

	h.Write([]byte(r.Command))
	h.Write([]byte(r.ModulePlatformID))
	h.Write([]byte(r.Arch))
	for _, repo := range r.Arguments.Repos {
		h.Write([]byte(repo.Hash()))
	}
	fmt.Fprintf(h, "%T", r.Arguments.Search.Latest)
	h.Write([]byte(strings.Join(r.Arguments.Search.Packages, "")))

	return fmt.Sprintf("%x", h.Sum(nil))
}

type sbomRequest struct {
	Type string `json:"type"`
}

// arguments for a osbuild-depsolve-dnf request
type arguments struct {
	// Repositories to use for depsolving
	Repos []repoConfig `json:"repos"`

	// Search terms to use with search command
	Search searchArgs `json:"search"`

	// Depsolve package sets and repository mappings for this request
	Transactions []transactionArgs `json:"transactions"`

	// Load repository configurations, gpg keys, and vars from an os-root-like
	// tree.
	RootDir string `json:"root_dir"`

	// Optional metadata to download for the repositories
	OptionalMetadata []string `json:"optional-metadata,omitempty"`

	// Optionally request an SBOM from depsolving
	Sbom *sbomRequest `json:"sbom,omitempty"`
}

type searchArgs struct {
	// Only include latest NEVRA when true
	Latest bool `json:"latest"`

	// List of package name globs to search for
	// If it has '*' it is passed to dnf glob search, if it has *name* it is passed
	// to substr matching, and if it has neither an exact match is expected.
	Packages []string `json:"packages"`
}

type transactionArgs struct {
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

type packageSpecs []packageSpec
type moduleSpecs map[string]moduleSpec

type depsolveResult struct {
	Packages packageSpecs          `json:"packages"`
	Repos    map[string]repoConfig `json:"repos"`
	Modules  moduleSpecs           `json:"modules"`

	// (optional) contains the solver used, e.g. "dnf5"
	Solver string `json:"solver,omitempty"`

	// (optional) contains the SBOM for the depsolved transaction
	SBOM json.RawMessage `json:"sbom,omitempty"`
}

// legacyPackageList represents the old 'PackageList' structure, which
// was used for both dump and search results. It is kept here for unmarshaling
// the results.
type legacyPackageList []struct {
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

type dumpResult = legacyPackageList
type searchResult = legacyPackageList

func (pl legacyPackageList) toRPMMD() rpmmd.PackageList {
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

// Package specification
type packageSpec struct {
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
type moduleSpec struct {
	ModuleConfigFile moduleConfigFile   `json:"module-file"`
	FailsafeFile     moduleFailsafeFile `json:"failsafe-file"`
}

type moduleConfigFile struct {
	Path string           `json:"path"`
	Data moduleConfigData `json:"data"`
}

type moduleConfigData struct {
	Name     string   `json:"name"`
	Stream   string   `json:"stream"`
	Profiles []string `json:"profiles"`
	State    string   `json:"state"`
}

type moduleFailsafeFile struct {
	Path string `json:"path"`
	Data string `json:"data"`
}

// osbuild-depsolve-dnf error structure
type Error struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

func (err Error) Error() string {
	return fmt.Sprintf("DNF error occurred: %s: %s", err.Kind, err.Reason)
}

// parseError parses the response from osbuild-depsolve-dnf into the Error type and appends
// the name and URL of a repository to all detected repository IDs in the
// message.
func parseError(data []byte, repos []repoConfig) Error {
	var e Error
	if len(data) == 0 {
		return Error{
			Kind:   "InternalError",
			Reason: "osbuild-depsolve-dnf output was empty",
		}
	}

	if err := json.Unmarshal(data, &e); err != nil {
		// dumping the error into the Reason can get noisy, but it's good for troubleshooting
		return Error{
			Kind:   "InternalError",
			Reason: fmt.Sprintf("Failed to unmarshal osbuild-depsolve-dnf error output %q: %s", string(data), err.Error()),
		}
	}

	// append to any instance of a repository ID the URL (or metalink, mirrorlist, etc)
	for _, repo := range repos {
		idstr := fmt.Sprintf("'%s'", repo.ID)
		var nameURL string
		if len(repo.BaseURLs) > 0 {
			nameURL = strings.Join(repo.BaseURLs, ",")
		} else if len(repo.Metalink) > 0 {
			nameURL = repo.Metalink
		} else if len(repo.MirrorList) > 0 {
			nameURL = repo.MirrorList
		}

		if len(repo.Name) > 0 {
			nameURL = fmt.Sprintf("%s: %s", repo.Name, nameURL)
		}
		e.Reason = strings.ReplaceAll(e.Reason, idstr, fmt.Sprintf("%s [%s]", idstr, nameURL))
	}

	return e
}
func ParseError(data []byte) Error {
	var e Error
	if err := json.Unmarshal(data, &e); err != nil {
		// dumping the error into the Reason can get noisy, but it's good for troubleshooting
		return Error{
			Kind:   "InternalError",
			Reason: fmt.Sprintf("Failed to unmarshal osbuild-depsolve-dnf error output %q: %s", string(data), err.Error()),
		}
	}
	return e
}

func run(dnfJsonCmd []string, req *Request, stderr io.Writer) ([]byte, error) {
	if len(dnfJsonCmd) == 0 {
		dnfJsonCmd = []string{findDepsolveDnf()}
	}
	if len(dnfJsonCmd) == 0 {
		return nil, fmt.Errorf("osbuild-depsolve-dnf command undefined")
	}
	ex := dnfJsonCmd[0]
	args := make([]string, len(dnfJsonCmd)-1)
	if len(dnfJsonCmd) > 1 {
		args = dnfJsonCmd[1:]
	}
	cmd := exec.Command(ex, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("creating stdin pipe for %s failed: %w", ex, err)
	}

	if stderr != nil {
		cmd.Stderr = stderr
	} else {
		cmd.Stderr = os.Stderr
	}
	stdout := new(bytes.Buffer)
	cmd.Stdout = stdout

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("starting %s failed: %w", ex, err)
	}

	err = json.NewEncoder(stdin).Encode(req)
	if err != nil {
		return nil, fmt.Errorf("encoding request for %s failed: %w", ex, err)
	}
	stdin.Close()

	err = cmd.Wait()
	output := stdout.Bytes()
	if runError, ok := err.(*exec.ExitError); ok && runError.ExitCode() != 0 {
		return nil, parseError(output, req.Arguments.Repos)
	}
	return output, nil
}
