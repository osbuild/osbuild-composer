// Package client - compose_test contains functions to check the compose API
// Copyright (C) 2020 by Red Hat, Inc.

// Tests should be self-contained and not depend on the state of the server
// They should use their own blueprints, not the default blueprints
// They should not assume version numbers for packages will match
// They should run tests that depend on previous results from the same function
// not from other functions.
//
// NOTE: The compose fail/finish tests use fake composes so the following are not
//
//	fully tested here:
//
//	* image download
//	* log download
//	* logs archive download
//	* cancel waiting compose
//	* cancel running compose
//
//	In addition osbuild-composer has not implemented:
//
//	* compose/results
//	* compose/metadata
package client

import (
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/weldr"
)

// Test the compose types API
func TestComposeTypesV0(t *testing.T) {
	composeTypes, resp, err := GetComposesTypesV0(testState.socket)
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.Greater(t, len(composeTypes), 0)
	var found bool
	for _, t := range composeTypes {
		if t.Name == testState.imageTypeName && t.Enabled == true {
			found = true
			break
		}
	}
	require.True(t, found, "%s not in list of compose types: %#v", testState.imageTypeName, composeTypes)
}

// Test compose with invalid type fails
func TestComposeInvalidTypeV0(t *testing.T) {
	// lorax-composer checks the blueprint name before checking the compose type
	// so we need to push an empty blueprint to make sure the right failure is checked
	bp := `
		name="test-compose-invalid-type-v0"
		description="TestComposeInvalidTypeV0"
		version="0.0.1"
		`
	resp, err := PostTOMLBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.True(t, resp.Status, "POST failed: %#v", resp)

	compose := `{
		"blueprint_name": "test-compose-invalid-type-v0",
		"compose_type": "snakes",
		"branch": "master"
	}`
	resp, err = PostComposeV0(testState.socket, compose)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp)
	require.False(t, resp.Status, "POST did not fail")
	require.Equal(t, len(resp.Errors), 1)
	require.Contains(t, resp.Errors[0].Msg, "snakes")
}

// Test compose for unknown blueprint fails
func TestComposeInvalidBlueprintV0(t *testing.T) {
	compose := fmt.Sprintf(`{
		"blueprint_name": "test-invalid-bp-compose-v0",
		"compose_type": "%s",
		"branch": "master"
	}`, testState.imageTypeName)
	resp, err := PostComposeV0(testState.socket, compose)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp)
	require.False(t, resp.Status, "POST did not fail")
	require.Equal(t, len(resp.Errors), 1)
	require.Contains(t, resp.Errors[0].Msg, "test-invalid-bp-compose-v0")
}

// Test compose for empty blueprint fails
func TestComposeEmptyBlueprintV0(t *testing.T) {
	compose := fmt.Sprintf(`{
		"blueprint_name": "",
		"compose_type": "%s",
		"branch": "master"
	}`, testState.imageTypeName)
	resp, err := PostComposeV0(testState.socket, compose)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp)
	require.False(t, resp.Status, "POST did not fail")
	require.Greater(t, len(resp.Errors), 0)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// Test compose for blueprint with invalid characters fails
func TestComposeInvalidCharsBlueprintV0(t *testing.T) {
	compose := fmt.Sprintf(`{
		"blueprint_name": "I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜",
		"compose_type": "%s",
		"branch": "master"
	}`, testState.imageTypeName)
	resp, err := PostComposeV0(testState.socket, compose)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp)
	require.False(t, resp.Status, "POST did not fail")
	require.Greater(t, len(resp.Errors), 0)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
}

// Test compose cancel for unknown uuid fails
func TestCancelUnknownComposeV0(t *testing.T) {
	status, resp, err := CancelComposeV0(testState.socket, "c91818f9-8025-47af-89d2-f030d7000c2c")
	require.NoError(t, err, "failed with a client error")
	assert.Equal(t, weldr.CancelComposeStatusV0{}, status)
	require.NotNil(t, resp)
	require.False(t, resp.Status, "Cancel did not fail")
	require.Equal(t, 1, len(resp.Errors), "%#v", resp)
	require.Equal(t, "UnknownUUID", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "c91818f9-8025-47af-89d2-f030d7000c2c")
}

// Test compose delete for unknown uuid
func TestDeleteUnknownComposeV0(t *testing.T) {
	status, resp, err := DeleteComposeV0(testState.socket, "c91818f9-8025-47af-89d2-f030d7000c2c")
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	// TODO -- fix the API Handler in osbuild-composer, should be no uuids
	assert.Equal(t, 0, len(status.UUIDs), "%#v", status)
	require.Equal(t, 1, len(status.Errors), "%#v", status)
	require.Equal(t, "UnknownUUID", status.Errors[0].ID)
	require.Contains(t, status.Errors[0].Msg, "c91818f9-8025-47af-89d2-f030d7000c2c")
}

