package rpmmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gobwas/glob"
	"github.com/osbuild/osbuild-composer/internal/rhsm"
)

type repository struct {
	Name           string   `json:"name"`
	BaseURL        string   `json:"baseurl,omitempty"`
	Metalink       string   `json:"metalink,omitempty"`
	MirrorList     string   `json:"mirrorlist,omitempty"`
	GPGKey         string   `json:"gpgkey,omitempty"`
	CheckGPG       bool     `json:"check_gpg,omitempty"`
	RHSM           bool     `json:"rhsm,omitempty"`
	MetadataExpire string   `json:"metadata_expire,omitempty"`
	ImageTypeTags  []string `json:"image_type_tags,omitempty"`
}

type dnfRepoConfig struct {
	ID             string `json:"id"`
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

// The inputs to depsolve, a set of packages to include and a set of
// packages to exclude.
type PackageSet struct {
	Include []string
	Exclude []string
}

// Append the Include and Exclude package list from another PackageSet and
// return the result.
func (ps PackageSet) Append(other PackageSet) PackageSet {
	ps.Include = append(ps.Include, other.Include...)
	ps.Exclude = append(ps.Exclude, other.Exclude...)
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
}

type dnfPackageSpec struct {
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
	Depsolve(packageSet PackageSet, repos []RepoConfig, modulePlatformID, arch, releasever string) ([]PackageSpec, map[string]string, error)
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

func runDNF(command string, arguments interface{}, result interface{}) error {
	var call = struct {
		Command   string      `json:"command"`
		Arguments interface{} `json:"arguments,omitempty"`
	}{
		command,
		arguments,
	}

	httpc := http.Client{
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", "/run/osbuild-dnf-json/api.sock")
			},
		},
	}

	bpost, err := json.Marshal(call)
	if err != nil {
		return err
	}

	response, err := httpc.Post("http://unix", "application/json", bytes.NewReader(bpost))
	if err != nil {
		return err
	}
	defer response.Body.Close()

	output, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return err
	}

	if response.StatusCode != http.StatusOK {
		var dnfError DNFError
		err = json.Unmarshal(output, &dnfError)
		if err != nil {
			return err
		}

		return &dnfError
	}

	err = json.Unmarshal(output, result)

	if err != nil {
		return err
	}

	return nil
}

type rpmmdImpl struct {
	CacheDir      string
	subscriptions *rhsm.Subscriptions
}

func NewRPMMD(cacheDir string) RPMMD {
	subscriptions, err := rhsm.LoadSystemSubscriptions()
	if err != nil {
		log.Println("Failed to load subscriptions. osbuild-composer will fail to build images if the "+
			"configured repositories require them:", err)
	} else if err == nil && subscriptions == nil {
		log.Println("This host is not subscribed to any RPM repositories. This is fine as long as " +
			"the configured sources don't enable \"rhsm\".")
	}
	return &rpmmdImpl{
		CacheDir:      cacheDir,
		subscriptions: subscriptions,
	}
}

func (repo RepoConfig) toDNFRepoConfig(rpmmd *rpmmdImpl, i int, arch, releasever string) (dnfRepoConfig, error) {
	id := strconv.Itoa(i)
	dnfRepo := dnfRepoConfig{
		ID:             id,
		BaseURL:        repo.BaseURL,
		Metalink:       repo.Metalink,
		MirrorList:     repo.MirrorList,
		GPGKey:         repo.GPGKey,
		IgnoreSSL:      repo.IgnoreSSL,
		MetadataExpire: repo.MetadataExpire,
	}
	if repo.RHSM {
		if rpmmd.subscriptions == nil {
			return dnfRepoConfig{}, fmt.Errorf("This system does not have any valid subscriptions. Subscribe it before specifying rhsm: true in sources.")
		}
		secrets, err := rpmmd.subscriptions.GetSecretsForBaseurl(repo.BaseURL, arch, releasever)
		if err != nil {
			return dnfRepoConfig{}, fmt.Errorf("RHSM secrets not found on the host for this baseurl: %s", repo.BaseURL)
		}
		dnfRepo.SSLCACert = secrets.SSLCACert
		dnfRepo.SSLClientKey = secrets.SSLClientKey
		dnfRepo.SSLClientCert = secrets.SSLClientCert
	}
	return dnfRepo, nil
}

