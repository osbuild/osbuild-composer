package rhel85_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/distro/distro_test_common"
	"github.com/osbuild/osbuild-composer/internal/distro/rhel85"
)

type rhelFamilyDistro struct {
	name   string
	distro distro.Distro
}

var rhelFamilyDistros = []rhelFamilyDistro{
	{
		name:   "rhel",
		distro: rhel85.New(),
	},
}

func TestArchitecture_ListImageTypes(t *testing.T) {
	imgMap := []struct {
		arch                     string
		imgNames                 []string
		rhelAdditionalImageTypes []string
	}{
		{
			arch:     "x86_64",
			imgNames: []string{},
		},
		{
			arch:     "aarch64",
			imgNames: []string{},
		},
		{
			arch:     "ppc64le",
			imgNames: []string{},
		},
		{
			arch:     "s390x",
			imgNames: []string{},
		},
	}

	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, mapping := range imgMap {
				arch, err := dist.distro.GetArch(mapping.arch)
				require.NoError(t, err)
				imageTypes := arch.ListImageTypes()

				var expectedImageTypes []string
				expectedImageTypes = append(expectedImageTypes, mapping.imgNames...)
				if dist.name == "rhel" {
					expectedImageTypes = append(expectedImageTypes, mapping.rhelAdditionalImageTypes...)
				}

				require.ElementsMatch(t, expectedImageTypes, imageTypes)
			}
		})
	}
}

func TestRhel85_ListArches(t *testing.T) {
	arches := rhel85.New().ListArches()
	assert.Equal(t, []string{"aarch64", "ppc64le", "s390x", "x86_64"}, arches)
}

func TestRhel85_GetArch(t *testing.T) {
	arches := []struct {
		name                  string
		errorExpected         bool
		errorExpectedInCentos bool
	}{
		{
			name: "x86_64",
		},
		{
			name: "aarch64",
		},
		{
			name: "ppc64le",
		},
		{
			name: "s390x",
		},
		{
			name:          "foo-arch",
			errorExpected: true,
		},
	}

	for _, dist := range rhelFamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, a := range arches {
				actualArch, err := dist.distro.GetArch(a.name)
				if a.errorExpected || (a.errorExpectedInCentos && dist.name == "centos") {
					assert.Nil(t, actualArch)
					assert.Error(t, err)
				} else {
					assert.Equal(t, a.name, actualArch.Name())
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestRhel85_Name(t *testing.T) {
	distro := rhel85.New()
	assert.Equal(t, "rhel-85", distro.Name())
}

func TestRhel85_ModulePlatformID(t *testing.T) {
	distro := rhel85.New()
	assert.Equal(t, "platform:el8", distro.ModulePlatformID())
}

func TestRhel85_KernelOption(t *testing.T) {
	distro_test_common.TestDistro_KernelOption(t, rhel85.New())
}
