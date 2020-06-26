package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
)

type OSBuildError struct {
	Message string
	Result  *common.ComposeResult
}

func (e *OSBuildError) Error() string {
	return e.Message
}

func parseOSBuildOutput(output bytes.Buffer) (*common.ComposeResult, error) {
	var result common.ComposeResult
	err := json.NewDecoder(&output).Decode(&result)
	if err != nil {
		return nil, fmt.Errorf("%v\nraw osbuild output:\n%s", err, output.String())
	}

	return &result, nil

}

func RunOSBuild(manifest distro.Manifest, store, outputDirectory string, errorWriter io.Writer) (*common.ComposeResult, error) {
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

	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer

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

	osbuildErr := cmd.Wait()
	osbuildResult, parseErr := parseOSBuildOutput(outBuffer)

	if osbuildErr != nil {
		if parseErr != nil {
			return nil, fmt.Errorf("running osbuild failed: %v\nparsing osbuild output also failed: %v", osbuildErr, parseErr)
		}

		return nil, &OSBuildError{
			Message: fmt.Sprintf("running osbuild failed: %v", osbuildErr),
			Result:  osbuildResult,
		}
	}

	if parseErr != nil {
		return nil, fmt.Errorf("osbuild succeeded but parsing its output failed: %v", parseErr)
	}

	return osbuildResult, nil
}
