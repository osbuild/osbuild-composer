package common

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"testing"
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
