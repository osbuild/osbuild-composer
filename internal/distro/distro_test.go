package distro_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora30"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora31"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"
)

func TestDistro_Manifest(t *testing.T) {
	distro_test_common.TestDistro_Manifest(
		t,
		"../../test/cases/",
		"*",
		fedora30.New(), fedora31.New(), fedora32.New(), rhel8.New(),
	)
}

// Test that all distros are registered properly and that Registry.List() works.
func TestDistro_RegistryList(t *testing.T) {
	expected := []string{
		"fedora-30",
		"fedora-31",
		"fedora-32",
		"rhel-8",
	}

	distros, err := distro.NewRegistry(fedora30.New(), fedora31.New(), fedora32.New(), rhel8.New())
	require.NoError(t, err)

	require.Equalf(t, expected, distros.List(), "unexpected list of distros")
}
