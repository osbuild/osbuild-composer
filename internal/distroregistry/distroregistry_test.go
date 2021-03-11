package distroregistry_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora33"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel84"
	"github.com/osbuild/osbuild-composer/internal/distroregistry"
)

// Test that all distros are registered properly and that Registry.List() works.
func TestRegistry_List(t *testing.T) {
	expected := []string{
		"centos-8",
		"fedora-32",
		"fedora-33",
		"rhel-8",
		"rhel-84",
	}

	distros, err := distroregistry.New(fedora32.New(), fedora33.New(), rhel8.New(), rhel84.New(), rhel84.NewCentos())
	require.NoError(t, err)

	require.Equalf(t, expected, distros.List(), "unexpected list of distros")
}
