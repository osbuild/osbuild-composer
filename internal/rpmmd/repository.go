package rpmmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/gobwas/glob"
)

type RepoConfig struct {
	Id             string `json:"id"`
	BaseURL        string `json:"baseurl,omitempty"`
	Metalink       string `json:"metalink,omitempty"`
	MirrorList     string `json:"mirrorlist,omitempty"`
	GPGKey         string `json:"gpgkey,omitempty"`
	IgnoreSSL      bool   `json:"ignoressl"`
	MetadataExpire string `json:"metadata_expire,omitempty"`
}

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
		Arch:      pkg.Arch,
		BuildTime: pkg.BuildTime.Format("2006-01-02T15:04:05"),
		Epoch:     pkg.Epoch,
		Release:   pkg.Release,
		Source: PackageSource{
			License: pkg.License,
			Version: pkg.Version,
		},
	}
}

func (pkg Package) ToPackageInfo() PackageInfo {
	return PackageInfo{
		Name:         pkg.Name,
		Summary:      pkg.Summary,
		Description:  pkg.Description,
		Homepage:     pkg.URL,
		Builds:       []PackageBuild{pkg.ToPackageBuild()},
		Dependencies: nil,
	}
}

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
}

type PackageSource struct {
	License string `json:"license"`
	Version string `json:"version"`
}

type PackageBuild struct {
	Arch      string        `json:"arch"`
	BuildTime string        `json:"build_time"`
	Epoch     uint          `json:"epoch"`
	Release   string        `json:"release"`
	Source    PackageSource `json:"source"`
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
	FetchMetadata(repos []RepoConfig, modulePlatformID string, arch string) (PackageList, map[string]string, error)

	// Depsolve takes a list of required content (specs), explicitly unwanted content (excludeSpecs), list
	// or repositories, and platform ID for modularity. It returns a list of all packages (with solved
	// dependencies) that will be installed into the system.
	Depsolve(specs, excludeSpecs []string, repos []RepoConfig, modulePlatformID, arch string) ([]PackageSpec, map[string]string, error)
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

func LoadRepositories(confPaths []string, distro string) (map[string][]RepoConfig, error) {
	var f *os.File
	var err error
	path := "/repositories/" + distro + ".json"

	for _, confPath := range confPaths {
		f, err = os.Open(confPath + path)
		if err == nil {
			break
		} else if !os.IsNotExist(err) {
			return nil, err
		}
	}
	if err != nil {
		return nil, &RepositoryError{"LoadRepositories failed: none of the provided paths contain distro configuration"}
	}
	defer f.Close()

	var repos map[string][]RepoConfig

	err = json.NewDecoder(f).Decode(&repos)
	if err != nil {
		return nil, err
	}

	return repos, nil
}

func runDNF(command string, arguments interface{}, result interface{}) error {
	var call = struct {
		Command   string      `json:"command"`
		Arguments interface{} `json:"arguments,omitempty"`
	}{
		command,
		arguments,
	}

	cmd := exec.Command("python3", "dnf-json")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	err = json.NewEncoder(stdin).Encode(call)
	if err != nil {
		return err
	}
	stdin.Close()

	output, err := ioutil.ReadAll(stdout)
	if err != nil {
		return err
	}

	err = cmd.Wait()

	const DnfErrorExitCode = 10
	if runError, ok := err.(*exec.ExitError); ok && runError.ExitCode() == DnfErrorExitCode {
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
	CacheDir string
}

func NewRPMMD(cacheDir string) RPMMD {
	return &rpmmdImpl{
		CacheDir: cacheDir,
	}
}

func (r *rpmmdImpl) FetchMetadata(repos []RepoConfig, modulePlatformID string, arch string) (PackageList, map[string]string, error) {
	var arguments = struct {
		Repos            []RepoConfig `json:"repos"`
		CacheDir         string       `json:"cachedir"`
		ModulePlatformID string       `json:"module_platform_id"`
		Arch             string       `json:"arch"`
	}{repos, r.CacheDir, modulePlatformID, arch}
	var reply struct {
		Checksums map[string]string `json:"checksums"`
		Packages  PackageList       `json:"packages"`
	}
	err := runDNF("dump", arguments, &reply)
	sort.Slice(reply.Packages, func(i, j int) bool {
		return reply.Packages[i].Name < reply.Packages[j].Name
	})
	return reply.Packages, reply.Checksums, err
}

func (r *rpmmdImpl) Depsolve(specs, excludeSpecs []string, repos []RepoConfig, modulePlatformID, arch string) ([]PackageSpec, map[string]string, error) {
	var arguments = struct {
		PackageSpecs     []string     `json:"package-specs"`
		ExcludSpecs      []string     `json:"exclude-specs"`
		Repos            []RepoConfig `json:"repos"`
		CacheDir         string       `json:"cachedir"`
		ModulePlatformID string       `json:"module_platform_id"`
		Arch             string       `json:"arch"`
	}{specs, excludeSpecs, repos, r.CacheDir, modulePlatformID, arch}
	var reply struct {
		Checksums    map[string]string `json:"checksums"`
		Dependencies []PackageSpec     `json:"dependencies"`
	}
	err := runDNF("depsolve", arguments, &reply)
	return reply.Dependencies, reply.Checksums, err
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

func (pkg *PackageInfo) FillDependencies(rpmmd RPMMD, repos []RepoConfig, modulePlatformID string, arch string) (err error) {
	pkg.Dependencies, _, err = rpmmd.Depsolve([]string{pkg.Name}, nil, repos, modulePlatformID, arch)
	return
}
