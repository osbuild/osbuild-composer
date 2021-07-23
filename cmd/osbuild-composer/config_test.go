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
	require.Equal(t, GetDefaultConfig(), config)
}

func TestNonExisting(t *testing.T) {
	config, err := LoadConfig("testdata/non-existing-config.toml")
	require.Error(t, err)
	require.True(t, os.IsNotExist(err))
	require.Nil(t, config)
}

func TestDefaultConfig(t *testing.T) {
	defaultConfig := GetDefaultConfig()
	require.Empty(t, defaultConfig.Koji)
	require.Empty(t, defaultConfig.Worker)
	require.False(t, defaultConfig.ComposerAPI.EnableJWT)
	require.Equal(t, "", defaultConfig.ComposerAPI.JWTKeysCA)
	require.False(t, defaultConfig.Worker.EnableJWT)
	require.Equal(t, "", defaultConfig.Worker.JWTKeysCA)

	expectedWeldrAPIConfig := WeldrAPIConfig{
		DistroConfigs: map[string]WeldrDistroConfig{
			"rhel-*": {
				[]string{
					"ec2",
					"ec2-ha",
				},
			},
		},
	}

	require.Equal(t, expectedWeldrAPIConfig, defaultConfig.WeldrAPI)
}

func TestConfig(t *testing.T) {
	config, err := LoadConfig("testdata/test.toml")
	require.NoError(t, err)
	require.NotNil(t, config)

	require.Equal(t, config.Koji.AllowedDomains, []string{"osbuild.org"})
	require.Equal(t, config.Koji.CA, "/etc/osbuild-composer/ca-crt.pem")

	require.Equal(t, config.Worker.AllowedDomains, []string{"osbuild.org"})
	require.Equal(t, config.Worker.CA, "/etc/osbuild-composer/ca-crt.pem")

	require.Equal(t, []string{"qcow2", "vmdk"}, config.WeldrAPI.DistroConfigs["*"].ImageTypeDenyList)
	require.Equal(t, []string{"qcow2"}, config.WeldrAPI.DistroConfigs["rhel-84"].ImageTypeDenyList)

	require.Equal(t, "overwrite-me-db", config.Worker.PGDatabase)

	require.NoError(t, os.Setenv("PGDATABASE", "composer-db"))
	config, err = LoadConfig("testdata/test.toml")
	require.NoError(t, err)
	require.NotNil(t, config)
	require.Equal(t, "composer-db", config.Worker.PGDatabase)

	require.True(t, config.ComposerAPI.EnableJWT)
	require.Equal(t, "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/certs", config.ComposerAPI.JWTKeysURL)
	require.Equal(t, "", config.ComposerAPI.JWTKeysCA)
	require.Equal(t, "/var/lib/osbuild-composer/acl", config.ComposerAPI.JWTACLFile)
}

func TestWeldrDistrosImageTypeDenyList(t *testing.T) {
	config, err := LoadConfig("testdata/test.toml")
	require.NoError(t, err)
	require.NotNil(t, config)

	expectedWeldrDistrosImageTypeDenyList := map[string][]string{
		"*":       {"qcow2", "vmdk"},
		"rhel-84": {"qcow2"},
	}

	require.Equal(t, expectedWeldrDistrosImageTypeDenyList, config.weldrDistrosImageTypeDenyList())
}
