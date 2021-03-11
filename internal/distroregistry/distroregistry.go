package distroregistry

import (
	"errors"
	"fmt"
	"sort"

	"github.com/osbuild/osbuild-composer/internal/distro"
)

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
