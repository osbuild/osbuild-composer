package distro

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/osbuild"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"

	"github.com/osbuild/osbuild-composer/internal/distro/fedora30"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel81"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel82"
)

type Distro interface {
	// Returns the name of the distro. This is the same name that was
	// passed to New().
	Name() string

	// Return strong-typed distribution
	Distribution() common.Distribution

	// Returns the module platform id of the distro. This is used by DNF
	// for modularity support.
	ModulePlatformID() string

	// Returns a list of repositories from which this distribution gets its
	// content.
	Repositories(arch string) []rpmmd.RepoConfig

	// Returns a sorted list of the output formats this distro supports.
	ListOutputFormats() []string

	// Returns the canonical filename and MIME type for a given output
	// format. `outputFormat` must be one returned by
	FilenameFromType(outputFormat string) (string, string, error)

	// Returns the proper image size for a given output format. If the size
	// is 0 the default value for the format will be returned.
	GetSizeForOutputType(outputFormat string, size uint64) uint64

	// Returns the base packages for a given output type and architecture
	BasePackages(outputFormat, outputArchitecture string) ([]string, []string, error)

	// Returns the build packages for a given output architecture
	BuildPackages(outputArchitecture string) ([]string, error)

	// Returns an osbuild manifest, containing the sources and pipeline necessary
	// to generates an image in the given output format with all packages and
	// customizations specified in the given blueprint.
	Manifest(b *blueprint.Blueprint, additionalRepos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, outputArchitecture, imageFormat string, size uint64) (*osbuild.Manifest, error)

	// Returns a osbuild runner that can be used on this distro.
	Runner() string
}

type Registry struct {
	distros map[common.Distribution]Distro
}

func NewRegistry(distros ...Distro) (*Registry, error) {
	reg := &Registry{
		distros: make(map[common.Distribution]Distro),
	}
	for _, distro := range distros {
		distroTag := distro.Distribution()
		if _, exists := reg.distros[distroTag]; exists {
			return nil, fmt.Errorf("NewRegistry: passed two distros with the same name: %s", distro.Name())
		}
		reg.distros[distroTag] = distro
	}
	return reg, nil
}

// Create a new Registry containing all known distros.
func NewDefaultRegistry(confPaths []string) (*Registry, error) {
	f30, err := fedora30.New(confPaths)
	if err != nil {
		return nil, fmt.Errorf("error loading fedora30: %v", err)
	}
	f31, err := fedora31.New(confPaths)
	if err != nil {
		return nil, fmt.Errorf("error loading fedora31: %v", err)
	}
	f32, err := fedora32.New(confPaths)
	if err != nil {
		return nil, fmt.Errorf("error loading fedora32: %v", err)
	}
	el81, err := rhel81.New(confPaths)
	if err != nil {
		return nil, fmt.Errorf("error loading rhel81: %v", err)
	}
	el82, err := rhel82.New(confPaths)
	if err != nil {
		return nil, fmt.Errorf("error loading rhel82: %v", err)
	}
	return NewRegistry(f30, f31, f32, el81, el82)
}

func (r *Registry) GetDistro(name string) Distro {
	distroTag, exists := common.DistributionFromString(name)
	if !exists {
		return nil
	}
	distro, ok := r.distros[distroTag]
	if !ok {
		return nil
	}

	return distro
}

// Returns the names of all distros in a Registry, sorted alphabetically.
func (r *Registry) List() []string {
	list := []string{}
	for _, distro := range r.distros {
		list = append(list, distro.Name())
	}
	sort.Strings(list)
	return list
}

func (r *Registry) FromHost() (Distro, error) {
	name, err := GetHostDistroName()
	if err != nil {
		return nil, err
	}

	d := r.GetDistro(name)
	if d == nil {
		return nil, errors.New("unknown distro: " + name)
	}

	return d, nil
}

func GetHostDistroName() (string, error) {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return "", err
	}
	defer f.Close()
	osrelease, err := readOSRelease(f)
	if err != nil {
		return "", err
	}

	name := osrelease["ID"] + "-" + osrelease["VERSION_ID"]
	return name, nil
}

func readOSRelease(r io.Reader) (map[string]string, error) {
	osrelease := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			return nil, errors.New("readOSRelease: invalid input")
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if value[0] == '"' {
			if len(value) < 2 || value[len(value)-1] != '"' {
				return nil, errors.New("readOSRelease: invalid input")
			}
			value = value[1 : len(value)-1]
		}

		osrelease[key] = value
	}

	return osrelease, nil
}
