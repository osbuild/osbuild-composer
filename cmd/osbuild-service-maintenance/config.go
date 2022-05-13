package main

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
)

// Do not write this config to logs or stdout, it contains secrets!
type Config struct {
	DryRun                bool   `env:"DRY_RUN"`
	MaxConcurrentRequests int    `env:"MAX_CONCURRENT_REQUESTS"`
	EnableDBMaintenance   bool   `env:"ENABLE_DB_MAINTENANCE"`
	EnableGCPMaintenance  bool   `env:"ENABLE_GCP_MAINTENANCE"`
	EnableAWSMaintenance  bool   `env:"ENABLE_AWS_MAINTENANCE"`
	PGHost                string `env:"PGHOST"`
	PGPort                string `env:"PGPORT"`
	PGDatabase            string `env:"PGDATABASE"`
	PGUser                string `env:"PGUSER"`
	PGPassword            string `env:"PGPASSWORD"`
	PGSSLMode             string `env:"PGSSLMODE"`
	AWSAccessKeyID        string `env:"AWS_ACCESS_KEY_ID"`
	AWSSecretAccessKey    string `env:"AWS_SECRET_ACCESS_KEY"`
}

type GCPCredentialsConfig struct {
	AuthProviderX509CertUrl string `json:"auth_provider_x509_cert_url" env:"GCP_AUTH_PROVIDER_X509_CERT_URL"`
	AuthUri                 string `json:"auth_uril" env:"GCP_AUTH_URI"`
	ClientEmail             string `json:"client_email" env:"GCP_CLIENT_EMAIL"`
	ClientId                string `json:"client_id" env:"GCP_CLIENT_ID"`
	ClientX509CertUrl       string `json:"client_x509_cert_url" env:"GCP_CLIENT_X509_CERT_URL"`
	PrivateKey              string `json:"private_key" env:"GCP_PRIVATE_KEY"`
	PrivateKeyId            string `json:"private_key_id" env:"GCP_PRIVATE_KEY_ID"`
	ProjectId               string `json:"project_id" env:"GCP_PROJECT_ID"`
	TokenUri                string `json:"token_uri" env:"GCP_TOKEN_URI"`
	Type                    string `json:"type" env:"GCP_TYPE"`
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
			case reflect.String:
				fieldV.SetString(confV)
			case reflect.Int:
				value, err := strconv.ParseInt(confV, 10, 64)
				if err != nil {
					return err
				}
				fieldV.SetInt(value)
			case reflect.Bool:
				value, err := strconv.ParseBool(confV)
				if err != nil {
					return err
				}
				fieldV.SetBool(value)
			default:
				return fmt.Errorf("Unsupported type")
			}
		}
	}
	return nil
}

func (gc *GCPCredentialsConfig) valid() bool {
	if gc.AuthProviderX509CertUrl == "" {
		return false
	}
	if gc.AuthUri == "" {
		return false
	}
	if gc.ClientEmail == "" {
		return false
	}
	if gc.ClientId == "" {
		return false
	}
	if gc.ClientX509CertUrl == "" {
		return false
	}
	if gc.PrivateKey == "" {
		return false
	}
	if gc.PrivateKeyId == "" {
		return false
	}
	if gc.ProjectId == "" {
		return false
	}
	if gc.TokenUri == "" {
		return false
	}
	if gc.Type == "" {
		return false
	}
	return true
}
