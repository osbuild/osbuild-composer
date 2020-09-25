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
	require.Empty(t, config.Koji.Servers)
	require.Empty(t, config.Koji.AllowedDomains)
	require.Empty(t, config.Koji.CA)
	require.Empty(t, config.Worker.AllowedDomains)
	require.Empty(t, config.Worker.CA)
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

	server, ok := config.Koji.Servers["example.com"]
	require.True(t, ok)
	require.NotNil(t, server.Kerberos)
	require.Equal(t, server.Kerberos.Principal, "example@osbuild.org")
	require.Equal(t, server.Kerberos.KeyTab, "/etc/osbuild-composer/osbuild.keytab")

	require.Equal(t, config.Koji.AllowedDomains, []string{"osbuild.org"})
	require.Equal(t, config.Koji.CA, "/etc/osbuild-composer/ca-crt.pem")

	require.Equal(t, config.Worker.AllowedDomains, []string{"osbuild.org"})
	require.Equal(t, config.Worker.CA, "/etc/osbuild-composer/ca-crt.pem")
}
