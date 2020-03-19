// Package weldr - json contains Exported API request/response structures
// Copyright (C) 2020 by Red Hat, Inc.
package weldr

import (
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

// StatusV0 is the response to /api/status from a v0+ server
type StatusV0 struct {
	API           string   `json:"api"`
	DBSupported   bool     `json:"db_supported"`
	DBVersion     string   `json:"db_version"`
	SchemaVersion string   `json:"schema_version"`
	Backend       string   `json:"backend"`
	Build         string   `json:"build"`
	Messages      []string `json:"messages"`
}

// BlueprintsListV0 is the response to /blueprints/list request
type BlueprintsListV0 struct {
	Total      uint     `json:"total"`
	Offset     uint     `json:"offset"`
	Limit      uint     `json:"limit"`
	Blueprints []string `json:"blueprints"`
}

// ResponseError holds the API response error details
type ResponseError struct {
	Code int    `json:"code,omitempty"`
	ID   string `json:"id"`
	Msg  string `json:"msg"`
}

// BlueprintsInfoV0 is the response to /blueprints/info?format=json request
type BlueprintsInfoV0 struct {
	Blueprints []blueprint.Blueprint `json:"blueprints"`
	Changes    []infoChange          `json:"changes"`
	Errors     []ResponseError       `json:"errors"`
}
type infoChange struct {
	Changed bool   `json:"changed"`
	Name    string `json:"name"`
}

// BlueprintsChangesV0 is the response to /blueprints/changes/ request
type BlueprintsChangesV0 struct {
	BlueprintsChanges []bpChange      `json:"blueprints"`
	Errors            []ResponseError `json:"errors"`
	Limit             uint            `json:"limit"`
	Offset            uint            `json:"offset"`
}
type bpChange struct {
	Changes []blueprint.Change `json:"changes"`
	Name    string             `json:"name"`
	Total   int                `json:"total"`
}

// BlueprintsDepsolveV0 is the response to /blueprints/depsolve/ request
type BlueprintsDepsolveV0 struct {
	Blueprints []depsolveEntry `json:"blueprints"`
	Errors     []ResponseError `json:"errors"`
}
type depsolveEntry struct {
	Blueprint    blueprint.Blueprint `json:"blueprint"`
	Dependencies []rpmmd.PackageSpec `json:"dependencies"`
}

// BlueprintsFreezeV0 is the response to /blueprints/freeze/ request
type BlueprintsFreezeV0 struct {
	Blueprints []blueprintFrozen `json:"blueprints"`
	Errors     []ResponseError   `json:"errors"`
}
type blueprintFrozen struct {
	Blueprint blueprint.Blueprint `json:"blueprint"`
}
