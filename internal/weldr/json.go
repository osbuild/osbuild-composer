// Package weldr - json contains Exported API request/response structures
// Copyright (C) 2020 by Red Hat, Inc.
package weldr

import (
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/store"
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

// SourceListV0 is the response to /source/list request
type SourceListV0 struct {
	Sources []string `json:"sources"`
}

// SourceInfoV0 is the response to a /source/info request
type SourceInfoV0 struct {
	Sources map[string]SourceConfigV0 `json:"sources"`
	Errors  []ResponseError           `json:"errors"`
}

// SourceConfig returns a SourceConfig struct populated with the supported variables
func (s *SourceInfoV0) SourceConfig(sourceName string) (ssc store.SourceConfig, ok bool) {
	si, ok := s.Sources[sourceName]
	if !ok {
		return ssc, false
	}

	return si.SourceConfig(), true
}

// SourceConfigV0 holds the source repository information
type SourceConfigV0 struct {
	Name     string   `json:"name" toml:"name"`
	Type     string   `json:"type" toml:"type"`
	URL      string   `json:"url" toml:"url"`
	CheckGPG bool     `json:"check_gpg" toml:"check_gpg"`
	CheckSSL bool     `json:"check_ssl" toml:"check_ssl"`
	System   bool     `json:"system" toml:"system"`
	Proxy    string   `json:"proxy" toml:"proxy"`
	GPGUrls  []string `json:"gpgkey_urls" toml:"gpgkey_urls"`
}

// SourceConfig returns a SourceConfig struct populated with the supported variables
// The store does not support proxy and gpgkey_urls
func (s *SourceConfigV0) SourceConfig() (ssc store.SourceConfig) {
	ssc.Name = s.Name
	ssc.Type = s.Type
	ssc.URL = s.URL
	ssc.CheckGPG = s.CheckGPG
	ssc.CheckSSL = s.CheckSSL

	return ssc
}

// ProjectsListV0 is the response to /projects/list request
type ProjectsListV0 struct {
	Total    uint                `json:"total"`
	Offset   uint                `json:"offset"`
	Limit    uint                `json:"limit"`
	Projects []rpmmd.PackageInfo `json:"projects"`
}

// ProjectsInfoV0 is the response to /projects/info request
type ProjectsInfoV0 struct {
	Projects []rpmmd.PackageInfo `json:"projects"`
}

// ProjectsDependenciesV0 is the response to /projects/depsolve request
type ProjectsDependenciesV0 struct {
	Projects []rpmmd.PackageSpec `json:"projects"`
}

type ModuleName struct {
	Name      string `json:"name"`
	GroupType string `json:"group_type"`
}

type ModulesListV0 struct {
	Total   uint         `json:"total"`
	Offset  uint         `json:"offset"`
	Limit   uint         `json:"limit"`
	Modules []ModuleName `json:"modules"`
}

// ModulesInfoV0 is the response to /modules/info request
type ModulesInfoV0 struct {
	Modules []rpmmd.PackageInfo `json:"modules"`
}
