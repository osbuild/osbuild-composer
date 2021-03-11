package distro_mock

import (
	"github.com/osbuild/osbuild-composer/internal/distro/fedoratest"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
)

func NewDefaultRegistry() (*distroregistry.Registry, error) {
	ftest := fedoratest.New()
	if ftest == nil {
		panic("Attempt to register Fedora test failed")
	}
	return distroregistry.New(ftest)
}
