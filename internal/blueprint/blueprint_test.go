package blueprint

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestDeepCopy(t *testing.T) {
	bpOrig := Blueprint{
		Name:        "deepcopy-test",
		Description: "Testing DeepCopy function",
		Version:     "0.0.1",
		Packages: []Package{
			{Name: "dep-package1", Version: "*"}},
		Modules: []Package{
			{Name: "dep-package2", Version: "*"}},
	}

	bpCopy, err := bpOrig.DeepCopy()
	require.NoError(t, err)
	require.Equalf(t, bpOrig, bpCopy, "Blueprints.DeepCopy is different from original.")

	// Modify the copy
	bpCopy.Packages[0].Version = "1.2.3"
	require.Equalf(t, bpOrig.Packages[0].Version, "*", "Blueprint.DeepCopy failed, original modified")

	// Modify the original
	bpOrig.Packages[0].Version = "42.0"
	require.Equalf(t, bpCopy.Packages[0].Version, "1.2.3", "Blueprint.DeepCopy failed, copy modified.")
}

func TestBlueprintInitialize(t *testing.T) {
	cases := []struct {
		NewBlueprint  Blueprint
		ExpectedError bool
	}{
		{Blueprint{Name: "bp-test-1", Description: "Empty version", Version: ""}, false},
		{Blueprint{Name: "bp-test-2", Description: "Invalid version 1", Version: "0"}, true},
		{Blueprint{Name: "bp-test-2", Description: "Invalid version 2", Version: "0.0"}, true},
		{Blueprint{Name: "bp-test-3", Description: "Invalid version 3", Version: "0.0.0.0"}, true},
		{Blueprint{Name: "bp-test-4", Description: "Invalid version 4", Version: "0.a.0"}, true},
		{Blueprint{Name: "bp-test-5", Description: "Invalid version 5", Version: "foo"}, true},
		{Blueprint{Name: "bp-test-7", Description: "Zero version", Version: "0.0.0"}, false},
		{Blueprint{Name: "bp-test-8", Description: "X.Y.Z version", Version: "2.1.3"}, false},
	}

	for _, c := range cases {
		bp := c.NewBlueprint
		err := bp.Initialize()
		assert.Equalf(t, (err != nil), c.ExpectedError, "Initialize(%#v) returnted an unexpected error: %#v", c.NewBlueprint, err)
	}
}

func TestBumpVersion(t *testing.T) {
	cases := []struct {
		NewBlueprint    Blueprint
		OldVersion      string
		ExpectedVersion string
	}{
		{Blueprint{Name: "bp-test-1", Description: "Empty version", Version: "0.0.1"}, "", "0.0.1"},
		{Blueprint{Name: "bp-test-2", Description: "Invalid version 1", Version: "0.0.1"}, "0", "0.0.1"},
		{Blueprint{Name: "bp-test-3", Description: "Invalid version 2", Version: "0.0.1"}, "0.0.0.0", "0.0.1"},
		{Blueprint{Name: "bp-test-4", Description: "Invalid version 3", Version: "0.0.1"}, "0.a.0", "0.0.1"},
		{Blueprint{Name: "bp-test-5", Description: "Invalid version 4", Version: "0.0.1"}, "foo", "0.0.1"},
		{Blueprint{Name: "bp-test-6", Description: "Invalid version 5", Version: "0.0.1"}, "0.0", "0.0.1"},
		{Blueprint{Name: "bp-test-8", Description: "Same version", Version: "4.2.0"}, "4.2.0", "4.2.1"},
	}

	for _, c := range cases {
		bp := c.NewBlueprint
		err := bp.Initialize()
		require.NoError(t, err)

		bp.BumpVersion(c.OldVersion)
		assert.Equalf(t, c.ExpectedVersion, bp.Version, "BumpVersion(%#v) is expected to return %#v, but instead returned %#v.", c.OldVersion, c.ExpectedVersion, bp.Version)
	}
}

func TestGetPackages(t *testing.T) {

	bp := Blueprint{
		Name:        "packages-test",
		Description: "Testing GetPackages function",
		Version:     "0.0.1",
		Packages: []Package{
			{Name: "tmux", Version: "1.2"}},
		Modules: []Package{
			{Name: "openssh-server", Version: "*"}},
		Groups: []Group{
			{Name: "anaconda-tools"}},
	}
	Received_packages := bp.GetPackages()
	assert.ElementsMatch(t, []string{"tmux-1.2", "openssh-server", "@anaconda-tools"}, Received_packages)
}
