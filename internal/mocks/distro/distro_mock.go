package distro_mock

import (
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/fedoratest"
)

func NewDefaultRegistry() *distro.Registry {
	ftest := fedoratest.New()
	if ftest == nil {
		panic("Attempt to register Fedora test failed")
	}
	return distro.NewRegistry(ftest)
}
