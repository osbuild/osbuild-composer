package main

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
)

type ComposerConfigFile struct {
	Koji               KojiAPIConfig     `toml:"koji"`
	Worker             WorkerAPIConfig   `toml:"worker"`
	WeldrAPI           WeldrAPIConfig    `toml:"weldr_api"`
	DistroAliases      map[string]string `toml:"distro_aliases" env:"DISTRO_ALIASES"`
	LogLevel           string            `toml:"log_level"`
	LogFormat          string            `toml:"log_format"`
	DNFJson            string            `toml:"dnf-json"`
	IgnoreMissingRepos bool              `toml:"ignore_missing_repos"`
	SplunkHost         string            `env:"SPLUNK_HEC_HOST"`
	SplunkPort         string            `env:"SPLUNK_HEC_PORT"`
	SplunkToken        string            `env:"SPLUNK_HEC_TOKEN"`
	GlitchTipDSN       string            `env:"GLITCHTIP_DSN"`
	DeploymentChannel  string            `env:"CHANNEL"`
}

type KojiAPIConfig struct {
	AllowedDomains          []string `toml:"allowed_domains"`
	CA                      string   `toml:"ca"`
	EnableTLS               bool     `toml:"enable_tls"`
	EnableMTLS              bool     `toml:"enable_mtls"`
	EnableJWT               bool     `toml:"enable_jwt"`
	JWTKeysURLs             []string `toml:"jwt_keys_urls"`
	JWTKeysCA               string   `toml:"jwt_ca_file"`
	JWTACLFile              string   `toml:"jwt_acl_file"`
	JWTTenantProviderFields []string `toml:"jwt_tenant_provider_fields"`
}

type WorkerAPIConfig struct {
	AllowedDomains          []string `toml:"allowed_domains"`
	CA                      string   `toml:"ca"`
	RequestJobTimeout       string   `toml:"request_job_timeout"`
	BasePath                string   `toml:"base_path"`
	EnableArtifacts         bool     `toml:"enable_artifacts"`
	PGHost                  string   `toml:"pg_host" env:"PGHOST"`
	PGPort                  string   `toml:"pg_port" env:"PGPORT"`
	PGDatabase              string   `toml:"pg_database" env:"PGDATABASE"`
	PGUser                  string   `toml:"pg_user" env:"PGUSER"`
	PGPassword              string   `toml:"pg_password" env:"PGPASSWORD"`
	PGSSLMode               string   `toml:"pg_ssl_mode" env:"PGSSLMODE"`
	PGMaxConns              int      `toml:"pg_max_conns" env:"PGMAXCONNS"`
	EnableTLS               bool     `toml:"enable_tls"`
	EnableMTLS              bool     `toml:"enable_mtls"`
	EnableJWT               bool     `toml:"enable_jwt"`
	JWTKeysURLs             []string `toml:"jwt_keys_urls"`
	JWTKeysCA               string   `toml:"jwt_ca_file"`
	JWTACLFile              string   `toml:"jwt_acl_file"`
	JWTTenantProviderFields []string `toml:"jwt_tenant_provider_fields"`
	WorkerHeartbeatTimeout  string   `toml:"worker_heartbeat_timeout"`
}

type WeldrAPIConfig struct {
	DistroConfigs map[string]WeldrDistroConfig `toml:"distros"`
}

type WeldrDistroConfig struct {
	ImageTypeDenyList []string `toml:"image_type_denylist"`
}

// weldrDistrosImageTypeDenyList returns a map of distro-specific Image Type
// deny lists for Weldr API.
func (c *ComposerConfigFile) weldrDistrosImageTypeDenyList() map[string][]string {
	distrosImageTypeDenyList := map[string][]string{}

	for distro, distroConfig := range c.WeldrAPI.DistroConfigs {
		if distroConfig.ImageTypeDenyList != nil {
			distrosImageTypeDenyList[distro] = append([]string{}, distroConfig.ImageTypeDenyList...)
		}
	}

	return distrosImageTypeDenyList
}

