package osbuild

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/hashicorp/go-version"

	"github.com/osbuild/images/data/dependencies"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/datasizes"
)

type MonitorType string

const (
	MonitorJSONSeq = "JSONSeqMonitor"
	MonitorNull    = "NullMonitor"
	MonitorLog     = "LogMonitor"
)

type OSBuildOptions struct {
	StoreDir  string
	OutputDir string
	ExtraEnv  []string

	Monitor   MonitorType
	MonitorFD uintptr

	JSONOutput bool

	CacheMaxSize int64
}

func NewOSBuildCmd(manifest []byte, exports, checkpoints []string, optsPtr *OSBuildOptions) *exec.Cmd {
	opts := common.ValueOrEmpty(optsPtr)

	cacheMaxSize := int64(20 * datasizes.GiB)
	if opts.CacheMaxSize != 0 {
		cacheMaxSize = opts.CacheMaxSize
	}

	// nolint: gosec
	cmd := exec.Command(
		"osbuild",
		"--store", opts.StoreDir,
		"--output-directory", opts.OutputDir,
		fmt.Sprintf("--cache-max-size=%v", cacheMaxSize),
		"-",
	)

	for _, export := range exports {
		cmd.Args = append(cmd.Args, "--export", export)
	}

	for _, checkpoint := range checkpoints {
		cmd.Args = append(cmd.Args, "--checkpoint", checkpoint)
	}

	if opts.Monitor != "" {
		cmd.Args = append(cmd.Args, fmt.Sprintf("--monitor=%s", opts.Monitor))
	}
	if opts.MonitorFD != 0 {
		cmd.Args = append(cmd.Args, fmt.Sprintf("--monitor-fd=%d", opts.MonitorFD))
	}
	if opts.JSONOutput {
		cmd.Args = append(cmd.Args, "--json")
	}

	cmd.Env = append(os.Environ(), opts.ExtraEnv...)
	cmd.Stdin = bytes.NewBuffer(manifest)
	return cmd
}

// Run an instance of osbuild, returning a parsed osbuild.Result.
//
// Note that osbuild returns non-zero when the pipeline fails. This function
// does not return an error in this case. Instead, the failure is communicated
// with its corresponding logs through osbuild.Result.
func RunOSBuild(manifest []byte, exports, checkpoints []string, errorWriter io.Writer, opts *OSBuildOptions) (*Result, error) {
	if err := CheckMinimumOSBuildVersion(); err != nil {
		return nil, err
	}

	var stdoutBuffer bytes.Buffer
	var res Result
	cmd := NewOSBuildCmd(manifest, exports, checkpoints, opts)

	if opts.JSONOutput {
		cmd.Stdout = &stdoutBuffer
	} else {
		cmd.Stdout = os.Stdout
	}
	cmd.Stderr = errorWriter

	err := cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("error starting osbuild: %v", err)
	}

	err = cmd.Wait()
	if opts.JSONOutput {
		// try to decode the output even though the job could have failed
		if stdoutBuffer.Len() == 0 {
			return nil, fmt.Errorf("osbuild did not return any output")
		}
		decodeErr := json.Unmarshal(stdoutBuffer.Bytes(), &res)
		if decodeErr != nil {
			return nil, fmt.Errorf("error decoding osbuild output: %v\nthe raw output:\n%s", decodeErr, stdoutBuffer.String())
		}
	}

	if err != nil {
		// ignore ExitError if output could be decoded correctly (only if running with --json)
		if _, isExitError := err.(*exec.ExitError); !isExitError || !opts.JSONOutput {
			return nil, fmt.Errorf("running osbuild failed: %v", err)
		}
	}

	return &res, nil
}

func CheckMinimumOSBuildVersion() error {
	osbuildVersion, err := OSBuildVersion()
	if err != nil {
		return fmt.Errorf("error getting osbuild version: %v", err)
	}

	minVersion, err := version.NewVersion(dependencies.MinimumOSBuildVersion())
	if err != nil {
		return fmt.Errorf("error parsing minimum osbuild version: %v", err)
	}

	currentVersion, err := version.NewVersion(osbuildVersion)
	if err != nil {
		return fmt.Errorf("error parsing current osbuild version: %v", err)
	}

	if currentVersion.LessThan(minVersion) {
		return fmt.Errorf("osbuild version %q is lower than the minimum required version %q",
			osbuildVersion, dependencies.MinimumOSBuildVersion())
	}

	return nil
}

// OSBuildVersion returns the version of osbuild.
func OSBuildVersion() (string, error) {
	var stdoutBuffer bytes.Buffer
	cmd := exec.Command("osbuild", "--version")
	cmd.Stdout = &stdoutBuffer

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("running osbuild failed: %v", err)
	}

	// osbuild --version prints the version in the form of "osbuild VERSION". Extract the version.
	version := strings.TrimPrefix(stdoutBuffer.String(), "osbuild ")
	// Remove the trailing newline.
	version = strings.TrimSpace(version)
	return version, nil
}

// OSBuildInspect converts a manifest to an inspected manifest.
func OSBuildInspect(manifest []byte) ([]byte, error) {
	cmd := exec.Command("osbuild", "--inspect")
	cmd.Stdin = bytes.NewBuffer(manifest)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return out, nil
}
