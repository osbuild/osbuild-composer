package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/sirupsen/logrus"
)

// Run an instance of osbuild, returning a parsed osbuild.Result.
//
// Note that osbuild returns non-zero when the pipeline fails. This function
// does not return an error in this case. Instead, the failure is communicated
// with its corresponding logs through osbuild.Result.
func RunOSBuild(manifest distro.Manifest, store, outputDirectory string, exports []string, errorWriter io.Writer) (*osbuild.Result, error) {
	cmd := exec.Command(
		"osbuild",
		"--store", store,
		"--output-directory", outputDirectory,
		"--json",
		"--json-mode", "progress", "-",
	)

	for _, export := range exports {
		cmd.Args = append(cmd.Args, "--export", export)
	}
	cmd.Stderr = errorWriter

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("error setting up stdin for osbuild: %v", err)
	}

	stdout, _ := cmd.StdoutPipe()

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

	// Get the logs from osbuild as they come though stdout.
	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 0, 64*1024)
	//authorize the scanner to allocate memory up until 4MB.
	scanner.Buffer(buf, 4*1024*1024)

	//deactivate line number and file name for these logs as it makes everything confusing without adding valuable
	//information
	logrus.Debugln("Get and print logs from Osbuild")
	logrus.SetReportCaller(false)
	defer logrus.SetReportCaller(true)

	var result osbuild.Result
	for scanner.Scan() {
		var log *osbuild.OsbuildLog
		//try to decode the received json as a message if it fails, then
		decodeErr := json.Unmarshal(scanner.Bytes(), &log)
		if decodeErr != nil {
			decodeErr := json.Unmarshal(scanner.Bytes(), &result)
			if decodeErr != nil {
				return nil, fmt.Errorf("error decoding osbuild output: %v\nthe raw output:\n%s", decodeErr, scanner.Text())
			}
		} else {
			logrus.Debug(log.Message)
		}
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
