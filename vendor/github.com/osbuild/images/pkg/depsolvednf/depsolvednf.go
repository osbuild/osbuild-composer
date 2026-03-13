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
	"cmp"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

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

	sbomType sbom.StandardType

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
	// Transactions is a list of package lists, one for each depsolve
	// transaction. Each transaction contains only the packages to be
	// installed that are unique to that transaction. The transaction results
	// are disjoint sets that should be installed in the order they appear in
	// the list.
	Transactions TransactionList
	Modules      []rpmmd.ModuleSpec
	Repos        []rpmmd.RepoConfig
	SBOM         *sbom.Document
	Solver       string
}

// DumpResult contains the results of a dump operation.
type DumpResult struct {
	Packages rpmmd.PackageList
	Repos    []rpmmd.RepoConfig
	Solver   string
}

// SearchResult contains the results of a search operation.
type SearchResult struct {
	Packages rpmmd.PackageList
	Repos    []rpmmd.RepoConfig
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

// solverCfg creates a solverConfig from the Solver's current state.
func (s *Solver) solverCfg() *solverConfig {
	return &solverConfig{
		modulePlatformID: s.modulePlatformID,
		arch:             s.arch,
		releaseVer:       s.releaseVer,
		cacheDir:         s.GetCacheDir(),
		rootDir:          s.rootDir,
		proxy:            s.proxy,
		subscriptions:    s.subscriptions,
	}
}

// hashRequest computes a SHA256 hash of the request raw data for caching.
func hashRequest(data []byte) string {
	h := sha256.Sum256(data)
	return fmt.Sprintf("%x", h[:])
}

// collectRepos extracts unique repos from package sets maintaining order
func collectRepos(pkgSets []rpmmd.PackageSet) []rpmmd.RepoConfig {
	seen := make(map[string]bool)
	repos := make([]rpmmd.RepoConfig, 0)
	for _, ps := range pkgSets {
		for _, repo := range ps.Repositories {
			id := repo.Hash()
			if !seen[id] {
				seen[id] = true
				repos = append(repos, repo)
			}
		}
	}
	return repos
}

// Set the SBOM type to generate with the depsolve.
func (s *Solver) SetSBOMType(sbomType sbom.StandardType) {
	s.sbomType = sbomType
}

// Depsolve the list of required package sets with explicit excludes using
// their associated repositories.  Each package set is depsolved as a separate
// transactions in a chain.  It returns a list of all packages (with solved
// dependencies) that will be installed into the system.
func (s *Solver) Depsolve(pkgSets []rpmmd.PackageSet, sbomType sbom.StandardType) (*DepsolveResult, error) {
	if err := validatePackageSetRepoChain(pkgSets); err != nil {
		return nil, err
	}

	// XXX: we should let the depsolver handle subscriptions: https://github.com/osbuild/images/issues/2055
	if err := validateSubscriptionsForRepos(pkgSets, s.subscriptions != nil, s.subscriptionsErr); err != nil {
		return nil, err
	}

	cfg := s.solverCfg()
	reqData, err := activeHandler.makeDepsolveRequest(cfg, pkgSets, sbomType)
	if err != nil {
		return nil, fmt.Errorf("makeDepsolveRequest failed: %w", err)
	}

	// collect all repos for error reporting
	allRepos := collectRepos(pkgSets)

	// get non-exclusive read lock
	s.cache.locker.RLock()
	defer s.cache.locker.RUnlock()

	output, err := run(s.depsolveDNFCmd, reqData, s.Stderr)
	if err != nil {
		return nil, parseError(output, allRepos, err)
	}

	// touch repos to now
	now := time.Now().Local()
	for _, r := range allRepos {
		// ignore errors
		_ = s.cache.touchRepo(r.Hash(), now)
	}
	s.cache.updateInfo()

	resultRaw, err := activeHandler.parseDepsolveResult(output)
	if err != nil {
		return nil, err
	}

	// Apply RHSM secrets to packages in each transaction as well.
	for _, transaction := range resultRaw.Transactions {
		applyRHSMSecrets(transaction, allRepos)
	}

	var sbomDoc *sbom.Document
	if sbomType != sbom.StandardTypeNone {
		sbomDoc, err = sbom.NewDocument(sbomType, resultRaw.SBOMRaw)
		if err != nil {
			return nil, fmt.Errorf("creating SBOM document failed: %w", err)
		}
	}

	return &DepsolveResult{
		Transactions: resultRaw.Transactions,
		Modules:      resultRaw.Modules,
		Repos:        resultRaw.Repos,
		SBOM:         sbomDoc,
		Solver:       resultRaw.Solver,
	}, nil
}

// DepsolveAll calls [Solver.Depsolve] with each package set slice in the map and
// returns a map of results with the corresponding keys as the input argument.
func (s *Solver) DepsolveAll(pkgSetsMap map[string][]rpmmd.PackageSet) (map[string]DepsolveResult, error) {
	results := make(map[string]DepsolveResult, len(pkgSetsMap))
	for name, pkgSet := range pkgSetsMap {
		res, err := s.Depsolve(pkgSet, s.sbomType)
		if err != nil {
			return nil, fmt.Errorf("error depsolving package sets for %q: %w", name, err)
		}
		results[name] = *res
	}
	return results, nil
}

// FetchMetadata returns the list of all the available packages in repos and
// their info.
func (s *Solver) FetchMetadata(repos []rpmmd.RepoConfig) (rpmmd.PackageList, error) {
	cfg := s.solverCfg()
	reqData, err := activeHandler.makeDumpRequest(cfg, repos)
	if err != nil {
		return nil, err
	}

	// get non-exclusive read lock
	s.cache.locker.RLock()
	defer s.cache.locker.RUnlock()

	// Is this cached?
	reqHash := hashRequest(reqData)
	if pkgs, ok := s.resultCache.Get(reqHash); ok {
		return pkgs, nil
	}

	rawRes, err := run(s.depsolveDNFCmd, reqData, s.Stderr)
	if err != nil {
		return nil, parseError(rawRes, repos, err)
	}

	// touch repos to now
	now := time.Now().Local()
	for _, r := range repos {
		// ignore errors
		_ = s.cache.touchRepo(r.Hash(), now)
	}
	s.cache.updateInfo()

	res, err := activeHandler.parseDumpResult(rawRes)
	if err != nil {
		return nil, err
	}

	// XXX: Cache and expose the whole operation result instead of just the packages in the future.

	pkgs := res.Packages
	slices.SortFunc(pkgs, func(a, b rpmmd.Package) int {
		return cmp.Compare(a.NVR(), b.NVR())
	})

	// Cache the results
	s.resultCache.Store(reqHash, pkgs)
	return pkgs, nil
}

// SearchMetadata searches for packages and returns a list of the info for matches.
func (s *Solver) SearchMetadata(repos []rpmmd.RepoConfig, packages []string) (rpmmd.PackageList, error) {
	cfg := s.solverCfg()
	reqData, err := activeHandler.makeSearchRequest(cfg, repos, packages)
	if err != nil {
		return nil, err
	}

	// get non-exclusive read lock
	s.cache.locker.RLock()
	defer s.cache.locker.RUnlock()

	// Is this cached?
	reqHash := hashRequest(reqData)
	if pkgs, ok := s.resultCache.Get(reqHash); ok {
		return pkgs, nil
	}

	rawRes, err := run(s.depsolveDNFCmd, reqData, s.Stderr)
	if err != nil {
		return nil, parseError(rawRes, repos, err)
	}

	// touch repos to now
	now := time.Now().Local()
	for _, r := range repos {
		// ignore errors
		_ = s.cache.touchRepo(r.Hash(), now)
	}
	s.cache.updateInfo()

	res, err := activeHandler.parseSearchResult(rawRes)
	if err != nil {
		return nil, err
	}

	// XXX: Cache and expose the whole operation result instead of just the packages in the future.

	pkgs := res.Packages
	slices.SortFunc(pkgs, func(a, b rpmmd.Package) int {
		return cmp.Compare(a.NVR(), b.NVR())
	})

	// Cache the results
	s.resultCache.Store(reqHash, pkgs)
	return pkgs, nil
}

// applyRHSMSecrets overrides the Secrets field on packages from RHSM or RHUI repos.
// The activeHandler sets "org.osbuild.mtls" for repos with SSLClientKey,
// but RHSM repos need "org.osbuild.rhsm" and RHUI repos need "org.osbuild.rhui" instead.
func applyRHSMSecrets(pkgs rpmmd.PackageList, repos []rpmmd.RepoConfig) {
	type repoFlags struct {
		rhsm bool
		rhui bool
	}
	repoMap := make(map[string]repoFlags)
	for _, repo := range repos {
		repoMap[repo.Hash()] = repoFlags{rhsm: repo.RHSM, rhui: repo.RHUI}
	}
	for i := range pkgs {
		flags := repoMap[pkgs[i].RepoID]
		if flags.rhui {
			pkgs[i].Secrets = "org.osbuild.rhui"
		} else if flags.rhsm {
			pkgs[i].Secrets = "org.osbuild.rhsm"
		}
	}
}

// validatePackageSetRepoChain validates that the repository chain is valid.
// It checks that:
//   - No package set has an empty Include list.
//   - Each package set uses all of the repositories used by its predecessor.
//     NOTE: Due to implementation limitations of DNF and osbuild-depsolve-dnf,
//     each package set in the chain must use all of the repositories used by
//     its predecessor.
func validatePackageSetRepoChain(pkgSets []rpmmd.PackageSet) error {
	// Check for empty Include lists
	for idx, ps := range pkgSets {
		if len(ps.Include) == 0 {
			return fmt.Errorf("packageSet %d has empty Include list", idx)
		}
	}

	if len(pkgSets) <= 1 {
		return nil
	}

	for dsIdx := 1; dsIdx < len(pkgSets); dsIdx++ {
		prevRepoIDs := make([]string, len(pkgSets[dsIdx-1].Repositories))
		for i, r := range pkgSets[dsIdx-1].Repositories {
			prevRepoIDs[i] = r.Hash()
		}

		currRepoIDs := make([]string, len(pkgSets[dsIdx].Repositories))
		for i, r := range pkgSets[dsIdx].Repositories {
			currRepoIDs[i] = r.Hash()
		}

		if len(currRepoIDs) < len(prevRepoIDs) {
			return fmt.Errorf("chained packageSet %d does not use all of the repos used by its predecessor", dsIdx)
		}

		for idx, repoID := range prevRepoIDs {
			if repoID != currRepoIDs[idx] {
				return fmt.Errorf("chained packageSet %d does not use all of the repos used by its predecessor", dsIdx)
			}
		}
	}
	return nil
}

// validateSubscriptionsForRepos checks that RHSM subscriptions are available
// for any repositories that require them. Repositories with RHUI set to true
// are skipped since they use cloud instance identity for authentication
// instead of RHSM entitlement certificates.
func validateSubscriptionsForRepos(pkgSets []rpmmd.PackageSet, haveSubscriptions bool, subsErr error) error {
	for _, ps := range pkgSets {
		for _, repo := range ps.Repositories {
			if repo.RHSM && !repo.RHUI && !haveSubscriptions {
				return fmt.Errorf("This system does not have any valid subscriptions. Subscribe it before specifying rhsm: true in sources (error details: %w)", subsErr)
			}
		}
	}
	return nil
}

// optionalMetadataForDistro returns optional repository metadata types
// that should be downloaded for the given distro.
func optionalMetadataForDistro(modulePlatformID string) []string {
	// filelist repo metadata is required when using newer versions of libdnf
	// with old repositories or packages that specify dependencies on files.
	// EL10+ and Fedora 40+ packaging guidelines prohibit depending on
	// filepaths so filelist downloads are disabled by default and are not
	// required when depsolving for those distros. Explicitly enable the option
	// for older distro versions in case we are using a newer libdnf.
	switch modulePlatformID {
	case "platform:f39", "platform:el7", "platform:el8", "platform:el9":
		return []string{"filelists"}
	}
	return nil
}

// osbuild-depsolve-dnf error structure
type Error struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
	Err    error  `json:"-"`
}

