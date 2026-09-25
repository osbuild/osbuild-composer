package rpmmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadRepositoriesFromReaderSSLFields verifies that sslcacert, sslclientkey
// and sslclientcert are parsed from the on-disk repository JSON and propagated
// into RepoConfig.  This is a regression test: the repository struct previously
// had no SSL fields so they were silently dropped, causing DNF to fail against
// Satellite/mTLS repositories.
func TestLoadRepositoriesFromReaderSSLFields(t *testing.T) {
	repoJSON := `{
		"x86_64": [
			{
				"name": "Red Hat Enterprise Linux 9 for x86_64 - BaseOS (RPMs)",
				"baseurl": "https://satellite.example.com/rhel9/baseos",
				"check_gpg": true,
				"sslcacert": "/etc/rhsm/ca/katello-server-ca.pem",
				"sslclientkey": "/etc/pki/entitlement/1234-key.pem",
				"sslclientcert": "/etc/pki/entitlement/1234.pem",
				"metadata_expire": "1"
			}
		]
	}`

	repos, err := LoadRepositoriesFromReader(strings.NewReader(repoJSON))
	require.NoError(t, err)

	x86Repos, ok := repos["x86_64"]
	require.True(t, ok, "expected x86_64 repos")
	require.Len(t, x86Repos, 1)

	repo := x86Repos[0]
	assert.Equal(t, "/etc/rhsm/ca/katello-server-ca.pem", repo.SSLCACert,
		"sslcacert must be parsed from repository JSON")
	assert.Equal(t, "/etc/pki/entitlement/1234-key.pem", repo.SSLClientKey,
		"sslclientkey must be parsed from repository JSON")
	assert.Equal(t, "/etc/pki/entitlement/1234.pem", repo.SSLClientCert,
		"sslclientcert must be parsed from repository JSON")
}