// Test compose info for unknown uuid
func TestUnknownComposeInfoV0(t *testing.T) {
	_, resp, err := GetComposeInfoV0(testState.socket, "c91818f9-8025-47af-89d2-f030d7000c2c")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp)
	require.False(t, resp.Status)
	require.Equal(t, 1, len(resp.Errors))
	require.Equal(t, "UnknownUUID", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "c91818f9-8025-47af-89d2-f030d7000c2c")
}

// Test compose metadata for unknown uuid
// TODO osbuild-composer has not implemented compose/metadata yet

// Test compose image for unknown uuid
func TestComposeInvalidImageV0(t *testing.T) {
	resp, err := WriteComposeImageV0(testState.socket, io.Discard, "c91818f9-8025-47af-89d2-f030d7000c2c")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp)
	require.False(t, resp.Status)
	require.Equal(t, 1, len(resp.Errors))
	require.Equal(t, "UnknownUUID", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "c91818f9-8025-47af-89d2-f030d7000c2c")
}

// Test compose logs for unknown uuid
func TestComposeInvalidLogsV0(t *testing.T) {
	resp, err := WriteComposeLogsV0(testState.socket, io.Discard, "c91818f9-8025-47af-89d2-f030d7000c2c")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp)
	require.False(t, resp.Status)
	require.Equal(t, 1, len(resp.Errors))
	require.Equal(t, "UnknownUUID", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "c91818f9-8025-47af-89d2-f030d7000c2c")
}

// Test compose log for unknown uuid
func TestComposeInvalidLogV0(t *testing.T) {
	resp, err := WriteComposeLogV0(testState.socket, io.Discard, "c91818f9-8025-47af-89d2-f030d7000c2c")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp)
	require.False(t, resp.Status)
	require.Equal(t, 1, len(resp.Errors))
	require.Equal(t, "UnknownUUID", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "c91818f9-8025-47af-89d2-f030d7000c2c")
}

// Test compose metadata for unknown uuid
func TestComposeInvalidMetadataV0(t *testing.T) {
	resp, err := WriteComposeMetadataV0(testState.socket, io.Discard, "c91818f9-8025-47af-89d2-f030d7000c2c")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp)
	require.False(t, resp.Status)
	require.Equal(t, 1, len(resp.Errors))
	require.Equal(t, "UnknownUUID", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "c91818f9-8025-47af-89d2-f030d7000c2c")
}

// Test compose results for unknown uuid
func TestComposeInvalidResultsV0(t *testing.T) {
	resp, err := WriteComposeResultsV0(testState.socket, io.Discard, "c91818f9-8025-47af-89d2-f030d7000c2c")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp)
	require.False(t, resp.Status)
	require.Equal(t, 1, len(resp.Errors))
	require.Equal(t, "UnknownUUID", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "c91818f9-8025-47af-89d2-f030d7000c2c")
}

// Test status filter for unknown uuid
func TestComposeInvalidStatusV0(t *testing.T) {
	status, resp, err := GetComposeStatusV0(testState.socket, "c91818f9-8025-47af-89d2-f030d7000c2c", "", "", "")
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.Equal(t, 0, len(status))
}

// Test status filter for unknown blueprint
func TestComposeUnknownBlueprintStatusV0(t *testing.T) {
	status, resp, err := GetComposeStatusV0(testState.socket, "*", "unknown-blueprint-test", "", "")
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.Equal(t, 0, len(status))
}

// Test status filter for blueprint with invalid characters
func TestComposeInvalidBlueprintStatusV0(t *testing.T) {
	status, resp, err := GetComposeStatusV0(testState.socket, "*", "I ｗ𝒊ll 𝟉ο𝘁 𝛠ａ𝔰ꜱ 𝘁𝒉𝝸𝚜", "", "")
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp)
	require.Equal(t, "InvalidChars", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "Invalid characters in API path")
	require.Equal(t, 0, len(status))
}

// Helper for searching compose results for a UUID
func UUIDInComposeResults(buildID uuid.UUID, results []weldr.ComposeEntryV0) bool {
	for idx := range results {
		if results[idx].ID == buildID {
			return true
		}
	}
	return false
}

// Helper to wait for a build id to not be in the queue
func WaitForBuild(socket *http.Client, buildID uuid.UUID) (*APIResponse, error) {
	for {
		queue, resp, err := GetComposeQueueV0(testState.socket)
		if err != nil {
			return nil, err
		}
		if resp != nil {
			return resp, nil
		}

		if !UUIDInComposeResults(buildID, queue.New) &&
			!UUIDInComposeResults(buildID, queue.Run) {
			break
		}
	}
	return nil, nil
}

