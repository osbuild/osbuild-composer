package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testDistro = "rhel-9"
	testArch   = "x86_64"
)

// writeRepoFile is a test helper that writes content to <dir>/<name>.
func writeRepoFile(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
}

func TestRepoRegistryFromYumReposD_BasicRepo(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "test.repo", `
[baseos]
name = BaseOS
baseurl = https://example.com/baseos
enabled = 1
gpgcheck = 1
gpgkey = file:///etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release
sslcacert = /etc/rhsm/ca/katello-server-ca.pem
sslclientkey = /etc/pki/entitlement/key.pem
sslclientcert = /etc/pki/entitlement/cert.pem
metadata_expire = 1
`)

	rr, err := repoRegistryFromYumReposDForDistroArch(dir, testDistro, testArch)
	require.NoError(t, err)
	require.NotNil(t, rr)

	require.Equal(t, []string{testDistro}, rr.ListDistros())

	repos, err := rr.ReposByArchName(testDistro, testArch, false)
	require.NoError(t, err)
	require.Len(t, repos, 1)
	assert.Equal(t, "BaseOS", repos[0].Name)
	assert.Equal(t, []string{"https://example.com/baseos"}, repos[0].BaseURLs)
	assert.Equal(t, []string{"file:///etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release"}, repos[0].GPGKeys)
	require.NotNil(t, repos[0].CheckGPG)
	assert.True(t, *repos[0].CheckGPG)
	assert.Equal(t, "/etc/rhsm/ca/katello-server-ca.pem", repos[0].SSLCACert)
	assert.Equal(t, "/etc/pki/entitlement/key.pem", repos[0].SSLClientKey)
	assert.Equal(t, "/etc/pki/entitlement/cert.pem", repos[0].SSLClientCert)
	assert.Equal(t, "1", repos[0].MetadataExpire)
}

func TestRepoRegistryFromYumReposD_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "baseos.repo", `
[baseos]
name = BaseOS
baseurl = https://example.com/baseos
enabled = 1
gpgcheck = 0
`)
	writeRepoFile(t, dir, "appstream.repo", `
[appstream]
name = AppStream
baseurl = https://example.com/appstream
enabled = 1
gpgcheck = 0
`)

	rr, err := repoRegistryFromYumReposDForDistroArch(dir, testDistro, testArch)
	require.NoError(t, err)
	require.NotNil(t, rr)

	repos, err := rr.ReposByArchName(testDistro, testArch, false)
	require.NoError(t, err)
	assert.Len(t, repos, 2)
}

func TestRepoRegistryFromYumReposD_MultipleReposInOneFile(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "redhat.repo", `
[rhel-baseos]
name = RHEL BaseOS
baseurl = https://satellite.example.com/baseos
enabled = 1
gpgcheck = 1
gpgkey = file:///etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release

[rhel-appstream]
name = RHEL AppStream
baseurl = https://satellite.example.com/appstream
enabled = 1
gpgcheck = 1
gpgkey = file:///etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release

[rhel-crb]
name = RHEL CRB
baseurl = https://satellite.example.com/crb
enabled = 0
gpgcheck = 1
`)

	rr, err := repoRegistryFromYumReposDForDistroArch(dir, testDistro, testArch)
	require.NoError(t, err)

	repos, err := rr.ReposByArchName(testDistro, testArch, false)
	require.NoError(t, err)
	// CRB is disabled, only 2 repos expected.
	require.Len(t, repos, 2)
	names := []string{repos[0].Name, repos[1].Name}
	assert.Contains(t, names, "RHEL BaseOS")
	assert.Contains(t, names, "RHEL AppStream")
}

func TestRepoRegistryFromYumReposD_SkipsDisabledRepos(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "mixed.repo", `
[enabled-repo]
name = Enabled
baseurl = https://example.com/enabled
enabled = 1
gpgcheck = 0

[disabled-repo]
name = Disabled
baseurl = https://example.com/disabled
enabled = 0
gpgcheck = 0
`)

	rr, err := repoRegistryFromYumReposDForDistroArch(dir, testDistro, testArch)
	require.NoError(t, err)
	require.NotNil(t, rr)

	repos, err := rr.ReposByArchName(testDistro, testArch, false)
	require.NoError(t, err)
	require.Len(t, repos, 1)
	assert.Equal(t, "Enabled", repos[0].Name)
}

