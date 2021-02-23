// +build integration

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/weldr"

	"testing"
)

func TestComposeCommands(t *testing.T) {
	// common setup
	tmpdir := NewTemporaryWorkDir(t, "osbuild-tests-")
	defer tmpdir.Close(t)

	bp := blueprint.Blueprint{
		Name:        "empty",
		Description: "Test blueprint in toml format",
		Packages: []blueprint.Package{{Name: "bash", Version: "*"}},
	}
	pushBlueprint(t, &bp)
	defer deleteBlueprint(t, &bp)

	runComposer(t, "blueprints", "save", "empty")
	_, err := os.Stat("empty.toml")
	require.NoError(t, err, "Error accessing 'empty.toml: %v'", err)

	runComposer(t, "compose", "types")
	runComposer(t, "compose", "status")
	runComposer(t, "compose", "list")
	runComposer(t, "compose", "list", "waiting")
	runComposer(t, "compose", "list", "running")
	runComposer(t, "compose", "list", "finished")
	runComposer(t, "compose", "list", "failed")

	// Full integration tests
	uuid := buildCompose(t, "empty", "qcow2")
	defer deleteCompose(t, uuid)

	runComposer(t, "compose", "info", uuid.String())

	runComposer(t, "compose", "metadata", uuid.String())
	_, err = os.Stat(uuid.String() + "-metadata.tar")
	require.NoError(t, err, "'%s-metadata.tar' not found", uuid.String())
	defer os.Remove(uuid.String() + "-metadata.tar")

	runComposer(t, "compose", "results", uuid.String())
	_, err = os.Stat(uuid.String() + ".tar")
	require.NoError(t, err, "'%s.tar' not found", uuid.String())
	defer os.Remove(uuid.String() + ".tar")

	// Just assert that result wasn't empty
	result := runComposer(t, "compose", "log", uuid.String())
	require.NotNil(t, result)
	result = runComposer(t, "compose", "log", uuid.String(), "1024")
	require.NotNil(t, result)

	runComposer(t, "compose", "logs", uuid.String())
	_, err = os.Stat(uuid.String() + "-logs.tar")
	require.NoError(t, err, "'%s-logs.tar' not found", uuid.String())
	defer os.Remove(uuid.String() + "-logs.tar")

	runComposer(t, "compose", "image", uuid.String())
	_, err = os.Stat(uuid.String() + "-disk.qcow2")
	require.NoError(t, err, "'%s-disk.qcow2' not found", uuid.String())
	defer os.Remove(uuid.String() + "-disk.qcow2")

	// workers ask the composer every 15 seconds if a compose was canceled.
	// Use 20 seconds here to be sure this is hit.
	uuid = startCompose(t, "empty", "qcow2")
	defer deleteCompose(t, uuid)
	runComposer(t, "compose", "cancel", uuid.String())
	time.Sleep(20 * time.Second)
	status := waitForCompose(t, uuid)
	assert.Equal(t, "FAILED", status)

	// Check that reusing the cache works ok
	uuid = buildCompose(t, "empty", "qcow2")
	defer deleteCompose(t, uuid)
}

func TestBlueprintCommands(t *testing.T) {
	// common setup
	tmpdir := NewTemporaryWorkDir(t, "osbuild-tests-")
	defer tmpdir.Close(t)

	bp := blueprint.Blueprint{
		Name:        "empty",
		Description: "Test empty blueprint in toml format",
	}
	pushBlueprint(t, &bp)
	defer deleteBlueprint(t, &bp)

	runComposer(t, "blueprints", "list")
	runComposer(t, "blueprints", "show", "empty")

	runComposer(t, "blueprints", "changes", "empty")
	runComposer(t, "blueprints", "diff", "empty", "NEWEST", "WORKSPACE")

	runComposer(t, "blueprints", "save", "empty")
	_, err := os.Stat("empty.toml")
	require.NoError(t, err, "'empty.toml' not found")
	defer os.Remove("empty.toml")

	runComposer(t, "blueprints", "depsolve", "empty")
	runComposer(t, "blueprints", "freeze", "empty")
	runComposer(t, "blueprints", "freeze", "show", "empty")
	runComposer(t, "blueprints", "freeze", "save", "empty")
	_, err = os.Stat("empty.frozen.toml")
	require.NoError(t, err, "'empty.frozen.toml' not found")
	defer os.Remove("empty.frozen.toml")

	runComposer(t, "blueprints", "tag", "empty")

	// undo the latest commit we can find
	var changes weldr.BlueprintsChangesV0
	rawReply := runComposerJSON(t, "blueprints", "changes", "empty")
	err = json.Unmarshal(rawReply, &changes)
	require.Nilf(t, err, "Error searching for commits to undo: %v", err)
	runComposer(t, "blueprints", "undo", "empty", changes.BlueprintsChanges[0].Changes[0].Commit)

	runComposer(t, "blueprints", "workspace", "empty")
}

