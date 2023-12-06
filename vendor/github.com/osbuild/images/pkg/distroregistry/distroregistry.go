package distroregistry

import (
	"fmt"
	"sort"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/fedora"
	"github.com/osbuild/images/pkg/distro/rhel7"
	"github.com/osbuild/images/pkg/distro/rhel8"
	"github.com/osbuild/images/pkg/distro/rhel9"
)

// When adding support for a new distribution, add it here.
// Note that this is a constant, do not write to this array.
var supportedDistros = []func() distro.Distro{
	fedora.NewF37,
	fedora.NewF38,
	fedora.NewF39,
	fedora.NewF40,

	rhel7.New,

	rhel8.New,
	rhel8.NewRHEL84,
	rhel8.NewRHEL85,
	rhel8.NewRHEL86,
	rhel8.NewRHEL87,
	rhel8.NewRHEL88,
	rhel8.NewRHEL89,
	rhel8.NewRHEL810,
	rhel8.NewCentos,

	rhel9.New,
	rhel9.NewRHEL90,
	rhel9.NewRHEL91,
	rhel9.NewRHEL92,
	rhel9.NewRHEL93,
	rhel9.NewRHEL94,
	rhel9.NewCentOS9,
}

type Registry struct {
	distros      map[string]distro.Distro
	hostDistro   distro.Distro
	hostArchName string
}

func New(hostDistro distro.Distro, distros ...distro.Distro) (*Registry, error) {
	reg := &Registry{
		distros:      make(map[string]distro.Distro),
		hostDistro:   hostDistro,
		hostArchName: arch.Current().String(),
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
	var hostDistro distro.Distro

	// First determine the name of the Host Distro
	// If there was an error, then the hostDistroName will be an empty string
	// and as a result, the hostDistro will have a nil value when calling New().
	// Getting the host distro later using FromHost() will return nil as well.
	hostDistroName, _, _, _ := common.GetHostDistroName()
	for _, supportedDistro := range supportedDistros {
		distro := supportedDistro()
		if distro.Name() == hostDistroName {
			hostDistro = supportedDistro()
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

// HostArchName returns the host's arch name
func (r *Registry) HostArchName() string {
	return r.hostArchName
}

// SetHostArchName can be used to override the host's arch name for testing
func (r *Registry) SetHostArchName(name string) {
	r.hostArchName = name
}
