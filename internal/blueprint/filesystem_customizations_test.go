package blueprint

import (
	"testing"

	"github.com/BurntSushi/toml"
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
