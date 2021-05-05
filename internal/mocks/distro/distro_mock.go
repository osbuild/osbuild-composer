package distro_mock

import (
	"github.com/osbuild/osbuild-composer/internal/distro/test_distro"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
)

func NewDefaultRegistry() (*distroregistry.Registry, error) {
	testDistro := test_distro.New()
	if testDistro == nil {
		panic("Attempt to register test distro failed")
	}
	return distroregistry.New(testDistro)
}