// Setup and run the failed compose tests
func TestFailedComposeV0(t *testing.T) {
	bp := `
		name="test-failed-compose-v0"
		description="TestFailedComposeV0"
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

	compose := fmt.Sprintf(`{
		"blueprint_name": "test-failed-compose-v0",
		"compose_type": "%s",
		"branch": "master"
	}`, testState.imageTypeName)
	// Create a failed test compose
	body, resp, err := PostJSON(testState.socket, "/api/v1/compose?test=1", compose)
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)

	response, err := NewComposeResponseV0(body)
	require.NoError(t, err, "failed with a client error")
	require.True(t, response.Status, "POST failed: %#v", response)
	buildID := response.BuildID

	// Wait until the build is not listed in the queue
	resp, err = WaitForBuild(testState.socket, buildID)
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)

	// Test finished after compose (should not have finished)
	finished, resp, err := GetFinishedComposesV0(testState.socket)
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.False(t, UUIDInComposeResults(buildID, finished))

	// Test failed after compose (should have failed)
	failed, resp, err := GetFailedComposesV0(testState.socket)
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.True(t, UUIDInComposeResults(buildID, failed), "%s not found in failed list: %#v", buildID, failed)

	// Test status filter on failed compose
	status, resp, err := GetComposeStatusV0(testState.socket, "*", "", "FAILED", "")
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.True(t, UUIDInComposeResults(buildID, status), "%s not found in status list: %#v", buildID, status)

	// Test status of build id
	status, resp, err = GetComposeStatusV0(testState.socket, buildID.String(), "", "", "")
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.True(t, UUIDInComposeResults(buildID, status), "%s not found in status list: %#v", buildID, status)

	// Test status filter using FINISHED, should not be listed
	status, resp, err = GetComposeStatusV0(testState.socket, "*", "", "FINISHED", "")
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.False(t, UUIDInComposeResults(buildID, status))

	// Test compose info for the failed compose
	info, resp, err := GetComposeInfoV0(testState.socket, buildID.String())
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.Equal(t, "FAILED", info.QueueStatus)
	require.Equal(t, buildID, info.ID)

	// Test requesting the compose logs for the failed build
	resp, err = WriteComposeLogsV0(testState.socket, io.Discard, buildID.String())
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)

	// Test requesting the compose metadata for the failed build
	resp, err = WriteComposeMetadataV0(testState.socket, io.Discard, buildID.String())
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)

	// Test requesting the compose results for the failed build
	resp, err = WriteComposeResultsV0(testState.socket, io.Discard, buildID.String())
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)

	// Test canceling the failed compose
	cancelStatus, resp, err := CancelComposeV0(testState.socket, buildID.String())
	require.NoError(t, err, "failed with a client error")
	assert.Equal(t, weldr.CancelComposeStatusV0{}, cancelStatus)
	require.NotNil(t, resp)
	require.False(t, resp.Status, "Cancel did not fail")
	require.Equal(t, 1, len(resp.Errors), "%#v", resp)
	require.Equal(t, "BuildInWrongState", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, buildID.String())
}

// Setup and run the finished compose tests
func TestFinishedComposeV0(t *testing.T) {
	bp := `
		name="test-finished-compose-v0"
		description="TestFinishedComposeV0"
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

	compose := fmt.Sprintf(`{
		"blueprint_name": "test-finished-compose-v0",
		"compose_type": "%s",
		"branch": "master"
	}`, testState.imageTypeName)
	// Create a finished test compose
	body, resp, err := PostJSON(testState.socket, "/api/v1/compose?test=2", compose)
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)

	response, err := NewComposeResponseV0(body)
	require.NoError(t, err, "failed with a client error")
	require.True(t, response.Status, "POST failed: %#v", response)
	buildID := response.BuildID

	// Wait until the build is not listed in the queue
	resp, err = WaitForBuild(testState.socket, buildID)
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)

	// Test failed after compose (should not have failed)
	failed, resp, err := GetFailedComposesV0(testState.socket)
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.False(t, UUIDInComposeResults(buildID, failed))

	// Test finished after compose (should have finished)
	finished, resp, err := GetFinishedComposesV0(testState.socket)
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.True(t, UUIDInComposeResults(buildID, finished), "%s not found in finished list: %#v", buildID, finished)

	// Test status filter on finished compose
	status, resp, err := GetComposeStatusV0(testState.socket, "*", "", "FINISHED", "")
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.True(t, UUIDInComposeResults(buildID, status), "%s not found in status list: %#v", buildID, status)

	// Test status of build id
	status, resp, err = GetComposeStatusV0(testState.socket, buildID.String(), "", "", "")
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.True(t, UUIDInComposeResults(buildID, status), "%s not found in status list: %#v", buildID, status)

	// Test status filter using FAILED, should not be listed
	status, resp, err = GetComposeStatusV0(testState.socket, "*", "", "FAILED", "")
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.False(t, UUIDInComposeResults(buildID, status))

	// Test compose info for the finished compose
	info, resp, err := GetComposeInfoV0(testState.socket, buildID.String())
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.Equal(t, "FINISHED", info.QueueStatus)
	require.Equal(t, buildID, info.ID)

	// Test requesting the compose logs for the finished build
	resp, err = WriteComposeLogsV0(testState.socket, io.Discard, buildID.String())
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)

	// Test requesting the compose metadata for the finished build
	resp, err = WriteComposeMetadataV0(testState.socket, io.Discard, buildID.String())
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)

	// Test requesting the compose results for the finished build
	resp, err = WriteComposeResultsV0(testState.socket, io.Discard, buildID.String())
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)

	// Test canceling the finished compose
	cancelStatus, resp, err := CancelComposeV0(testState.socket, buildID.String())
	require.NoError(t, err, "failed with a client error")
	assert.Equal(t, weldr.CancelComposeStatusV0{}, cancelStatus)
	require.NotNil(t, resp)
	require.False(t, resp.Status, "Cancel did not fail")
	require.Equal(t, 1, len(resp.Errors), "%#v", resp)
	require.Equal(t, "BuildInWrongState", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, buildID.String())
}

