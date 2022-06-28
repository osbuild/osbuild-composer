package main

import (
	"os"

	"github.com/BurntSushi/toml"
	"github.com/sirupsen/logrus"
)

type workerConfig struct {
	Composer *struct {
		Proxy string `toml:"proxy"`
	} `toml:"composer"`
	Koji map[string]struct {
		Kerberos *struct {
			Principal string `toml:"principal"`
			KeyTab    string `toml:"keytab"`
		} `toml:"kerberos,omitempty"`
		RelaxTimeoutFactor uint `toml:"relax_timeout_factor"`
	} `toml:"koji"`
	GCP *struct {
		Credentials string `toml:"credentials"`
	} `toml:"gcp"`
	Azure *struct {
		Credentials string `toml:"credentials"`
	} `toml:"azure"`
	AWS *struct {
		Credentials string `toml:"credentials"`
		Bucket      string `toml:"bucket"`
	} `toml:"aws"`
	GenericS3 *struct {
		Credentials         string `toml:"credentials"`
		Endpoint            string `toml:"endpoint"`
		Region              string `toml:"region"`
		Bucket              string `toml:"bucket"`
		CABundle            string `toml:"ca_bundle"`
		SkipSSLVerification bool   `toml:"skip_ssl_verification"`
	} `toml:"generic_s3"`
	Authentication *struct {
		OAuthURL         string `toml:"oauth_url"`
		OfflineTokenPath string `toml:"offline_token"`
		ClientId         string `toml:"client_id"`
		ClientSecretPath string `toml:"client_secret"`
	} `toml:"authentication"`
	// default value: /api/worker/v1
	BasePath string `toml:"base_path"`
	DNFJson  string `toml:"dnf-json"`
}

func parseConfig(file string) (*workerConfig, error) {
	var config workerConfig
	_, err := toml.DecodeFile(file, &config)
	if err != nil {
		// Return error only when we failed to decode the file.
		// A non-existing config isn't an error, use defaults in this case.
		if !os.IsNotExist(err) {
			return nil, err
		}

		logrus.Info("Configuration file not found, using defaults")
	}
	if config.BasePath == "" {
		config.BasePath = "/api/worker/v1"
	}

	return &config, nil
}
