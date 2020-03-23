// Package weldrcheck - projects contains functions to check the projects API
// Copyright (C) 2020 by Red Hat, Inc.

// Tests should be self-contained and not depend on the state of the server
// They should use their own blueprints, not the default blueprints
// They should not assume version numbers for packages will match
// They should run tests that depend on previous results from the same function
// not from other functions.

// +build integration

package weldrcheck

import (
	//	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/client"
)

// List all the projects
func TestListAllProjectsV0(t *testing.T) {
	projs, api, err := client.ListAllProjectsV0(testState.socket)
	require.NoError(t, err)
	require.Nil(t, api, "ListAllProjects failed: %#v", api)
	require.True(t, len(projs) > 1, "Not enough projects returned")
}

// List some of the projects
func TestListSomeProjectsV0(t *testing.T) {
	projs, api, err := client.ListSomeProjectsV0(testState.socket, 0, 5)
	require.NoError(t, err)
	require.Nil(t, api, "ListSomeProjects failed: %#v", api)
	require.True(t, len(projs) == 5, "Not enough projects returned")
}

// Get info on a specific project
func TestOneProjectsInfoV0(t *testing.T) {
	projs, api, err := client.GetProjectsInfoV0(testState.socket, "bash")
	require.NoError(t, err)
	require.Nil(t, api, "GetProjectsInfo failed: %#v", api)
	require.True(t, len(projs) == 1, "Not enough projects returned")
}

// Get info on a two specific projects
func TestTwoProjectsInfoV0(t *testing.T) {
	projs, api, err := client.GetProjectsInfoV0(testState.socket, "bash,tmux")
	require.NoError(t, err)
	require.Nil(t, api, "GetProjectsInfo failed: %#v", api)
	require.True(t, len(projs) == 2, "Not enough projects returned")
}

// Test an invalid info request
func TestEmptyProjectsInfoV0(t *testing.T) {
	projs, api, err := client.GetProjectsInfoV0(testState.socket, "")
	require.NoError(t, err)
	require.NotNil(t, api, "did not return an error")
	require.False(t, api.Status, "wrong Status (true)")
	require.True(t, len(projs) == 0)
}

// Depsolve projects
func TestDepsolveOneProjectV0(t *testing.T) {
	deps, api, err := client.DepsolveProjectsV0(testState.socket, "bash")
	require.NoError(t, err)
	require.Nil(t, api, "DepsolveProjects failed: %#v", api)
	require.True(t, len(deps) > 2, "Not enough dependencies returned")
}

// Depsolve several projects
func TestDepsolveTwoProjectV0(t *testing.T) {
	deps, api, err := client.DepsolveProjectsV0(testState.socket, "bash,tmux")
	require.NoError(t, err)
	require.Nil(t, api, "DepsolveProjects failed: %#v", api)
	require.True(t, len(deps) > 2, "Not enough dependencies returned")
}

// Depsolve empty projects list
func TestEmptyDepsolveProjectV0(t *testing.T) {
	deps, api, err := client.DepsolveProjectsV0(testState.socket, "")
	require.NoError(t, err)
	require.NotNil(t, api, "did not return an error")
	require.False(t, api.Status, "wrong Status (true)")
	require.True(t, len(deps) == 0)
}
