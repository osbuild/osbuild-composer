// +build integration

package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"

	"github.com/stretchr/testify/require"
	"testing"
)

func TestEverything(t *testing.T) {
	// Smoke tests, until functionality tested fully
	// that the calls succeed and return valid output
	runComposerCLI(t, false, "compose", "types")
	runComposerCLI(t, false, "compose", "status")
	runComposerCLI(t, false, "compose", "list")
	runComposerCLI(t, false, "compose", "list", "waiting")
	runComposerCLI(t, false, "compose", "list", "running")
	runComposerCLI(t, false, "compose", "list", "finished")
	runComposerCLI(t, false, "compose", "list", "failed")
	// runCommand(false, "compose", "log", UUID, [<SIZE>])
	// runCommand(false, "compose", "cancel", UUID)
	// runCommand(false, "compose", "delete", UUID)
	// runCommand(false, "compose", "info", UUID)
	// runCommand(false, "compose", "metadata", UUID)
	// runCommand(false, "compose", "logs", UUID)
	// runCommand(false, "compose", "results", UUID)
	// runCommand(false, "compose", "image", UUID)
	runComposerCLI(t, false, "blueprints", "list")
	// runCommand(false, "blueprints", "show", BLUEPRINT,....)
	// runCommand(false, "blueprints", "changes", BLUEPRINT,....)
	// runCommand(false, "blueprints", "diff", BLUEPRINT, FROM/NEWEST, TO/NEWEST/WORKSPACE)
	// runCommand(false, "blueprints", "save", BLUEPRINT,...)
	// runCommand(false, "blueprints", "delete", BLUEPRINT)
	// runCommand(false, "blueprints", "depsolve", BLUEPRINT,...)
	// runCommand(false, "blueprints", "push", BLUEPRINT.TOML)
	// runCommand(false, "blueprints", "freeze", BLUEPRINT,...)
	// runCommand(false, "blueprints", "freeze", "show", BLUEPRINT,...)
	// runCommand(false, "blueprints", "freeze", "save", BLUEPRINT,...)
	// runCommand(false, "blueprints", "tag", BLUEPRINT)
	// runCommand(false, "blueprints", "undo", BLUEPRINT, COMMIT)
	// runCommand(false, "blueprints", "workspace", BLUEPRINT)
	runComposerCLI(t, false, "modules", "list")
	runComposerCLI(t, false, "projects", "list")
	runComposerCLI(t, false, "projects", "info", "filesystem")
	runComposerCLI(t, false, "projects", "info", "filesystem", "kernel")
	runComposerCLI(t, false, "sources", "list")
	runComposerCLI(t, false, "status", "show")

	// Full integration tests
	testCompose(t, "ami")
}

func TestSources(t *testing.T) {
	sources_toml, err := ioutil.TempFile("", "SOURCES-*.TOML")
	require.NoErrorf(t, err, "Could not create temporary file: %v", err)
	defer os.Remove(sources_toml.Name())

	_, err = sources_toml.Write([]byte(`name = "osbuild-test-addon-source"
url = "file://REPO-PATH"
type = "yum-baseurl"
proxy = "https://proxy-url/"
check_ssl = true
check_gpg = true
gpgkey_urls = ["https://url/path/to/gpg-key"]
`))
	require.NoError(t, err)

	runComposerCLI(t, false, "sources", "list")
	runComposerCLI(t, false, "sources", "add", sources_toml.Name())
	runComposerCLI(t, false, "sources", "info", "osbuild-test-addon-source")
	runComposerCLI(t, false, "sources", "change", sources_toml.Name())
	runComposerCLI(t, false, "sources", "delete", "osbuild-test-addon-source")
}

func testCompose(t *testing.T, outputType string) {
	tmpdir := NewTemporaryWorkDir(t, "osbuild-tests-")
	defer tmpdir.Close(t)

	bp := blueprint.Blueprint{
		Name:        "empty",
		Description: "Test empty blueprint in toml format",
	}
	pushBlueprint(t, &bp)
	defer deleteBlueprint(t, &bp)

	runComposerCLI(t, false, "blueprints", "save", "empty")
	_, err := os.Stat("empty.toml")
	require.Nilf(t, err, "Error accessing 'empty.toml: %v'", err)

	uuid := startCompose(t, "empty", outputType)
	defer deleteCompose(t, uuid)
	status := waitForCompose(t, uuid)
	require.Equalf(t, "FINISHED", status, "Unexpected compose result: %s", status)

	runComposerCLI(t, false, "compose", "image", uuid.String())
}

func startCompose(t *testing.T, name, outputType string) uuid.UUID {
	var reply struct {
		BuildID uuid.UUID `json:"build_id"`
		Status  bool      `json:"status"`
	}
	rawReply := runComposerCLI(t, false, "compose", "start", name, outputType)
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
	rawReply := runComposerCLI(t, false, "compose", "delete", id.String())
	err := json.Unmarshal(rawReply, &reply)
	require.Nilf(t, err, "Unexpected reply: %v", err)
	require.Zerof(t, len(reply.Errors), "Unexpected errors")
	require.Equalf(t, 1, len(reply.IDs), "Unexpected number of UUIDs returned: %d", len(reply.IDs))
	require.Truef(t, reply.IDs[0].Status, "Unexpected status %v", reply.IDs[0].Status)
}

func waitForCompose(t *testing.T, uuid uuid.UUID) string {
	for {
		status := getComposeStatus(t, true, uuid)
		if status == "FINISHED" || status == "FAILED" {
			return status
		}
		time.Sleep(time.Second)
	}
}

func getComposeStatus(t *testing.T, quiet bool, uuid uuid.UUID) string {
	var reply struct {
		QueueStatus string `json:"queue_status"`
	}
	rawReply := runComposerCLI(t, quiet, "compose", "info", uuid.String())
	err := json.Unmarshal(rawReply, &reply)
	require.Nilf(t, err, "Unexpected reply: %v", err)

	return reply.QueueStatus
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
	rawReply := runComposerCLI(t, false, "blueprints", "push", tmpfile.Name())
	err = json.Unmarshal(rawReply, &reply)
	require.Nilf(t, err, "Unexpected reply: %v", err)
	require.Truef(t, reply.Status, "Unexpected status %v", reply.Status)
}

func deleteBlueprint(t *testing.T, bp *blueprint.Blueprint) {
	var reply struct {
		Status bool `json:"status"`
	}
	rawReply := runComposerCLI(t, false, "blueprints", "delete", bp.Name)
	err := json.Unmarshal(rawReply, &reply)
	require.Nilf(t, err, "Unexpected reply: %v", err)
	require.Truef(t, reply.Status, "Unexpected status %v", reply.Status)
}

func runComposerCLI(t *testing.T, quiet bool, command ...string) json.RawMessage {
	command = append([]string{"--json"}, command...)
	cmd := exec.Command("composer-cli", command...)
	stdout, err := cmd.StdoutPipe()
	require.Nilf(t, err, "Could not create command: %v", err)

	err = cmd.Start()
	require.Nilf(t, err, "Could not start command: %v", err)

	var result json.RawMessage

	contents, err := ioutil.ReadAll(stdout)
	require.Nilf(t, err, "Could not read stdout from command: %v", err)

	if len(contents) != 0 {
		err = json.Unmarshal(contents, &result)
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

	err = cmd.Wait()
	require.Nilf(t, err, "Command failed: %v", err)

	buffer := bytes.Buffer{}
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(result)
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
