package main

import (
	"fmt"
	"io"
	"os"
	"reflect"

	"github.com/BurntSushi/toml"
)

type ComposerConfigFile struct {
	Koji struct {
		AllowedDomains []string `toml:"allowed_domains"`
		CA             string   `toml:"ca"`
	} `toml:"koji"`
	Worker struct {
		AllowedDomains []string `toml:"allowed_domains"`
		CA             string   `toml:"ca"`
		PGHost         string   `toml:"pg_host" env:"PGHOST"`
		PGPort         string   `toml:"pg_port" env:"PGPORT"`
		PGDatabase     string   `toml:"pg_database" env:"PGDATABASE"`
		PGUser         string   `toml:"pg_user" env:"PGUSER"`
		PGPassword     string   `toml:"pg_password" env:"PGPASSWORD"`
		PGSSLMode      string   `toml:"pg_ssl_mode" env:"PGSSLMODE"`
		EnableJWT      bool     `toml:"enable_jwt"`
		JWTKeysURL     string   `toml:"jwt_keys_url"`
		JWTKeysCA      string   `toml:"jwt_ca_file"`
		JWTACLFile     string   `toml:"jwt_acl_file"`
	} `toml:"worker"`
	ComposerAPI struct {
		EnableJWT  bool   `toml:"enable_jwt"`
		JWTKeysURL string `toml:"jwt_keys_url"`
		JWTKeysCA  string `toml:"jwt_ca_file"`
		JWTACLFile string `toml:"jwt_acl_file"`
	} `toml:"composer_api"`
	WeldrAPI WeldrAPIConfig `toml:"weldr_api"`
	logLevel string         `toml:"log_level"`
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
// - 'ec2' and 'ec2-ha' image types on 'rhel-85' are not exposed via Weldr API
func GetDefaultConfig() *ComposerConfigFile {
	return &ComposerConfigFile{
		WeldrAPI: WeldrAPIConfig{
			map[string]WeldrDistroConfig{
				"rhel-*": {
					ImageTypeDenyList: []string{
						"ec2",
						"ec2-ha",
					},
				},
			},
		},
	}
}

func LoadConfig(name string) (*ComposerConfigFile, error) {
	c := GetDefaultConfig()
	_, err := toml.DecodeFile(name, c)
	if err != nil {
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

func DumpConfig(c *ComposerConfigFile, w io.Writer) error {
	return toml.NewEncoder(w).Encode(c)
}