func TestRepoRegistryFromYumReposD_EnabledDefaultsToTrue(t *testing.T) {
	dir := t.TempDir()
	// No "enabled" key — should default to included.
	writeRepoFile(t, dir, "implicit.repo", `
[implicit-enabled]
name = Implicit Enabled
baseurl = https://example.com/implicit
gpgcheck = 0
`)

	rr, err := repoRegistryFromYumReposDForDistroArch(dir, testDistro, testArch)
	require.NoError(t, err)

	repos, err := rr.ReposByArchName(testDistro, testArch, false)
	require.NoError(t, err)
	require.Len(t, repos, 1)
	assert.Equal(t, "Implicit Enabled", repos[0].Name)
}

func TestRepoRegistryFromYumReposD_SSLFieldsMapped(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "rhsm.repo", `
[myrepo]
name = My Repo
baseurl = https://satellite.example.com/content/os
enabled = 1
gpgcheck = 1
gpgkey = file:///etc/pki/rpm-gpg/RPM-GPG-KEY-redhat-release
sslverify = 0
sslcacert = /etc/rhsm/ca/katello-server-ca.pem
sslclientkey = /etc/pki/entitlement/1234-key.pem
sslclientcert = /etc/pki/entitlement/1234.pem
metadata_expire = 86400
`)

	rr, err := repoRegistryFromYumReposDForDistroArch(dir, testDistro, testArch)
	require.NoError(t, err)

	repos, err := rr.ReposByArchName(testDistro, testArch, false)
	require.NoError(t, err)
	require.Len(t, repos, 1)
	r := repos[0]
	assert.Equal(t, "/etc/rhsm/ca/katello-server-ca.pem", r.SSLCACert)
	assert.Equal(t, "/etc/pki/entitlement/1234-key.pem", r.SSLClientKey)
	assert.Equal(t, "/etc/pki/entitlement/1234.pem", r.SSLClientCert)
	require.NotNil(t, r.IgnoreSSL)
	assert.True(t, *r.IgnoreSSL) // sslverify=0 → IgnoreSSL=true
	assert.Equal(t, "86400", r.MetadataExpire)
}

func TestRepoRegistryFromYumReposD_IgnoresNonRepoFiles(t *testing.T) {
	dir := t.TempDir()
	// .txt file must not be parsed — only *.repo files are picked up.
	writeRepoFile(t, dir, "README.txt", "this is not a repo file")
	writeRepoFile(t, dir, "valid.repo", `
[baseos]
name = BaseOS
baseurl = https://example.com/baseos
enabled = 1
gpgcheck = 0
`)

	rr, err := repoRegistryFromYumReposDForDistroArch(dir, testDistro, testArch)
	require.NoError(t, err)
	require.NotNil(t, rr)

	repos, err := rr.ReposByArchName(testDistro, testArch, false)
	require.NoError(t, err)
	assert.Len(t, repos, 1)
}

func TestRepoRegistryFromYumReposD_AllDisabledReturnsError(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "all-disabled.repo", `
[repo-a]
name = Repo A
baseurl = https://example.com/a
enabled = 0

[repo-b]
name = Repo B
baseurl = https://example.com/b
enabled = 0
`)

	_, err := repoRegistryFromYumReposDForDistroArch(dir, testDistro, testArch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no enabled repositories found")
}

func TestRepoRegistryFromYumReposD_EmptyDirReturnsError(t *testing.T) {
	dir := t.TempDir()

	_, err := repoRegistryFromYumReposDForDistroArch(dir, testDistro, testArch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no enabled repositories found")
}

func TestRepoRegistryFromYumReposD_NonExistentDirReturnsError(t *testing.T) {
	_, err := repoRegistryFromYumReposDForDistroArch("/nonexistent/path/yum.repos.d", testDistro, testArch)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no enabled repositories found")
}

func TestRepoRegistryFromYumReposD_MultiValueGPGKeyAndBaseurl(t *testing.T) {
	dir := t.TempDir()
	// DNF allows multiple GPG keys and base URLs via backslash line continuation.
	// ini.v1 joins continuation lines with a space; strings.Fields then splits them.
	writeRepoFile(t, dir, "multi.repo", "[multi]\nname = Multi Key Repo\nbaseurl = https://mirror1.example.com/os \\\n https://mirror2.example.com/os\nenabled = 1\ngpgcheck = 1\ngpgkey = file:///etc/pki/rpm-gpg/RPM-GPG-KEY-A \\\n file:///etc/pki/rpm-gpg/RPM-GPG-KEY-B\n")

	rr, err := repoRegistryFromYumReposDForDistroArch(dir, testDistro, testArch)
	require.NoError(t, err)

	repos, err := rr.ReposByArchName(testDistro, testArch, false)
	require.NoError(t, err)
	require.Len(t, repos, 1)
	assert.Len(t, repos[0].BaseURLs, 2)
	assert.Len(t, repos[0].GPGKeys, 2)
}
