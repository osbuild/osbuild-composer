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
	Id         string `json:"id"`
	Name       string `json:"name"`
	BaseURL    string `json:"baseurl,omitempty"`
	Metalink   string `json:"metalink,omitempty"`
	MirrorList string `json:"mirrorlist,omitempty"`
	Checksum   string `json:"checksum,omitempty"`
	GPGKey     string `json:"gpgkey,omitempty"`
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
	return PackageBuild{
		Arch:      pkg.Arch,
		BuildTime: pkg.BuildTime,
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
	Name    string `json:"name"`
	Epoch   uint   `json:"epoch"`
	Version string `json:"version,omitempty"`
	Release string `json:"release,omitempty"`
	Arch    string `json:"arch,omitempty"`
}

type PackageSource struct {
	License string `json:"license"`
	Version string `json:"version"`
}

type PackageBuild struct {
	Arch      string        `json:"arch"`
	BuildTime time.Time     `json:"build_time"`
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
	FetchPackageList(repos []RepoConfig) (PackageList, error)
	Depsolve(specs []string, repos []RepoConfig) ([]PackageSpec, error)
}

type DNFError struct {
	Kind   string `json:"kind"`
	Reason string `json:"reason"`
}

func (err *DNFError) Error() string {
	return fmt.Sprintf("DNF error occured: %s: %s", err.Kind, err.Reason)
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

type rpmmdImpl struct{}

func NewRPMMD() RPMMD {
	return &rpmmdImpl{}
}

func (*rpmmdImpl) FetchPackageList(repos []RepoConfig) (PackageList, error) {
	var arguments = struct {
		Repos []RepoConfig `json:"repos"`
	}{repos}
	var packages PackageList
	err := runDNF("dump", arguments, &packages)
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Name < packages[j].Name
	})
	return packages, err
}

func (*rpmmdImpl) Depsolve(specs []string, repos []RepoConfig) ([]PackageSpec, error) {
	var arguments = struct {
		PackageSpecs []string     `json:"package-specs"`
		Repos        []RepoConfig `json:"repos"`
	}{specs, repos}
	var dependencies []PackageSpec
	err := runDNF("depsolve", arguments, &dependencies)
	return dependencies, err
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

func (pkg *PackageInfo) FillDependencies(rpmmd RPMMD, repos []RepoConfig) (err error) {
	pkg.Dependencies, err = rpmmd.Depsolve([]string{pkg.Name}, repos)
	return
}
