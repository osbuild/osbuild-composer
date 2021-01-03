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
		IdentityFilter []string `toml:"identity_filter"`
		PGHost         string   `toml:"pg_host" env:"PGHOST"`
		PGPort         string   `toml:"pg_port" env:"PGPORT"`
		PGDatabase     string   `toml:"pg_database" env:"PGDATABASE"`
		PGUser         string   `toml:"pg_user" env:"PGUSER"`
		PGPassword     string   `toml:"pg_password" env:"PGPASSWORD"`
		PGSSLMode      string   `toml:"pg_ssl_mode" env:"PGSSLMODE"`
	} `toml:"worker"`
	ComposerAPI struct {
		IdentityFilter []string `toml:"identity_filter"`
	} `toml:"composer_api"`
}

func LoadConfig(name string) (*ComposerConfigFile, error) {
	var c ComposerConfigFile
	_, err := toml.DecodeFile(name, &c)
	if err != nil {
		return nil, err
	}
	err = loadConfigFromEnv(&c)
	if err != nil {
		return nil, err
	}
	return &c, nil
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
		case reflect.Slice:
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
