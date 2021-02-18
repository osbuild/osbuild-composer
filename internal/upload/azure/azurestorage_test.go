package azure

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomStorageAccountName(t *testing.T) {
	randomName := RandomStorageAccountName("ib")

	assert.Len(t, randomName, 24)

	r := regexp.MustCompile(`^[\d\w]{24}$`)
	assert.True(t, r.MatchString(randomName), "the returned name should be 24 characters long and contain only alphanumerical characters")
}