func (err Error) Error() string {
	if err.Err != nil {
		return fmt.Sprintf("DNF error occurred: %s: %s: %s", err.Kind, err.Reason, err.Err)
	} else {
		return fmt.Sprintf("DNF error occurred: %s: %s", err.Kind, err.Reason)
	}
}

func (err Error) Unwrap() error {
	return err.Err
}

// parseError parses the response from osbuild-depsolve-dnf into the Error type and appends
// the name and URL of a repository to all detected repository IDs in the
// message.
func parseError(data []byte, repos []rpmmd.RepoConfig, cmdError error) Error {
	var e Error
	if len(data) == 0 {
		return Error{
			Kind:   "InternalError",
			Reason: "osbuild-depsolve-dnf output was empty",
			Err:    cmdError,
		}
	}

	if err := json.Unmarshal(data, &e); err != nil {
		// dumping the error into the Reason can get noisy, but it's good for troubleshooting
		return Error{
			Kind:   "InternalError",
			Reason: fmt.Sprintf("Failed to unmarshal osbuild-depsolve-dnf error output %q: %s", string(data), err.Error()),
			Err:    cmdError,
		}
	}

	// append to any instance of a repository ID the URL (or metalink, mirrorlist, etc)
	for _, repo := range repos {
		idstr := fmt.Sprintf("'%s'", repo.Hash())
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

func run(dnfJsonCmd []string, reqData []byte, stderr io.Writer) ([]byte, error) {
	if len(dnfJsonCmd) == 0 {
		dnfJsonCmd = []string{findDepsolveDnf()}
	}
	if len(dnfJsonCmd) == 0 || len(dnfJsonCmd[0]) == 0 {
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

	_, err = stdin.Write(reqData)
	if err != nil {
		return nil, fmt.Errorf("writing request data to stdin for %s failed: %w", ex, err)
	}
	stdin.Close()

	err = cmd.Wait()
	output := stdout.Bytes()
	if runError, ok := err.(*exec.ExitError); ok && runError.ExitCode() != 0 {
		return output, fmt.Errorf("depsolve failed with exit code %d", runError.ExitCode())
	}
	return output, nil
}
