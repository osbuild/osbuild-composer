package common

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCurrentArchAMD64(t *testing.T) {
	origRuntimeGOARCH := RuntimeGOARCH
	defer func() { RuntimeGOARCH = origRuntimeGOARCH }()
	RuntimeGOARCH = "amd64"
	assert.Equal(t, "x86_64", CurrentArch())
}

func TestCurrentArchARM64(t *testing.T) {
	origRuntimeGOARCH := RuntimeGOARCH
	defer func() { RuntimeGOARCH = origRuntimeGOARCH }()
	RuntimeGOARCH = "arm64"
	assert.Equal(t, "aarch64", CurrentArch())
}

func TestCurrentArchPPC64LE(t *testing.T) {
	origRuntimeGOARCH := RuntimeGOARCH
	defer func() { RuntimeGOARCH = origRuntimeGOARCH }()
	RuntimeGOARCH = "ppc64le"
	assert.Equal(t, "ppc64le", CurrentArch())
}

func TestCurrentArchS390X(t *testing.T) {
	origRuntimeGOARCH := RuntimeGOARCH
	defer func() { RuntimeGOARCH = origRuntimeGOARCH }()
	RuntimeGOARCH = "s390x"
	assert.Equal(t, "s390x", CurrentArch())
}

func TestCurrentArchUnsupported(t *testing.T) {
	origRuntimeGOARCH := RuntimeGOARCH
	defer func() { RuntimeGOARCH = origRuntimeGOARCH }()
	RuntimeGOARCH = "UKNOWN"
	assert.PanicsWithValue(t, "unsupported architecture", func() { CurrentArch() })
}

func TestPanicOnError(t *testing.T) {
	err := errors.New("Error message")
	assert.PanicsWithValue(t, err, func() { PanicOnError(err) })
}

func TestIsStringInSortedSlice(t *testing.T) {
	assert.True(t, IsStringInSortedSlice([]string{"bart", "homer", "lisa", "marge"}, "homer"))
	assert.False(t, IsStringInSortedSlice([]string{"bart", "lisa", "marge"}, "homer"))
	assert.False(t, IsStringInSortedSlice([]string{"bart", "lisa", "marge"}, ""))
	assert.False(t, IsStringInSortedSlice([]string{}, "homer"))
}

func TestDataSizeToUint64(t *testing.T) {
	cases := []struct {
		input   string
		success bool
		output  uint64
	}{
		{"123", true, 123},
		{"123 kB", true, 123000},
		{"123 KiB", true, 123 * 1024},
		{"123 MB", true, 123 * 1000 * 1000},
		{"123 MiB", true, 123 * 1024 * 1024},
		{"123 GB", true, 123 * 1000 * 1000 * 1000},
		{"123 GiB", true, 123 * 1024 * 1024 * 1024},
		{"123 TB", true, 123 * 1000 * 1000 * 1000 * 1000},
		{"123 TiB", true, 123 * 1024 * 1024 * 1024 * 1024},
		{"123kB", true, 123000},
		{"123KiB", true, 123 * 1024},
		{" 123  ", true, 123},
		{"  123kB  ", true, 123000},
		{"  123KiB  ", true, 123 * 1024},
		{"123 KB", false, 0},
		{"123 mb", false, 0},
		{"123 PB", false, 0},
		{"123 PiB", false, 0},
	}

	for _, c := range cases {
		result, err := DataSizeToUint64(c.input)
		if c.success {
			require.Nil(t, err)
			assert.EqualValues(t, c.output, result)
		} else {
			assert.NotNil(t, err)
		}
	}
}
