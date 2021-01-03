package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmpty(t *testing.T) {
	config, err := LoadConfig("testdata/empty-config.toml")
	require.NoError(t, err)
	require.NotNil(t, config)
	require.Empty(t, config.Koji.AllowedDomains)
	require.Empty(t, config.Koji.CA)
	require.Empty(t, config.Worker.AllowedDomains)
	require.Empty(t, config.Worker.CA)
	require.Empty(t, config.Worker.PGDatabase)
}

func TestNonExisting(t *testing.T) {
	config, err := LoadConfig("testdata/non-existing-config.toml")
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
	require.Nil(t, config)
}

func TestConfig(t *testing.T) {
	config, err := LoadConfig("testdata/test.toml")
	require.NoError(t, err)
	require.NotNil(t, config)

	require.Equal(t, config.Koji.AllowedDomains, []string{"osbuild.org"})
	require.Equal(t, config.Koji.CA, "/etc/osbuild-composer/ca-crt.pem")

	require.Equal(t, config.Worker.AllowedDomains, []string{"osbuild.org"})
	require.Equal(t, config.Worker.CA, "/etc/osbuild-composer/ca-crt.pem")

	require.Equal(t, "overwrite-me-db", config.Worker.PGDatabase)

	require.NoError(t, os.Setenv("PGDATABASE", "composer-db"))
	config, err = LoadConfig("testdata/test.toml")
	require.NoError(t, err)
	require.NotNil(t, config)
	require.Equal(t, "composer-db", config.Worker.PGDatabase)
}
