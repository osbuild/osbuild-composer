// Package weldrcheck - blueprints contains functions to check the blueprints API
// Copyright (C) 2020 by Red Hat, Inc.
package weldrcheck

import (
	"log"
	"reflect"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/osbuild/osbuild-composer/internal/client"
)

type checkBlueprintsV0 struct {
	socket string
}

// Run will execute the API V0 Blueprint check functions
// This will call all of the methods that start with 'Check', passing them a pointer to a
// checkBlueprintsV0 struct and expecting a bool to be returned to indicate whether or not the test
// passed.
// If any of the tests fail it will return false.
func (c *checkBlueprintsV0) Run() bool {
	pass := true

	// Construct a reflect.Type to use for checking method type signatures
	boolType := reflect.TypeOf(true)
	structType := reflect.TypeOf(c)
	funcType := reflect.FuncOf([]reflect.Type{structType}, []reflect.Type{boolType}, false)

	structValue := reflect.ValueOf(c)
	// Get all the exported methods on this struct named 'Check*' and run them
	for i := 0; i < structType.NumMethod(); i++ {
		method := structType.Method(i)
		// Make sure it starts with Check and matches the type signature
		if strings.HasPrefix(method.Name, "Check") {
			if method.Type != funcType {
				log.Printf("ERROR: Check function '%s' has wrong type: %s", method.Name, method.Type)
				pass = false
				continue
			}

			r := structValue.Method(i).Call(nil)
			if !r[0].Bool() {
				pass = false
			}
		}
	}

	return pass
}

