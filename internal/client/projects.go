// Package client - projects contains functions for the projects API
// Copyright (C) 2020 by Red Hat, Inc.
package client

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/weldr"
)

// ListAllProjectsV0 returns a list of all the available project names
func ListAllProjectsV0(socket *http.Client) ([]weldr.PackageInfo, *APIResponse, error) {
	body, resp, err := GetJSONAll(socket, "/api/v0/projects/list")
	if resp != nil || err != nil {
		return nil, resp, err
	}
	var list weldr.ProjectsListV0
	err = json.Unmarshal(body, &list)
	if err != nil {
		return nil, nil, err
	}
	return list.Projects, nil, nil
}

// ListSomeProjectsV0 returns a list of all the available project names
func ListSomeProjectsV0(socket *http.Client, offset, limit int) ([]weldr.PackageInfo, *APIResponse, error) {
	path := fmt.Sprintf("/api/v0/projects/list?offset=%d&limit=%d", offset, limit)
	body, resp, err := GetRaw(socket, "GET", path)
	if resp != nil || err != nil {
		return nil, resp, err
	}
	var list weldr.ProjectsListV0
	err = json.Unmarshal(body, &list)
	if err != nil {
		return nil, nil, err
	}
	return list.Projects, nil, nil
}

// GetProjectsInfoV0 returns detailed project info on the named projects
func GetProjectsInfoV0(socket *http.Client, projNames string) ([]weldr.PackageInfo, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/projects/info/"+projNames)
	if resp != nil || err != nil {
		return nil, resp, err
	}
	var list weldr.ProjectsInfoV0
	err = json.Unmarshal(body, &list)
	if err != nil {
		return nil, nil, err
	}
	return list.Projects, nil, nil
}

// DepsolveProjectsV0 returns the dependencies of the names projects
func DepsolveProjectsV0(socket *http.Client, projNames string) ([]rpmmd.PackageSpec, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/projects/depsolve/"+projNames)
	if resp != nil || err != nil {
		return nil, resp, err
	}
	var deps weldr.ProjectsDependenciesV0
	err = json.Unmarshal(body, &deps)
	if err != nil {
		return nil, nil, err
	}
	return deps.Projects, nil, nil
}
