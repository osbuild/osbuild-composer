package blueprint

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlueprintParse(t *testing.T) {
	blueprint := `
name = "test"
description = "Test"
version = "0.0.0"

[[packages]]
name = "httpd"
version = "2.4.*"

[[customizations.filesystem]]
mountpoint = "/var"
size = 2147483648

[[customizations.filesystem]]
mountpoint = "/opt"
size = "20 GB"
`

	var bp Blueprint
	err := toml.Unmarshal([]byte(blueprint), &bp)
	require.Nil(t, err)
	assert.Equal(t, bp.Name, "test")
	assert.Equal(t, "/var", bp.Customizations.Filesystem[0].Mountpoint)
	assert.Equal(t, uint64(2147483648), bp.Customizations.Filesystem[0].MinSize)
	assert.Equal(t, "/opt", bp.Customizations.Filesystem[1].Mountpoint)
	assert.Equal(t, uint64(20*1000*1000*1000), bp.Customizations.Filesystem[1].MinSize)

	blueprint = `{
		"name": "test",
		"customizations": {
		  "filesystem": [{
			"mountpoint": "/opt",
			"minsize": "20 GiB"
		  }]
		}
	  }`
	err = json.Unmarshal([]byte(blueprint), &bp)
	require.Nil(t, err)
	assert.Equal(t, bp.Name, "test")
	assert.Equal(t, "/opt", bp.Customizations.Filesystem[0].Mountpoint)
	assert.Equal(t, uint64(20*1024*1024*1024), bp.Customizations.Filesystem[0].MinSize)
}

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

	bpCopy := bpOrig.DeepCopy()
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
	assert.ElementsMatch(t, []string{"tmux-1.2", "openssh-server", "@anaconda-tools", "kernel"}, Received_packages)
}

func TestKernelNameCustomization(t *testing.T) {
	kernels := []string{"kernel", "kernel-debug", "kernel-rt"}

	for _, k := range kernels {
		// kernel in customizations
		bp := Blueprint{
			Name:        "kernel-test",
			Description: "Testing GetPackages function with custom Kernel",
			Version:     "0.0.1",
			Packages: []Package{
				{Name: "tmux", Version: "1.2"}},
			Modules: []Package{
				{Name: "openssh-server", Version: "*"}},
			Groups: []Group{
				{Name: "anaconda-tools"}},
			Customizations: &Customizations{
				Kernel: &KernelCustomization{
					Name: k,
				},
			},
		}
		Received_packages := bp.GetPackages()
		assert.ElementsMatch(t, []string{"tmux-1.2", "openssh-server", "@anaconda-tools", k}, Received_packages)
	}

	for _, k := range kernels {
		// kernel in packages
		bp := Blueprint{
			Name:        "kernel-test",
			Description: "Testing GetPackages function with custom Kernel",
			Version:     "0.0.1",
			Packages: []Package{
				{Name: "tmux", Version: "1.2"},
				{Name: k},
			},
			Modules: []Package{
				{Name: "openssh-server", Version: "*"}},
			Groups: []Group{
				{Name: "anaconda-tools"}},
		}
		Received_packages := bp.GetPackages()

		// adds default kernel as well
		assert.ElementsMatch(t, []string{"tmux-1.2", k, "openssh-server", "@anaconda-tools", "kernel"}, Received_packages)
	}

	for _, bk := range kernels {
		for _, ck := range kernels {
			// all combos of both kernels
			bp := Blueprint{
				Name:        "kernel-test",
				Description: "Testing GetPackages function with custom Kernel",
				Version:     "0.0.1",
				Packages: []Package{
					{Name: "tmux", Version: "1.2"},
					{Name: bk},
				},
				Modules: []Package{
					{Name: "openssh-server", Version: "*"}},
				Groups: []Group{
					{Name: "anaconda-tools"}},
				Customizations: &Customizations{
					Kernel: &KernelCustomization{
						Name: ck,
					},
				},
			}
			Received_packages := bp.GetPackages()
			// both kernels are included, even if they're the same
			assert.ElementsMatch(t, []string{"tmux-1.2", bk, "openssh-server", "@anaconda-tools", ck}, Received_packages)
		}
	}
}

// TestBlueprintPasswords check to make sure all passwords are hashed
func TestBlueprintPasswords(t *testing.T) {
	blueprint := `
name = "test"
description = "Test"
version = "0.0.0"

[[customizations.user]]
name = "bart"
password = "nobodysawmedoit"

[[customizations.user]]
name = "lisa"
password = "$6$RWdHzrPfoM6BMuIP$gKYlBXQuJgP.G2j2twbOyxYjFDPUQw8Jp.gWe1WD/obX0RMyfgw5vt.Mn/tLLX4mQjaklSiIzoAW3HrVQRg4Q."

[[customizations.user]]
name = "maggie"
password = ""
`

	var bp Blueprint
	err := toml.Unmarshal([]byte(blueprint), &bp)
	require.Nil(t, err)
	require.Nil(t, bp.Initialize())

	// Note: User entries are in the same order as the toml
	users := bp.Customizations.GetUsers()
	assert.Equal(t, "bart", users[0].Name)
	assert.True(t, strings.HasPrefix(*users[0].Password, "$6$"))
	assert.Equal(t, "lisa", users[1].Name)
	assert.Equal(t, "$6$RWdHzrPfoM6BMuIP$gKYlBXQuJgP.G2j2twbOyxYjFDPUQw8Jp.gWe1WD/obX0RMyfgw5vt.Mn/tLLX4mQjaklSiIzoAW3HrVQRg4Q.", *users[1].Password)
	assert.Equal(t, "maggie", users[2].Name)
	assert.Nil(t, users[2].Password)
}
