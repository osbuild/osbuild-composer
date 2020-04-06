// Package client - source_test contains functions to check the source API
// Copyright (C) 2020 by Red Hat, Inc.

// Tests should be self-contained and not depend on the state of the server
// They should use their own blueprints, not the default blueprints
// They should not assume version numbers for packages will match
// They should run tests that depend on previous results from the same function
// not from other functions.
package client

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// POST a new JSON source
func TestPOSTJSONSourceV0(t *testing.T) {
	source := `{
		"name": "package-repo-json-v0",
		"url": "file://REPO-PATH",
		"type": "yum-baseurl",
		"proxy": "https://proxy-url/",
		"check_ssl": true,
		"check_gpg": true,
		"gpgkey_urls": ["https://url/path/to/gpg-key"]
	}`
	source = strings.Replace(source, "REPO-PATH", testState.repoDir, 1)

	resp, err := PostJSONSourceV0(testState.socket, source)
	require.NoError(t, err, "POST source failed with a client error")
	require.True(t, resp.Status, "POST source failed: %#v", resp)

	resp, err = DeleteSourceV0(testState.socket, "package-repo-json-v0")
	require.NoError(t, err, "DELETE source failed with a client error")
	require.True(t, resp.Status, "DELETE source failed: %#v", resp)
}