// GetDefaultConfig returns the default configuration of osbuild-composer
// Defaults:
//   - 'azure-rhui', 'azure-sap-rhui', 'ec2', 'ec2-ha', 'ec2-sap' image types on 'rhel-*'
//     are not exposed via Weldr API
func GetDefaultConfig() *ComposerConfigFile {
	return &ComposerConfigFile{
		Koji: KojiAPIConfig{
			EnableTLS:  true,
			EnableMTLS: true,
			EnableJWT:  false,
		},
		Worker: WorkerAPIConfig{
			RequestJobTimeout:      "0",
			BasePath:               "/api/worker/v1",
			EnableArtifacts:        true,
			EnableTLS:              true,
			EnableMTLS:             true,
			EnableJWT:              false,
			WorkerHeartbeatTimeout: "1h",
		},
		WeldrAPI: WeldrAPIConfig{
			map[string]WeldrDistroConfig{
				"rhel-*": {
					ImageTypeDenyList: []string{
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
					ImageTypeDenyList: []string{
						"iot-bootable-container",
					},
				},
			},
		},
		DistroAliases: map[string]string{
			"rhel-7":  "rhel-7.9",
			"rhel-8":  "rhel-8.10",
			"rhel-9":  "rhel-9.6",
			"rhel-10": "rhel-10.0",
		},
		LogLevel:           "info",
		LogFormat:          "journal",
		DNFJson:            "/usr/libexec/osbuild-depsolve-dnf",
		IgnoreMissingRepos: false,
	}
}

func LoadConfig(name string) (*ComposerConfigFile, error) {
	c := GetDefaultConfig()
	_, err := toml.DecodeFile(name, c)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	err = loadConfigFromEnv(c)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// envStrToMap converts map string to map[string]string
func envStrToMap(s string) (map[string]string, error) {
	result := map[string]string{}
	if s == "" {
		return result, nil
	}

	parts := strings.Split(s, ",")
	for _, part := range parts {
		keyValue := strings.Split(part, "=")
		if len(keyValue) != 2 {
			return nil, fmt.Errorf("Invalid key-value pair in map string: %s", part)
		}
		result[keyValue[0]] = keyValue[1]
	}
	return result, nil
}

func loadConfigFromEnv(intf interface{}) error {
	t := reflect.TypeOf(intf).Elem()
	v := reflect.ValueOf(intf).Elem()

	for i := 0; i < v.NumField(); i++ {
		fieldT := t.Field(i)
		fieldV := v.Field(i)
		kind := fieldV.Kind()

		switch kind {
		case reflect.String:
			key, ok := fieldT.Tag.Lookup("env")
			if !ok {
				continue
			}
			confV, ok := os.LookupEnv(key)
			if !ok {
				continue
			}
			fieldV.SetString(confV)
		case reflect.Int:
			key, ok := fieldT.Tag.Lookup("env")
			if !ok {
				continue
			}
			confV, ok := os.LookupEnv(key)
			if !ok {
				continue
			}
			value, err := strconv.ParseInt(confV, 10, 64)
			if err != nil {
				return err
			}
			fieldV.SetInt(value)
		case reflect.Bool:
			// no-op
			continue
		case reflect.Slice:
			// no-op
			continue
		case reflect.Map:
			key, ok := fieldT.Tag.Lookup("env")
			if !ok {
				continue
			}
			// handle only map[string]string
			if fieldV.Type().Key().Kind() != reflect.String || fieldV.Type().Elem().Kind() != reflect.String {
				return fmt.Errorf("Unsupported map type for loading from ENV: %s", kind)
			}
			confV, ok := os.LookupEnv(key)
			if !ok {
				continue
			}
			value, err := envStrToMap(confV)
			if err != nil {
				return err
			}
			// Don't override the whole map, just update the keys that are present in the env.
			// This is consistent with how loading config from the file works.
			for k, v := range value {
				fieldV.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(v))
			}
		case reflect.Struct:
			err := loadConfigFromEnv(fieldV.Addr().Interface())
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("Unsupported type: %s", kind)
		}
	}
	return nil
}

func DumpConfig(c ComposerConfigFile, w io.Writer) error {
	// sensor sensitive fields
	c.Worker.PGPassword = ""
	return toml.NewEncoder(w).Encode(c)
}
