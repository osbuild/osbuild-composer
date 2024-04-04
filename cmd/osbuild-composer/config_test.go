package main

import (
	"bytes"
	"os"
	"strings"
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
	require.Nil(t, err)
	require.Equal(t, config, GetDefaultConfig())
}

func TestDefaultConfig(t *testing.T) {
	defaultConfig := GetDefaultConfig()

	require.False(t, defaultConfig.Koji.EnableJWT)
	require.Equal(t, "", defaultConfig.Koji.JWTKeysCA)
	require.False(t, defaultConfig.Worker.EnableJWT)
	require.Equal(t, "", defaultConfig.Worker.JWTKeysCA)

	require.Equal(t, KojiAPIConfig{
		EnableTLS:  true,
		EnableMTLS: true,
		EnableJWT:  false,
	}, defaultConfig.Koji)

	require.Equal(t, WorkerAPIConfig{
		RequestJobTimeout:      "0",
		BasePath:               "/api/worker/v1",
		EnableArtifacts:        true,
		EnableTLS:              true,
		EnableMTLS:             true,
		EnableJWT:              false,
		WorkerHeartbeatTimeout: "1h",
	}, defaultConfig.Worker)

	expectedWeldrAPIConfig := WeldrAPIConfig{
		DistroConfigs: map[string]WeldrDistroConfig{
			"rhel-*": {
				[]string{
					"azure-eap7-rhui",
					"azure-rhui",
					"azure-sap-rhui",
					"ec2",
					"ec2-ha",
					"ec2-sap",
					"gce-rhui",
				},
			},
			"fedora-*": {
				[]string{
					"iot-bootable-container",
				},
			},
		},
	}

	require.Equal(t, expectedWeldrAPIConfig, defaultConfig.WeldrAPI)

	expectedDistroAliases := map[string]string{
		"rhel-10": "rhel-10.0",
		"rhel-7":  "rhel-7.9",
		"rhel-8":  "rhel-8.10",
		"rhel-9":  "rhel-9.5",
	}
	require.Equal(t, expectedDistroAliases, defaultConfig.DistroAliases)

	require.Equal(t, "journal", defaultConfig.LogFormat)
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

	require.False(t, config.Koji.EnableJWT)
	require.Equal(t, []string{"https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/certs"}, config.Koji.JWTKeysURLs)
	require.Equal(t, "", config.Koji.JWTKeysCA)
	require.Equal(t, "/var/lib/osbuild-composer/acl", config.Koji.JWTACLFile)

	// 'rhel-8' and 'rhel-9' aliases are overwritten by the config file
	expectedDistroAliases := map[string]string{
		"rhel-10": "rhel-10.0", // this value is from the default config
		"rhel-7":  "rhel-7.9",  // this value is from the default config
		"rhel-8":  "rhel-8.9",
		"rhel-9":  "rhel-9.3",
	}
	require.Equal(t, expectedDistroAliases, config.DistroAliases)
}

func TestWeldrDistrosImageTypeDenyList(t *testing.T) {
	config, err := LoadConfig("testdata/test.toml")
	require.NoError(t, err)
	require.NotNil(t, config)

	// rhel config is overridden, but fedora is unaffected
	expectedWeldrDistrosImageTypeDenyList := map[string][]string{
		"*":        {"qcow2", "vmdk"},
		"fedora-*": {"iot-bootable-container"},
		"rhel-84":  {"qcow2"},
	}

	require.Equal(t, expectedWeldrDistrosImageTypeDenyList, config.weldrDistrosImageTypeDenyList())
}

func TestDumpConfig(t *testing.T) {
	config := &ComposerConfigFile{
		Worker: WorkerAPIConfig{
			PGPassword: "sensitive",
		},
	}

	var buf bytes.Buffer
	require.NoError(t, DumpConfig(*config, &buf))
	require.Contains(t, buf.String(), "pg_password = \"\"")
	require.NotContains(t, buf.String(), "sensitive")
	// DumpConfig takes a copy
	require.Equal(t, "sensitive", config.Worker.PGPassword)
}

func TestEnvStrToMap(t *testing.T) {
	env := "key1=value1,key2=value2,key3=value3"
	expected := map[string]string{
		"key1": "value1",
		"key2": "value2",
		"key3": "value3",
	}
	got, err := envStrToMap(env)
	require.NoError(t, err)
	require.Equal(t, expected, got)
}

func TestConfigFromEnv(t *testing.T) {
	// simulate the environment variables
	currentEnv := os.Environ()
	defer func() {
		os.Clearenv()
		for _, env := range currentEnv {
			pair := strings.SplitN(env, "=", 2)
			_ = os.Setenv(pair[0], pair[1])
		}
	}()

	os.Setenv("DISTRO_ALIASES", "rhel-7=rhel-7.9,rhel-8=rhel-8.9,rhel-9=rhel-9.3,rhel-10.0=rhel-9.5")
	expectedDistroAliases := map[string]string{
		"rhel-7":    "rhel-7.9",
		"rhel-8":    "rhel-8.9",
		"rhel-9":    "rhel-9.3",
		"rhel-10.0": "rhel-9.5",
	}

	config, err := LoadConfig("")
	require.NoError(t, err)
	require.Equal(t, expectedDistroAliases, config.DistroAliases)
}
