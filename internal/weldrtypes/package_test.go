package weldrtypes

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var packageList = []DepsolvedPackageInfo{
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

func TestDepsolvedPackageInfoGetEVRA(t *testing.T) {
	require.Equal(t, "3.3a-3.fc38.x86_64", packageList[0].EVRA())
	require.Equal(t, "1:2.06-94.fc38.noarch", packageList[1].EVRA())
}

func TestDepsolvedPackageInfoGetNEVRA(t *testing.T) {
	require.Equal(t, "tmux-3.3a-3.fc38.x86_64", packageList[0].NEVRA())
	require.Equal(t, "grub2-1:2.06-94.fc38.noarch", packageList[1].NEVRA())
}
