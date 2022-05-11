package azure

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandomStorageAccountName(t *testing.T) {
	randomName := RandomStorageAccountName("ib")

	assert.Len(t, randomName, 24)

	r := regexp.MustCompile(`^[\d\w]{24}$`)
	assert.True(t, r.MatchString(randomName), "the returned name should be 24 characters long and contain only alphanumerical characters")
}

func TestEnsureVHDExtension(t *testing.T) {
	tests := []struct {
		s    string
		want string
	}{
		{s: "toucan.zip", want: "toucan.zip.vhd"},
		{s: "kingfisher.vhd", want: "kingfisher.vhd"},
	}
	for _, tt := range tests {
		t.Run(tt.s, func(t *testing.T) {
			require.Equal(t, tt.want, EnsureVHDExtension(tt.s))
		})
	}
}
