package distroregistry

import (
	"testing"

	"github.com/stretchr/testify/require"
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
