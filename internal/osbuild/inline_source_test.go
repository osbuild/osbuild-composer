package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInlineSource(t *testing.T) {

	assert := assert.New(t)

	tests := []struct {
		data    string
		hash    string
		encoded string
	}{
		{"42\n", "sha256:084c799cd551dd1d8d5c5f9a5d593b2e931f5e36122ee5c793c1d08a19839cc0", "NDIK"},
		{"Hallo Welt\n", "sha256:f950375066d74787f31cbd8f9f91c71819357cad243fb9d4a0d9ef4fa76709e0", "SGFsbG8gV2VsdAo="},
	}

	ils := NewInlineSource()

	for _, tt := range tests {
		hash := ils.AddItem(tt.data)
		assert.Equal(tt.hash, hash)

		item := ils.Items[hash]
		assert.Equal(item.Data, tt.encoded)
	}

}