func (r *rpmmdImpl) FetchMetadata(repos []RepoConfig, modulePlatformID, arch, releasever string) (PackageList, map[string]string, error) {
	var dnfRepoConfigs []dnfRepoConfig
	for i, repo := range repos {
		dnfRepo, err := repo.toDNFRepoConfig(r, i, arch, releasever)
		if err != nil {
			return nil, nil, err
		}
		dnfRepoConfigs = append(dnfRepoConfigs, dnfRepo)
	}

	var arguments = struct {
		Repos            []dnfRepoConfig `json:"repos"`
		CacheDir         string          `json:"cachedir"`
		ModulePlatformID string          `json:"module_platform_id"`
		Arch             string          `json:"arch"`
	}{dnfRepoConfigs, r.CacheDir, modulePlatformID, arch}
	var reply struct {
		Checksums map[string]string `json:"checksums"`
		Packages  PackageList       `json:"packages"`
	}

	err := runDNF("dump", arguments, &reply)

	sort.Slice(reply.Packages, func(i, j int) bool {
		return reply.Packages[i].Name < reply.Packages[j].Name
	})
	checksums := make(map[string]string)
	for i, repo := range repos {
		checksums[repo.Name] = reply.Checksums[strconv.Itoa(i)]
	}
	return reply.Packages, checksums, err
}

func (r *rpmmdImpl) Depsolve(packageSet PackageSet, repos []RepoConfig, modulePlatformID, arch, releasever string) ([]PackageSpec, map[string]string, error) {
	var dnfRepoConfigs []dnfRepoConfig

	for i, repo := range repos {
		dnfRepo, err := repo.toDNFRepoConfig(r, i, arch, releasever)
		if err != nil {
			return nil, nil, err
		}
		dnfRepoConfigs = append(dnfRepoConfigs, dnfRepo)
	}

	var arguments = struct {
		PackageSpecs     []string        `json:"package-specs"`
		ExcludSpecs      []string        `json:"exclude-specs"`
		Repos            []dnfRepoConfig `json:"repos"`
		CacheDir         string          `json:"cachedir"`
		ModulePlatformID string          `json:"module_platform_id"`
		Arch             string          `json:"arch"`
	}{packageSet.Include, packageSet.Exclude, dnfRepoConfigs, r.CacheDir, modulePlatformID, arch}
	var reply struct {
		Checksums    map[string]string `json:"checksums"`
		Dependencies []dnfPackageSpec  `json:"dependencies"`
	}
	err := runDNF("depsolve", arguments, &reply)

	dependencies := make([]PackageSpec, len(reply.Dependencies))
	for i, pack := range reply.Dependencies {
		id, err := strconv.Atoi(pack.RepoID)
		if err != nil {
			panic(err)
		}
		repo := repos[id]
		dep := reply.Dependencies[i]
		dependencies[i].Name = dep.Name
		dependencies[i].Epoch = dep.Epoch
		dependencies[i].Version = dep.Version
		dependencies[i].Release = dep.Release
		dependencies[i].Arch = dep.Arch
		dependencies[i].RemoteLocation = dep.RemoteLocation
		dependencies[i].Checksum = dep.Checksum
		dependencies[i].CheckGPG = repo.CheckGPG
		if repo.RHSM {
			dependencies[i].Secrets = "org.osbuild.rhsm"
		}
	}

	return dependencies, reply.Checksums, err
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

func (pkg *PackageInfo) FillDependencies(rpmmd RPMMD, repos []RepoConfig, modulePlatformID, arch, releasever string) (err error) {
	pkg.Dependencies, _, err = rpmmd.Depsolve(PackageSet{Include: []string{pkg.Name}}, repos, modulePlatformID, arch, releasever)
	return
}
