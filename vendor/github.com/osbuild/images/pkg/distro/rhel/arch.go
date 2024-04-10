package rhel

import (
	"errors"
	"fmt"
	"sort"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/platform"
)

type Architecture struct {
	distro           *Distribution
	arch             arch.Arch
	imageTypes       map[string]distro.ImageType
	imageTypeAliases map[string]string
}

func (a *Architecture) Name() string {
	return a.arch.String()
}

func (a *Architecture) Distro() distro.Distro {
	return a.distro
}

func (a *Architecture) ListImageTypes() []string {
	itNames := make([]string, 0, len(a.imageTypes))
	for name := range a.imageTypes {
		itNames = append(itNames, name)
	}
	sort.Strings(itNames)
	return itNames
}

func (a *Architecture) GetImageType(name string) (distro.ImageType, error) {
	t, exists := a.imageTypes[name]
	if !exists {
		aliasForName, exists := a.imageTypeAliases[name]
		if !exists {
			return nil, errors.New("invalid image type: " + name)
		}
		t, exists = a.imageTypes[aliasForName]
		if !exists {
			panic(fmt.Sprintf("image type '%s' is an alias to a non-existing image type '%s'", name, aliasForName))
		}
	}
	return t, nil
}

func (a *Architecture) AddImageTypes(platform platform.Platform, imageTypes ...*ImageType) {
	if a.imageTypes == nil {
		a.imageTypes = map[string]distro.ImageType{}
	}
	for idx := range imageTypes {
		it := imageTypes[idx]
		it.arch = a
		it.platform = platform
		a.imageTypes[it.name] = it
		for _, alias := range it.NameAliases {
			if a.imageTypeAliases == nil {
				a.imageTypeAliases = map[string]string{}
			}
			if existingAliasFor, exists := a.imageTypeAliases[alias]; exists {
				panic(fmt.Sprintf("image type alias '%s' for '%s' is already defined for another image type '%s'", alias, it.name, existingAliasFor))
			}
			a.imageTypeAliases[alias] = it.name
		}
	}
}

func NewArchitecture(distro *Distribution, arch arch.Arch) *Architecture {
	return &Architecture{
		distro: distro,
		arch:   arch,
	}
}
