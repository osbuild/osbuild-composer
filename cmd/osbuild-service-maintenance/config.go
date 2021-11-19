package main

import (
	"fmt"
	"os"
	"reflect"
)

// Do not write this config to logs or stdout, it contains secrets!
type Config struct {
	DryRun                 string `env:"DRY_RUN"`
	MaxConcurrentRequests  string `env:"MAX_CONCURRENT_REQUESTS"`
	PGHost                 string `env:"PGHOST"`
	PGPort                 string `env:"PGPORT"`
	PGDatabase             string `env:"PGDATABASE"`
	PGUser                 string `env:"PGUSER"`
	PGPassword             string `env:"PGPASSWORD"`
	PGSSLMode              string `env:"PGSSLMODE"`
	GoogleApplicationCreds string `env:"GOOGLE_APPLICATION_CREDENTIALS"`
	AWSAccessKeyID         string `env:"AWS_ACCESS_KEY_ID"`
	AWSSecretAccessKey     string `env:"AWS_SECRET_ACCESS_KEY"`
}

// *string means the value is not required
// string means the value is required and should have a default value
func LoadConfigFromEnv(intf interface{}) error {
	t := reflect.TypeOf(intf).Elem()
	v := reflect.ValueOf(intf).Elem()

	for i := 0; i < v.NumField(); i++ {
		fieldT := t.Field(i)
		fieldV := v.Field(i)
		key, ok := fieldT.Tag.Lookup("env")
		if !ok {
			return fmt.Errorf("No env tag in config field")
		}

		confV, ok := os.LookupEnv(key)
		kind := fieldV.Kind()
		if ok {
			switch kind {
			case reflect.Ptr:
				if fieldT.Type.Elem().Kind() != reflect.String {
					return fmt.Errorf("Unsupported type")
				}
				fieldV.Set(reflect.ValueOf(&confV))
			case reflect.String:
				fieldV.SetString(confV)
			default:
				return fmt.Errorf("Unsupported type")
			}
		}
	}
	return nil
}
