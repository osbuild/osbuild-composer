package rpmmd

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gobwas/glob"
)

type repository struct {
	Name           string   `json:"name"`
	BaseURL        string   `json:"baseurl,omitempty"`
	Metalink       string   `json:"metalink,omitempty"`
	MirrorList     string   `json:"mirrorlist,omitempty"`
	GPGKey         string   `json:"gpgkey,omitempty"`
	CheckGPG       bool     `json:"check_gpg,omitempty"`
	IgnoreSSL      bool     `json:"ignore_ssl,omitempty"`
	RHSM           bool     `json:"rhsm,omitempty"`
	MetadataExpire string   `json:"metadata_expire,omitempty"`
	ImageTypeTags  []string `json:"image_type_tags,omitempty"`
}

type RepoConfig struct {
	Name           string
	BaseURL        string
	Metalink       string
	MirrorList     string
	GPGKey         string
	CheckGPG       bool
	IgnoreSSL      bool
	MetadataExpire string
	RHSM           bool
	ImageTypeTags  []string
	PackageSets    []string
}

// Hash calculates an ID string that uniquely represents a repository
// configuration.  The Name and ImageTypeTags fields are not considered in the
// calculation.
func (r *RepoConfig) Hash() string {
	bts := func(b bool) string {
		return fmt.Sprintf("%T", b)
	}
	return fmt.Sprintf("%x", sha256.Sum256([]byte(r.BaseURL+r.Metalink+r.MirrorList+r.GPGKey+bts(r.CheckGPG)+bts(r.IgnoreSSL)+r.MetadataExpire+bts(r.RHSM))))
}

type DistrosRepoConfigs map[string]map[string][]RepoConfig

type PackageList []Package

type Package struct {
	Name        string
	Summary     string
	Description string
	URL         string
	Epoch       uint
	Version     string
	Release     string
	Arch        string
	BuildTime   time.Time
	License     string
}

func (pkg Package) ToPackageBuild() PackageBuild {
	// Convert the time to the API time format
	return PackageBuild{
		Arch:           pkg.Arch,
		BuildTime:      pkg.BuildTime.Format("2006-01-02T15:04:05"),
		Epoch:          pkg.Epoch,
		Release:        pkg.Release,
		Changelog:      "CHANGELOG_NEEDED", // the same value as lorax-composer puts here
		BuildConfigRef: "BUILD_CONFIG_REF", // the same value as lorax-composer puts here
		BuildEnvRef:    "BUILD_ENV_REF",    // the same value as lorax-composer puts here
		Source: PackageSource{
			License:   pkg.License,
			Version:   pkg.Version,
			SourceRef: "SOURCE_REF", // the same value as lorax-composer puts here
		},
	}
}

func (pkg Package) ToPackageInfo() PackageInfo {
	return PackageInfo{
		Name:         pkg.Name,
		Summary:      pkg.Summary,
		Description:  pkg.Description,
		Homepage:     pkg.URL,
		UpstreamVCS:  "UPSTREAM_VCS", // the same value as lorax-composer puts here
		Builds:       []PackageBuild{pkg.ToPackageBuild()},
		Dependencies: nil,
	}
}

// The inputs to depsolve, a set of packages to include and a set of packages
// to exclude. The Repositories are used when depsolving this package set in
// addition to the base repositories.
type PackageSet struct {
	Include      []string
	Exclude      []string
	Repositories []RepoConfig
}

// Append the Include and Exclude package list from another PackageSet and
// return the result.
func (ps PackageSet) Append(other PackageSet) PackageSet {
	ps.Include = append(ps.Include, other.Include...)
	ps.Exclude = append(ps.Exclude, other.Exclude...)
	return ps
}

// ResolveConflictsExclude resolves conflicting Include and Exclude package lists
// content by deleting packages listed as Excluded from the Include list.
func (ps PackageSet) ResolveConflictsExclude() PackageSet {
	excluded := map[string]struct{}{}
	for _, pkg := range ps.Exclude {
		excluded[pkg] = struct{}{}
	}

	newInclude := []string{}
	for _, pkg := range ps.Include {
		_, found := excluded[pkg]
		if !found {
			newInclude = append(newInclude, pkg)
		}
	}
	ps.Include = newInclude
	return ps
}

