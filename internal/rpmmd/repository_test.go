package rpmmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPackageSpecGetEVRA(t *testing.T) {
	specs := []PackageSpec{
		{
			Name:    "tmux",
			Epoch:   0,
			Version: "3.3a",
			Release: "3.fc38",
			Arch:    "x86_64",
		},
		{
			Name:    "grub2",
			Epoch:   1,
			Version: "2.06",
			Release: "94.fc38",
			Arch:    "noarch",
		},
	}

	assert.Equal(t, "3.3a-3.fc38.x86_64", specs[0].GetEVRA())
	assert.Equal(t, "1:2.06-94.fc38.noarch", specs[1].GetEVRA())

}

func TestPackageSpecGetNEVRA(t *testing.T) {
	specs := []PackageSpec{
		{
			Name:    "tmux",
			Epoch:   0,
			Version: "3.3a",
			Release: "3.fc38",
			Arch:    "x86_64",
		},
		{
			Name:    "grub2",
			Epoch:   1,
			Version: "2.06",
			Release: "94.fc38",
			Arch:    "noarch",
		},
	}

	assert.Equal(t, "tmux-3.3a-3.fc38.x86_64", specs[0].GetNEVRA())
	assert.Equal(t, "grub2-1:2.06-94.fc38.noarch", specs[1].GetNEVRA())

}
