package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
)

func RunOSBuild(manifest *osbuild.Manifest, store string, errorWriter io.Writer) (*common.ComposeResult, error) {
	cmd := exec.Command(
		"osbuild",
		"--store", store,
		"--json", "-",
	)
	cmd.Stderr = errorWriter

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("error setting up stdin for osbuild: %v", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("error setting up stdout for osbuild: %v", err)
	}

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("error starting osbuild: %v", err)
	}

	err = json.NewEncoder(stdin).Encode(manifest)
	if err != nil {
		return nil, fmt.Errorf("error encoding osbuild pipeline: %v", err)
	}
	// FIXME: handle or comment this possible error
	_ = stdin.Close()

	var result common.ComposeResult
	err = json.NewDecoder(stdout).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("error decoding osbuild output: %#v", err)
	}

	err = cmd.Wait()
	if err != nil {
		return &result, err
	}

	return &result, nil
}
