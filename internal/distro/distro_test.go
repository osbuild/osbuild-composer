package distro_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora32"
	"github.com/osbuild/osbuild-composer/internal/distro/fedora33"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel84"
)

func TestDistro_Manifest(t *testing.T) {
	distro_test_common.TestDistro_Manifest(
		t,
		"../../test/data/manifests/",
		"*",
		fedora32.New(), fedora33.New(), rhel8.New(), rhel84.New(),
	)
}

// Test that all distros are registered properly and that Registry.List() works.
func TestDistro_RegistryList(t *testing.T) {
	expected := []string{
		"fedora-32",
		"fedora-33",
		"rhel-8",
		"rhel-84",
	}

	distros, err := distro.NewRegistry(fedora32.New(), fedora33.New(), rhel8.New(), rhel84.New())
	require.NoError(t, err)

	require.Equalf(t, expected, distros.List(), "unexpected list of distros")
}
