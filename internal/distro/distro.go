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
)

// A Distro represents composer's notion of what a given distribution is.
type Distro interface {
	// Returns the name of the distro.
	Name() string

	// Return strong-typed distribution
	Distribution() common.Distribution

	// Returns the module platform id of the distro. This is used by DNF
	// for modularity support.
	ModulePlatformID() string

	// Returns an object representing the given architecture as support
	// by this distro.
	GetArch(arch string) (Arch, error)

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
	Manifest(b *blueprint.Customizations, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, outputArchitecture, imageFormat string, size uint64) (*osbuild.Manifest, error)

	// Returns a osbuild runner that can be used on this distro.
	Runner() string
}

// An Arch represents a given distribution's support for a given architecture.
type Arch interface {
	// Returns the name of the architecture.
	Name() string

	// Returns a sorted list of the names of the image types this architecture
	// supports.
	ListImageTypes() []string

	// Returns an object representing a given image format for this architecture,
	// on this distro.
	GetImageType(imageType string) (ImageType, error)
}

// An ImageType represents a given distribution's support for a given Image Type
// for a given architecture.
type ImageType interface {
	// Returns the name of the image type.
	Name() string

	// Returns the canonical filename for the image type.
	Filename() string

	// Retrns the MIME-type for the image type.
	MIMEType() string

	// Returns the proper image size for a given output format. If the input size
	// is 0 the default value for the format will be returned.
	Size(size uint64) uint64

	// Returns the default packages to include and exclude when making the image
	// type.
	BasePackages() ([]string, []string)

	// Returns the build packages for the output type.
	BuildPackages() []string

	// Returns an osbuild manifest, containing the sources and pipeline necessary
	// to build an image, given output format with all packages and customizations
	// specified in the given blueprint.
	Manifest(b *blueprint.Customizations, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, size uint64) (*osbuild.Manifest, error)
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

// List returns the names of all distros in a Registry, sorted alphabetically.
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