// POST an empty JSON source
func TestPOSTEmptyJSONSourceV0(t *testing.T) {
	resp, err := PostJSONSourceV0(testState.socket, "")
	require.NoError(t, err, "POST source failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// POST an invalid JSON source
func TestPOSTInvalidJSONSourceV0(t *testing.T) {
	// Missing quote in url
	source := `{
		"name": "package-repo-json-v0",
		"url": "file://REPO-PATH,
		"type": "yum-baseurl",
		"proxy": "https://proxy-url/",
		"check_ssl": true,
		"check_gpg": true,
		"gpgkey_urls": ["https://url/path/to/gpg-key"]
	}`

	resp, err := PostJSONSourceV0(testState.socket, source)
	require.NoError(t, err, "POST source failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// POST a new TOML source
func TestPOSTTOMLSourceV0(t *testing.T) {
	source := `
		name = "package-repo-toml-v0"
		url = "file://REPO-PATH"
		type = "yum-baseurl"
		proxy = "https://proxy-url/"
		check_ssl = true
		check_gpg = true
		gpgkey_urls = ["https://url/path/to/gpg-key"]
	`
	source = strings.Replace(source, "REPO-PATH", testState.repoDir, 1)

	resp, err := PostTOMLSourceV0(testState.socket, source)
	require.NoError(t, err, "POST source failed with a client error")
	require.True(t, resp.Status, "POST source failed: %#v", resp)

	resp, err = DeleteSourceV0(testState.socket, "package-repo-toml-v0")
	require.NoError(t, err, "DELETE source failed with a client error")
	require.True(t, resp.Status, "DELETE source failed: %#v", resp)
}

// POST an empty TOML source
func TestPOSTEmptyTOMLSourceV0(t *testing.T) {
	resp, err := PostTOMLSourceV0(testState.socket, "")
	require.NoError(t, err, "POST source failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// POST an invalid TOML source
func TestPOSTInvalidTOMLSourceV0(t *testing.T) {
	// Missing quote in url
	source := `
		name = "package-repo-toml-v0"
		url = "file://REPO-PATH
		type = "yum-baseurl"
		proxy = "https://proxy-url/"
		check_ssl = true
		check_gpg = true
		gpgkey_urls = ["https://url/path/to/gpg-key"]
	`

	resp, err := PostTOMLSourceV0(testState.socket, source)
	require.NoError(t, err, "POST source failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// POST a wrong TOML source
func TestPOSTWrongTOMLSourceV0(t *testing.T) {
	// Should not have a [] section
	source := `
		[package-repo-toml-v0]
		name = "package-repo-toml-v0"
		url = "file://REPO-PATH
		type = "yum-baseurl"
		proxy = "https://proxy-url/"
		check_ssl = true
		check_gpg = true
		gpgkey_urls = ["https://url/path/to/gpg-key"]
	`

	resp, err := PostTOMLSourceV0(testState.socket, source)
	require.NoError(t, err, "POST source failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// list sources
func TestListSourcesV0(t *testing.T) {
	sources := []string{`{
		"name": "package-repo-1",
		"url": "file://REPO-PATH",
		"type": "yum-baseurl",
		"proxy": "https://proxy-url/",
		"check_ssl": true,
		"check_gpg": true,
		"gpgkey_urls": ["https://url/path/to/gpg-key"]
	}`,
		`{
		"name": "package-repo-2",
		"url": "file://REPO-PATH",
		"type": "yum-baseurl",
		"proxy": "https://proxy-url/",
		"check_ssl": true,
		"check_gpg": true,
		"gpgkey_urls": ["https://url/path/to/gpg-key"]
	}`}

	for i := range sources {
		source := strings.Replace(sources[i], "REPO-PATH", testState.repoDir, 1)
		resp, err := PostJSONSourceV0(testState.socket, source)
		require.NoError(t, err, "POST source failed with a client error")
		require.True(t, resp.Status, "POST source failed: %#v", resp)
	}

	// Remove the test sources, ignoring any errors
	defer func() {
		for _, n := range []string{"package-repo-1", "package-repo-2"} {
			resp, err := DeleteSourceV0(testState.socket, n)
			require.NoError(t, err, "DELETE source failed with a client error")
			require.True(t, resp.Status, "DELETE source failed: %#v", resp)
		}
	}()

	// Get the list of sources
	list, api, err := ListSourcesV0(testState.socket)
	require.NoError(t, err, "GET source failed with a client error")
	require.Nil(t, api, "ListSources failed: %#v", api)
	require.True(t, len(list) > 1, "Not enough sources returned")
	require.Contains(t, list, "package-repo-1")
	require.Contains(t, list, "package-repo-2")
}

// Get the source info
func TestGetSourceInfoV0(t *testing.T) {
	source := `
		name = "package-repo-info-v0"
		url = "file://REPO-PATH"
		type = "yum-baseurl"
		proxy = "https://proxy-url/"
		check_ssl = true
		check_gpg = true
		gpgkey_urls = ["https://url/path/to/gpg-key"]
	`
	source = strings.Replace(source, "REPO-PATH", testState.repoDir, 1)

	resp, err := PostTOMLSourceV0(testState.socket, source)
	require.NoError(t, err, "POST source failed with a client error")
	require.True(t, resp.Status, "POST source failed: %#v", resp)

	info, resp, err := GetSourceInfoV0(testState.socket, "package-repo-info-v0")
	require.NoError(t, err, "GET source failed with a client error")
	require.Nil(t, resp, "GET source failed: %#v", resp)
	require.Contains(t, info, "package-repo-info-v0", "No source info returned")
	require.Equal(t, "package-repo-info-v0", info["package-repo-info-v0"].Name)
	require.Equal(t, "file://"+testState.repoDir, info["package-repo-info-v0"].URL)

	resp, err = DeleteSourceV0(testState.socket, "package-repo-info-v0")
	require.NoError(t, err, "DELETE source failed with a client error")
	require.True(t, resp.Status, "DELETE source failed: %#v", resp)
}

func UploadUserDefinedSources(t *testing.T, sources []string) {
	for i := range sources {
		source := strings.Replace(sources[i], "REPO-PATH", testState.repoDir, 1)
		resp, err := PostJSONSourceV0(testState.socket, source)
		require.NoError(t, err, "POST source failed with a client error")
		require.True(t, resp.Status, "POST source failed: %#v", resp)
	}
}

// verify user defined sources are not present
func VerifyNoUserDefinedSources(t *testing.T, source_names []string) {
	list, api, err := ListSourcesV0(testState.socket)
	require.NoError(t, err, "GET source failed with a client error")
	require.Nil(t, api, "ListSources failed: %#v", api)
	require.GreaterOrEqual(t, len(list), 1, "Not enough sources returned")
	for i := range source_names {
		require.NotContains(t, list, source_names[i])
	}
}

func TestDeleteUserDefinedSourcesV0(t *testing.T) {
	source_names := []string{"package-repo-1", "package-repo-2"}
	sources := []string{`{
		"name": "package-repo-1",
		"url": "file://REPO-PATH",
		"type": "yum-baseurl",
		"proxy": "https://proxy-url/",
		"check_ssl": true,
		"check_gpg": true,
		"gpgkey_urls": ["https://url/path/to/gpg-key"]
	}`,
		`{
		"name": "package-repo-2",
		"url": "file://REPO-PATH",
		"type": "yum-baseurl",
		"proxy": "https://proxy-url/",
		"check_ssl": true,
		"check_gpg": true,
		"gpgkey_urls": ["https://url/path/to/gpg-key"]
	}`}

	// verify test starts without user defined sources
	VerifyNoUserDefinedSources(t, source_names)

	// post user defined sources
	UploadUserDefinedSources(t, sources)
	// note: not verifying user defined sources have been pushed b/c correct
	// operation of PostJSONSourceV0 is validated in the test functions above

	// Remove the test sources
	for _, n := range source_names {
		resp, err := DeleteSourceV0(testState.socket, n)
		require.NoError(t, err, "DELETE source failed with a client error")
		require.True(t, resp.Status, "DELETE source failed: %#v", resp)
	}

	// verify removed sources are not present after removal
	VerifyNoUserDefinedSources(t, source_names)
}

func Index(vs []string, t string) int {
	for i, v := range vs {
		if v == t {
			return i
		}
	}
	return -1
}

func Include(vs []string, t string) bool {
	return Index(vs, t) >= 0
}

func TestDeleteSystemSourcesV0(t *testing.T) {
	sources_list, api, err := ListSourcesV0(testState.socket)
	require.NoError(t, err, "GET source failed with a client error")
	require.Nil(t, api, "ListSources failed: %#v", api)

	for _, repo_name := range []string{"test-system-repo", "fedora", "baseos"} {
		// skip repository names which are not present b/c this test can be
		// executed both as a unit test and as an integration test
		if !Include(sources_list, repo_name) {
			continue
		}

		// try removing system source
		resp, err := DeleteSourceV0(testState.socket, repo_name)
		require.NoError(t, err, "DELETE source failed with a client error")
		require.True(t, resp.Status, "DELETE source failed: %#v", resp)

		// verify that system sources are still there
		list, api, err := ListSourcesV0(testState.socket)
		require.NoError(t, err, "GET source failed with a client error")
		require.Nil(t, api, "ListSources failed: %#v", api)
		require.GreaterOrEqual(t, len(list), 1, "Not enough sources returned")
		require.Contains(t, list, repo_name)
	}
}
