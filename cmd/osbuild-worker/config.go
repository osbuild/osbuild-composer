package main

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
	"github.com/osbuild/osbuild-composer/internal/upload/azure"
	"github.com/sirupsen/logrus"
)

type composerConfig struct {
	Proxy string `toml:"proxy"`
}

type kerberosConfig struct {
	Principal string `toml:"principal"`
	KeyTab    string `toml:"keytab"`
}

type kojiServerConfig struct {
	Kerberos           *kerberosConfig `toml:"kerberos,omitempty"`
	RelaxTimeoutFactor uint            `toml:"relax_timeout_factor"`
}

type gcpConfig struct {
	Credentials string `toml:"credentials"`
	Bucket      string `toml:"bucket"`
}

type azureConfig struct {
	Credentials   string `toml:"credentials"`
	UploadThreads int    `toml:"upload_threads"`
}

type awsConfig struct {
	Credentials string `toml:"credentials"`
	Bucket      string `toml:"bucket"`
}

type ociConfig struct {
	Credentials string `toml:"credentials"`
}

type genericS3Config struct {
	Credentials         string `toml:"credentials"`
	Endpoint            string `toml:"endpoint"`
	Region              string `toml:"region"`
	Bucket              string `toml:"bucket"`
	CABundle            string `toml:"ca_bundle"`
	SkipSSLVerification bool   `toml:"skip_ssl_verification"`
}

type authenticationConfig struct {
	OAuthURL         string `toml:"oauth_url"`
	OfflineTokenPath string `toml:"offline_token"`
	ClientId         string `toml:"client_id"`
	ClientSecretPath string `toml:"client_secret"`
}

type containersConfig struct {
	AuthFilePath string `toml:"auth_file_path"`
	Domain       string `toml:"domain"`
	PathPrefix   string `toml:"path_prefix"`
	CertPath     string `toml:"cert_path"`
	TLSVerify    bool   `toml:"tls_verify"`
}

type workerConfig struct {
	Composer       *composerConfig             `toml:"composer"`
	Koji           map[string]kojiServerConfig `toml:"koji"`
	GCP            *gcpConfig                  `toml:"gcp"`
	Azure          *azureConfig                `toml:"azure"`
	AWS            *awsConfig                  `toml:"aws"`
	GenericS3      *genericS3Config            `toml:"generic_s3"`
	Authentication *authenticationConfig       `toml:"authentication"`
	Containers     *containersConfig           `toml:"containers"`
	OCI            *ociConfig                  `toml:"oci"`
	// default value: /api/worker/v1
	BasePath string `toml:"base_path"`
	DNFJson  string `toml:"dnf-json"`
}

func parseConfig(file string) (*workerConfig, error) {
	// set defaults
	config := workerConfig{
		BasePath: "/api/worker/v1",
	}

	_, err := toml.DecodeFile(file, &config)
	if err != nil {
		// Return error only when we failed to decode the file.
		// A non-existing config isn't an error, use defaults in this case.
		if !os.IsNotExist(err) {
			return nil, err
		}

		logrus.Info("Configuration file not found, using defaults")
	}

	// set defaults for Azure only if the config section is present
	if config.Azure != nil {
		if config.Azure.UploadThreads == 0 {
			config.Azure.UploadThreads = azure.DefaultUploadThreads
		} else if config.Azure.UploadThreads < 0 {
			return nil, fmt.Errorf("invalid number of Azure upload threads: %d", config.Azure.UploadThreads)
		}
	}

	return &config, nil
}
