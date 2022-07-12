package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewFDOStageForRootCerts(t *testing.T) {

	assert := assert.New(t)

	tests := []struct {
		data string
		hash string
	}{
		{"42\n", "sha256:084c799cd551dd1d8d5c5f9a5d593b2e931f5e36122ee5c793c1d08a19839cc0"},
		{"Hallo Welt\n", "sha256:f950375066d74787f31cbd8f9f91c71819357cad243fb9d4a0d9ef4fa76709e0"},
	}

	for _, tt := range tests {
		stage := NewFDOStageForRootCerts(tt.data)

		inputs := stage.Inputs.(*FDOStageInputs)
		certs := inputs.RootCerts

		assert.Len(certs.References, 1)
		assert.Equal(certs.References[0], tt.hash)

	}
}
