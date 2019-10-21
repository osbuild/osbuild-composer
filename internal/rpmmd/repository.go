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
	Name     string `json:"name"`
	Epoch    uint   `json:"epoch"`
	Version  string `json:"version,omitempty"`
	Release  string `json:"release,omitempty"`
	Arch     string `json:"arch,omitempty"`
	Checksum string `json:"checksum"`
	URL      string `json:"url"`
}

func runDNF(command string, arguments []string, result interface{}) error {
	cmd := exec.Command("dnf-json", append([]string{command}, arguments...)...)
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	err = json.NewDecoder(stdout).Decode(result)
	if err != nil {
		return err
	}

	return cmd.Wait()
}

func FetchPackageList(repo RepoConfig) (PackageList, error) {
	var packages PackageList
	err := runDNF("dump", nil, &packages)
	return packages, err
}

func Depsolve(specs ...string) ([]PackageSpec, error) {
	var dependencies []PackageSpec
	err := runDNF("depsolve", specs, &dependencies)
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