// POST a new TOML blueprint
func (c *checkBlueprintsV0) CheckPostTOML() bool {
	name := "POST of a TOML blueprint"

	bp := `
		name="test-toml-blueprint-v0"
		description="postTOMLBlueprintV0"
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
	resp, err := client.PostTOMLBlueprintV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}
	log.Printf("OK: %s was successful", name)
	return true
}

// POST a new JSON blueprint
func (c *checkBlueprintsV0) CheckPostJSON() bool {
	name := "POST of a JSON blueprint"

	bp := `{
		"name": "test-json-blueprint-v0",
		"description": "postJSONBlueprintV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	resp, err := client.PostJSONBlueprintV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}
	log.Printf("OK: %s was successful", name)
	return true
}

// POST a blueprint to the workspace as TOML
func (c *checkBlueprintsV0) CheckPostTOMLWS() bool {
	name := "POST TOML blueprint to workspace"

	bp := `
		name="test-toml-blueprint-ws-v0"
		description="postTOMLBlueprintWSV0"
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
	resp, err := client.PostTOMLWorkspaceV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}
	log.Printf("OK: %s was successful", name)
	return true
}

// POST a blueprint to the workspace as JSON
func (c *checkBlueprintsV0) CheckPostJSONWS() bool {
	name := "POST JSON blueprint to workspace"

	bp := `{
		"name": "test-json-blueprint-ws-v0",
		"description": "postJSONBlueprintWSV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	resp, err := client.PostJSONWorkspaceV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}
	log.Printf("OK: %s was successful", name)
	return true
}

// delete a blueprint
func (c *checkBlueprintsV0) CheckDelete() bool {
	name := "DELETE blueprint"

	// POST a blueprint to delete
	bp := `{
		"name": "test-delete-blueprint-v0",
		"description": "deleteBlueprintV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	resp, err := client.PostJSONBlueprintV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	// Delete the blueprint
	resp, err = client.DeleteBlueprintV0(c.socket, "test-delete-blueprint-v0")
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	log.Printf("OK: %s was successful", name)
	return true
}

// delete a new blueprint from the workspace
func (c *checkBlueprintsV0) CheckDeleteNewWS() bool {
	name := "DELETE new blueprint from workspace"

	// POST a blueprint to delete
	bp := `{
		"name": "test-delete-new-blueprint-ws-v0",
		"description": "deleteNewBlueprintWSV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	resp, err := client.PostJSONWorkspaceV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	// Delete the blueprint
	resp, err = client.DeleteWorkspaceV0(c.socket, "test-delete-new-blueprint-ws-v0")
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	log.Printf("OK: %s was successful", name)
	return true
}

// delete blueprint changes from the workspace
func (c *checkBlueprintsV0) CheckDeleteChangesWS() bool {
	name := "DELETE blueprint changes from workspace"

	// POST a blueprint first
	bp := `{
		"name": "test-delete-blueprint-changes-ws-v0",
		"description": "deleteBlueprintChangesWSV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	resp, err := client.PostJSONBlueprintV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	// Post blueprint changes to the workspace
	bp = `{
		"name": "test-delete-blueprint-changes-ws-v0",
		"description": "workspace copy",
		"version": "0.2.0",
		"packages": [{"name": "frobozz", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	resp, err = client.PostJSONWorkspaceV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	// Get the blueprint, make sure it is the modified one and that changes = true
	info, api, err := client.GetBlueprintsInfoJSONV0(c.socket, "test-delete-blueprint-changes-ws-v0")
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s BlueprintsInfo request failed: %s", name, api)
		return false
	}

	if len(info.Blueprints) < 1 {
		log.Printf("FAIL: %s failed: No blueprints returned", name)
		return false
	}

	if len(info.Changes) < 1 {
		log.Printf("FAIL: %s failed: No change states returned", name)
		return false
	}

	if info.Blueprints[0].Name != "test-delete-blueprint-changes-ws-v0" {
		log.Printf("FAIL: %s failed: wrong blueprint returned", name)
		return false
	}

	if info.Changes[0].Name != "test-delete-blueprint-changes-ws-v0" {
		log.Printf("FAIL: %s failed: wrong change state returned", name)
		return false
	}

	if !info.Changes[0].Changed {
		log.Printf("FAIL: %s failed: wrong change state returned (false)", name)
		return false
	}

	if info.Blueprints[0].Description != "workspace copy" {
		log.Printf("FAIL: %s failed: workspace copy not returned", name)
		return false
	}

	// Delete the blueprint from the workspace
	resp, err = client.DeleteWorkspaceV0(c.socket, "test-delete-blueprint-changes-ws-v0")
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	// Get the blueprint, make sure it is the un-modified one
	info, api, err = client.GetBlueprintsInfoJSONV0(c.socket, "test-delete-blueprint-changes-ws-v0")
	if err != nil {
		log.Printf("FAIL: %s failed: %s", name, err)
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s BlueprintsInfo request failed: %s", name, api)
		return false
	}

	if len(info.Blueprints) < 1 {
		log.Printf("FAIL: %s failed: No blueprints returned", name)
		return false
	}

	if len(info.Changes) < 1 {
		log.Printf("FAIL: %s failed: No change states returned", name)
		return false
	}

	if info.Blueprints[0].Name != "test-delete-blueprint-changes-ws-v0" {
		log.Printf("FAIL: %s failed: wrong blueprint returned", name)
		return false
	}

	if info.Changes[0].Name != "test-delete-blueprint-changes-ws-v0" {
		log.Printf("FAIL: %s failed: wrong change state returned", name)
		return false
	}

	if info.Changes[0].Changed {
		log.Printf("FAIL: %s failed: wrong change state returned (true)", name)
		return false
	}

	if info.Blueprints[0].Description != "deleteBlueprintChangesWSV0" {
		log.Printf("FAIL: %s failed: original blueprint not returned", name)
		return false
	}

	log.Printf("OK: %s was successful", name)
	return true
}

// list blueprints
func (c *checkBlueprintsV0) CheckList() bool {
	name := "List blueprints"
	// Post a couple of blueprints
	bps := []string{`{
		"name": "test-list-blueprint-1-v0",
		"description": "listBlueprintsV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`,
		`{
		"name": "test-list-blueprint-2-v0",
		"description": "listBlueprintsV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`}

	for i := range bps {
		resp, err := client.PostJSONBlueprintV0(c.socket, bps[i])
		if err != nil {
			log.Printf("FAIL: %s failed with a client error: %s", name, err)
			return false
		}
		if !resp.Status {
			log.Printf("FAIL: %s failed: %s", name, resp)
			return false
		}
	}

	// Get the list of blueprints
	list, api, err := client.ListBlueprintsV0(c.socket)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err.Error())
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s ListBlueprints failed: %s", name, api)
		return false
	}

	// Make sure the blueprints are in the list
	sorted := sort.StringSlice(list)
	if !isStringInSlice(sorted, "test-list-blueprint-1-v0") ||
		!isStringInSlice(sorted, "test-list-blueprint-2-v0") {
		log.Printf("FAIL: %s failed", name)
		return false
	}

	log.Printf("OK: %s was successful", name)
	return true
}

// get blueprint contents as TOML
func (c *checkBlueprintsV0) CheckGetTOML() bool {
	name := "Get TOML Blueprint"
	bp := `{
		"name": "test-get-blueprint-1-v0",
		"description": "getTOMLBlueprintV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	// Post a blueprint
	resp, err := client.PostJSONBlueprintV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	// Get it as TOML
	body, api, err := client.GetBlueprintInfoTOMLV0(c.socket, "test-get-blueprint-1-v0")
	if err != nil {
		log.Printf("FAIL: %s failed: %s", name, err)
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s GetBlueprintInfo failed: %s", name, api)
		return false
	}

	if len(body) == 0 {
		log.Printf("FAIL: %s failed: body of response is empty", name)
		return false
	}

	// Can it be decoded as TOML?
	var decoded interface{}
	if _, err := toml.Decode(body, &decoded); err != nil {
		log.Printf("FAIL: %s failed: %s", name, err)
		return false
	}

	log.Printf("OK: %s was successful", name)
	return true
}

// get blueprint contents as JSON
func (c *checkBlueprintsV0) CheckGetJSON() bool {
	name := "Get JSON Blueprint"
	bp := `{
		"name": "test-get-blueprint-2-v0",
		"description": "getJSONBlueprintV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	// Post a blueprint
	resp, err := client.PostJSONBlueprintV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	// Get the blueprint and its changed state
	info, api, err := client.GetBlueprintsInfoJSONV0(c.socket, "test-get-blueprint-2-v0")
	if err != nil {
		log.Printf("FAIL: %s failed: %s", name, err)
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s GetBlueprintsInfo failed: %s", name, api)
		return false
	}

	if len(info.Blueprints) < 1 {
		log.Printf("FAIL: %s failed: No blueprints returned", name)
		return false
	}

	if len(info.Changes) < 1 {
		log.Printf("FAIL: %s failed: No change states returned", name)
		return false
	}

	if info.Blueprints[0].Name != "test-get-blueprint-2-v0" {
		log.Printf("FAIL: %s failed: wrong blueprint returned", name)
		return false
	}

	if info.Changes[0].Name != "test-get-blueprint-2-v0" {
		log.Printf("FAIL: %s failed: wrong change state returned", name)
		return false
	}

	if info.Changes[0].Changed {
		log.Printf("FAIL: %s failed: unexpected changes", name)
		return false
	}

	log.Printf("OK: %s was successful", name)
	return true
}

// pushing the same blueprint bumps the version number returned by show
func (c *checkBlueprintsV0) CheckBumpVersion() bool {
	name := "Bump Blueprint Version number"
	bp := `{
		"name": "test-bump-blueprint-1-v0",
		"description": "bumpBlueprintVersionV0",
		"version": "2.1.2",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`

	// List blueprints
	list, api, err := client.ListBlueprintsV0(c.socket)
	if err != nil {
		log.Printf("FAIL: %s failed: %s", name, err.Error())
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s ListBlueprints failed: %s", name, api)
		return false
	}

	// If the blueprint already exists it needs to be deleted to start from a known state
	sorted := sort.StringSlice(list)
	if isStringInSlice(sorted, "test-bump-blueprint-1-v0") {
		// Delete this blueprint if it already exists
		resp, err := client.DeleteBlueprintV0(c.socket, "test-bump-blueprint-1-v0")
		if err != nil {
			log.Printf("FAIL: %s failed with a client error: %s", name, err)
			return false
		}
		if !resp.Status {
			log.Printf("FAIL: %s failed: %s", name, resp)
			return false
		}
	}

	// Post a blueprint
	resp, err := client.PostJSONBlueprintV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	// Post a blueprint again to bump verion to 2.1.3
	resp, err = client.PostJSONBlueprintV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	// Get the blueprint and its changed state
	info, api, err := client.GetBlueprintsInfoJSONV0(c.socket, "test-bump-blueprint-1-v0")
	if err != nil {
		log.Printf("FAIL: %s failed: %s", name, err)
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s GetBlueprintsInfo failed: %s", name, api)
		return false
	}

	if len(info.Blueprints) < 1 {
		log.Printf("FAIL: %s failed: No blueprints returned", name)
		return false
	}

	if info.Blueprints[0].Name != "test-bump-blueprint-1-v0" {
		log.Printf("FAIL: %s failed: wrong blueprint returned", name)
		return false
	}

	if info.Blueprints[0].Version != "2.1.3" {
		log.Printf("FAIL: %s failed: wrong blueprint version", name)
		return false
	}

	log.Printf("OK: %s was successful", name)
	return true
}

// Make several changes to a blueprint and list the changes
func (c *checkBlueprintsV0) CheckBlueprintChangesV0() bool {
	name := "List blueprint changes"

	bps := []string{`{
		"name": "test-blueprint-changes-v0",
		"description": "CheckBlueprintChangesV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}]
	}`,
		`{
		"name": "test-blueprint-changes-v0",
		"description": "CheckBlueprintChangesV0",
		"version": "0.1.0",
		"packages": [{"name": "bash", "version": "*"}, {"name": "tmux", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`,
		`{
		"name": "test-blueprint-changes-v0",
		"description": "CheckBlueprintChangesV0",
		"version": "0.1.1",
		"packages": [{"name": "bash", "version": "*"}, {"name": "tmux", "version": "*"}],
		"modules": [],
		"customizations": {"user": [{"name": "root", "password": "asdasdasd"}]}
	}`}

	// Push 3 changes to the blueprint
	for i := range bps {
		resp, err := client.PostJSONBlueprintV0(c.socket, bps[i])
		if err != nil {
			log.Printf("FAIL: %s failed with a client error: %s", name, err)
			return false
		}
		if !resp.Status {
			log.Printf("FAIL: %s failed: %s", name, resp)
			return false
		}
	}

	// List the changes
	changes, api, err := client.GetBlueprintsChangesV0(c.socket, []string{"test-blueprint-changes-v0"})
	if err != nil {
		log.Printf("FAIL: %s failed: %s", name, err)
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s GetBlueprintsChanges failed: %s", name, api)
		return false
	}

	if len(changes.BlueprintsChanges) != 1 {
		log.Printf("FAIL: %s failed: No changes returned", name)
		return false
	}

	if changes.BlueprintsChanges[0].Name != "test-blueprint-changes-v0" {
		log.Printf("FAIL: %s failed: Wrong blueprint changes returned", name)
		return false
	}

	if len(changes.BlueprintsChanges[0].Changes) < 3 {
		log.Printf("FAIL: %s failed: Wrong number of changes returned", name)
		return false
	}

	log.Printf("OK: %s was successful", name)
	return true
}

// Undo blueprint changes
func (c *checkBlueprintsV0) CheckUndoBlueprintV0() bool {
	name := "Undo blueprint changes"

	bps := []string{`{
		"name": "test-undo-blueprint-v0",
		"description": "CheckUndoBlueprintV0",
		"version": "0.0.5",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`,
		`{
		"name": "test-undo-blueprint-v0",
		"description": "CheckUndoBlueprintv0",
		"version": "0.0.6",
		"packages": [{"name": "bash", "version": "0.5.*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`}

	// Push original version of the blueprint
	resp, err := client.PostJSONBlueprintV0(c.socket, bps[0])
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	// Get the commit hash
	changes, api, err := client.GetBlueprintsChangesV0(c.socket, []string{"test-undo-blueprint-v0"})
	if err != nil {
		log.Printf("FAIL: %s failed: %s", name, err)
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s GetBlueprintsChanges failed: %s", name, api)
		return false
	}

	if len(changes.BlueprintsChanges) != 1 {
		log.Printf("FAIL: %s failed: No changes returned", name)
		return false
	}

	if len(changes.BlueprintsChanges[0].Changes) < 1 {
		log.Printf("FAIL: %s failed: Wrong number of changes returned", name)
		return false
	}
	commit := changes.BlueprintsChanges[0].Changes[0].Commit

	if len(commit) == 0 {
		log.Printf("FAIL: %s failed: First commit is empty", name)
		return false
	}

	// Push the new version with wrong bash version
	resp, err = client.PostJSONBlueprintV0(c.socket, bps[1])
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	// Get the blueprint, confirm bash version is '0.5.*'
	info, api, err := client.GetBlueprintsInfoJSONV0(c.socket, "test-undo-blueprint-v0")
	if err != nil {
		log.Printf("FAIL: %s failed: %s", name, err)
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s GetBlueprintsInfo failed: %s", name, api)
		return false
	}

	if len(info.Blueprints) < 1 {
		log.Printf("FAIL: %s failed: No blueprints returned", name)
		return false
	}

	if len(info.Blueprints[0].Packages) < 1 {
		log.Printf("FAIL: %s failed: No packages in the blueprint", name)
		return false
	}

	if info.Blueprints[0].Packages[0].Name != "bash" ||
		info.Blueprints[0].Packages[0].Version != "0.5.*" {
		log.Printf("FAIL: %s failed to push change: Wrong package in the blueprint: %s", name, info.Blueprints[0].Packages[0])
		log.Printf("%#v", info)
		return false
	}

	// Revert the blueprint to the original version
	resp, err = client.UndoBlueprintChangeV0(c.socket, "test-undo-blueprint-v0", commit)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s failed: %s", name, resp)
		return false
	}

	// Get the blueprint, confirm bash version is '*'
	info, api, err = client.GetBlueprintsInfoJSONV0(c.socket, "test-undo-blueprint-v0")
	if err != nil {
		log.Printf("FAIL: %s failed: %s", name, err)
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s GetBlueprintsInfo failed: %s", name, api)
		return false
	}

	if len(info.Blueprints) < 1 {
		log.Printf("FAIL: %s failed: No blueprints returned", name)
		return false
	}

	if len(info.Blueprints[0].Packages) < 1 {
		log.Printf("FAIL: %s failed: No packages in the blueprint", name)
		return false
	}

	if info.Blueprints[0].Packages[0].Name != "bash" ||
		info.Blueprints[0].Packages[0].Version != "*" {
		log.Printf("FAIL: %s failed to undo change: Wrong package in the blueprint: %s", name, info.Blueprints[0].Packages[0])
		log.Printf("%#v", info)
		return false
	}

	log.Printf("OK: %s was successful", name)
	return true
}

// Tag a blueprint with a new revision
// The blueprint revision tag cannot be reset, it always increments by one, and cannot be deleted.
// So to test tagging we tag two blueprint changes and make sure the second is first +1
func (c *checkBlueprintsV0) CheckBlueprintTagV0() bool {
	name := "Tag a blueprint"

	bps := []string{`{
		"name": "test-tag-blueprint-v0",
		"description": "CheckBlueprintTagV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "0.1.*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`,
		`{
		"name": "test-tag-blueprint-v0",
		"description": "CheckBlueprintTagV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "0.5.*"}],
		"modules": [{"name": "util-linux", "version": "*"}],
		"customizations": {"user": [{"name": "root", "password": "qweqweqwe"}]}
	}`}

	// Push a blueprint
	resp, err := client.PostJSONBlueprintV0(c.socket, bps[0])
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s POST failed: %s", name, resp)
		return false
	}

	// Tag the blueprint
	tagResp, err := client.TagBlueprintV0(c.socket, "test-tag-blueprint-v0")
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !tagResp.Status {
		log.Printf("FAIL: %s Tag failed: %s", name, tagResp)
		return false
	}

	// Get changes, get the blueprint's revision
	changes, api, err := client.GetBlueprintsChangesV0(c.socket, []string{"test-tag-blueprint-v0"})
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s GetBlueprintsChange failed: %s", name, api)
		return false
	}

	if len(changes.BlueprintsChanges) != 1 {
		log.Printf("FAIL: %s failed: No changes returned", name)
		return false
	}

	if len(changes.BlueprintsChanges[0].Changes) < 1 {
		log.Printf("FAIL: %s failed: Wrong number of changes returned", name)
		return false
	}

	revision := changes.BlueprintsChanges[0].Changes[0].Revision
	if revision == nil || *revision == 0 {
		log.Printf("FAIL: %s failed: Revision is zero", name)
		return false
	}

	// Push a new version of the blueprint
	resp, err = client.PostJSONBlueprintV0(c.socket, bps[1])
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s POST failed: %s", name, resp)
		return false
	}

	// Tag the blueprint
	tagResp, err = client.TagBlueprintV0(c.socket, "test-tag-blueprint-v0")
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !tagResp.Status {
		log.Printf("FAIL: %s Tag failed: %s", name, tagResp)
		return false
	}

	// Get changes, confirm that Revision is revision +1
	changes, api, err = client.GetBlueprintsChangesV0(c.socket, []string{"test-tag-blueprint-v0"})
	if err != nil {
		log.Printf("FAIL: %s failed: %s", name, err)
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s GetBlueprintsChanges failed: %s", name, api)
		return false
	}

	if len(changes.BlueprintsChanges) != 1 {
		log.Printf("FAIL: %s failed: No changes returned", name)
		return false
	}

	if len(changes.BlueprintsChanges[0].Changes) < 1 {
		log.Printf("FAIL: %s failed: Wrong number of changes returned", name)
		return false
	}

	newRevision := changes.BlueprintsChanges[0].Changes[0].Revision
	if newRevision == nil || *newRevision != *revision+1 {
		log.Printf("FAIL: %s failed: Revision is not %d", name, *revision+1)
		return false
	}

	log.Printf("OK: %s was successful", name)
	return true
}