func TestModulesCommands(t *testing.T) {
	runComposer(t, "modules", "list")
}

func TestProjectsCommands(t *testing.T) {
	runComposer(t, "projects", "list")
	runComposer(t, "projects", "info", "filesystem")
	runComposer(t, "projects", "info", "filesystem", "kernel")
}

func TestStatusCommands(t *testing.T) {
	runComposer(t, "status", "show")
}

func TestSourcesCommands(t *testing.T) {
	sources_toml, err := ioutil.TempFile("", "SOURCES-*.TOML")
	require.NoErrorf(t, err, "Could not create temporary file: %v", err)
	defer os.Remove(sources_toml.Name())

	_, err = sources_toml.Write([]byte(`id = "osbuild-test-addon-source"
name = "Testing sources add command"
url = "file://REPO-PATH"
type = "yum-baseurl"
proxy = "https://proxy-url/"
check_ssl = true
check_gpg = true
gpgkey_urls = ["https://url/path/to/gpg-key"]
`))
	require.NoError(t, err)

	runComposer(t, "sources", "list")
	runComposer(t, "sources", "add", sources_toml.Name())
	runComposer(t, "sources", "info", "osbuild-test-addon-source")
	runComposer(t, "sources", "change", sources_toml.Name())
	runComposer(t, "sources", "delete", "osbuild-test-addon-source")
}

func buildCompose(t *testing.T, bpName string, outputType string) uuid.UUID {
	uuid := startCompose(t, bpName, outputType)
	status := waitForCompose(t, uuid)
	logs := getLogs(t, uuid)
	assert.NotEmpty(t, logs, "logs are empty after the build is finished/failed")

	if !assert.Equalf(t, "FINISHED", status, "Unexpected compose result: %s", status) {
		log.Print("logs from the build: ", logs)
		t.FailNow()
	}

	return uuid
}

func startCompose(t *testing.T, name, outputType string) uuid.UUID {
	var reply struct {
		BuildID uuid.UUID `json:"build_id"`
		Status  bool      `json:"status"`
	}
	rawReply := runComposerJSON(t, "compose", "start", name, outputType)
	err := json.Unmarshal(rawReply, &reply)
	require.Nilf(t, err, "Unexpected reply: %v", err)
	require.Truef(t, reply.Status, "Unexpected status %v", reply.Status)

	return reply.BuildID
}

func deleteCompose(t *testing.T, id uuid.UUID) {
	type deleteUUID struct {
		ID     uuid.UUID `json:"uuid"`
		Status bool      `json:"status"`
	}
	var reply struct {
		IDs    []deleteUUID  `json:"uuids"`
		Errors []interface{} `json:"errors"`
	}
	rawReply := runComposerJSON(t, "compose", "delete", id.String())
	err := json.Unmarshal(rawReply, &reply)
	require.Nilf(t, err, "Unexpected reply: %v", err)
	require.Zerof(t, len(reply.Errors), "Unexpected errors")
	require.Equalf(t, 1, len(reply.IDs), "Unexpected number of UUIDs returned: %d", len(reply.IDs))
	require.Truef(t, reply.IDs[0].Status, "Unexpected status %v", reply.IDs[0].Status)
}

func waitForCompose(t *testing.T, uuid uuid.UUID) string {
	for {
		status := getComposeStatus(t, uuid)
		if status == "FINISHED" || status == "FAILED" {
			return status
		}
		time.Sleep(time.Second)
	}
}

func getComposeStatus(t *testing.T, uuid uuid.UUID) string {
	var reply struct {
		QueueStatus string `json:"queue_status"`
	}
	rawReply := runComposerJSON(t, "compose", "info", uuid.String())
	err := json.Unmarshal(rawReply, &reply)
	require.Nilf(t, err, "Unexpected reply: %v", err)

	return reply.QueueStatus
}

func getLogs(t *testing.T, uuid uuid.UUID) string {
	cmd := exec.Command("composer-cli", "compose", "log", uuid.String())
	cmd.Stderr = os.Stderr
	stdoutReader, err := cmd.StdoutPipe()
	require.NoError(t, err)

	err = cmd.Start()
	require.NoError(t, err)

	var buffer bytes.Buffer
	_, err = buffer.ReadFrom(stdoutReader)
	require.NoError(t, err)

	err = cmd.Wait()
	require.NoError(t, err)

	return buffer.String()
}

