package distro_mock

import (
	"github.com/osbuild/images/pkg/distro/test_distro"
	"github.com/osbuild/images/pkg/distroregistry"
)

func NewDefaultRegistry() (*distroregistry.Registry, error) {
	testDistro := test_distro.New()
	if testDistro == nil {
		panic("Attempt to register test distro failed")
	}
	return distroregistry.New(nil, testDistro)
}
