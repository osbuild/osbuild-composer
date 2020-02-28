package main

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/blueprint"
)

func main() {
	// Smoke tests, until functionality tested fully
	// that the calls succeed and return valid output
	runComposerCLI(false, "compose", "types")
	runComposerCLI(false, "compose", "status")
	runComposerCLI(false, "compose", "list")
	runComposerCLI(false, "compose", "list", "waiting")
	runComposerCLI(false, "compose", "list", "running")
	runComposerCLI(false, "compose", "list", "finished")
	runComposerCLI(false, "compose", "list", "failed")
	// runCommand(false, "compose", "log", UUID, [<SIZE>])
	// runCommand(false, "compose", "cancel", UUID)
	// runCommand(false, "compose", "delete", UUID)
	// runCommand(false, "compose", "info", UUID)
	// runCommand(false, "compose", "metadata", UUID)
	// runCommand(false, "compose", "logs", UUID)
	// runCommand(false, "compose", "results", UUID)
	// runCommand(false, "compose", "image", UUID)
	runComposerCLI(false, "blueprints", "list")
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
	runComposerCLI(false, "modules", "list")
	runComposerCLI(false, "projects", "list")
	runComposerCLI(false, "projects", "info", "filesystem")
	runComposerCLI(false, "projects", "info", "filesystem", "kernel")
	runComposerCLI(false, "sources", "list")
	// runCommand(false, "sources", "info", "fedora")
	// runCommand(false, "sources", "info", "fedora", "fedora-updates")
	// runCommand(false, "sources", "add" SOURCES.TOML)
	// runCommand(false, "sources", "change" SOURCES.TOML)
	// runCommand(false, "sources", "delete" SOURCES.TOML)
	runComposerCLI(false, "status", "show")

	// Full integration tests
	testCompose("ami")
	testCompose("ext4-filesystem")
	testCompose("openstack")
	testCompose("partitioned-disk")
	testCompose("qcow2")
	testCompose("tar")
	testCompose("vhd")
	testCompose("vmdk")
}

func testCompose(outputType string) {
	bp := blueprint.Blueprint{
		Name:        "empty",
		Description: "Test empty blueprint in toml format",
	}
	pushBlueprint(&bp)
	defer deleteBlueprint(&bp)

	uuid := startCompose("empty", outputType)
	defer deleteCompose(uuid)
	status := waitForCompose(uuid)
	if status != "FINISHED" {
		log.Fatalf("Unexpected compose result: %s", status)
	}
}

func startCompose(name, outputType string) uuid.UUID {
	var reply struct {
		BuildID uuid.UUID `json:"build_id"`
		Status  bool      `json:"status"`
	}
	rawReply := runComposerCLI(false, "compose", "start", name, outputType)
	err := json.Unmarshal(rawReply, &reply)
	if err != nil {
		log.Fatalf("Unexpected reply: " + err.Error())
	}
	if !reply.Status {
		log.Fatalf("Unexpected status %v", reply.Status)
	}

	return reply.BuildID
}

func deleteCompose(id uuid.UUID) {
	type deleteUUID struct {
		ID     uuid.UUID `json:"uuid"`
		Status bool      `json:"status"`
	}
	var reply struct {
		IDs    []deleteUUID  `json:"uuids"`
		Errors []interface{} `json:"errors"`
	}
	rawReply := runComposerCLI(false, "compose", "delete", id.String())
	err := json.Unmarshal(rawReply, &reply)
	if err != nil {
		log.Fatalf("Unexpected reply: " + err.Error())
	}
	if len(reply.Errors) != 0 {
		log.Fatalf("Unexpected errors")
	}
	if len(reply.IDs) != 1 {
		log.Fatalf("Unexpected number of UUIDs returned: %d", len(reply.IDs))
	}
	if !reply.IDs[0].Status {
		log.Fatalf("Unexpected status %v", reply.IDs[0].Status)
	}
}

func waitForCompose(uuid uuid.UUID) string {
	for {
		status := getComposeStatus(true, uuid)
		if status == "FINISHED" || status == "FAILED" {
			return status
		}
		time.Sleep(time.Second)
	}
}

func getComposeStatus(quiet bool, uuid uuid.UUID) string {
	var reply struct {
		QueueStatus string `json:"queue_status"`
	}
	rawReply := runComposerCLI(quiet, "compose", "info", uuid.String())
	err := json.Unmarshal(rawReply, &reply)
	if err != nil {
		log.Fatalf("Unexpected reply: " + err.Error())
	}

	return reply.QueueStatus
}

func pushBlueprint(bp *blueprint.Blueprint) {
	tmpfile, err := ioutil.TempFile("", "osbuild-test-")
	if err != nil {
		log.Fatalf("Could not create temporary file: " + err.Error())
	}
	defer os.Remove(tmpfile.Name())

	err = toml.NewEncoder(tmpfile).Encode(bp)
	if err != nil {
		log.Fatalf("Could not marshapl blueprint TOML: " + err.Error())
	}
	if err := tmpfile.Close(); err != nil {
		log.Fatalf("Could not close toml file: " + err.Error())
	}

	var reply struct {
		Status bool `json:"status"`
	}
	rawReply := runComposerCLI(false, "blueprints", "push", tmpfile.Name())
	err = json.Unmarshal(rawReply, &reply)
	if err != nil {
		log.Fatalf("Unexpected reply: " + err.Error())
	}
	if !reply.Status {
		log.Fatalf("Unexpected status %v", reply.Status)
	}
}

func deleteBlueprint(bp *blueprint.Blueprint) {
	var reply struct {
		Status bool `json:"status"`
	}
	rawReply := runComposerCLI(false, "blueprints", "delete", bp.Name)
	err := json.Unmarshal(rawReply, &reply)
	if err != nil {
		log.Fatalf("Unexpected reply: " + err.Error())
	}
	if !reply.Status {
		log.Fatalf("Unexpected status %v", reply.Status)
	}
}

func runComposerCLI(quiet bool, command ...string) json.RawMessage {
	command = append([]string{"--json"}, command...)
	cmd := exec.Command("composer-cli", command...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("Colud not create command: " + err.Error())
	}

	if !quiet {
		log.Printf("$ composer-cli %s\n", strings.Join(command, " "))
	}

	cmd.Start()

	var result json.RawMessage

	contents, err := ioutil.ReadAll(stdout)
	if err != nil {
		log.Fatalf("Could not read stdout from command: " + err.Error())
	}

	if len(contents) != 0 {
		err = json.Unmarshal(contents, &result)
		if err != nil {
			// We did not get JSON, try interpreting it as TOML
			var data interface{}
			err = toml.Unmarshal(contents, &data)
			if err != nil {
				log.Fatalf("Could not parse output: %s", err.Error())
			}
			buffer := bytes.Buffer{}
			err = json.NewEncoder(&buffer).Encode(data)
			if err != nil {
				log.Fatalf("Could not remarshal TOML to JSON: %s", err.Error())
			}
			err = json.NewDecoder(&buffer).Decode(&result)
			if err != nil {
				log.Fatalf("Could not decode the remarshalled JSON: %s", err.Error())
			}
		}
	}

	err = cmd.Wait()
	if err != nil {
		log.Fatalf("Command failed: " + err.Error())
	}

	buffer := bytes.Buffer{}
	encoder := json.NewEncoder(&buffer)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(result)
	if err != nil {
		log.Fatalf("Could not remarshal the recevied JSON: " + err.Error())
	}

	if !quiet {
		log.Printf("Return:\n%s\n", buffer.String())
	}

	return result
}
