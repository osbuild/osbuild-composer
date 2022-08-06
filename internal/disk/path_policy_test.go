package disk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathPolicyCheck(t *testing.T) {
	assert := assert.New(t)

	entires := map[string]PathPolicy{
		"/":          {Exact: true},
		"/boot":      {Exact: true},
		"/boot/efi":  {Exact: true},
		"/var":       {},
		"/var/empty": {Deny: true},
		"/srv":       {},
		"/home":      {},
	}

	policies := NewPathPolicies(entires)
	assert.NotNil(policies)

	tests := map[string]bool{
		"/":                true,
		"/custom":          false,
		"/boot":            true,
		"/boot/grub2":      false,
		"/boot/efi":        true,
		"/boot/efi/redora": false,
		"/srv":             true,
		"/srv/www":         true,
		"/srv/www/data":    true,
		"/var":             true,
		"/var/log":         true,
		"/var/empty":       false,
		"/var/empty/dir":   false,
	}

	for k, v := range tests {
		err := policies.Check(k)
		if v {
			assert.NoError(err)
		} else {
			assert.Errorf(err, "unexpected error for path '%s'", k)

		}
	}
}
