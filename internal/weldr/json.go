// Package weldr - json contains Exported API request/response structures
// Copyright (C) 2020 by Red Hat, Inc.
package weldr

import (
	"github.com/google/uuid"

	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
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

// BlueprintsChangesV0Weldr is the response to /blueprints/changes/ request using weldr-client
type BlueprintsChangesV0Weldr struct {
	Body BlueprintsChangesV0 `json:"body"`
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

// SourceListV1 is the response to /source/list request
type SourceListV1 struct {
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

// SourceConfig interface defines the common functions needed to query the SourceConfigV0/V1 structs
type SourceConfig interface {
	GetKey() string
	GetName() string
	GetType() string
	SourceConfig() store.SourceConfig
}

// NewSourceConfigV0 converts a store.SourceConfig to a SourceConfigV0
// The store does not support proxy and gpgkeys
func NewSourceConfigV0(s store.SourceConfig) SourceConfigV0 {
	var sc SourceConfigV0

	sc.Name = s.Name
	sc.Type = s.Type
	sc.URL = s.URL
	sc.CheckGPG = s.CheckGPG
	sc.CheckSSL = s.CheckSSL
	sc.System = s.System

	return sc
}

// SourceConfigV0 holds the source repository information
type SourceConfigV0 struct {
	Name           string   `json:"name" toml:"name"`
	Type           string   `json:"type" toml:"type"`
	URL            string   `json:"url" toml:"url"`
	CheckGPG       bool     `json:"check_gpg" toml:"check_gpg"`
	CheckSSL       bool     `json:"check_ssl" toml:"check_ssl"`
	System         bool     `json:"system" toml:"system"`
	Proxy          string   `json:"proxy,omitempty" toml:"proxy,omitempty"`
	GPGKeys        []string `json:"gpgkeys,omitempty" toml:"gpgkeys,omitempty"`
	ModuleHotfixes *bool    `json:"module_hotfixes,omitempty"`
}

// Key return the key, .Name in this case
func (s SourceConfigV0) GetKey() string {
	return s.Name
}

// Name return the .Name field
func (s SourceConfigV0) GetName() string {
	return s.Name
}

// Type return the .Type field
func (s SourceConfigV0) GetType() string {
	return s.Type
}

// SourceConfig returns a SourceConfig struct populated with the supported variables
// The store does not support proxy and gpgkeys
func (s SourceConfigV0) SourceConfig() (ssc store.SourceConfig) {
	ssc.Name = s.Name
	ssc.Type = s.Type
	ssc.URL = s.URL
	ssc.CheckGPG = s.CheckGPG
	ssc.CheckSSL = s.CheckSSL
	if s.ModuleHotfixes != nil {
		modHotfixesVal := *s.ModuleHotfixes
		ssc.ModuleHotfixes = &modHotfixesVal
	}

	return ssc
}

// SourceInfoResponseV0
type SourceInfoResponseV0 struct {
	Sources map[string]SourceConfigV0 `json:"sources"`
	Errors  []responseError           `json:"errors"`
}

// NewSourceConfigV1 converts a store.SourceConfig to a SourceConfigV1
// The store does not support proxy and gpgkeys
func NewSourceConfigV1(id string, s store.SourceConfig) SourceConfigV1 {
	var sc SourceConfigV1

	sc.ID = id
	sc.Name = s.Name
	sc.Type = s.Type
	sc.URL = s.URL
	sc.CheckGPG = s.CheckGPG
	sc.CheckSSL = s.CheckSSL
	sc.System = s.System
	sc.Distros = s.Distros
	sc.RHSM = s.RHSM
	sc.CheckRepoGPG = s.CheckRepoGPG
	sc.GPGKeys = s.GPGKeys
	if s.ModuleHotfixes != nil {
		modHotfixesVal := *s.ModuleHotfixes
		sc.ModuleHotfixes = &modHotfixesVal
	}

	return sc
}

// SourceConfigV1 holds the source repository information
type SourceConfigV1 struct {
	ID             string   `json:"id" toml:"id"`
	Name           string   `json:"name" toml:"name"`
	Type           string   `json:"type" toml:"type"`
	URL            string   `json:"url" toml:"url"`
	CheckGPG       bool     `json:"check_gpg" toml:"check_gpg"`
	CheckSSL       bool     `json:"check_ssl" toml:"check_ssl"`
	System         bool     `json:"system" toml:"system"`
	Proxy          string   `json:"proxy,omitempty" toml:"proxy,omitempty"`
	GPGKeys        []string `json:"gpgkeys,omitempty" toml:"gpgkeys,omitempty"`
	Distros        []string `json:"distros,omitempty" toml:"distros,omitempty"`
	RHSM           bool     `json:"rhsm" toml:"rhsm"`
	CheckRepoGPG   bool     `json:"check_repogpg" toml:"check_repogpg"`
	ModuleHotfixes *bool    `json:"module_hotfixes,omitempty" toml:"module_hotfixes,omitempty"`
}

// Key returns the key, .ID in this case
func (s SourceConfigV1) GetKey() string {
	return s.ID
}

// Name return the .Name field
func (s SourceConfigV1) GetName() string {
	return s.Name
}

// Type return the .Type field
func (s SourceConfigV1) GetType() string {
	return s.Type
}

// SourceConfig returns a SourceConfig struct populated with the supported variables
// The store does not support proxy and gpgkeys
func (s SourceConfigV1) SourceConfig() (ssc store.SourceConfig) {
	ssc.Name = s.Name
	ssc.Type = s.Type
	ssc.URL = s.URL
	ssc.CheckGPG = s.CheckGPG
	ssc.CheckSSL = s.CheckSSL
	ssc.Distros = s.Distros
	ssc.RHSM = s.RHSM
	ssc.CheckRepoGPG = s.CheckRepoGPG
	ssc.GPGKeys = s.GPGKeys
	if s.ModuleHotfixes != nil {
		modHotfixesVal := *s.ModuleHotfixes
		ssc.ModuleHotfixes = &modHotfixesVal
	}

	return ssc
}

// SourceInfoResponseV1
type SourceInfoResponseV1 struct {
	Sources map[string]SourceConfigV1 `json:"sources"`
	Errors  []responseError           `json:"errors"`
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

type ComposeRequestV0 struct {
	BlueprintName string `json:"blueprint_name"`
	ComposeType   string `json:"compose_type"`
	Branch        string `json:"branch"`
}
type ComposeResponseV0 struct {
	BuildID uuid.UUID `json:"build_id"`
	Status  bool      `json:"status"`
}

// This is similar to weldr.ComposeEntry but different because internally the image types are capitalized
type ComposeEntryV0 struct {
	ID          uuid.UUID              `json:"id"`
	Blueprint   string                 `json:"blueprint"`
	Version     string                 `json:"version"`
	ComposeType string                 `json:"compose_type"`
	ImageSize   uint64                 `json:"image_size"` // This is user-provided image size, not actual file size
	QueueStatus common.ImageBuildState `json:"queue_status"`
	JobCreated  float64                `json:"job_created"`
	JobStarted  float64                `json:"job_started,omitempty"`
	JobFinished float64                `json:"job_finished,omitempty"`
	Uploads     []uploadResponse       `json:"uploads,omitempty"`
}

type ComposeFinishedResponseV0 struct {
	Finished []ComposeEntryV0 `json:"finished"`
}
type ComposeFailedResponseV0 struct {
	Failed []ComposeEntryV0 `json:"failed"`
}
type ComposeStatusResponseV0 struct {
	UUIDs []ComposeEntryV0 `json:"uuids"`
}

type ComposeTypeV0 struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type ComposeTypesResponseV0 struct {
	Types []ComposeTypeV0 `json:"types"`
}

type DeleteComposeStatusV0 struct {
	UUID   uuid.UUID `json:"uuid"`
	Status bool      `json:"status"`
}

type DeleteComposeResponseV0 struct {
	UUIDs  []DeleteComposeStatusV0 `json:"uuids"`
	Errors []ResponseError         `json:"errors"`
}

type CancelComposeStatusV0 struct {
	UUID   uuid.UUID `json:"uuid"`
	Status bool      `json:"status"`
}

// NOTE: This does not include the lorax-composer specific 'config' field
type ComposeInfoResponseV0 struct {
	ID        uuid.UUID            `json:"id"`
	Blueprint *blueprint.Blueprint `json:"blueprint"` // blueprint not frozen!
	Commit    string               `json:"commit"`    // empty for now
	Deps      struct {
		Packages []rpmmd.Package `json:"packages"`
	} `json:"deps"`
	ComposeType string           `json:"compose_type"`
	QueueStatus string           `json:"queue_status"`
	ImageSize   uint64           `json:"image_size"`
	Uploads     []uploadResponse `json:"uploads,omitempty"`
}

type ComposeQueueResponseV0 struct {
	New []ComposeEntryV0 `json:"new"`
	Run []ComposeEntryV0 `json:"run"`
}
