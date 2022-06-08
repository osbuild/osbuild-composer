// Package dnfjson is an interface to the dnf-json Python script that is
// packaged with the osbuild-composer project. The core component of this
// package is the Solver type. The Solver can be configured with
// distribution-specific values (platform ID, architecture, and version
// information) and provides methods for dependency resolution (Depsolve) and
// retrieving a full list of repository package metadata (FetchMetadata).
//
// Alternatively, a BaseSolver can be created which represents an un-configured
// Solver. This type can't be used for depsolving, but can be used to create
// configured Solver instances sharing the same cache directory and
// subscription credentials.
//
// This package relies on the types defined in rpmmd to describe RPM package
// metadata.
package dnfjson

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/osbuild/osbuild-composer/internal/rhsm"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

// BaseSolver defines the basic solver configuration without platform
// information. It can be used to create configured Solver instances with the
// NewWithConfig() method. A BaseSolver maintains the global repository cache
// directory and system subscription information.
type BaseSolver struct {
	subscriptions *rhsm.Subscriptions

	// Cache information
	cache *rpmCache

	// Path to the dnf-json binary and optional args (default: "/usr/libexec/osbuild-composer/dnf-json")
	dnfJsonCmd []string
}

// Create a new unconfigured BaseSolver (without platform information). It can
// be used to create configured Solver instances with the NewWithConfig()
// method. Creating a BaseSolver also loads system subscription information.
func NewBaseSolver(cacheDir string) *BaseSolver {
	subscriptions, _ := rhsm.LoadSystemSubscriptions()
	return &BaseSolver{
		cache:         newRPMCache(cacheDir, 524288000), // 500 MiB
		subscriptions: subscriptions,
		dnfJsonCmd:    []string{"/usr/libexec/osbuild-composer/dnf-json"},
	}
}

// SetMaxCacheSize sets the maximum size for the global repository metadata
// cache. This is the maximum size of the cache after a CleanCache()
// call. Cache cleanup is never performed automatically.
func (s *BaseSolver) SetMaxCacheSize(size uint64) {
	s.cache.maxSize = size
}

// SetDNFJSONPath sets the path to the dnf-json binary and optionally any command line arguments.
func (s *BaseSolver) SetDNFJSONPath(cmd string, args ...string) {
	s.dnfJsonCmd = append([]string{cmd}, args...)
}

// NewWithConfig initialises a Solver with the platform information and the
// BaseSolver's subscription info, cache directory, and dnf-json path.
func (bs *BaseSolver) NewWithConfig(modulePlatformID string, releaseVer string, arch string) *Solver {
	s := new(Solver)
	s.BaseSolver = *bs
	s.modulePlatformID = modulePlatformID
	s.arch = arch
	s.releaseVer = releaseVer
	return s
}

// CleanCache deletes the least recently used repository metadata caches until
// the total size of the cache falls below the configured maximum size (see
// SetMaxCacheSize()).
func (bs *BaseSolver) CleanCache() error {
	return bs.cache.shrink()
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
}

// Create a new Solver with the given configuration. Initialising a Solver also loads system subscription information.
func NewSolver(modulePlatformID string, releaseVer string, arch string, cacheDir string) *Solver {
	s := NewBaseSolver(cacheDir)
	return s.NewWithConfig(modulePlatformID, releaseVer, arch)
}

// Depsolve the list of required package sets with explicit excludes using
// their associated repositories.  Each package set is depsolved as a separate
// transactions in a chain.  It returns a list of all packages (with solved
// dependencies) that will be installed into the system.
func (s *Solver) Depsolve(pkgSets []rpmmd.PackageSet) ([]rpmmd.PackageSpec, error) {
	req, repoMap, err := s.makeDepsolveRequest(pkgSets)
	if err != nil {
		return nil, err
	}

	// get non-exclusive read lock
	s.cache.locker.RLock()
	defer s.cache.locker.RUnlock()

	output, err := run(s.dnfJsonCmd, req)
	if err != nil {
		return nil, err
	}
	// touch repos to now
	now := time.Now().Local()
	for _, r := range repoMap {
		// ignore errors
		_ = s.cache.touchRepo(r.Hash(), now)
	}
	s.cache.updateInfo()

	var result packageSpecs
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}

	return result.toRPMMD(repoMap), nil
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

	result, err := run(s.dnfJsonCmd, req)
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

	var pkgs rpmmd.PackageList
	if err := json.Unmarshal(result, &pkgs); err != nil {
		return nil, err
	}

	sortID := func(pkg rpmmd.Package) string {
		return fmt.Sprintf("%s-%s-%s", pkg.Name, pkg.Version, pkg.Release)
	}
	sort.Slice(pkgs, func(i, j int) bool {
		return sortID(pkgs[i]) < sortID(pkgs[j])
	})
	return pkgs, nil
}

