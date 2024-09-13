package blueprint

import (
	"encoding/json"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/pathpolicy"
	"github.com/stretchr/testify/assert"
)

func TestGetFilesystems(t *testing.T) {

	expectedFilesystems := []FilesystemCustomization{
		{
			MinSize:    1024,
			Mountpoint: "/",
		},
	}

	TestCustomizations := Customizations{
		Filesystem: expectedFilesystems,
	}

	retFilesystems := TestCustomizations.GetFilesystems()

	assert.ElementsMatch(t, expectedFilesystems, retFilesystems)
}

func TestGetFilesystemsMinSize(t *testing.T) {

	expectedFilesystems := []FilesystemCustomization{
		{
			MinSize:    1024,
			Mountpoint: "/",
		},
		{
			MinSize:    4096,
			Mountpoint: "/var",
		},
	}

	TestCustomizations := Customizations{
		Filesystem: expectedFilesystems,
	}

	retFilesystemsSize := TestCustomizations.GetFilesystemsMinSize()

	assert.EqualValues(t, uint64(5120), retFilesystemsSize)
}

func TestGetFilesystemsMinSizeNonSectorSize(t *testing.T) {

	expectedFilesystems := []FilesystemCustomization{
		{
			MinSize:    1025,
			Mountpoint: "/",
		},
		{
			MinSize:    4097,
			Mountpoint: "/var",
		},
	}

	TestCustomizations := Customizations{
		Filesystem: expectedFilesystems,
	}

	retFilesystemsSize := TestCustomizations.GetFilesystemsMinSize()

	assert.EqualValues(t, uint64(5632), retFilesystemsSize)
}

func TestGetFilesystemsMinSizeTOML(t *testing.T) {

	tests := []struct {
		Name  string
		TOML  string
		Want  []FilesystemCustomization
		Error bool
	}{
		{
			Name: "size set, no minsize",
			TOML: `
[[customizations.filesystem]]
mountpoint = "/var"
size = 1024
			`,
			Want:  []FilesystemCustomization{{MinSize: 1024, Mountpoint: "/var"}},
			Error: false,
		},
		{
			Name: "size set (string), no minsize",
			TOML: `
[[customizations.filesystem]]
mountpoint = "/var"
size = "1KiB"
			`,
			Want:  []FilesystemCustomization{{MinSize: 1024, Mountpoint: "/var"}},
			Error: false,
		},
		{
			Name: "minsize set, no size",
			TOML: `
[[customizations.filesystem]]
mountpoint = "/var"
minsize = 1024
			`,
			Want:  []FilesystemCustomization{{MinSize: 1024, Mountpoint: "/var"}},
			Error: false,
		},
		{
			Name: "minsize set (string), no size",
			TOML: `
[[customizations.filesystem]]
mountpoint = "/var"
minsize = "1KiB"
			`,
			Want:  []FilesystemCustomization{{MinSize: 1024, Mountpoint: "/var"}},
			Error: false,
		},
		{
			Name: "size and minsize set",
			TOML: `
[[customizations.filesystem]]
mountpoint = "/var"
size = 1024
minsize = 1024
			`,
			Want:  []FilesystemCustomization{},
			Error: true,
		},
		{
			Name: "size and minsize not set",
			TOML: `
[[customizations.filesystem]]
mountpoint = "/var"
			`,
			Want:  []FilesystemCustomization{},
			Error: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {

			var blueprint Blueprint
			err := toml.Unmarshal([]byte(tt.TOML), &blueprint)

			if tt.Error {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, blueprint.Customizations)
				assert.Equal(t, tt.Want, blueprint.Customizations.Filesystem)
			}
		})

	}

}

// ensure all fields that are supported are filled here
var allFieldsFsc = blueprint.FilesystemCustomization{
	Mountpoint: "/data",
	MinSize:    1234567890,
}

func TestFilesystemCustomizationMarshalUnmarshalTOML(t *testing.T) {
	b, err := toml.Marshal(allFieldsFsc)
	assert.NoError(t, err)

	var fsc blueprint.FilesystemCustomization
	err = toml.Unmarshal(b, &fsc)
	assert.NoError(t, err)
	assert.Equal(t, fsc, allFieldsFsc)
}

func TestFilesystemCustomizationMarshalUnmarshalJSON(t *testing.T) {
	b, err := json.Marshal(allFieldsFsc)
	assert.NoError(t, err)

	var fsc blueprint.FilesystemCustomization
	err = json.Unmarshal(b, &fsc)
	assert.NoError(t, err)
	assert.Equal(t, fsc, allFieldsFsc)
}

func TestFilesystemCustomizationUnmarshalTOMLUnhappy(t *testing.T) {
	cases := []struct {
		name  string
		input string
		err   string
	}{
		{
			name: "mountpoint not string",
			input: `mountpoint = 42
			minsize = 42`,
			err: "toml: line 0: TOML unmarshal: mountpoint must be string, got 42 of type int64",
		},
		{
			name: "misize nor string nor int",
			input: `mountpoint="/"
			minsize = true`,
			err: "toml: line 0: TOML unmarshal: minsize must be integer or string, got true of type bool",
		},
		{
			name: "misize not parseable",
			input: `mountpoint="/"
			minsize = "20 KG"`,
			err: "toml: line 0: TOML unmarshal: minsize is not valid filesystem size (unknown data size units in string: 20 KG)",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var fsc blueprint.FilesystemCustomization
			err := toml.Unmarshal([]byte(c.input), &fsc)
			assert.EqualError(t, err, c.err)
		})
	}
}

func TestFilesystemCustomizationUnmarshalJSONUnhappy(t *testing.T) {
	cases := []struct {
		name  string
		input string
		err   string
	}{
		{
			name:  "mountpoint not string",
			input: `{"mountpoint": 42, "minsize": 42}`,
			err:   "JSON unmarshal: mountpoint must be string, got 42 of type float64",
		},
		{
			name:  "misize nor string nor int",
			input: `{"mountpoint":"/", "minsize": true}`,
			err:   "JSON unmarshal: minsize must be float64 number or string, got true of type bool",
		},
		{
			name:  "misize not parseable",
			input: `{ "mountpoint": "/", "minsize": "20 KG"}`,
			err:   "JSON unmarshal: minsize is not valid filesystem size (unknown data size units in string: 20 KG)",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var fsc blueprint.FilesystemCustomization
			err := json.Unmarshal([]byte(c.input), &fsc)
			assert.EqualError(t, err, c.err)
		})
	}
}

func TestCheckMountpointsPolicy(t *testing.T) {
	policy := pathpolicy.NewPathPolicies(map[string]pathpolicy.PathPolicy{
		"/": {Exact: true},
	})

	mps := []blueprint.FilesystemCustomization{
		{Mountpoint: "/foo"},
		{Mountpoint: "/boot/"},
	}

	expectedErr := `The following errors occurred while setting up custom mountpoints:
path "/foo" is not allowed
path "/boot/" must be canonical`
	err := blueprint.CheckMountpointsPolicy(mps, policy)
	assert.EqualError(t, err, expectedErr)
}