func TestComposeSupportedMountPointV0(t *testing.T) {

	bp := `
		name="test-compose-supported-mountpoint-v0"
		description="TestComposeSupportedMountPointV0"
		version="0.0.1"
		[[customizations.filesystem]]
		mountpoint = "/"
		size = 4294967296
		`
	resp, err := PostTOMLBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.True(t, resp.Status, "POST failed: %#v", resp)

	compose := fmt.Sprintf(`{
		"blueprint_name": "test-compose-supported-mountpoint-v0",
		"compose_type": "%s",
		"branch": "master"
	}`, testState.imageTypeName)

	// Create a finished test compose
	body, resp, err := PostJSON(testState.socket, "/api/v1/compose?test=2", compose)
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)

	response, err := NewComposeResponseV0(body)
	require.NoError(t, err, "failed with a client error")
	require.True(t, response.Status, "POST failed: %#v", response)
	buildID := response.BuildID

	// Wait until the build is not listed in the queue
	resp, err = WaitForBuild(testState.socket, buildID)
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)

	// Test failed after compose (should not have failed)
	failed, resp, err := GetFailedComposesV0(testState.socket)
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.False(t, UUIDInComposeResults(buildID, failed))

	// Test finished after compose (should have finished)
	finished, resp, err := GetFinishedComposesV0(testState.socket)
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.True(t, UUIDInComposeResults(buildID, finished), "%s not found in finished list: %#v", buildID, finished)

	// Test status filter on finished compose
	status, resp, err := GetComposeStatusV0(testState.socket, "*", "", "FINISHED", "")
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.True(t, UUIDInComposeResults(buildID, status), "%s not found in status list: %#v", buildID, status)

	// Test status of build id
	status, resp, err = GetComposeStatusV0(testState.socket, buildID.String(), "", "", "")
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.True(t, UUIDInComposeResults(buildID, status), "%s not found in status list: %#v", buildID, status)

	// Test status filter using FAILED, should not be listed
	status, resp, err = GetComposeStatusV0(testState.socket, "*", "", "FAILED", "")
	require.NoError(t, err, "failed with a client error")
	require.Nil(t, resp)
	require.False(t, UUIDInComposeResults(buildID, status))

}

func TestComposeUnsupportedMountPointV0(t *testing.T) {
	bp := `
		name="test-compose-unsupported-mountpoint-v0"
		description="TestComposeUnsupportedMountPointV0"
		version="0.0.1"
		[[customizations.filesystem]]
		mountpoint = "/etc"
		size = 4294967296
		`
	resp, err := PostTOMLBlueprintV0(testState.socket, bp)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp)

	compose := fmt.Sprintf(`{
		"blueprint_name": "test-compose-unsupported-mountpoint-v0",
		"compose_type": "%s",
		"branch": "master"
	}`, testState.imageTypeName)

	// Create a finished test compose
	body, resp, err := PostJSON(testState.socket, "/api/v1/compose?test=2", compose)
	require.NoError(t, err, "failed with a client error")
	require.NotNil(t, resp)
	require.Equal(t, "ManifestCreationFailed", resp.Errors[0].ID)
	require.Contains(t, resp.Errors[0].Msg, "path \"/etc\" is not allowed")
	require.Equal(t, 0, len(body))
}
