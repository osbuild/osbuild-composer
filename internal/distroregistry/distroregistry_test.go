package distroregistry

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"
)

// Test that all distros are registered properly and that Registry.List() works.
func TestRegistry_List(t *testing.T) {
	// build expected distros
	var expected []string
	for _, distroInitializer := range supportedDistros {
		d := distroInitializer()
		expected = append(expected, d.Name())
	}

	distros := NewDefault()

	require.ElementsMatch(t, expected, distros.List(), "unexpected list of distros")
}

func TestRegistry_GetDistro(t *testing.T) {
	distros := NewDefault()

	t.Run("distro exists", func(t *testing.T) {
		expectedDistro := rhel8.New()
		require.Equal(t, expectedDistro.Name(), distros.GetDistro(expectedDistro.Name()).Name())
	})

	t.Run("distro doesn't exist", func(t *testing.T) {
		require.Nil(t, distros.GetDistro("toucan-os"))
	})
}
