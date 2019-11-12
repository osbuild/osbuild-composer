package rpmmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"sort"
	"time"
)

type RepoConfig struct {
	Id         string `json:"id"`
	Name       string `json:"name"`
	BaseURL    string `json:"baseurl,omitempty"`
	Metalink   string `json:"metalink,omitempty"`
	MirrorList string `json:"mirrorlist,omitempty"`
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

type PackageSpec struct {
	Name    string `json:"name"`
	Epoch   uint   `json:"epoch"`
	Version string `json:"version,omitempty"`
	Release string `json:"release,omitempty"`
	Arch    string `json:"arch,omitempty"`
}

type RPMMD interface {
	FetchPackageList(repos []RepoConfig) (PackageList, error)
	Depsolve(specs []string, repos []RepoConfig) ([]PackageSpec, error)
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

	err = json.NewDecoder(stdout).Decode(result)
	if err != nil {
		return err
	}

	return cmd.Wait()
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

func (packages PackageList) Search(name string) (int, int) {
	first := sort.Search(len(packages), func(i int) bool {
		return packages[i].Name >= name
	})

	if first == len(packages) || packages[first].Name != name {
		return first, 0
	}

	last := first + 1
	for last < len(packages) && packages[last].Name == name {
		last++
	}

	return first, last - first
}
