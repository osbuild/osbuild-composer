// Package client - modules_test contains functions to check the modules API
// Copyright (C) 2020 by Red Hat, Inc.

// Tests should be self-contained and not depend on the state of the server
// They should use their own blueprints, not the default blueprints
// They should not assume version numbers for packages will match
// They should run tests that depend on previous results from the same function
// not from other functions.
package client

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// List all the modules
func TestListAllModulesV0(t *testing.T) {
	modules, api, err := ListAllModulesV0(testState.socket)
	require.NoError(t, err)
	require.Nil(t, api, "ListAllModules failed: %#v", api)
	require.True(t, len(modules) > 1, "Not enough modules returned")
}

// List some modules
func TestListSomeModulesV0(t *testing.T) {
	modules, api, err := ListSomeModulesV0(testState.socket, 0, 5)
	require.NoError(t, err)
	require.Nil(t, api, "ListSomeProjects failed: %#v", api)
	require.True(t, len(modules) == 5, "Not enough modules returned")
}

// List one module
func TestListOneModulesV0(t *testing.T) {
	modules, api, err := ListModulesV0(testState.socket, "bash")
	require.NoError(t, err)
	require.Nil(t, api, "ListModules failed: %#v", api)
	require.True(t, len(modules) == 1, "Not enough modules returned")
}

// List two modules
func TestListTwoModulesV0(t *testing.T) {
	modules, api, err := ListModulesV0(testState.socket, "bash,tmux")
	require.NoError(t, err)
	require.Nil(t, api, "ListModules failed: %#v", api)
	require.True(t, len(modules) == 2, "Not enough modules returned")
}

// Get info on a specific module
func TestOneModuleInfoV0(t *testing.T) {
	modules, api, err := GetModulesInfoV0(testState.socket, "bash")
	require.NoError(t, err)
	require.Nil(t, api, "GetModulesInfo failed: %#v", api)
	require.True(t, len(modules) == 1, "Not enough modules returned: %#v", modules)
}

// Get info on two specific modules
func TestTwoModuleInfoV0(t *testing.T) {
	modules, api, err := GetModulesInfoV0(testState.socket, "bash,tmux")
	require.NoError(t, err)
	require.Nil(t, api, "GetModulesInfo failed: %#v", api)
	require.True(t, len(modules) == 2, "Not enough modules returned: %#v", modules)
}

// Test an invalid info request
func TestEmptyModuleInfoV0(t *testing.T) {
	modules, api, err := GetModulesInfoV0(testState.socket, "")
	require.NoError(t, err)
	require.NotNil(t, api, "did not return an error")
	require.False(t, api.Status, "wrong Status (true)")
	require.True(t, len(modules) == 0)
}
