// Package client - blueprints_test contains functions to check the blueprints API
// Copyright (C) 2020 by Red Hat, Inc.

// Tests should be self-contained and not depend on the state of the server
// They should use their own blueprints, not the default blueprints
// They should not assume version numbers for packages will match
// They should run tests that depend on previous results from the same function
// not from other functions.
// The blueprint version number may get bumped if the server has had tests run before
// do not assume the bp version will match unless first deleting the old one.
package client

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"testing"

	"github.com/osbuild/osbuild-composer/internal/common"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// POST a new TOML blueprint
func TestPostTOMLBlueprintV0(t *testing.T) {
	bp := `
		name="test-toml-blueprint-v0"
		description="postTOMLBlueprintV0"
		version="0.0.1"
		[[packages]]
		name="bash"
		version="*"

		[[modules]]
		name="util-linux"
		version="*"

		[[customizations.user]]
		name="root"
		password="qweqweqwe"
		`
	resp, err := PostTOMLBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.True(t, resp.Status, "POST failed: %#v", resp)
}

// POST an invalid TOML blueprint
func TestPostInvalidTOMLBlueprintV0(t *testing.T) {
	// Use a blueprint that's missing a trailing ']' on package
	bp := `
		name="test-invalid-toml-blueprint-v0"
		version="0.0.1"
		description="postInvalidTOMLBlueprintV0"
		[package
		name="bash"
		version="*"
		`
	resp, err := PostTOMLBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// POST an empty TOML blueprint
func TestPostEmptyTOMLBlueprintV0(t *testing.T) {
	resp, err := PostTOMLBlueprintV0(testState.socket, "")
	require.NoError(t, err, "failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// POST a TOML blueprint with an empty name
func TestPostEmptyNameTOMLBlueprintV0(t *testing.T) {
	// Use a blueprint with an empty Name
	bp := `
		name=""
		version="0.0.1"
		description="postEmptyNameTOMLBlueprintV0"
		[package]
		name="bash"
		version="*"
		`
	resp, err := PostTOMLBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp, "did not return an error")
	require.False(t, resp.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// POST a TOML blueprint with an invalid name chars
func TestPostInvalidNameTOMLBlueprintV0(t *testing.T) {
	// Use a blueprint with invalid Name
	bp := `
		name="I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜"
		version="0.0.1"
		description="postInvalidNameTOMLBlueprintV0"
		[package]
		name="bash"
		version="*"
		`
	resp, err := PostTOMLBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp, "did not return an error")
	require.False(t, resp.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// POST a new JSON blueprint
func TestPostJSONBlueprintV0(t *testing.T) {
	bp := `{
		"name": "test-json-blueprint-v0",
		"description": "postJSONBlueprintV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.True(t, resp.Status, "POST failed: %#v", resp)
}

// POST an invalid JSON blueprint
func TestPostInvalidJSONBlueprintV0(t *testing.T) {
	// Use a blueprint that's missing a trailing '"' on name
	bp := `{
		"name": "test-invalid-json-blueprint-v0",
		"version": "0.0.1",
		"description": "postInvalidJSONBlueprintV0",
		"modules": [{"name: "util-linux", "version": "*"}],
	}`

	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// POST an empty JSON blueprint
func TestPostEmptyJSONBlueprintV0(t *testing.T) {
	resp, err := PostJSONBlueprintV0(testState.socket, "")
	require.NoError(t, err, "failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// POST a JSON blueprint with an empty name
func TestPostEmptyNameJSONBlueprintV0(t *testing.T) {
	// Use a blueprint with an empty Name
	bp := `{
		"name": "",
		"version": "0.0.1",
		"description": "postEmptyNameJSONBlueprintV0",
		"modules": [{"name": "util-linux", "version": "*"}]
	}`

	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp, "did not return an error")
	require.False(t, resp.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// POST a JSON blueprint with invalid name chars
func TestPostInvalidNameJSONBlueprintV0(t *testing.T) {
	// Use a blueprint with an empty Name
	bp := `{
		"name": "I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜",
		"description": "postInvalidNameJSONBlueprintV0",
		"version": "0.0.1",
		"modules": [{"name": "util-linux", "version": "*"}]
	}`

	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp, "did not return an error")
	require.False(t, resp.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// POST a blueprint to the workspace as TOML
func TestPostTOMLWorkspaceV0(t *testing.T) {
	bp := `
		name="test-toml-blueprint-ws-v0"
		description="postTOMLBlueprintWSV0"
		version="0.0.1"
		[[packages]]
		name="bash"
		version="*"

		[[modules]]
		name="util-linux"
		version="*"

		[[customizations.user]]
		name="root"
		password="qweqweqwe"
		`
	resp, err := PostTOMLWorkspaceV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.True(t, resp.Status, "POST failed: %#v", resp)
}

// POST an invalid TOML blueprint to the workspace
func TestPostInvalidTOMLWorkspaceV0(t *testing.T) {
	// Use a blueprint that's missing a trailing ']' on package
	bp := `
		name="test-invalid-toml-blueprint-ws-v0"
		version="0.0.1"
		description="postInvalidTOMLWorkspaceV0"
		[package
		name="bash"
		version="*"
		`
	resp, err := PostTOMLWorkspaceV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// POST an empty TOML blueprint to the workspace
func TestPostEmptyTOMLWorkspaceV0(t *testing.T) {
	resp, err := PostTOMLWorkspaceV0(testState.socket, "")
	require.NoError(t, err, "failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// POST a TOML blueprint with an empty name to the workspace
func TestPostEmptyNameTOMLWorkspaceV0(t *testing.T) {
	// Use a blueprint with an empty Name
	bp := `
		name=""
		version="0.0.1"
		description="postEmptyNameTOMLWorkspaceV0"
		[package]
		name="bash"
		version="*"
		`
	resp, err := PostTOMLWorkspaceV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp, "did not return an error")
	require.False(t, resp.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// POST a TOML blueprint with an invalid name chars to the workspace
func TestPostInvalidNameTOMLWorkspaceV0(t *testing.T) {
	// Use a blueprint with invalid Name
	bp := `
		name="I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜"
		version="0.0.1"
		description="postInvalidNameTOMLWorkspaceV0"
		[package]
		name="bash"
		version="*"
		`
	resp, err := PostTOMLWorkspaceV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp, "did not return an error")
	require.False(t, resp.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// POST a blueprint to the workspace as JSON
func TestPostJSONWorkspaceV0(t *testing.T) {
	bp := `{
		"name": "test-json-blueprint-ws-v0",
		"description": "postJSONBlueprintWSV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	resp, err := PostJSONWorkspaceV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.True(t, resp.Status, "POST failed: %#v", resp)
}

// POST an invalid JSON blueprint to the workspace
func TestPostInvalidJSONWorkspaceV0(t *testing.T) {
	// Use a blueprint that's missing a trailing '"' on name
	bp := `{
		"name": "test-invalid-json-blueprint-ws-v0",
		"version": "0.0.1",
		"description": "TestPostInvalidJSONWorkspaceV0",
		"modules": [{"name: "util-linux", "version": "*"}],
	}`

	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// POST an empty JSON blueprint to the workspace
func TestPostEmptyJSONWorkspaceV0(t *testing.T) {
	resp, err := PostJSONBlueprintV0(testState.socket, "")
	require.NoError(t, err, "failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// POST a JSON blueprint with an empty name to the workspace
func TestPostEmptyNameJSONWorkspaceV0(t *testing.T) {
	// Use a blueprint with an empty Name
	bp := `{
		"name": "",
		"version": "0.0.1",
		"description": "TestPostEmptyNameJSONWorkspaceV0",
		"modules": [{"name": "util-linux", "version": "*"}]
	}`

	resp, err := PostJSONWorkspaceV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp, "did not return an error")
	require.False(t, resp.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// POST a JSON blueprint with invalid name chars to the workspace
func TestPostInvalidNameJSONWorkspaceV0(t *testing.T) {
	// Use a blueprint with an empty Name
	bp := `{
		"name": "I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜",
		"version": "0.0.1",
		"description": "TestPostInvalidNameJSONWorkspaceV0",
		"modules": [{"name": "util-linux", "version": "*"}]
	}`

	resp, err := PostJSONWorkspaceV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp, "did not return an error")
	require.False(t, resp.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// delete a blueprint
func TestDeleteBlueprintV0(t *testing.T) {
	// POST a blueprint to delete
	bp := `{
		"name": "test-delete-blueprint-v0",
		"description": "deleteBlueprintV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.True(t, resp.Status, "POST failed: %#v", resp)

	// Delete the blueprint
	resp, err = DeleteBlueprintV0(testState.socket, "test-delete-blueprint-v0")
	require.NoError(t, err, "DELETE failed with a client error")
	require.True(t, resp.Status, "DELETE failed: %#v", resp)
}

// delete a non-existent blueprint
func TestDeleteNonBlueprint0(t *testing.T) {
	resp, err := DeleteBlueprintV0(testState.socket, "test-delete-non-blueprint-v0")
	require.NoError(t, err, "failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// delete a blueprint with invalid name characters
func TestDeleteInvalidBlueprintV0(t *testing.T) {
	resp, err := DeleteBlueprintV0(testState.socket, "I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp, "did not return an error")
	require.False(t, resp.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// delete a new blueprint from the workspace
func TestDeleteNewWorkspaceV0(t *testing.T) {
	// POST a blueprint to delete
	bp := `{
		"name": "test-delete-new-blueprint-ws-v0",
		"description": "deleteNewBlueprintWSV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	resp, err := PostJSONWorkspaceV0(testState.socket, bp)
	require.NoError(t, err, "POST failed with a client error")
	require.True(t, resp.Status, "POST failed: %#v", resp)

	// Delete the blueprint
	resp, err = DeleteWorkspaceV0(testState.socket, "test-delete-new-blueprint-ws-v0")
	require.NoError(t, err, "DELETE failed with a client error")
	require.True(t, resp.Status, "DELETE failed: %#v", resp)
}

// delete blueprint changes from the workspace
func TestDeleteChangesWorkspaceV0(t *testing.T) {
	// POST a blueprint first
	bp := `{
		"name": "test-delete-blueprint-changes-ws-v0",
		"description": "deleteBlueprintChangesWSV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "POST blueprint failed with a client error")
	require.True(t, resp.Status, "POST blueprint failed: %#v", resp)

	// Post blueprint changes to the workspace
	bp = `{
		"name": "test-delete-blueprint-changes-ws-v0",
		"description": "workspace copy",
		"version": "0.2.0",
		"packages": [{"name": "frobozz", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	resp, err = PostJSONWorkspaceV0(testState.socket, bp)
	require.NoError(t, err, "POST workspace failed with a client error")
	require.True(t, resp.Status, "POST workspace failed: %#v", resp)

	// Get the blueprint, make sure it is the modified one and that changes = true
	info, api, err := GetBlueprintsInfoJSONV0(testState.socket, "test-delete-blueprint-changes-ws-v0")
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "GET blueprint request failed: %#v", api)
	require.Greater(t, len(info.Blueprints), 0, "No blueprints returned")
	require.Greater(t, len(info.Changes), 0, "No change states returned")
	require.Equal(t, "test-delete-blueprint-changes-ws-v0", info.Blueprints[0].Name, "wrong blueprint returned")
	require.Equal(t, "test-delete-blueprint-changes-ws-v0", info.Changes[0].Name, "wrong change state returned")
	require.True(t, info.Changes[0].Changed, "wrong change state returned (false)")
	require.Equal(t, "workspace copy", info.Blueprints[0].Description, "workspace copy not returned")

	// Delete the blueprint from the workspace
	resp, err = DeleteWorkspaceV0(testState.socket, "test-delete-blueprint-changes-ws-v0")
	require.NoError(t, err, "DELETE workspace failed with a client error")
	require.True(t, resp.Status, "DELETE workspace failed: %#v", resp)

	// Get the blueprint, make sure it is the un-modified one
	info, api, err = GetBlueprintsInfoJSONV0(testState.socket, "test-delete-blueprint-changes-ws-v0")
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "GET blueprint request failed: %#v", api)
	require.Greater(t, len(info.Blueprints), 0, "No blueprints returned")
	require.Greater(t, len(info.Changes), 0, "No change states returned")
	require.Equal(t, "test-delete-blueprint-changes-ws-v0", info.Blueprints[0].Name, "wrong blueprint returned")
	require.Equal(t, "test-delete-blueprint-changes-ws-v0", info.Changes[0].Name, "wrong change state returned")
	require.False(t, info.Changes[0].Changed, "wrong change state returned (true)")
	require.Equal(t, "deleteBlueprintChangesWSV0", info.Blueprints[0].Description, "original blueprint not returned")
}

// delete a non-existent blueprint workspace
func TestDeleteNonWorkspace0(t *testing.T) {
	resp, err := DeleteWorkspaceV0(testState.socket, "test-delete-non-blueprint-ws-v0")
	require.NoError(t, err, "failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// delete a blueprint with invalid name characters
func TestDeleteInvalidWorkspaceV0(t *testing.T) {
	resp, err := DeleteWorkspaceV0(testState.socket, "I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp, "did not return an error")
	require.False(t, resp.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// list blueprints
func TestListBlueprintsV0(t *testing.T) {
	// Post a couple of blueprints
	bps := []string{`{
		"name": "test-list-blueprint-1-v0",
		"description": "listBlueprintsV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`,
		`{
		"name": "test-list-blueprint-2-v0",
		"description": "listBlueprintsV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`}

	for i := range bps {
		resp, err := PostJSONBlueprintV0(testState.socket, bps[i])
		require.NoError(t, err, "POST blueprint failed with a client error")
		require.True(t, resp.Status, "POST blueprint failed: %#v", resp)
	}

	// Get the list of blueprints
	list, api, err := ListBlueprintsV0(testState.socket)
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "ListBlueprints failed: %#v", api)
	require.Contains(t, list, "test-list-blueprint-1-v0")
	require.Contains(t, list, "test-list-blueprint-2-v0")
}

// get blueprint contents as TOML
func TestGetTOMLBlueprintV0(t *testing.T) {
	bp := `{
		"name": "test-get-blueprint-1-v0",
		"description": "getTOMLBlueprintV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	// Post a blueprint
	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "POST blueprint failed with a client error")
	require.True(t, resp.Status, "POST blueprint failed: %#v", resp)

	// Get it as TOML
	body, api, err := GetBlueprintInfoTOMLV0(testState.socket, "test-get-blueprint-1-v0")
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "GetBlueprintInfoTOML failed: %#v", api)
	require.Greater(t, len(body), 0, "body of response is empty")

	// Can it be decoded as TOML?
	var decoded interface{}
	_, err = toml.Decode(body, &decoded)
	require.NoError(t, err, "TOML decode failed")
}

// get non-existent blueprint contents as TOML
func TestGetNonTOMLBlueprintV0(t *testing.T) {
	_, api, err := GetBlueprintInfoTOMLV0(testState.socket, "test-get-non-blueprint-1-v0")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, api, "did not return an error")
	require.False(t, api.Status, "wrong Status (true)")
}

// get blueprint with invalid name characters
func TestGetInvalidTOMLBlueprintV0(t *testing.T) {
	_, api, err := GetBlueprintInfoTOMLV0(testState.socket, "I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, api, "did not return an error")
	require.False(t, api.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", api.Errors[0].ID)
	require.Contains(t, api.Errors[0].Msg, "Invalid characters in API path")
}

// get blueprint contents as JSON
func TestGetJSONBlueprintV0(t *testing.T) {
	bp := `{
		"name": "test-get-blueprint-2-v0",
		"description": "getJSONBlueprintV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	// Post a blueprint
	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "POST blueprint failed with a client error")
	require.True(t, resp.Status, "POST blueprint failed: %#v", resp)

	// Get the blueprint and its changed state
	info, api, err := GetBlueprintsInfoJSONV0(testState.socket, "test-get-blueprint-2-v0")
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "GetBlueprintInfoJSON failed: %#v", api)
	require.Greater(t, len(info.Blueprints), 0, "No blueprints returned")
	require.Greater(t, len(info.Changes), 0, "No change states returned")
	require.Equal(t, "test-get-blueprint-2-v0", info.Blueprints[0].Name, "wrong blueprint returned")
	require.Equal(t, "test-get-blueprint-2-v0", info.Changes[0].Name, "wrong change state returned")
	require.False(t, info.Changes[0].Changed, "wrong change state returned (true)")
}

// get non-existent blueprint contents as JSON
func TestGetNonJSONBkueprintV0(t *testing.T) {
	resp, api, err := GetBlueprintsInfoJSONV0(testState.socket, "test-get-non-blueprint-1-v0")
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "ListBlueprints failed: %#v", api)
	require.Greater(t, len(resp.Errors), 0, "failed with no error: %#v", resp)
}

// pushing the same blueprint bumps the version number returned by show
func TestBumpBlueprintVersionV0(t *testing.T) {
	bp := `{
		"name": "test-bump-blueprint-1-v0",
		"description": "bumpBlueprintVersionV0",
		"version": "2.1.2",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	// List blueprints
	list, api, err := ListBlueprintsV0(testState.socket)
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "ListBlueprints failed: %#v", api)

	// If the blueprint already exists it needs to be deleted to start from a known state
	sorted := sort.StringSlice(list)
	if common.IsStringInSortedSlice(sorted, "test-bump-blueprint-1-v0") {
		// Delete this blueprint if it already exists
		resp, err := DeleteBlueprintV0(testState.socket, "test-bump-blueprint-1-v0")
		require.NoError(t, err, "DELETE blueprint failed with a client error")
		require.True(t, resp.Status, "DELETE blueprint failed: %#v", resp)
	}

	// Post a blueprint
	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "POST blueprint failed with a client error")
	require.True(t, resp.Status, "POST blueprint failed: %#v", resp)

	// Post a blueprint again to bump verion to 2.1.3
	resp, err = PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "POST blueprint 2nd time failed with a client error")
	require.True(t, resp.Status, "POST blueprint 2nd time failed: %#v", resp)

	// Get the blueprint and its changed state
	info, api, err := GetBlueprintsInfoJSONV0(testState.socket, "test-bump-blueprint-1-v0")
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "GetBlueprintsInfoJSON failed: %#v", api)
	require.Greater(t, len(info.Blueprints), 0, "No blueprints returned")
	require.Equal(t, "test-bump-blueprint-1-v0", info.Blueprints[0].Name, "wrong blueprint returned")
	require.Equal(t, "2.1.3", info.Blueprints[0].Version, "wrong blueprint version")
}

// Make several changes to a blueprint and list the changes
func TestBlueprintChangesV0(t *testing.T) {
	bps := []string{`{
		"name": "test-blueprint-changes-v0",
		"description": "CheckBlueprintChangesV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}]
	}`,
		`{
		"name": "test-blueprint-changes-v0",
		"description": "CheckBlueprintChangesV0",
		"version": "0.1.0",
		"packages": [{"name": "bash", "version": "*"}, {"name": "tmux", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`,
		`{
		"name": "test-blueprint-changes-v0",
		"description": "CheckBlueprintChangesV0",
		"version": "0.1.1",
		"packages": [{"name": "bash", "version": "*"}, {"name": "tmux", "version": "*"}],
		"modules": [],
		"customizations": {"user": [{"name": "root", "password": "asdasdasd"}]}
	}`}

	// Push 3 changes to the blueprint
	for i := range bps {
		resp, err := PostJSONBlueprintV0(testState.socket, bps[i])
		require.NoError(t, err, "POST blueprint #%d failed with a client error")
		require.True(t, resp.Status, "POST blueprint #%d failed: %#v", i, resp)
	}

	// List the changes
	changes, api, err := GetBlueprintsChangesV0(testState.socket, []string{"test-blueprint-changes-v0"})
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "GetBlueprintsChanges failed: %#v", api)
	require.Equal(t, 1, len(changes.BlueprintsChanges), "No changes returned")
	require.Equal(t, "test-blueprint-changes-v0", changes.BlueprintsChanges[0].Name, "Wrong blueprint changes returned")
	require.Greater(t, len(changes.BlueprintsChanges[0].Changes), 2, "Wrong number of changes returned")
}

// Get changes for a non-existent blueprint
func TestBlueprintNonChangesV0(t *testing.T) {
	resp, api, err := GetBlueprintsChangesV0(testState.socket, []string{"test-non-blueprint-changes-v0"})
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "GetBlueprintsChanges failed: %#v", api)
	require.Greater(t, len(resp.Errors), 0, "failed with no error: %#v", resp)
}

// get changes with invalid name characters
func TestInvalidBlueprintChangesV0(t *testing.T) {
	_, api, err := GetBlueprintsChangesV0(testState.socket, []string{"I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜"})
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, api, "did not return an error")
	require.False(t, api.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", api.Errors[0].ID)
	require.Contains(t, api.Errors[0].Msg, "Invalid characters in API path")
}

// Undo blueprint changes
func TestUndoBlueprintV0(t *testing.T) {
	bps := []string{`{
		"name": "test-undo-blueprint-v0",
		"description": "CheckUndoBlueprintV0",
		"version": "0.0.5",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`,
		`{
		"name": "test-undo-blueprint-v0",
		"description": "CheckUndoBlueprintv0",
		"version": "0.0.6",
		"packages": [{"name": "bash", "version": "0.5.*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`}

	// Push original version of the blueprint
	resp, err := PostJSONBlueprintV0(testState.socket, bps[0])
	require.NoError(t, err, "POST blueprint #0 failed with a client error")
	require.True(t, resp.Status, "POST blueprint #0 failed: %#v", resp)

	// Get the commit hash
	changes, api, err := GetBlueprintsChangesV0(testState.socket, []string{"test-undo-blueprint-v0"})
	require.NoError(t, err, "GET blueprint #0 failed with a client error")
	require.Nil(t, api, "GetBlueprintsChanges #0 failed: %#v", api)
	require.Equal(t, 1, len(changes.BlueprintsChanges), "No changes returned")
	require.Greater(t, len(changes.BlueprintsChanges[0].Changes), 0, "Wrong number of changes returned")
	commit := changes.BlueprintsChanges[0].Changes[0].Commit
	require.NotEmpty(t, commit, "First commit is empty")

	// Push the new version with wrong bash version
	resp, err = PostJSONBlueprintV0(testState.socket, bps[1])
	require.NoError(t, err, "POST blueprint #1 failed with a client error")
	require.True(t, resp.Status, "POST blueprint #1 failed: %#v", resp)

	// Get the blueprint, confirm bash version is '0.5.*'
	info, api, err := GetBlueprintsInfoJSONV0(testState.socket, "test-undo-blueprint-v0")
	require.NoError(t, err, "GET blueprint #1 failed with a client error")
	require.Nil(t, api, "GetBlueprintsInfo #1 failed: %#v", api)
	require.Greater(t, len(info.Blueprints), 0, "No blueprints returned")
	require.Greater(t, len(info.Blueprints[0].Packages), 0, "No packages in the blueprint")
	require.Equal(t, "bash", info.Blueprints[0].Packages[0].Name, "Wrong package in blueprint")
	require.Equal(t, "0.5.*", info.Blueprints[0].Packages[0].Version, "Wrong version in blueprint")

	// Revert the blueprint to the original version
	resp, err = UndoBlueprintChangeV0(testState.socket, "test-undo-blueprint-v0", commit)
	require.NoError(t, err, "Undo blueprint failed with a client error")
	require.True(t, resp.Status, "Undo blueprint failed: %#v", resp)

	// Get the blueprint, confirm bash version is '*'
	info, api, err = GetBlueprintsInfoJSONV0(testState.socket, "test-undo-blueprint-v0")
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "GetBlueprintsInfo failed: %#v", api)
	require.Greater(t, len(info.Blueprints), 0, "No blueprints returned")
	require.Greater(t, len(info.Blueprints[0].Packages), 0, "No packages in the blueprint")
	require.Equal(t, "bash", info.Blueprints[0].Packages[0].Name, "Wrong package in blueprint")
	require.Equal(t, "*", info.Blueprints[0].Packages[0].Version, "Wrong version in blueprint")
}

// Undo non-existent commit blueprint changes
func TestUndoBlueprintNonCommitV0(t *testing.T) {
	bps := []string{`{
		"name": "test-undo-blueprint-non-commit-v0",
		"description": "CheckUndoBlueprintNonCommitV0",
		"version": "0.0.5",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`,
		`{
		"name": "test-undo-blueprint-non-commit-v0",
		"description": "CheckUndoBlueprintNonCommitv0",
		"version": "0.0.6",
		"packages": [{"name": "bash", "version": "0.5.*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`}

	for i := range bps {
		resp, err := PostJSONBlueprintV0(testState.socket, bps[i])
		require.NoError(t, err, "POST blueprint #%d failed with a client error")
		require.True(t, resp.Status, "POST blueprint #%d failed: %#v", i, resp)
	}

	resp, err := UndoBlueprintChangeV0(testState.socket, "test-undo-blueprint-non-commit-v0", "FFFF")
	require.NoError(t, err, "POST blueprint failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// Undo non-existent blueprint changes
func TestUndoNonBlueprintV0(t *testing.T) {
	resp, err := UndoBlueprintChangeV0(testState.socket, "test-undo-non-blueprint-v0", "FFFF")
	require.NoError(t, err, "blueprint failed with a client error")
	require.False(t, resp.Status, "did not return an error")
}

// undo a blueprint with invalid name characters
func TestUndoInvalidBlueprintV0(t *testing.T) {
	resp, err := UndoBlueprintChangeV0(testState.socket, "I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜", "FFFF")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp, "did not return an error")
	require.False(t, resp.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// undo a blueprint with invalid commit characters
func TestUndoInvalidBlueprintCommitV0(t *testing.T) {
	resp, err := UndoBlueprintChangeV0(testState.socket, "test-undo-non-blueprint-v0", "I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp, "did not return an error")
	require.False(t, resp.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// Tag a blueprint with a new revision
// The blueprint revision tag cannot be reset, it always increments by one, and cannot be deleted.
// So to test tagging we tag two blueprint changes and make sure the second is first +1
func TestBlueprintTagV0(t *testing.T) {
	bps := []string{`{
		"name": "test-tag-blueprint-v0",
		"description": "CheckBlueprintTagV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "0.1.*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`,
		`{
		"name": "test-tag-blueprint-v0",
		"description": "CheckBlueprintTagV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "0.5.*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`}

	// Push a blueprint
	resp, err := PostJSONBlueprintV0(testState.socket, bps[0])
	require.NoError(t, err, "POST blueprint #0 failed with a client error")
	require.True(t, resp.Status, "POST blueprint #0 failed: %#v", resp)

	// Tag the blueprint
	tagResp, err := TagBlueprintV0(testState.socket, "test-tag-blueprint-v0")
	require.NoError(t, err, "Tag blueprint #0 failed with a client error")
	require.True(t, tagResp.Status, "Tag blueprint #0 failed: %#v", resp)

	// Get changes, get the blueprint's revision
	changes, api, err := GetBlueprintsChangesV0(testState.socket, []string{"test-tag-blueprint-v0"})
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "GetBlueprintsChanges failed: %#v", api)
	require.Equal(t, 1, len(changes.BlueprintsChanges), "No changes returned")
	require.Greater(t, len(changes.BlueprintsChanges[0].Changes), 0, "Wrong number of changes returned")

	revision := changes.BlueprintsChanges[0].Changes[0].Revision
	require.NotNil(t, revision, "Revision is zero")
	require.NotEqual(t, 0, *revision, "Revision is zero")

	// Push a new version of the blueprint
	resp, err = PostJSONBlueprintV0(testState.socket, bps[1])
	require.NoError(t, err, "POST blueprint #1 failed with a client error")
	require.True(t, resp.Status, "POST blueprint #1 failed: %#v", resp)

	// Tag the blueprint
	tagResp, err = TagBlueprintV0(testState.socket, "test-tag-blueprint-v0")
	require.NoError(t, err, "Tag blueprint #1 failed with a client error")
	require.True(t, tagResp.Status, "Tag blueprint #1 failed: %#v", resp)

	// Get changes, confirm that Revision is revision +1
	changes, api, err = GetBlueprintsChangesV0(testState.socket, []string{"test-tag-blueprint-v0"})
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "GetBlueprintsChanges failed: %#v", api)
	require.Equal(t, 1, len(changes.BlueprintsChanges), "No changes returned")
	require.Greater(t, len(changes.BlueprintsChanges[0].Changes), 0, "Wrong number of changes returned")

	newRevision := changes.BlueprintsChanges[0].Changes[0].Revision
	require.NotNil(t, newRevision, "Revision is not %d", *revision+1)
	require.Equal(t, *revision+1, *newRevision, "Revision is not %d", *revision+1)
}

// Tag a non-existent blueprint
func TestNonBlueprintTagV0(t *testing.T) {
	tagResp, err := TagBlueprintV0(testState.socket, "test-tag-non-blueprint-v0")
	require.NoError(t, err, "failed with a client error")
	require.False(t, tagResp.Status, "did not return an error")
}

// tag a blueprint with invalid name characters
func TestTagInvalidBlueprintV0(t *testing.T) {
	resp, err := TagBlueprintV0(testState.socket, "I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp, "did not return an error")
	require.False(t, resp.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// depsolve a blueprint with packages and modules
func TestBlueprintDepsolveV0(t *testing.T) {
	bp := `{
		"name": "test-deps-blueprint-v0",
		"description": "CheckBlueprintDepsolveV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}]
	}`

	// Push a blueprint
	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "POST blueprint failed with a client error")
	require.True(t, resp.Status, "POST blueprint failed: %#v", resp)

	// Depsolve the blueprint
	deps, api, err := DepsolveBlueprintV0(testState.socket, "test-deps-blueprint-v0")
	require.NoError(t, err, "Depsolve blueprint failed with a client error")
	require.Nil(t, api, "DepsolveBlueprint failed: %#v", api)
	require.Greater(t, len(deps.Blueprints), 0, "No blueprint dependencies returned")
	require.Greater(t, len(deps.Blueprints[0].Dependencies), 2, "Not enough dependencies returned")

	// TODO
	// Get the bash and util-linux dependencies and make sure their versions are not *

}

// depsolve a blueprint with package name glob
func TestBlueprintDepsolveGlobsV0(t *testing.T) {
	// Depends on real packages, only run as an integration test
	if testState.unitTest {
		t.Skip()
	}
	bp := `{
		"name": "test-deps-blueprint-globs-v0",
		"description": "CheckBlueprintDepsolveGlobsV0",
		"version": "0.0.1",
		"packages": [{"name": "tmux", "version": "*"},
		{"name": "openssh-*", "version": "*"}]
	}`

	// Push a blueprint
	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "POST blueprint failed with a client error")
	require.True(t, resp.Status, "POST blueprint failed: %#v", resp)

	// Depsolve the blueprint
	deps, api, err := DepsolveBlueprintV0(testState.socket, "test-deps-blueprint-globs-v0")
	require.NoError(t, err, "Depsolve blueprint failed with a client error")
	require.Nil(t, api, "DepsolveBlueprint failed: %#v", api)
	require.Greater(t, len(deps.Blueprints), 0, "No blueprint dependencies returned")
	require.Greater(t, len(deps.Blueprints[0].Dependencies), 2, "Not enough dependencies returned")

	// Did it include tmux? Did it include openssh-clients and openssh-server?
	var names []string
	for _, d := range deps.Blueprints[0].Dependencies {
		names = append(names, d.Name)
	}
	sort.Strings(names)
	assert.True(t, common.IsStringInSortedSlice(names, "openssh-clients"))
	assert.True(t, common.IsStringInSortedSlice(names, "openssh-server"))
	assert.True(t, common.IsStringInSortedSlice(names, "tmux"))
}

// depsolve a non-existent blueprint
func TestNonBlueprintDepsolveV0(t *testing.T) {
	resp, api, err := DepsolveBlueprintV0(testState.socket, "test-deps-non-blueprint-v0")
	require.NoError(t, err, "Depsolve blueprint failed with a client error")
	require.Nil(t, api, "DepsolveBlueprint failed: %#v", api)
	require.Greater(t, len(resp.Errors), 0, "failed with no error: %#v", resp)
}

// depsolve a blueprint with invalid name characters
func TestDepsolveInvalidBlueprintV0(t *testing.T) {
	_, api, err := DepsolveBlueprintV0(testState.socket, "I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, api, "did not return an error")
	require.False(t, api.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", api.Errors[0].ID)
	require.Contains(t, api.Errors[0].Msg, "Invalid characters in API path")
}

// depsolve a blueprint with a depsolve that will fail
func TestBadDepsolveBlueprintV0(t *testing.T) {
	if testState.unitTest {
		t.Skip()
	}
	bp := `{
		"name": "test-baddep-blueprint-v0",
		"description": "TestBadDepsolveBlueprintV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}, {"name": "bad-package", "version": "1.32.1"}]
	}`

	// Push a blueprint
	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "POST blueprint failed with a client error")
	require.True(t, resp.Status, "POST blueprint failed: %#v", resp)

	// Depsolve the blueprint
	deps, api, err := DepsolveBlueprintV0(testState.socket, "test-baddep-blueprint-v0")
	require.NoError(t, err, "Depsolve blueprint failed with a client error")
	require.Nil(t, api, "DepsolveBlueprint failed: %#v", api)
	require.Greater(t, len(deps.Blueprints), 0, "No blueprints returned")
	require.Greater(t, len(deps.Errors), 0, "Not enough errors returned")
}

// freeze a blueprint
func TestBlueprintFreezeV0(t *testing.T) {
	if testState.unitTest {
		// The unit test mock server uses packages named dep-packageN
		bp := `{
			"name": "test",
			"description": "CheckBlueprintFreezeV0",
			"version": "0.0.1",
			"packages": [{"name": "dep-package1", "version": "*"}],
			"modules": [{"name": "dep-package2", "version": "*"}]
	    }`

		// Push a blueprint
		resp, err := PostJSONBlueprintV0(testState.socket, bp)
		require.NoError(t, err, "POST blueprint failed with a client error")
		require.True(t, resp.Status, "POST blueprint failed: %#v", resp)

		// The unit test server returns hard-coded frozen packages
		// in the modules section with an empty packages list
		frozen, api, err := FreezeBlueprintV0(testState.socket, "test")
		require.NoError(t, err, "Freeze blueprint failed with a client error")
		require.Nil(t, api, "FreezeBlueprint failed: %#v", api)
		require.Greater(t, len(frozen.Blueprints), 0, "No frozen blueprints returned")
		require.Equal(t, 1, len(frozen.Blueprints[0].Blueprint.Packages), "No frozen packages returned")
		require.Equal(t, 1, len(frozen.Blueprints[0].Blueprint.Modules), "No frozen modules returned")
		return
	}

	// Integration test
	bp := `{
		"name": "test-freeze-blueprint-v0",
		"description": "CheckBlueprintFreezeV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}]
	}`

	// Push a blueprint
	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "POST blueprint failed with a client error")
	require.True(t, resp.Status, "POST blueprint failed: %#v", resp)

	// Freeze the blueprint
	frozen, api, err := FreezeBlueprintV0(testState.socket, "test-freeze-blueprint-v0")
	require.NoError(t, err, "Freeze blueprint failed with a client error")
	require.Nil(t, api, "FreezeBlueprint failed: %#v", api)
	require.Greater(t, len(frozen.Blueprints), 0, "No frozen blueprints returned")
	require.Greater(t, len(frozen.Blueprints[0].Blueprint.Packages), 0, "No frozen packages returned")
	require.Equal(t, "bash", frozen.Blueprints[0].Blueprint.Packages[0].Name, "Wrong package in frozen blueprint")
	require.NotEqual(t, "*", frozen.Blueprints[0].Blueprint.Packages[0].Version, "Wrong version in frozen blueprint")
	require.Greater(t, len(frozen.Blueprints[0].Blueprint.Modules), 0, "No frozen modules returned")
	require.Equal(t, "util-linux", frozen.Blueprints[0].Blueprint.Modules[0].Name, "Wrong module in frozen blueprint")
	require.NotEqual(t, "*", frozen.Blueprints[0].Blueprint.Modules[0].Version, "Wrong version in frozen blueprint module")
}

func TestBlueprintFreezeGlobsV0(t *testing.T) {
	// Test needs real packages, skip it for unit testing
	if testState.unitTest {
		t.Skip()
	}

	// works with osbuild-composer v83 and later
	rpm_q := exec.Command("rpm", "-q", "--qf", "%{version}", "osbuild-composer")
	out, err := rpm_q.CombinedOutput()
	if err != nil {
		assert.Fail(t, fmt.Sprintf("Error during rpm -q: %s", err))
	}

	rpm_version, err := strconv.Atoi(string(out))
	if err != nil {
		assert.Fail(t, "Error during str-int conversion", err)
	}

	if rpm_version < 83 {
		t.Skip()
	}

	bp := `{
		"name": "test-freeze-blueprint-glob-v0",
		"description": "TestBlueprintFreezeGlobsV0",
		"version": "0.0.1",
		"packages": [{"name": "tmux", "version": "?.*"},
		{"name": "openssh-*", "version": "*"}]
	}`

	// Push a blueprint
	resp, err := PostJSONBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "POST blueprint failed with a client error")
	require.True(t, resp.Status, "POST blueprint failed: %#v", resp)

	// Freeze the blueprint
	frozen, api, err := FreezeBlueprintV0(testState.socket, "test-freeze-blueprint-glob-v0")
	require.NoError(t, err, "Freeze blueprint failed with a client error")
	require.Nil(t, api, "FreezeBlueprint failed: %#v", api)
	require.Greater(t, len(frozen.Blueprints), 0, "No frozen blueprints returned")
	require.Greater(t, len(frozen.Blueprints[0].Blueprint.Packages), 0, "No frozen packages returned")

	var names []string
	for _, p := range frozen.Blueprints[0].Blueprint.Packages {
		assert.NotContains(t, p.Version, "*")
		assert.NotContains(t, p.Version, "?")
		names = append(names, p.Name)
	}
	sort.Strings(names)
	t.Log(names)
	assert.True(t, common.IsStringInSortedSlice(names, "openssh-clients"))
	assert.True(t, common.IsStringInSortedSlice(names, "openssh-server"))
	assert.True(t, common.IsStringInSortedSlice(names, "tmux"))

}

// freeze a non-existent blueprint
func TestNonBlueprintFreezeV0(t *testing.T) {
	resp, api, err := FreezeBlueprintV0(testState.socket, "test-freeze-non-blueprint-v0")
	require.NoError(t, err, "Freeze blueprint failed with a client error")
	require.Nil(t, api, "FreezeBlueprint failed: %#v", api)
	require.Greater(t, len(resp.Errors), 0, "failed with no error: %#v", resp)
}

// freeze a blueprint with invalid name characters
func TestFreezeInvalidBlueprintV0(t *testing.T) {
	_, api, err := FreezeBlueprintV0(testState.socket, "I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, api, "did not return an error")
	require.False(t, api.Status, "wrong Status (true)")
	require.Equal(t, "InvalidChars", api.Errors[0].ID)
	require.Contains(t, api.Errors[0].Msg, "Invalid characters in API path")
}

// Make several changes to a blueprint and get the oldest change by commit hash
func TestBlueprintChangeV1(t *testing.T) {
	bps := []string{`{
		"name": "test-blueprint-change-v1",
		"description": "CheckBlueprintChangeV1",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}]
	}`,
		`{
		"name": "test-blueprint-change-v1",
		"description": "CheckBlueprintChangeV1",
		"version": "0.1.0",
		"packages": [{"name": "bash", "version": "*"}, {"name": "tmux", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`,
		`{
		"name": "test-blueprint-change-v1",
		"description": "CheckBlueprintChangeV1",
		"version": "0.1.1",
		"packages": [{"name": "bash", "version": "*"}, {"name": "tmux", "version": "*"}],
		"modules": [],
		"customizations": {"user": [{"name": "root", "password": "asdasdasd"}]}
	}`}

	// Push 3 changes to the blueprint
	for i := range bps {
		resp, err := PostJSONBlueprintV0(testState.socket, bps[i])
		require.NoError(t, err, "POST blueprint #%d failed with a client error")
		require.True(t, resp.Status, "POST blueprint #%d failed: %#v", i, resp)
	}

	// List the changes
	changes, api, err := GetBlueprintsChangesV0(testState.socket, []string{"test-blueprint-change-v1"})
	require.NoError(t, err, "GET blueprint failed with a client error")
	require.Nil(t, api, "GetBlueprintsChanges failed: %#v", api)
	require.Equal(t, 1, len(changes.BlueprintsChanges), "No changes returned")
	require.Equal(t, "test-blueprint-change-v1", changes.BlueprintsChanges[0].Name, "Wrong blueprint changes returned")
	require.Greater(t, len(changes.BlueprintsChanges[0].Changes), 2, "Wrong number of changes returned")

	// Get the oldest commit
	commit := changes.BlueprintsChanges[0].Changes[2].Commit
	t.Logf("commit = %s", commit)
	t.Logf("changes = %v", changes)
	bp, api, err := GetBlueprintChangeV1(testState.socket, "test-blueprint-change-v1", commit)
	require.NoError(t, err, "GET blueprint change failed with a client error")
	require.Nil(t, api, "GET blueprint change failed: %#v", api)
	require.NotNil(t, bp, "GET blueprint change failed: missing blueprint")
	assert.Equal(t, bp.Version, "0.0.1")
}

func TestEmptyPackageNameBlueprintJsonV0(t *testing.T) {
	for _, tc := range []struct {
		packageSnippet string
		specificErr    string
	}{
		{
			`"packages": [{"name": "", "version": "*"}]`,
			"Entry #1 has version '*' but no name.",
		}, {
			`"packages": [{"name": ""}]`,
			"Entry #1 has no name.",
		},
	} {
		bp := `{
 		"name": "test-emptypackage-blueprint-v0",
 		"description": "TestEmptyPackageNameBlueprintV0",
 		"version": "0.0.1",
		` + tc.packageSnippet + `}`

		expectedErrorPrefix := "BlueprintsError: All package entries need to contain the name of the package."
		expectedErr := expectedErrorPrefix + " " + tc.specificErr

		resp, err := PostJSONBlueprintV0(testState.socket, bp)
		require.NoError(t, err, "failed with a client error")
		require.False(t, resp.Status, "Negative status expected.")
		require.Equal(t, len(resp.Errors), 1, "There should be exactly one error")
		require.Equal(t, resp.Errors[0].String(), expectedErr, "Error message shall match exactly")
	}
}

func TestEmptyPackageNameBlueprintTOMLV0(t *testing.T) {
	for _, tc := range []struct {
		packageSnippet string
		specificErr    string
	}{
		{
			"[[packages]]\nversion = \"*\"\n",
			"Entry #1 has version '*' but no name.",
		}, {
			"[[packages]]\n",
			"Entry #1 has no name.",
		},
	} {
		bp := `name = "EMPTY-PACKAGE-NAME"
           description = "empty package name"
           version = "0.0.1"
           ` + tc.packageSnippet

		expectedErrorPrefix := "BlueprintsError: All package entries need to contain the name of the package."
		expectedErr := expectedErrorPrefix + " " + tc.specificErr

		resp, err := PostTOMLBlueprintV0(testState.socket, bp)
		require.NoError(t, err, "failed with a client error")
		require.False(t, resp.Status, "Negative status expected.")
		require.Equal(t, len(resp.Errors), 1, "There should be exactly one error")
		require.Equal(t, resp.Errors[0].String(), expectedErr, "Error message shall match exactly")
	}
}
