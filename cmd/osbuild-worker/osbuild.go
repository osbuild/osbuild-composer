package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
)

// Run an instance of osbuild, returning a parsed osbuild.Result.
//
// Note that osbuild returns non-zero when the pipeline fails. This function
// does not return an error in this case. Instead, the failure is communicated
// with its corresponding logs through osbuild.Result.
func RunOSBuild(manifest distro.Manifest, store, outputDirectory string, errorWriter io.Writer) (*osbuild.Result, error) {
	cmd := exec.Command(
		"osbuild",
		"--store", store,
		"--output-directory", outputDirectory,
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

	var result osbuild.Result
	err = json.NewDecoder(stdout).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("error decoding osbuild output: %#v", err)
	}

	err = cmd.Wait()
	if err != nil {
		// ignore ExitError if output could be decoded correctly
		if _, isExitError := err.(*exec.ExitError); !isExitError {
			return nil, fmt.Errorf("running osbuild failed: %v", err)
		}
	}

	return &result, nil
}