// TODO: the public API of this package should not be reused for serialization.
type PackageSpec struct {
	Name           string `json:"name"`
	Epoch          uint   `json:"epoch"`
	Version        string `json:"version,omitempty"`
	Release        string `json:"release,omitempty"`
	Arch           string `json:"arch,omitempty"`
	RemoteLocation string `json:"remote_location,omitempty"`
	Checksum       string `json:"checksum,omitempty"`
	Secrets        string `json:"secrets,omitempty"`
	CheckGPG       bool   `json:"check_gpg,omitempty"`
	IgnoreSSL      bool   `json:"ignore_ssl,omitempty"`
}

type PackageSource struct {
	License   string   `json:"license"`
	Version   string   `json:"version"`
	SourceRef string   `json:"source_ref"`
	Metadata  struct{} `json:"metadata"` // it's just an empty struct in lorax-composer
}

type PackageBuild struct {
	Arch           string        `json:"arch"`
	BuildTime      string        `json:"build_time"`
	Epoch          uint          `json:"epoch"`
	Release        string        `json:"release"`
	Source         PackageSource `json:"source"`
	Changelog      string        `json:"changelog"`
	BuildConfigRef string        `json:"build_config_ref"`
	BuildEnvRef    string        `json:"build_env_ref"`
	Metadata       struct{}      `json:"metadata"` // it's just an empty struct in lorax-composer
}

type PackageInfo struct {
	Name         string         `json:"name"`
	Summary      string         `json:"summary"`
	Description  string         `json:"description"`
	Homepage     string         `json:"homepage"`
	UpstreamVCS  string         `json:"upstream_vcs"`
	Builds       []PackageBuild `json:"builds"`
	Dependencies []PackageSpec  `json:"dependencies,omitempty"`
}

type RPMMD interface {
	// FetchMetadata returns all metadata about the repositories we use in the code. Specifically it is a
	// list of packages and dictionary of checksums of the repositories.
	FetchMetadata(repos []RepoConfig, modulePlatformID, arch, releasever string) (PackageList, map[string]string, error)

	// Depsolve takes a list of required content (specs), explicitly unwanted content (excludeSpecs), list
	// or repositories, and platform ID for modularity. It returns a list of all packages (with solved
	// dependencies) that will be installed into the system.
	//
	// NOTE: Kept for legacy purposes (Weldr). New code should default to using DepsolvePackageSets().
	Depsolve(packageSet PackageSet, repos []RepoConfig, modulePlatformID, arch, releasever string) ([]PackageSpec, map[string]string, error)

	// DepsolvePackageSets takes a map of package sets chains, which should be depsolved as separate transactions,
	// a map of package sets (included and excluded), a list of common and package-set-specific repositories,
	// platform ID for modularity, architecture and release version. It returns a map of Package Specs, depsolved
	// in a chain or alone, as defined based on the provided arguments.
	DepsolvePackageSets(packageSetsChains map[string][]string, packageSets map[string]PackageSet, repos []RepoConfig, packageSetsRepos map[string][]RepoConfig, modulePlatformID, arch, releasever string) (map[string][]PackageSpec, error)
}

type DNFError struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

func (err *DNFError) Error() string {
	return fmt.Sprintf("DNF error occured: %s: %s", err.Kind, err.Reason)
}

type RepositoryError struct {
	msg string
}

func (re *RepositoryError) Error() string {
	return re.msg
}

func GetVerStrFromPackageSpecList(pkgs []PackageSpec, packageName string) (string, error) {
	for _, pkg := range pkgs {
		if pkg.Name == packageName {
			return fmt.Sprintf("%s-%s.%s", pkg.Version, pkg.Release, pkg.Arch), nil
		}
	}
	return "", fmt.Errorf("package %q not found in the PackageSpec list", packageName)
}

func GetVerStrFromPackageSpecListPanic(pkgs []PackageSpec, packageName string) string {
	pkgVerStr, err := GetVerStrFromPackageSpecList(pkgs, packageName)
	if err != nil {
		panic(err)
	}
	return pkgVerStr
}

