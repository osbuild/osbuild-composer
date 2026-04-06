package flatpak

import (
	"fmt"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/ostree"
)

type SourceSpec struct {
	Registry  Registry
	Reference Reference
}

// A flatpak source can return (based on the type of the registry in the `SourceSpec`) either a
// container or an ostree commit.
type Spec struct {
	ContainerSpec *container.Spec
	CommitSpec    *ostree.CommitSpec
}

func Resolve(source SourceSpec) (Spec, error) {
	spec, err := source.Registry.Query(source.Reference.String())
	if err != nil {
		return Spec{}, err
	}

	if spec == nil {
		return Spec{}, fmt.Errorf("registry query returned nil spec")
	}

	return *spec, nil
}

func ResolveAll(sources map[string][]SourceSpec) (map[string][]Spec, error) {
	flatpaks := make(map[string][]Spec, len(sources))

	for name, srcList := range sources {
		specs := make([]Spec, len(srcList))
		for i, src := range srcList {
			res, err := Resolve(src)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve flatpak: %w", err)
			}
			specs[i] = res
		}
		flatpaks[name] = specs
	}

	return flatpaks, nil
}
