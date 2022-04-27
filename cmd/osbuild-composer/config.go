package main

import (
	"fmt"
	"io"
	"os"
	"reflect"
	"strconv"

	"github.com/BurntSushi/toml"
)

type ComposerConfigFile struct {
	Koji         KojiAPIConfig   `toml:"koji"`
	Worker       WorkerAPIConfig `toml:"worker"`
	WeldrAPI     WeldrAPIConfig  `toml:"weldr_api"`
	SyslogServer string          `toml:"syslog_server" env:"SYSLOG_SERVER"`
	LogLevel     string          `toml:"log_level"`
	LogFormat    string          `toml:"log_format"`
	DNFJson      string          `toml:"dnf-json"`
}

type KojiAPIConfig struct {
	AllowedDomains          []string  `toml:"allowed_domains"`
	CA                      string    `toml:"ca"`
	EnableTLS               bool      `toml:"enable_tls"`
	EnableMTLS              bool      `toml:"enable_mtls"`
	EnableJWT               bool      `toml:"enable_jwt"`
	JWTKeysURLs             []string  `toml:"jwt_keys_urls"`
	JWTKeysCA               string    `toml:"jwt_ca_file"`
	JWTACLFile              string    `toml:"jwt_acl_file"`
	JWTTenantProviderFields []string  `toml:"jwt_tenant_provider_fields"`
	AWS                     AWSConfig `toml:"aws_config"`
}

type AWSConfig struct {
	Bucket string `toml:"bucket"`
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
// - 'azure-rhui', 'ec2', 'ec2-ha', 'ec2-sap' image types on 'rhel-85' are not exposed via Weldr API
func GetDefaultConfig() *ComposerConfigFile {
	return &ComposerConfigFile{
		Koji: KojiAPIConfig{
			EnableTLS:  true,
			EnableMTLS: true,
			EnableJWT:  false,
			AWS: AWSConfig{
				Bucket: "image-builder.service",
			},
		},
		Worker: WorkerAPIConfig{
			RequestJobTimeout: "0",
			BasePath:          "/api/worker/v1",
			EnableArtifacts:   true,
			EnableTLS:         true,
			EnableMTLS:        true,
			EnableJWT:         false,
		},
		WeldrAPI: WeldrAPIConfig{
			map[string]WeldrDistroConfig{
				"rhel-*": {
					ImageTypeDenyList: []string{
						"azure-rhui",
						"ec2",
						"ec2-ha",
						"ec2-sap",
						"gce-rhui",
					},
				},
			},
		},
		LogLevel:  "info",
		LogFormat: "text",
		DNFJson:   "/usr/libexec/osbuild-composer/dnf-json",
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
			// no-op
			continue
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
