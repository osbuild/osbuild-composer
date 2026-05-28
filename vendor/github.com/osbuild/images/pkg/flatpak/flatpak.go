package flatpak

import (
	"fmt"
	"strings"

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
	ociClients := make(map[string]*OCIRegistryIndex)
	defer func() {
		for _, c := range ociClients {
			c.Close()
		}
	}()

	flatpaks := make(map[string][]Spec, len(sources))

	for name, srcList := range sources {
		specs := make([]Spec, len(srcList))
		for i, src := range srcList {
			if src.Registry.Type == REGISTRY_TYPE_OCI {
				uri, found := strings.CutPrefix(src.Registry.URI, "oci+")
				if !found {
					return nil, fmt.Errorf("flatpak registry %q: missing oci+ prefix", src.Registry.URI)
				}
				idx, ok := ociClients[uri]
				if !ok {
					var err error
					idx, err = NewOCIRegistryIndex(uri, "linux", "latest")
					if err != nil {
						return nil, err
					}
					ociClients[uri] = idx
				}
				res, err := src.Registry.queryOCIWithIndex(idx, src.Reference.String())
				if err != nil {
					return nil, fmt.Errorf("failed to resolve flatpak: %w", err)
				}
				specs[i] = *res
				continue
			}

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
