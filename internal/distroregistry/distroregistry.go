package distroregistry

import (
	"errors"
	"fmt"
	"sort"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora33"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel84"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel85"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel90"
)

// When adding support for a new distribution, add it here.
// Note that this is a constant, do not write to this array.
var supportedDistros = []func() distro.Distro{
	fedora32.New,
	fedora33.New,
	rhel8.New,
	rhel84.New,
	rhel84.NewCentos,
	rhel85.New,
	rhel90.New,
}

type Registry struct {
	distros map[string]distro.Distro
}

func New(distros ...distro.Distro) (*Registry, error) {
	reg := &Registry{
		distros: make(map[string]distro.Distro),
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
	for _, distroInitializer := range supportedDistros {
		distros = append(distros, distroInitializer())
	}

	registry, err := New(distros...)
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

func (r *Registry) FromHost() (distro.Distro, bool, bool, error) {
	name, beta, isStream, err := distro.GetHostDistroName()
	if err != nil {
		return nil, false, false, err
	}

	d := r.GetDistro(name)
	if d == nil {
		return nil, false, false, errors.New("unknown distro: " + name)
	}

	return d, beta, isStream, nil
}