func loadRepositoriesFromFile(filename string) (map[string][]RepoConfig, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var reposMap map[string][]repository
	repoConfigs := make(map[string][]RepoConfig)

	err = json.NewDecoder(f).Decode(&reposMap)
	if err != nil {
		return nil, err
	}

	for arch, repos := range reposMap {
		for _, repo := range repos {
			config := RepoConfig{
				Name:           repo.Name,
				BaseURL:        repo.BaseURL,
				Metalink:       repo.Metalink,
				MirrorList:     repo.MirrorList,
				GPGKey:         repo.GPGKey,
				CheckGPG:       repo.CheckGPG,
				RHSM:           repo.RHSM,
				MetadataExpire: repo.MetadataExpire,
				ImageTypeTags:  repo.ImageTypeTags,
			}

			repoConfigs[arch] = append(repoConfigs[arch], config)
		}
	}

	return repoConfigs, nil
}

// LoadAllRepositories loads all repositories for given distros from the given list of paths.
// Behavior is the same as with the LoadRepositories() method.
func LoadAllRepositories(confPaths []string) (DistrosRepoConfigs, error) {
	distrosRepoConfigs := DistrosRepoConfigs{}

	for _, confPath := range confPaths {
		reposPath := filepath.Join(confPath, "repositories")

		fileEntries, err := ioutil.ReadDir(reposPath)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		for _, fileEntry := range fileEntries {
			// Skip all directories
			if fileEntry.IsDir() {
				continue
			}

			// distro repositories definition is expected to be named "<distro_name>.json"
			if strings.HasSuffix(fileEntry.Name(), ".json") {
				distro := strings.TrimSuffix(fileEntry.Name(), ".json")

				// skip the distro repos definition, if it has been already read
				_, ok := distrosRepoConfigs[distro]
				if ok {
					continue
				}

				configFile := filepath.Join(reposPath, fileEntry.Name())
				distroRepos, err := loadRepositoriesFromFile(configFile)
				if err != nil {
					return nil, err
				}

				log.Println("Loaded repository configuration file:", configFile)

				distrosRepoConfigs[distro] = distroRepos
			}
		}
	}

	return distrosRepoConfigs, nil
}

// LoadRepositories loads distribution repositories from the given list of paths.
// If there are duplicate distro repositories definitions found in multiple paths, the first
// encounter is preferred. For this reason, the order of paths in the passed list should
// reflect the desired preference.
func LoadRepositories(confPaths []string, distro string) (map[string][]RepoConfig, error) {
	var repoConfigs map[string][]RepoConfig
	path := "/repositories/" + distro + ".json"

	for _, confPath := range confPaths {
		var err error
		repoConfigs, err = loadRepositoriesFromFile(confPath + path)
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		// Found the distro repository configs in the current path
		if repoConfigs != nil {
			break
		}
	}

	if repoConfigs == nil {
		return nil, &RepositoryError{"LoadRepositories failed: none of the provided paths contain distro configuration"}
	}

	return repoConfigs, nil
}

func (packages PackageList) Search(globPatterns ...string) (PackageList, error) {
	var globs []glob.Glob

	for _, globPattern := range globPatterns {
		g, err := glob.Compile(globPattern)
		if err != nil {
			return nil, err
		}

		globs = append(globs, g)
	}

	var foundPackages PackageList

	for _, pkg := range packages {
		for _, g := range globs {
			if g.Match(pkg.Name) {
				foundPackages = append(foundPackages, pkg)
				break
			}
		}
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})

	return foundPackages, nil
}

func (packages PackageList) ToPackageInfos() []PackageInfo {
	resultsNames := make(map[string]int)
	var results []PackageInfo

	for _, pkg := range packages {
		if index, ok := resultsNames[pkg.Name]; ok {
			foundPkg := &results[index]

			foundPkg.Builds = append(foundPkg.Builds, pkg.ToPackageBuild())
		} else {
			newIndex := len(results)
			resultsNames[pkg.Name] = newIndex

			packageInfo := pkg.ToPackageInfo()

			results = append(results, packageInfo)
		}
	}

	return results
}
