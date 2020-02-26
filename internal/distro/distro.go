package distro

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/osbuild"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"

	"github.com/osbuild/osbuild-composer/internal/distro/fedora30"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
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

	// Returns an osbuild pipeline that generates an image in the given
	// output format with all packages and customizations specified in the
	// given blueprint.
	Pipeline(b *blueprint.Blueprint, additionalRepos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, checksums map[string]string, outputArchitecture, outputFormat string, size uint64) (*osbuild.Pipeline, error)

	// Returns an osbuild sources object that is required for building the
	// corresponding pipeline containing the given packages.
	Sources(packages []rpmmd.PackageSpec) *osbuild.Sources

	// Returns a osbuild runner that can be used on this distro.
	Runner() string
}

type Registry struct {
	distros map[common.Distribution]Distro
}

func WithSingleDistro(dist Distro) *Registry {
	reg := &Registry{
		distros: make(map[common.Distribution]Distro),
	}
	reg.register(dist)
	return reg
}

func NewRegistry(confPaths []string) *Registry {
	distros := &Registry{
		distros: make(map[common.Distribution]Distro),
	}
	f30 := fedora30.New(confPaths)
	if f30 == nil {
		panic("Attempt to register Fedora 30 failed")
	}
	distros.register(f30)
	f31 := fedora31.New(confPaths)
	if f31 == nil {
		panic("Attempt to register Fedora 31 failed")
	}
	distros.register(f31)
	f32 := fedora32.New(confPaths)
	if f32 == nil {
		panic("Attempt to register Fedora 32 failed")
	}
	distros.register(f32)
	el82 := rhel82.New(confPaths)
	if el82 == nil {
		panic("Attempt to register RHEL 8.2 failed")
	}
	distros.register(el82)
	return distros
}

func (r *Registry) register(distro Distro) {
	distroTag := distro.Distribution()
	if _, exists := r.distros[distroTag]; exists {
		panic("a distro with this name already exists: " + distro.Name())
	}
	r.distros[distroTag] = distro
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
