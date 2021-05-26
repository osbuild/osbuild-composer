package distroregistry

import (
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora33"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel84"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel85"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel90"
	"github.com/osbuild/osbuild-composer/internal/distro/unsupported"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

// When adding support for a new distribution, add it here.
// Note that this is a constant, do not write to this array.
var supportedDistros = []supportedDistro{
	{fedora32.New, fedora32.NewHostDistro},
	{fedora33.New, fedora33.NewHostDistro},
	{rhel8.New, rhel8.NewHostDistro},
	{rhel84.New, rhel84.NewHostDistro},
	{rhel84.NewCentos, rhel84.NewCentosHostDistro},
	{rhel85.New, rhel85.NewHostDistro},
	{rhel90.New, rhel90.NewHostDistro},
}

type supportedDistro struct {
	defaultDistro func() distro.Distro
	hostDistro    func(name string) distro.Distro
}

type Registry struct {
	distros    map[string]distro.Distro
	hostDistro distro.Distro
}

func New(hostDistro distro.Distro, distros ...distro.Distro) (*Registry, error) {
	reg := &Registry{
		distros:    make(map[string]distro.Distro),
		hostDistro: hostDistro,
	}
	for _, d := range distros {
		name := d.Name()
		if _, exists := reg.distros[name]; exists {
			return nil, fmt.Errorf("New: passed two distros with the same name: %s", d.Name())
		}
		reg.distros[name] = d
	}
	return reg, nil
}

// NewDefault creates a Registry with all distributions supported by
// osbuild-composer. If you need to add a distribution here, see the
// supportedDistros variable.
func NewDefault() *Registry {
	var distros []distro.Distro
	var hostDistro distro.Distro = nil

	// First determine the name of the Host Distro
	// If there was an error, then the hostDistroName will be an empty string
	// and as a result, the hostDistro will have a nil value when calling New().
	// Getting the host distro later using FromHost() will return nil as well.
	hostDistroName, hostDistroIsBeta, hostDistroIsStream, _ := distro.GetHostDistroName()

	for _, supportedDistro := range supportedDistros {
		distro := supportedDistro.defaultDistro()

		if distro.Name() == hostDistroName {
			hostDistro = supportedDistro.hostDistro(
				mangleHostDistroName(distro.Name(), hostDistroIsBeta, hostDistroIsStream),
			)
		}

		distros = append(distros, distro)
	}

	registry, err := New(hostDistro, distros...)
	if err != nil {
		panic(fmt.Sprintf("two supported distros have the same name, this is a programming error: %v", err))
	}

	return registry
}

func (r *Registry) GetDistro(name string) distro.Distro {
	d, ok := r.distros[name]
	if !ok {
		return nil
	}

	return d
}

// List returns the names of all distros in a Registry, sorted alphabetically.
func (r *Registry) List() []string {
	list := []string{}
	for _, d := range r.distros {
		list = append(list, d.Name())
	}
	sort.Strings(list)
	return list
}

func mangleHostDistroName(name string, isBeta, isStream bool) string {
	hostDistroName := name
	if strings.HasPrefix(hostDistroName, "rhel-8") {
		hostDistroName = "rhel-8"
	}
	if isBeta {
		hostDistroName += "-beta"
	}

	// override repository for centos stream, remove when CentOS 8 is EOL
	if isStream && hostDistroName == "centos-8" {
		hostDistroName = "centos-stream-8"
	}

	return hostDistroName
}

// FromHost returns a distro instance, that is specific to the host.
// Its name may differ from other supported distros, if the host version
// is e.g. a Beta or a Stream.
func (r *Registry) FromHost() distro.Distro {
	return r.hostDistro
}

// FromHost returns a distro instance, that is specific to the host.
// Its name may differ from other supported distros, if the host version
// is e.g. a Beta or a Stream.
func (r *Registry) FallbackHost() (distro.Distro, string, map[string][]rpmmd.RepoConfig) {
	hostDistroName, _, _, _ := distro.GetHostDistroName()

	parts := strings.Split(hostDistroName, "-")
	if len(parts) != 2 {
		panic(hostDistroName + " is not valid distro name")
	}
	distroVersion := parts[1]

	if strings.HasPrefix(hostDistroName, "fedora-") {
		log.Println("Warning: " + hostDistroName + " is not supported Fedora version. " +
			"osbuild-composer will start with Fedora 33 image definitions but this might not work!")
		r.hostDistro = fedora33.NewUnreleasedDistro(hostDistroName, distroVersion)
	}
	return r.hostDistro, hostDistroName, unsupported.GenerateFedoraRepositories(distroVersion)
}
