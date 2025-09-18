// Package client - modules contains functions for the modules API
// Copyright (C) 2020 by Red Hat, Inc.
package client

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/osbuild/osbuild-composer/internal/weldr"
)

// ListAllModulesV0 returns a list of all the available module names
func ListAllModulesV0(socket *http.Client) ([]weldr.ModuleName, *APIResponse, error) {
	body, resp, err := GetJSONAll(socket, "/api/v0/modules/list")
	if resp != nil || err != nil {
		return nil, resp, err
	}
	var list weldr.ModulesListV0
	err = json.Unmarshal(body, &list)
	if err != nil {
		return nil, nil, err
	}
	return list.Modules, nil, nil
}

// ListSomeModulesV0 returns a list of all the available modules names
func ListSomeModulesV0(socket *http.Client, offset, limit int) ([]weldr.ModuleName, *APIResponse, error) {
	path := fmt.Sprintf("/api/v0/modules/list?offset=%d&limit=%d", offset, limit)
	body, resp, err := GetRaw(socket, "GET", path)
	if resp != nil || err != nil {
		return nil, resp, err
	}
	var list weldr.ModulesListV0
	err = json.Unmarshal(body, &list)
	if err != nil {
		return nil, nil, err
	}
	return list.Modules, nil, nil
}

// ListModulesV0 returns a list of all the available modules names
func ListModulesV0(socket *http.Client, moduleNames string) ([]weldr.ModuleName, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/modules/list/"+moduleNames)
	if resp != nil || err != nil {
		return nil, resp, err
	}
	var list weldr.ModulesListV0
	err = json.Unmarshal(body, &list)
	if err != nil {
		return nil, nil, err
	}
	return list.Modules, nil, nil
}

// GetModulesInfoV0 returns detailed module info on the named modules
func GetModulesInfoV0(socket *http.Client, modulesNames string) ([]weldr.PackageInfo, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/modules/info/"+modulesNames)
	if resp != nil || err != nil {
		return nil, resp, err
	}
	var list weldr.ModulesInfoV0
	err = json.Unmarshal(body, &list)
	if err != nil {
		return nil, nil, err
	}
	return list.Modules, nil, nil
}
