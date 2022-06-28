package main

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_parseConfig(t *testing.T) {
	tests := []struct {
		name   string
		config string
		want   *workerConfig
	}{
		{
			name: "basic",
			config: `
# comment
base_path = "/api/image-builder-worker/v1"
dnf-json = "/usr/libexec/dnf-json"

[composer]
proxy = "http://proxy.example.com"

[koji."kojihub.example.com"]
relax_timeout_factor = 5

[koji."kojihub.example.com".kerberos]
principal = "toucan-automation@EXAMPLE.COM"
keytab = "/etc/osbuild-worker/client.keytab"

[koji."kojihub.stage.example.com"]
relax_timeout_factor = 42

[koji."kojihub.stage.example.com".kerberos]
principal = "toucan-automation-stage@EXAMPLE.COM"
keytab = "/etc/osbuild-worker/client-stage.keytab"

[gcp]
credentials = "/etc/osbuild-worker/gcp-creds"

[azure]
credentials = "/etc/osbuild-worker/azure-creds"

[aws]
credentials = "/etc/osbuild-worker/aws-creds"
bucket = "buckethead"

[generic_s3]
credentials = "/etc/osbuild-worker/s3-creds"
endpoint = "http://s3.example.com"
region = "us-east-1"
bucket = "slash"
ca_bundle = "/etc/osbuild-worker/s3-ca-bundle"
skip_ssl_verification = true

[authentication]
oauth_url = "https://example.com/token"
client_id = "toucan"
client_secret = "/etc/osbuild-worker/client_secret"
offline_token = "/etc/osbuild-worker/offline_token"
`,
			want: &workerConfig{
				BasePath: "/api/image-builder-worker/v1",
				DNFJson:  "/usr/libexec/dnf-json",
				Composer: &composerConfig{
					Proxy: "http://proxy.example.com",
				},
				Koji: map[string]kojiServerConfig{
					"kojihub.example.com": {
						Kerberos: &kerberosConfig{
							Principal: "toucan-automation@EXAMPLE.COM",
							KeyTab:    "/etc/osbuild-worker/client.keytab",
						},
						RelaxTimeoutFactor: 5,
					},
					"kojihub.stage.example.com": {
						Kerberos: &kerberosConfig{
							Principal: "toucan-automation-stage@EXAMPLE.COM",
							KeyTab:    "/etc/osbuild-worker/client-stage.keytab",
						},
						RelaxTimeoutFactor: 42,
					},
				},
				GCP: &gcpConfig{
					Credentials: "/etc/osbuild-worker/gcp-creds",
				},
				Azure: &azureConfig{
					Credentials: "/etc/osbuild-worker/azure-creds",
				},
				AWS: &awsConfig{
					Credentials: "/etc/osbuild-worker/aws-creds",
					Bucket:      "buckethead",
				},
				GenericS3: &genericS3Config{
					Credentials:         "/etc/osbuild-worker/s3-creds",
					Endpoint:            "http://s3.example.com",
					Region:              "us-east-1",
					Bucket:              "slash",
					CABundle:            "/etc/osbuild-worker/s3-ca-bundle",
					SkipSSLVerification: true,
				},
				Authentication: &authenticationConfig{
					OAuthURL:         "https://example.com/token",
					OfflineTokenPath: "/etc/osbuild-worker/offline_token",
					ClientId:         "toucan",
					ClientSecretPath: "/etc/osbuild-worker/client_secret",
				},
			},
		},
		{
			name:   "default",
			config: ``,
			want: &workerConfig{
				BasePath: "/api/worker/v1",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configFile := prepareConfig(t, tt.config)
			got, err := parseConfig(configFile)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}

	t.Run("non-existing", func(t *testing.T) {
		got, err := parseConfig("/osbuild/b19b8798-5f76-4b09-9e56-5978df8f6cde")
		require.NoError(t, err)

		// check that the defaults are loaded
		require.Equal(t, tests[1].want, got)
	})

	t.Run("wrong config", func(t *testing.T) {
		configFile := prepareConfig(t, `[unclosed_section`)

		_, err := parseConfig(configFile)
		require.Error(t, err)
	})
}

func prepareConfig(t *testing.T, config string) string {
	dir := t.TempDir()
	configFile := path.Join(dir, "config.toml")
	f, err := os.Create(configFile)
	require.NoError(t, err)

	_, err = f.WriteString(config)
	require.NoError(t, err)

	require.NoError(t, f.Close())
	return configFile
}
