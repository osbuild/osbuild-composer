package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmpty(t *testing.T) {
	config, err := LoadConfig("testdata/empty-config.toml")
	require.NoError(t, err)
	require.Nil(t, config.Koji)
	require.Nil(t, config.Worker)
}

func TestNonExisting(t *testing.T) {
	config, err := LoadConfig("testdata/non-existing-config.toml")
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
	require.Nil(t, config)
}
