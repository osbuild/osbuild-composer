package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild1"
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

	var stdoutBuffer bytes.Buffer
	cmd.Stdout = &stdoutBuffer

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("error starting osbuild: %v", err)
	}

	err = json.NewEncoder(stdin).Encode(manifest)
	if err != nil {
		return nil, fmt.Errorf("error encoding osbuild pipeline: %v", err)
	}

	err = stdin.Close()
	if err != nil {
		return nil, fmt.Errorf("error closing osbuild's stdin: %v", err)
	}

	err = cmd.Wait()

	// try to decode the output even though the job could have failed
	var result osbuild.Result
	decodeErr := json.Unmarshal(stdoutBuffer.Bytes(), &result)
	if decodeErr != nil {
		return nil, fmt.Errorf("error decoding osbuild output: %v\nthe raw output:\n%s", decodeErr, stdoutBuffer.String())
	}

	if err != nil {
		// ignore ExitError if output could be decoded correctly
		if _, isExitError := err.(*exec.ExitError); !isExitError {
			return nil, fmt.Errorf("running osbuild failed: %v", err)
		}
	}

	return &result, nil
}
