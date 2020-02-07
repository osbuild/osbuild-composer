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
func PostTOMLBlueprintV0(socket, blueprint string) *APIResponse {
	body, err := PostTOML(socket, "/api/v0/blueprints/new", blueprint)
	if err != nil {
		return err
	}
	return NewAPIResponse(body)
}

// PostTOMLWorkspaceV0 sends a TOML blueprint string to the API
// and returns an APIResponse
func PostTOMLWorkspaceV0(socket, blueprint string) *APIResponse {
	body, err := PostTOML(socket, "/api/v0/blueprints/workspace", blueprint)
	if err != nil {
		return err
	}
	return NewAPIResponse(body)
}

// PostJSONBlueprintV0 sends a JSON blueprint string to the API
// and returns an APIResponse
func PostJSONBlueprintV0(socket, blueprint string) *APIResponse {
	body, err := PostJSON(socket, "/api/v0/blueprints/new", blueprint)
	if err != nil {
		return err
	}
	return NewAPIResponse(body)
}

// PostJSONWorkspaceV0 sends a JSON blueprint string to the API
// and returns an APIResponse
func PostJSONWorkspaceV0(socket, blueprint string) *APIResponse {
	body, err := PostJSON(socket, "/api/v0/blueprints/workspace", blueprint)
	if err != nil {
		return err
	}
	return NewAPIResponse(body)
}

// DeleteBlueprintV0 deletes the named blueprint and returns an APIResponse
func DeleteBlueprintV0(socket, bpName string) *APIResponse {
	body, err := DeleteRaw(socket, "/api/v0/blueprints/delete/"+bpName)
	if err != nil {
		return err
	}
	return NewAPIResponse(body)
}

// DeleteWorkspaceV0 deletes the named blueprint's workspace and returns an APIResponse
func DeleteWorkspaceV0(socket, bpName string) *APIResponse {
	body, err := DeleteRaw(socket, "/api/v0/blueprints/workspace/"+bpName)
	if err != nil {
		return err
	}
	return NewAPIResponse(body)
}

// ListBlueprintsV0 returns a list of blueprint names
func ListBlueprintsV0(socket string) ([]string, *APIResponse) {
	body, err := GetJSONAll(socket, "/api/v0/blueprints/list")
	if err != nil {
		return nil, err
	}
	var resp BlueprintsListV0
	jerr := json.Unmarshal(body, &resp)
	if jerr != nil {
		return nil, clientError(err)
	}
	return resp.Blueprints, nil
}

// GetBlueprintInfoTOMLV0 returns the requested blueprint as a TOML string
func GetBlueprintInfoTOMLV0(socket, bpName string) (string, *APIResponse) {
	body, err := GetRaw(socket, "GET", "/api/v0/blueprints/info/"+bpName+"?format=toml")
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// GetBlueprintsInfoJSONV0 returns the requested blueprints and their changed state
func GetBlueprintsInfoJSONV0(socket, bpName string) (BlueprintsInfoV0, *APIResponse) {
	body, err := GetRaw(socket, "GET", "/api/v0/blueprints/info/"+bpName)
	if err != nil {
		return BlueprintsInfoV0{}, err
	}
	var resp BlueprintsInfoV0
	jerr := json.Unmarshal(body, &resp)
	if jerr != nil {
		return BlueprintsInfoV0{}, clientError(err)
	}
	return resp, nil
}

// GetBlueprintsChangesV0 returns the changes to the listed blueprints
func GetBlueprintsChangesV0(socket string, bpNames []string) (BlueprintsChangesV0, *APIResponse) {
	names := strings.Join(bpNames, ",")
	body, err := GetRaw(socket, "GET", "/api/v0/blueprints/changes/"+names)
	if err != nil {
		return BlueprintsChangesV0{}, err
	}
	var resp BlueprintsChangesV0
	jerr := json.Unmarshal(body, &resp)
	if jerr != nil {
		return BlueprintsChangesV0{}, clientError(err)
	}
	return resp, nil
}

// UndoBlueprintChangeV0 reverts a blueprint to a previous commit
func UndoBlueprintChangeV0(socket, blueprint, commit string) *APIResponse {
	request := fmt.Sprintf("/api/v0/blueprints/undo/%s/%s", blueprint, commit)
	body, err := PostRaw(socket, request, "", nil)
	if err != nil {
		return err
	}
	return NewAPIResponse(body)
}

// TagBlueprintV0 tags the current blueprint commit as a new revision
func TagBlueprintV0(socket, blueprint string) *APIResponse {
	body, err := PostRaw(socket, "/api/v0/blueprints/tag/"+blueprint, "", nil)
	if err != nil {
		return err
	}
	return NewAPIResponse(body)
}

// DepsolveBlueprintV0 depsolves the listed blueprint
func DepsolveBlueprintV0(socket, blueprint string) (BlueprintsDepsolveV0, *APIResponse) {
	body, err := GetRaw(socket, "GET", "/api/v0/blueprints/depsolve/"+blueprint)
	if err != nil {
		return BlueprintsDepsolveV0{}, err
	}
	var resp BlueprintsDepsolveV0
	jerr := json.Unmarshal(body, &resp)
	if jerr != nil {
		return BlueprintsDepsolveV0{}, clientError(err)
	}
	return resp, nil
}

// FreezeBlueprintV0 depsolves the listed blueprint and returns the blueprint with frozen package
// versions
func FreezeBlueprintV0(socket, blueprint string) (BlueprintsFreezeV0, *APIResponse) {
	body, err := GetRaw(socket, "GET", "/api/v0/blueprints/freeze/"+blueprint)
	if err != nil {
		return BlueprintsFreezeV0{}, err
	}
	var resp BlueprintsFreezeV0
	jerr := json.Unmarshal(body, &resp)
	if jerr != nil {
		return BlueprintsFreezeV0{}, clientError(err)
	}
	return resp, nil
}
