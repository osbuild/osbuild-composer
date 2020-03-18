// Package weldrcheck - source contains functions to check the source API
// Copyright (C) 2020 by Red Hat, Inc.

// Tests should be self-contained and not depend on the state of the server
// They should use their own blueprints, not the default blueprints
// They should not assume version numbers for packages will match
// They should run tests that depend on previous results from the same function
// not from other functions.

// +build integration

package weldrcheck

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/client"
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

	resp, err := client.PostJSONSourceV0(testState.socket, source)
	require.NoError(t, err, "POST source failed with a client error")
	require.True(t, resp.Status, "POST source failed: %#v", resp)

	resp, err = client.DeleteSourceV0(testState.socket, "package-repo-json-v0")
	require.NoError(t, err, "DELETE source failed with a client error")
	require.True(t, resp.Status, "DELETE source failed: %#v", resp)
}

// POST an empty JSON source
func TestPOSTEmptyJSONSourceV0(t *testing.T) {
	resp, err := client.PostJSONSourceV0(testState.socket, "")
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

	resp, err := client.PostJSONSourceV0(testState.socket, source)
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

	resp, err := client.PostTOMLSourceV0(testState.socket, source)
	require.NoError(t, err, "POST source failed with a client error")
	require.True(t, resp.Status, "POST source failed: %#v", resp)

	resp, err = client.DeleteSourceV0(testState.socket, "package-repo-toml-v0")
	require.NoError(t, err, "DELETE source failed with a client error")
	require.True(t, resp.Status, "DELETE source failed: %#v", resp)
}

// POST an empty TOML source
func TestPOSTEmptyTOMLSourceV0(t *testing.T) {
	resp, err := client.PostTOMLSourceV0(testState.socket, "")
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

	resp, err := client.PostTOMLSourceV0(testState.socket, source)
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
		resp, err := client.PostJSONSourceV0(testState.socket, source)
		require.NoError(t, err, "POST source failed with a client error")
		require.True(t, resp.Status, "POST source failed: %#v", resp)
	}

	// Remove the test sources, ignoring any errors
	defer func() {
		for _, n := range []string{"package-repo-1", "package-repo-2"} {
			resp, err := client.DeleteSourceV0(testState.socket, n)
			require.NoError(t, err, "DELETE source failed with a client error")
			require.True(t, resp.Status, "DELETE source failed: %#v", resp)
		}
	}()

	// Get the list of sources
	list, api, err := client.ListSourcesV0(testState.socket)
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

	resp, err := client.PostTOMLSourceV0(testState.socket, source)
	require.NoError(t, err, "POST source failed with a client error")
	require.True(t, resp.Status, "POST source failed: %#v", resp)

	info, resp, err := client.GetSourceInfoV0(testState.socket, "package-repo-info-v0")
	require.NoError(t, err, "GET source failed with a client error")
	require.Nil(t, resp, "GET source failed: %#v", resp)
	require.Contains(t, info, "package-repo-info-v0", "No source info returned")
	require.Equal(t, "package-repo-info-v0", info["package-repo-info-v0"].Name)
	require.Equal(t, "file://"+testState.repoDir, info["package-repo-info-v0"].URL)

	resp, err = client.DeleteSourceV0(testState.socket, "package-repo-info-v0")
	require.NoError(t, err, "DELETE source failed with a client error")
	require.True(t, resp.Status, "DELETE source failed: %#v", resp)
}