func (s *Solver) reposFromRPMMD(rpmRepos []rpmmd.RepoConfig) ([]repoConfig, error) {
	dnfRepos := make([]repoConfig, len(rpmRepos))
	for idx, rr := range rpmRepos {
		dr := repoConfig{
			ID:             rr.Hash(),
			Name:           rr.Name,
			BaseURL:        rr.BaseURL,
			Metalink:       rr.Metalink,
			MirrorList:     rr.MirrorList,
			GPGKey:         rr.GPGKey,
			IgnoreSSL:      rr.IgnoreSSL,
			MetadataExpire: rr.MetadataExpire,
		}
		if rr.RHSM {
			if s.subscriptions == nil {
				return nil, fmt.Errorf("This system does not have any valid subscriptions. Subscribe it before specifying rhsm: true in sources.")
			}
			secrets, err := s.subscriptions.GetSecretsForBaseurl(rr.BaseURL, s.arch, s.releaseVer)
			if err != nil {
				return nil, fmt.Errorf("RHSM secrets not found on the host for this baseurl: %s", rr.BaseURL)
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
	ID             string `json:"id"`
	Name           string `json:"name,omitempty"`
	BaseURL        string `json:"baseurl,omitempty"`
	Metalink       string `json:"metalink,omitempty"`
	MirrorList     string `json:"mirrorlist,omitempty"`
	GPGKey         string `json:"gpgkey,omitempty"`
	IgnoreSSL      bool   `json:"ignoressl"`
	SSLCACert      string `json:"sslcacert,omitempty"`
	SSLClientKey   string `json:"sslclientkey,omitempty"`
	SSLClientCert  string `json:"sslclientcert,omitempty"`
	MetadataExpire string `json:"metadata_expire,omitempty"`
}

// Helper function for creating a depsolve request payload.
// The request defines a sequence of transactions, each depsolving one of the
// elements of `pkgSets` in the order they appear.  The `repoConfigs` are used
// as the base repositories for all transactions.  The extra repository configs
// in `pkgsetsRepos` are used for each of the `pkgSets` with matching index.
// The length of `pkgsetsRepos` must match the length of `pkgSets` or be empty
// (nil or empty slice).
//
// NOTE: Due to implementation limitations of DNF and dnf-json, each package set
// in the chain must use all of the repositories used by its predecessor.
// An error is returned if this requirement is not met.
func (s *Solver) makeDepsolveRequest(pkgSets []rpmmd.PackageSet) (*Request, map[string]rpmmd.RepoConfig, error) {

	// dedupe repository configurations but maintain order
	// the order in which repositories are added to the request affects the
	// order of the dependencies in the result
	repos := make([]rpmmd.RepoConfig, 0)
	rpmRepoMap := make(map[string]rpmmd.RepoConfig)

	for _, ps := range pkgSets {
		for _, repo := range ps.Repositories {
			id := repo.Hash()
			if _, ok := rpmRepoMap[id]; !ok {
				rpmRepoMap[id] = repo
				repos = append(repos, repo)
			}
		}
	}

	transactions := make([]transactionArgs, len(pkgSets))
	for dsIdx, pkgSet := range pkgSets {
		transactions[dsIdx] = transactionArgs{
			PackageSpecs: pkgSet.Include,
			ExcludeSpecs: pkgSet.Exclude,
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
		Repos:        dnfRepoMap,
		Transactions: transactions,
	}

	req := Request{
		Command:          "depsolve",
		ModulePlatformID: s.modulePlatformID,
		Arch:             s.arch,
		CacheDir:         s.cache.root,
		Arguments:        args,
	}

	return &req, rpmRepoMap, nil
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
		CacheDir:         s.cache.root,
		Arguments: arguments{
			Repos: dnfRepos,
		},
	}
	return &req, nil
}

// convert internal a list of PackageSpecs to the rpmmd equivalent and attach
// key and subscription information based on the repository configs.
func (pkgs packageSpecs) toRPMMD(repos map[string]rpmmd.RepoConfig) []rpmmd.PackageSpec {
	rpmDependencies := make([]rpmmd.PackageSpec, len(pkgs))
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
		rpmDependencies[i].RemoteLocation = dep.RemoteLocation
		rpmDependencies[i].Checksum = dep.Checksum
		rpmDependencies[i].CheckGPG = repo.CheckGPG
		if repo.RHSM {
			rpmDependencies[i].Secrets = "org.osbuild.rhsm"
		}
	}
	return rpmDependencies
}

// Request command and arguments for dnf-json
type Request struct {
	// Command should be either "depsolve" or "dump"
	Command string `json:"command"`

	// Platform ID, e.g., "platform:el8"
	ModulePlatformID string `json:"module_platform_id"`

	// System architecture
	Arch string `json:"arch"`

	// Cache directory for the DNF metadata
	CacheDir string `json:"cachedir"`

	// Arguments for the action defined by Command
	Arguments arguments `json:"arguments"`
}

// arguments for a dnf-json request
type arguments struct {
	// Repositories to use for depsolving
	Repos []repoConfig `json:"repos"`

	// Depsolve package sets and repository mappings for this request
	Transactions []transactionArgs `json:"transactions"`
}

type transactionArgs struct {
	// Packages to depsolve
	PackageSpecs []string `json:"package-specs"`

	// Packages to exclude from results
	ExcludeSpecs []string `json:"exclude-specs"`

	// IDs of repositories to use for this depsolve
	RepoIDs []string `json:"repo-ids"`
}

type packageSpecs []PackageSpec

// Package specification
type PackageSpec struct {
	Name           string `json:"name"`
	Epoch          uint   `json:"epoch"`
	Version        string `json:"version,omitempty"`
	Release        string `json:"release,omitempty"`
	Arch           string `json:"arch,omitempty"`
	RepoID         string `json:"repo_id,omitempty"`
	Path           string `json:"path,omitempty"`
	RemoteLocation string `json:"remote_location,omitempty"`
	Checksum       string `json:"checksum,omitempty"`
	Secrets        string `json:"secrets,omitempty"`
}

// dnf-json error structure
type Error struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

func (err Error) Error() string {
	return fmt.Sprintf("DNF error occurred: %s: %s", err.Kind, err.Reason)
}

// parseError parses the response from dnf-json into the Error type.
func parseError(data []byte) Error {
	var e Error
	if err := json.Unmarshal(data, &e); err != nil {
		// dumping the error into the Reason can get noisy, but it's good for troubleshooting
		return Error{
			Kind:   "InternalError",
			Reason: fmt.Sprintf("Failed to unmarshal dnf-json error output %q: %s", string(data), err.Error()),
		}
	}
	return e
}

func run(dnfJsonCmd []string, req *Request) ([]byte, error) {
	if len(dnfJsonCmd) == 0 {
		return nil, fmt.Errorf("dnf-json command undefined")
	}
	ex := dnfJsonCmd[0]
	args := make([]string, len(dnfJsonCmd)-1)
	if len(dnfJsonCmd) > 1 {
		args = dnfJsonCmd[1:]
	}
	cmd := exec.Command(ex, args...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	cmd.Stderr = os.Stderr
	stdout := new(bytes.Buffer)
	cmd.Stdout = stdout

	err = cmd.Start()
	if err != nil {
		return nil, err
	}

	err = json.NewEncoder(stdin).Encode(req)
	if err != nil {
		return nil, err
	}
	stdin.Close()

	err = cmd.Wait()
	output := stdout.Bytes()
	if runError, ok := err.(*exec.ExitError); ok && runError.ExitCode() != 0 {
		return nil, parseError(output)
	}

	return output, nil
}