func pushBlueprint(t *testing.T, bp *blueprint.Blueprint) {
	tmpfile, err := ioutil.TempFile("", "osbuild-test-")
	require.Nilf(t, err, "Could not create temporary file: %v", err)
	defer os.Remove(tmpfile.Name())

	err = toml.NewEncoder(tmpfile).Encode(bp)
	require.Nilf(t, err, "Could not marshal blueprint TOML: %v", err)
	err = tmpfile.Close()
	require.Nilf(t, err, "Could not close toml file: %v", err)

	var reply struct {
		Status bool `json:"status"`
	}
	rawReply := runComposerJSON(t, "blueprints", "push", tmpfile.Name())
	err = json.Unmarshal(rawReply, &reply)
	require.Nilf(t, err, "Unexpected reply: %v", err)
	require.Truef(t, reply.Status, "Unexpected status %v", reply.Status)
}

func deleteBlueprint(t *testing.T, bp *blueprint.Blueprint) {
	var reply struct {
		Status bool `json:"status"`
	}
	rawReply := runComposerJSON(t, "blueprints", "delete", bp.Name)
	err := json.Unmarshal(rawReply, &reply)
	require.Nilf(t, err, "Unexpected reply: %v", err)
	require.Truef(t, reply.Status, "Unexpected status %v", reply.Status)
}

func runComposer(t *testing.T, command ...string) []byte {
	fmt.Printf("Running composer-cli %s\n", strings.Join(command, " "))
	cmd := exec.Command("composer-cli", command...)
	stdout, err := cmd.StdoutPipe()
	require.Nilf(t, err, "Could not create command: %v", err)
	stderr, err := cmd.StderrPipe()
	require.Nilf(t, err, "Could not create command: %v", err)

	err = cmd.Start()
	require.Nilf(t, err, "Could not start command: %v", err)

	contents, err := ioutil.ReadAll(stdout)
	require.NoError(t, err, "Could not read stdout from command")

	errcontents, err := ioutil.ReadAll(stderr)
	require.NoError(t, err, "Could not read stderr from command")

	err = cmd.Wait()
	require.NoErrorf(t, err, "Command failed (%v): %v", err, string(errcontents))

	return contents
}

func runComposerJSON(t *testing.T, command ...string) json.RawMessage {
	command = append([]string{"--json"}, command...)
	contents := runComposer(t, command...)

	var result json.RawMessage

	if len(contents) != 0 {
		err := json.Unmarshal(contents, &result)
		if err != nil {
			// We did not get JSON, try interpreting it as TOML
			var data interface{}
			err = toml.Unmarshal(contents, &data)
			require.Nilf(t, err, "Could not parse output: %v", err)
			buffer := bytes.Buffer{}
			err = json.NewEncoder(&buffer).Encode(data)
			require.Nilf(t, err, "Could not remarshal TOML to JSON: %v", err)
			err = json.NewDecoder(&buffer).Decode(&result)
			require.Nilf(t, err, "Could not decode the remarshalled JSON: %v", err)
		}
	}

	buffer := bytes.Buffer{}
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	err := encoder.Encode(result)
	require.Nilf(t, err, "Could not remarshal the recevied JSON: %v", err)

	return result
}

type TemporaryWorkDir struct {
	OldWorkDir string
	Path       string
}

// Creates a new temporary directory based on pattern and changes the current
// working directory to it.
//
// Example:
//   d := NewTemporaryWorkDir(t, "foo-*")
//   defer d.Close(t)
func NewTemporaryWorkDir(t *testing.T, pattern string) TemporaryWorkDir {
	var d TemporaryWorkDir
	var err error

	d.OldWorkDir, err = os.Getwd()
	require.Nilf(t, err, "os.GetWd: %v", err)

	d.Path, err = ioutil.TempDir("", pattern)
	require.Nilf(t, err, "ioutil.TempDir: %v", err)

	err = os.Chdir(d.Path)
	require.Nilf(t, err, "os.ChDir: %v", err)

	return d
}

// Change back to the previous working directory and removes the temporary one.
func (d *TemporaryWorkDir) Close(t *testing.T) {
	var err error

	err = os.Chdir(d.OldWorkDir)
	require.Nilf(t, err, "os.ChDir: %v", err)

	err = os.RemoveAll(d.Path)
	require.Nilf(t, err, "os.RemoveAll: %v", err)
}
