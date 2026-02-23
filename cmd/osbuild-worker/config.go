package main

import (
	"fmt"
	"os"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/osbuild/images/pkg/cloud/azure"
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
	RelaxTimeoutFactor time.Duration   `toml:"relax_timeout_factor"`
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
	Credentials   string `toml:"credentials"`
	S3Credentials string `toml:"s3_credentials"`
	Bucket        string `toml:"bucket"`
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

type executorConfig struct {
	Type       string `toml:"type"`
	IAMProfile string `toml:"iam_profile"`
	KeyName    string `toml:"key_name"`
}

type repositoryMTLSConfig struct {
	BaseURL        string `toml:"baseurl"`
	CA             string `toml:"ca"`
	MTLSClientKey  string `toml:"mtls_client_key"`
	MTLSClientCert string `toml:"mtls_client_cert"`
	Proxy          string `toml:"proxy"`
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
	// default value: &{ Type: host }
	OSBuildExecutor      *executorConfig       `toml:"osbuild_executor"`
	RepositoryMTLSConfig *repositoryMTLSConfig `toml:"repository_mtls"`
	// something like "production" or "staging" to be added to logging
	DeploymentChannel string `toml:"deployment_channel"`
	// clean store between runs, this should only be used with workers running on AWS within an ASG
	CleanStore bool `toml:"clean_store"`
}

func parseConfig(file string) (*workerConfig, error) {
	// set defaults
	config := workerConfig{
		BasePath: "/api/worker/v1",
		OSBuildExecutor: &executorConfig{
			Type: "host",
		},
		DeploymentChannel: "local",
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

	switch config.OSBuildExecutor.Type {
	case "host", "aws.ec2", "qemu.kvm":
		// good and supported
	default:
		return nil, fmt.Errorf("OSBuildExecutor needs to be host, aws.ec2, or qemu.kvm. Got: %s.", config.OSBuildExecutor)
	}

	return &config, nil
}
