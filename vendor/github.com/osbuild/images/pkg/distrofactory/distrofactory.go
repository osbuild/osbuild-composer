package distrofactory

import (
	"fmt"
	"sort"

	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/generic"
	"github.com/osbuild/images/pkg/distro/rhel/rhel10"
	"github.com/osbuild/images/pkg/distro/rhel/rhel7"
	"github.com/osbuild/images/pkg/distro/rhel/rhel8"
	"github.com/osbuild/images/pkg/distro/rhel/rhel9"
	"github.com/osbuild/images/pkg/distro/test_distro"
)

// FactoryFunc is a function that returns a distro.Distro for a given distro
// represented as a string. If the string does not represent a distro, that can
// be detected by the factory, it should return nil.
type FactoryFunc func(idStr string) distro.Distro

// Factory is a list of distro.Distro factories.
type Factory struct {
	factories []FactoryFunc

	// distro ID string aliases
	aliases map[string]string
}

// getDistro returns the distro.Distro that matches the given distro ID. If no
// distro.Distro matches the given distro ID, it returns nil. If multiple distro
// factories match the given distro ID, it panics.
func (f *Factory) getDistro(name string) distro.Distro {
	var match distro.Distro
	for _, f := range f.factories {
		if d := f(name); d != nil {
			if match != nil {
				panic(fmt.Sprintf("distro ID was matched by multiple distro factories: %v, %v", match, d))
			}
			match = d
		}
	}

	return match
}

// GetDistro returns the distro.Distro that matches the given distro ID. If no
// distro.Distro matches the given distro ID, it tries to translate the given
// distro ID using the aliases map and tries again. If no distro.Distro matches
// the given distro ID, it returns nil. If multiple distro factories match the
// given distro ID, it panics.
func (f *Factory) GetDistro(name string) distro.Distro {
	match := f.getDistro(name)

	if alias, ok := f.aliases[name]; match == nil && ok {
		match = f.getDistro(alias)
	}

	return match
}

// FromHost returns a distro.Distro instance, that is specific to the host.
// If the host distro is not supported, nil is returned.
func (f *Factory) FromHost() distro.Distro {
	hostDistroName, _ := distro.GetHostDistroName()
	return f.GetDistro(hostDistroName)
}

// RegisterAliases configures the factory with aliases for distro names.
// The provided aliases map has the following constraints:
// - An alias must not mask an existing distro.
// - An alias target must map to an existing distro.
func (f *Factory) RegisterAliases(aliases map[string]string) error {
	var errors []string
	for alias, target := range aliases {
		var targetExists bool
		for _, factory := range f.factories {
			if factory(alias) != nil {
				errors = append(errors, fmt.Sprintf("alias '%s' masks an existing distro", alias))
			}
			if factory(target) != nil {
				targetExists = true
			}
		}
		if !targetExists {
			errors = append(errors, fmt.Sprintf("alias '%s' targets a non-existing distro '%s'", alias, target))
		}
	}

	// NB: iterating over a map of aliases is not deterministic, so sort the
	// errors to make the output deterministic
	sort.Strings(errors)

	if len(errors) > 0 {
		return fmt.Errorf("invalid aliases: %q", errors)
	}

	f.aliases = aliases
	return nil
}

// New returns a Factory of distro.Distro factories for the given distros.
func New(factories ...FactoryFunc) *Factory {
	return &Factory{
		factories: factories,
	}
}

// NewDefault returns a Factory of distro.Distro factories for all supported
// distros.
func NewDefault() *Factory {
	return New(
		generic.DistroFactory,
		rhel7.DistroFactory,
		rhel8.DistroFactory,
		rhel9.DistroFactory,
		rhel10.DistroFactory,
	)
}

// NewTestDefault returns a Factory of distro.Distro factory for the test_distro.
func NewTestDefault() *Factory {
	return New(
		test_distro.DistroFactory,
	)
}