// depsolve a blueprint with packages and modules
func (c *checkBlueprintsV0) CheckBlueprintDepsolveV0() bool {
	name := "Depsolve a blueprint"

	bp := `{
		"name": "test-deps-blueprint-v0",
		"description": "CheckBlueprintDepsolveV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}]
	}`

	// Push a blueprint
	resp, err := client.PostJSONBlueprintV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s POST failed: %s", name, resp)
		return false
	}

	// Depsolve the blueprint
	deps, api, err := client.DepsolveBlueprintV0(c.socket, "test-deps-blueprint-v0")
	if err != nil {
		log.Printf("FAIL: %s failed: %s", name, err.Error())
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s DepsolveBlueprint failed: %s", name, api)
		return false
	}

	if len(deps.Blueprints) < 1 {
		log.Printf("FAIL: %s failed: No blueprint dependencies returned", name)
		return false
	}

	if len(deps.Blueprints[0].Dependencies) < 3 {
		log.Printf("FAIL: %s failed: Not enough dependencies returned", name)
		return false
	}

	// TODO
	// Get the bash and util-linux dependencies and make sure their versions are not *

	log.Printf("OK: %s was successful", name)
	return true
}

// freeze a blueprint
func (c *checkBlueprintsV0) CheckBlueprintFreezeV0() bool {
	name := "Freeze a blueprint"

	bp := `{
		"name": "test-freeze-blueprint-v0",
		"description": "CheckBlueprintFreezeV0",
		"version": "0.0.1",
		"packages": [{"name": "bash", "version": "*"}],
		"modules": [{"name": "util-linux", "version": "*"}]
	}`

	// Push a blueprint
	resp, err := client.PostJSONBlueprintV0(c.socket, bp)
	if err != nil {
		log.Printf("FAIL: %s failed with a client error: %s", name, err)
		return false
	}
	if !resp.Status {
		log.Printf("FAIL: %s POST failed: %s", name, resp)
		return false
	}

	// Freeze the blueprint
	frozen, api, err := client.FreezeBlueprintV0(c.socket, "test-freeze-blueprint-v0")
	if err != nil {
		log.Printf("FAIL: %s failed: %s", name, err.Error())
		return false
	}
	if api != nil {
		log.Printf("FAIL: %s FreezeBlueprint failed: %s", name, api)
		return false
	}

	if len(frozen.Blueprints) < 1 {
		log.Printf("FAIL: %s failed: No frozen blueprints returned", name)
		return false
	}

	if len(frozen.Blueprints[0].Blueprint.Packages) < 1 {
		log.Printf("FAIL: %s failed: No frozen packages returned", name)
		return false
	}

	if frozen.Blueprints[0].Blueprint.Packages[0].Name != "bash" ||
		frozen.Blueprints[0].Blueprint.Packages[0].Version == "*" {
		log.Printf("FAIL: %s failed: Incorrect frozen packages", name)
		return false
	}

	if len(frozen.Blueprints[0].Blueprint.Modules) < 1 {
		log.Printf("FAIL: %s failed: No frozen modules returned", name)
		return false
	}

	if frozen.Blueprints[0].Blueprint.Modules[0].Name != "util-linux" ||
		frozen.Blueprints[0].Blueprint.Modules[0].Version == "*" {
		log.Printf("FAIL: %s failed: Incorrect frozen modules", name)
		return false
	}

	log.Printf("OK: %s was successful", name)
	return true
}

// diff of blueprint changes
func (c *checkBlueprintsV0) CheckBlueprintDiffV0() bool {
	name := "Diff of blueprint changes"
	log.Printf("SKIP: %s was skipped, needs to be implemented", name)
	return true
}
