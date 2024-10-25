package rhel

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/runner"
)

// DefaultDistroImageConfigFunc is a function that returns the default image
// configuration for a distribution.
type DefaultDistroImageConfigFunc func(d *Distribution) *distro.ImageConfig

type Distribution struct {
	name               string
	DistCodename       string
	product            string
	osVersion          string
	releaseVersion     string
	modulePlatformID   string
	vendor             string
	ostreeRefTmpl      string
	runner             runner.Runner
	arches             map[string]distro.Arch
	DefaultImageConfig DefaultDistroImageConfigFunc

	// distro specific function to check options per image type
	CheckOptions CheckOptionsFunc
}

func (d *Distribution) Name() string {
	return d.name
}

func (d *Distribution) Codename() string {
	return d.DistCodename
}

func (d *Distribution) Releasever() string {
	return d.releaseVersion
}

func (d *Distribution) OsVersion() string {
	return d.osVersion
}

func (d *Distribution) Product() string {
	return d.product
}

func (d *Distribution) ModulePlatformID() string {
	return d.modulePlatformID
}

func (d *Distribution) OSTreeRef() string {
	return d.ostreeRefTmpl
}

func (d *Distribution) Vendor() string {
	return d.vendor
}

func (d *Distribution) ListArches() []string {
	archNames := make([]string, 0, len(d.arches))
	for name := range d.arches {
		archNames = append(archNames, name)
	}
	sort.Strings(archNames)
	return archNames
}

func (d *Distribution) GetArch(name string) (distro.Arch, error) {
	arch, exists := d.arches[name]
	if !exists {
		return nil, errors.New("invalid architecture: " + name)
	}
	return arch, nil
}

func (d *Distribution) AddArches(arches ...*Architecture) {
	if d.arches == nil {
		d.arches = map[string]distro.Arch{}
	}

	// Do not make copies of architectures, as opposed to image types,
	// because architecture definitions are not used by more than a single
	// distro definition.
	for idx := range arches {
		d.arches[arches[idx].Name()] = arches[idx]
	}
}

func (d *Distribution) IsRHEL() bool {
	return strings.HasPrefix(d.name, "rhel")
}

func (d *Distribution) GetDefaultImageConfig() *distro.ImageConfig {
	if d.DefaultImageConfig == nil {
		return nil
	}

	return d.DefaultImageConfig(d)
}

func NewDistribution(name string, major, minor int) (*Distribution, error) {
	var rd *Distribution
	switch name {
	case "rhel":
		if major < 0 {
			return nil, errors.New("Invalid RHEL major version (must be positive)")
		}

		if minor < 0 {
			return nil, errors.New("RHEL requires a minor version")
		}

		rd = &Distribution{
			name:             fmt.Sprintf("rhel-%d.%d", major, minor),
			product:          "Red Hat Enterprise Linux",
			osVersion:        fmt.Sprintf("%d.%d", major, minor),
			releaseVersion:   fmt.Sprintf("%d", major),
			modulePlatformID: fmt.Sprintf("platform:el%d", major),
			vendor:           "redhat",
			ostreeRefTmpl:    fmt.Sprintf("rhel/%d/%%s/edge", major),
			runner:           &runner.RHEL{Major: uint64(major), Minor: uint64(minor)},
		}
	case "centos":
		if minor != -1 {
			return nil, fmt.Errorf("CentOS does not have minor versions, but got %d", minor)
		}

		rd = &Distribution{
			name:             fmt.Sprintf("centos-%d", major),
			product:          "CentOS Stream",
			osVersion:        fmt.Sprintf("%d-stream", major),
			releaseVersion:   fmt.Sprintf("%d", major),
			modulePlatformID: fmt.Sprintf("platform:el%d", major),
			vendor:           "centos",
			ostreeRefTmpl:    fmt.Sprintf("centos/%d/%%s/edge", major),
			runner:           &runner.CentOS{Version: uint64(major)},
		}
	default:
		return nil, fmt.Errorf("unknown distro name: %s", name)
	}

	return rd, nil
}
