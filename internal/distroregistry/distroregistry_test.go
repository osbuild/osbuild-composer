package distroregistry

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel8"
)

// Test that all distros are registered properly and that Registry.List() works.
func TestRegistry_List(t *testing.T) {
	// build expected distros
	var expected []string
	for _, supportedDistro := range supportedDistros {
		d := supportedDistro.defaultDistro()
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

func TestRegistry_mangleHostDistroName(t *testing.T) {

	type args struct {
		name     string
		isBeta   bool
		isStream bool
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{"fedora-33", args{"fedora-33", false, false}, "fedora-33"},
		{"fedora-33 beta", args{"fedora-33", true, false}, "fedora-33-beta"},
		{"fedora-33 stream", args{"fedora-33", false, true}, "fedora-33"},
		{"fedora-33 beta stream", args{"fedora-33", true, true}, "fedora-33-beta"},

		{"rhel-8", args{"rhel-8", false, false}, "rhel-8"},
		{"rhel-8 beta", args{"rhel-8", true, false}, "rhel-8-beta"},
		{"rhel-8 stream", args{"rhel-8", false, true}, "rhel-8"},
		{"rhel-8 beta stream", args{"rhel-8", true, true}, "rhel-8-beta"},

		{"rhel-84", args{"rhel-84", false, false}, "rhel-8"},
		{"rhel-84 beta", args{"rhel-84", true, false}, "rhel-8-beta"},
		{"rhel-84 stream", args{"rhel-84", false, true}, "rhel-8"},
		{"rhel-84 beta stream", args{"rhel-84", true, true}, "rhel-8-beta"},

		{"centos-8", args{"centos-8", false, false}, "centos-8"},
		{"centos-8 beta", args{"centos-8", true, false}, "centos-8-beta"},
		{"centos-8 stream", args{"centos-8", false, true}, "centos-stream-8"},
		{"centos-8 beta stream", args{"centos-8", true, true}, "centos-8-beta"},

		{"rhel-90", args{"rhel-90", false, false}, "rhel-90"},
		{"rhel-90 beta", args{"rhel-90", true, false}, "rhel-90-beta"},
		{"rhel-90 stream", args{"rhel-90", false, true}, "rhel-90"},
		{"rhel-90 beta stream", args{"rhel-90", true, true}, "rhel-90-beta"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mangledName := mangleHostDistroName(tt.args.name, tt.args.isBeta, tt.args.isStream)
			require.Equalf(
				t,
				tt.want,
				mangledName,
				"mangleHostDistroName() name:%s, isBeta:%s, isStream:%s =\nExpected: %s\nGot: %s\n",
				tt.args.name,
				tt.args.isBeta,
				tt.args.isStream,
				tt.want,
				mangledName,
			)
		})
	}
}

func TestRegistry_FromHost(t *testing.T) {
	//  expected distros
	var distros []distro.Distro
	for _, supportedDistro := range supportedDistros {
		distros = append(distros, supportedDistro.defaultDistro())
	}

	t.Run("host distro is nil", func(t *testing.T) {
		registry, err := New(nil, distros...)
		require.Nil(t, err)
		require.NotNil(t, registry)
		require.Nil(t, registry.FromHost())
	})

	t.Run("host distro not nil", func(t *testing.T) {
		// NOTE(akoutsou): The arguments to NewHostDistro are ignored since RHEL 8.6
		// The function signature will change in the near future.
		hostDistro := rhel8.NewHostDistro("", "", "")
		fmt.Println(hostDistro.Name())
		registry, err := New(hostDistro, distros...)
		require.Nil(t, err)
		require.NotNil(t, registry)

		gotDistro := registry.FromHost()
		require.NotNil(t, gotDistro)
		require.Equal(t, gotDistro.Name(), hostDistro.Name())
	})
}
