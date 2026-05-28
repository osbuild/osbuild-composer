package flatpak

import (
	"errors"
	"net/url"
	"strings"
)

var ErrUnknownRegistryType = errors.New("unknown registry type")

type RegistryType uint

const (
	REGISTRY_TYPE_UNKNOWN RegistryType = iota
	REGISTRY_TYPE_OCI
)

type Registry struct {
	RemoteName string
	Type       RegistryType
	URI        string
}

func NewRegistryFromURI(uri string) (*Registry, error) {
	registryType, err := registryTypeFromURI(uri)
	if err != nil {
		return nil, err
	}

	if registryType == REGISTRY_TYPE_UNKNOWN {
		return nil, errors.New("unknown registry type")
	}

	return &Registry{
		URI:  uri,
		Type: registryType,
	}, nil
}

func (r *Registry) queryOCI(ref string) (*Spec, error) {
	uri, found := strings.CutPrefix(r.URI, "oci+")
	if !found {
		// panic instead of error since this is unhandleable and we should really
		// not be able to get here since the registry type is determined *based*
		// on this URI
		panic("uri missing oci+ prefix")
	}

	idx, err := NewOCIRegistryIndex(uri, "linux", "latest")
	if err != nil {
		return nil, err
	}
	defer idx.Close()

	return r.queryOCIWithIndex(idx, ref)
}

// queryOCIWithIndex resolves ref using an existing index client (shared across
// multiple refs in [ResolveAll]). idx must have been constructed from this
// registry's oci+ URI (without the prefix).
func (r *Registry) queryOCIWithIndex(idx *OCIRegistryIndex, ref string) (*Spec, error) {
	containerSpec, err := idx.Query(ref)
	if err != nil {
		return nil, err
	}
	return &Spec{ContainerSpec: containerSpec}, nil
}

func (r *Registry) Query(ref string) (*Spec, error) {
	switch r.Type {
	case REGISTRY_TYPE_OCI:
		return r.queryOCI(ref)
	default:
		return nil, errors.New("unsupported registry type")
	}
}

func registryTypeFromURI(uri string) (RegistryType, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return REGISTRY_TYPE_UNKNOWN, err
	}

	switch u.Scheme {
	case "oci+https":
		return REGISTRY_TYPE_OCI, nil
	default:
		return REGISTRY_TYPE_UNKNOWN, nil
	}
}
