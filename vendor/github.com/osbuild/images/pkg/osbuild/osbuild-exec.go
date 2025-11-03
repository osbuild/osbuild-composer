package osbuild

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

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

var osbuildCmd = "osbuild"

type OSBuildOptions struct {
	StoreDir  string
	OutputDir string

	Exports     []string
	Checkpoints []string
	ExtraEnv    []string

	// If specified, the mutex is used for the syncwriter so the caller may write to the build
	// log as well. Also note that in case BuildLog is specified, stderr will be combined into
	// stdout.
	BuildLog   io.Writer
	BuildLogMu *sync.Mutex
	Stdout     io.Writer
	Stderr     io.Writer

	// If MonitorFD is set, a file (MonitorW) needs to be inherited by the osbuild process. The
	// caller should make sure to close it afterwards.
	Monitor     MonitorType
	MonitorFile *os.File

	JSONOutput bool

	CacheMaxSize int64
}

func NewOSBuildCmd(manifest []byte, optsPtr *OSBuildOptions) *exec.Cmd {
	opts := common.ValueOrEmpty(optsPtr)

	cacheMaxSize := int64(20 * datasizes.GiB)
	if opts.CacheMaxSize != 0 {
		cacheMaxSize = opts.CacheMaxSize
	}

	// nolint: gosec
	cmd := exec.Command(
		osbuildCmd,
		"--store", opts.StoreDir,
		"--output-directory", opts.OutputDir,
		fmt.Sprintf("--cache-max-size=%v", cacheMaxSize),
		"-",
	)

	for _, export := range opts.Exports {
		cmd.Args = append(cmd.Args, "--export", export)
	}

	for _, checkpoint := range opts.Checkpoints {
		cmd.Args = append(cmd.Args, "--checkpoint", checkpoint)
	}

	if opts.Monitor != "" {
		cmd.Args = append(cmd.Args, fmt.Sprintf("--monitor=%s", opts.Monitor))
	}

	if opts.MonitorFile != nil {
		cmd.Args = append(cmd.Args, "--monitor-fd=3")
		cmd.ExtraFiles = []*os.File{opts.MonitorFile}
	}

	if opts.JSONOutput {
		cmd.Args = append(cmd.Args, "--json")
	}

	// Default to os stdout/stderr. This is for maximum compatibility with the existing
	// bootc-image-builder in "verbose" mode where stdout, stderr come directly from osbuild.
	var stdout, stderr io.Writer
	stdout = os.Stdout
	if opts.Stdout != nil {
		stdout = opts.Stdout
	}
	cmd.Stdout = stdout
	stderr = os.Stderr
	if opts.Stderr != nil {
		stderr = opts.Stderr
	}
	cmd.Stderr = stderr

	if opts.BuildLog != nil {
		// There is a slight wrinkle here: when requesting a buildlog we can no longer write
		// to separate stdout/stderr streams without being racy and give potential
		// out-of-order output (which is very bad and confusing in a log). The reason is
		// that if cmd.Std{out,err} are different "go" will start two go-routine to
		// monitor/copy those are racy when both stdout,stderr output happens close together
		// (TestRunOSBuildWithBuildlog demos that). We cannot have our cake and eat it so
		// here we need to combine osbuilds stderr into our stdout.
		// stdout → syncw → multiw → stdoutw or os stdout
		// stderr ↗↗↗             → buildlog
		var mw io.Writer
		if opts.BuildLogMu == nil {
			opts.BuildLogMu = new(sync.Mutex)
		}
		mw = newSyncedWriter(opts.BuildLogMu, io.MultiWriter(stdout, opts.BuildLog))
		cmd.Stdout = mw
		cmd.Stderr = mw
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
func RunOSBuild(manifest []byte, optsPtr *OSBuildOptions) (*Result, error) {
	opts := common.ValueOrEmpty(optsPtr)

	if err := CheckMinimumOSBuildVersion(); err != nil {
		return nil, err
	}

	var stdoutBuffer bytes.Buffer
	var res Result
	cmd := NewOSBuildCmd(manifest, &opts)

	if opts.JSONOutput {
		cmd.Stdout = &stdoutBuffer
	}
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
	cmd := exec.Command(osbuildCmd, "--version")
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
	cmd := exec.Command(osbuildCmd, "--inspect")
	cmd.Stdin = bytes.NewBuffer(manifest)

	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	return out, nil
}

type syncedWriter struct {
	mu *sync.Mutex
	w  io.Writer
}

func newSyncedWriter(mu *sync.Mutex, w io.Writer) io.Writer {
	return &syncedWriter{mu: mu, w: w}
}

func (sw *syncedWriter) Write(p []byte) (n int, err error) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	return sw.w.Write(p)
}
