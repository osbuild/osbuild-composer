// Package client - blueprints contains functions for the blueprint API
// Copyright (C) 2020 by Red Hat, Inc.
package client

import (
	"encoding/json"
	"fmt"
	"strings"
)

// PostTOMLBlueprintV0 sends a TOML blueprint string to the API
// and returns an APIResponse
func PostTOMLBlueprintV0(socket, blueprint string) (*APIResponse, error) {
	body, resp, err := PostTOML(socket, "/api/v0/blueprints/new", blueprint)
	if resp != nil || err != nil {
		return resp, err
	}
	return NewAPIResponse(body)
}

// PostTOMLWorkspaceV0 sends a TOML blueprint string to the API
// and returns an APIResponse
func PostTOMLWorkspaceV0(socket, blueprint string) (*APIResponse, error) {
	body, resp, err := PostTOML(socket, "/api/v0/blueprints/workspace", blueprint)
	if resp != nil || err != nil {
		return resp, err
	}
	return NewAPIResponse(body)
}

// PostJSONBlueprintV0 sends a JSON blueprint string to the API
// and returns an APIResponse
func PostJSONBlueprintV0(socket, blueprint string) (*APIResponse, error) {
	body, resp, err := PostJSON(socket, "/api/v0/blueprints/new", blueprint)
	if resp != nil || err != nil {
		return resp, err
	}
	return NewAPIResponse(body)
}

// PostJSONWorkspaceV0 sends a JSON blueprint string to the API
// and returns an APIResponse
func PostJSONWorkspaceV0(socket, blueprint string) (*APIResponse, error) {
	body, resp, err := PostJSON(socket, "/api/v0/blueprints/workspace", blueprint)
	if resp != nil || err != nil {
		return resp, err
	}
	return NewAPIResponse(body)
}

// DeleteBlueprintV0 deletes the named blueprint and returns an APIResponse
func DeleteBlueprintV0(socket, bpName string) (*APIResponse, error) {
	body, resp, err := DeleteRaw(socket, "/api/v0/blueprints/delete/"+bpName)
	if resp != nil || err != nil {
		return resp, err
	}
	return NewAPIResponse(body)
}

// DeleteWorkspaceV0 deletes the named blueprint's workspace and returns an APIResponse
func DeleteWorkspaceV0(socket, bpName string) (*APIResponse, error) {
	body, resp, err := DeleteRaw(socket, "/api/v0/blueprints/workspace/"+bpName)
	if resp != nil || err != nil {
		return resp, err
	}
	return NewAPIResponse(body)
}

// ListBlueprintsV0 returns a list of blueprint names
func ListBlueprintsV0(socket string) ([]string, *APIResponse, error) {
	body, resp, err := GetJSONAll(socket, "/api/v0/blueprints/list")
	if resp != nil || err != nil {
		return nil, resp, err
	}
	var list BlueprintsListV0
	err = json.Unmarshal(body, &list)
	if err != nil {
		return nil, nil, err
	}
	return list.Blueprints, nil, nil
}

// GetBlueprintInfoTOMLV0 returns the requested blueprint as a TOML string
func GetBlueprintInfoTOMLV0(socket, bpName string) (string, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/blueprints/info/"+bpName+"?format=toml")
	if resp != nil || err != nil {
		return "", resp, err
	}
	return string(body), nil, nil
}

// GetBlueprintsInfoJSONV0 returns the requested blueprints and their changed state
func GetBlueprintsInfoJSONV0(socket, bpName string) (BlueprintsInfoV0, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/blueprints/info/"+bpName)
	if resp != nil || err != nil {
		return BlueprintsInfoV0{}, resp, err
	}
	var info BlueprintsInfoV0
	err = json.Unmarshal(body, &info)
	if err != nil {
		return BlueprintsInfoV0{}, nil, err
	}
	return info, nil, nil
}

// GetBlueprintsChangesV0 returns the changes to the listed blueprints
func GetBlueprintsChangesV0(socket string, bpNames []string) (BlueprintsChangesV0, *APIResponse, error) {
	names := strings.Join(bpNames, ",")
	body, resp, err := GetRaw(socket, "GET", "/api/v0/blueprints/changes/"+names)
	if resp != nil || err != nil {
		return BlueprintsChangesV0{}, resp, err
	}
	var changes BlueprintsChangesV0
	err = json.Unmarshal(body, &changes)
	if err != nil {
		return BlueprintsChangesV0{}, nil, err
	}
	return changes, nil, nil
}

// UndoBlueprintChangeV0 reverts a blueprint to a previous commit
func UndoBlueprintChangeV0(socket, blueprint, commit string) (*APIResponse, error) {
	request := fmt.Sprintf("/api/v0/blueprints/undo/%s/%s", blueprint, commit)
	body, resp, err := PostRaw(socket, request, "", nil)
	if resp != nil || err != nil {
		return resp, err
	}
	return NewAPIResponse(body)
}

// TagBlueprintV0 tags the current blueprint commit as a new revision
func TagBlueprintV0(socket, blueprint string) (*APIResponse, error) {
	body, resp, err := PostRaw(socket, "/api/v0/blueprints/tag/"+blueprint, "", nil)
	if resp != nil || err != nil {
		return resp, err
	}
	return NewAPIResponse(body)
}

// DepsolveBlueprintV0 depsolves the listed blueprint
func DepsolveBlueprintV0(socket, blueprint string) (BlueprintsDepsolveV0, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/blueprints/depsolve/"+blueprint)
	if resp != nil || err != nil {
		return BlueprintsDepsolveV0{}, resp, err
	}
	var deps BlueprintsDepsolveV0
	err = json.Unmarshal(body, &deps)
	if err != nil {
		return BlueprintsDepsolveV0{}, nil, err
	}
	return deps, nil, nil
}

// FreezeBlueprintV0 depsolves the listed blueprint and returns the blueprint with frozen package
// versions
func FreezeBlueprintV0(socket, blueprint string) (BlueprintsFreezeV0, *APIResponse, error) {
	body, resp, err := GetRaw(socket, "GET", "/api/v0/blueprints/freeze/"+blueprint)
	if resp != nil || err != nil {
		return BlueprintsFreezeV0{}, resp, err
	}
	var frozen BlueprintsFreezeV0
	err = json.Unmarshal(body, &frozen)
	if err != nil {
		return BlueprintsFreezeV0{}, nil, err
	}
	return frozen, nil, nil
}
