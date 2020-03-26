// +build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/internal/common"
)

type testcaseStruct struct {
	ComposeRequest struct {
		Distro   string
		Arch     string
		Filename string
	} `json:"compose-request"`
	Manifest  json.RawMessage
	ImageInfo json.RawMessage `json:"image-info"`
	Boot      *struct {
		Type string
	}
}

// runOsbuild runs osbuild with the specified manifest and store.
func runOsbuild(manifest []byte, store string) (string, error) {
	cmd := getOsbuildCommand(store)

	cmd.Stderr = os.Stderr
	cmd.Stdin = bytes.NewReader(manifest)
	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer

	err := cmd.Run()

	if err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			var formattedOutput bytes.Buffer
			_ = json.Indent(&formattedOutput, outBuffer.Bytes(), "", "  ")
			return "", fmt.Errorf("running osbuild failed: %s", formattedOutput.String())
		}
		return "", fmt.Errorf("running osbuild failed from an unexpected reason: %#v", err)
	}

	var result struct {
		OutputID string `json:"output_id"`
	}

	err = json.NewDecoder(&outBuffer).Decode(&result)
	if err != nil {
		return "", fmt.Errorf("cannot decode osbuild output: %#v", err)
	}

	return result.OutputID, nil
}

// extractXZ extracts an xz archive, it's just a simple wrapper around unxz(1).
func extractXZ(archivePath string) error {
	cmd := exec.Command("unxz", archivePath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cannot extract xz archive: %#v", err)
	}

	return nil
}

// splitExtension returns a file extension as the second return value and
// the rest as the first return value.
// The functionality should be the same as Python splitext's
func splitExtension(path string) (string, string) {
	ex := filepath.Ext(path)
	base := strings.TrimSuffix(path, ex)

	return base, ex
}

// testImageInfo runs image-info on image specified by imageImage and
// compares the result with expected image info
func testImageInfo(t *testing.T, imagePath string, rawImageInfoExpected []byte) {
	var imageInfoExpected interface{}
	err := json.Unmarshal(rawImageInfoExpected, &imageInfoExpected)
	require.Nilf(t, err, "cannot decode expected image info: %#v", err)

	cmd := exec.Command(imageInfoPath, imagePath)
	cmd.Stderr = os.Stderr
	reader, writer := io.Pipe()
	cmd.Stdout = writer

	err = cmd.Start()
	require.Nilf(t, err, "image-info cannot start: %#v", err)

	var imageInfoGot interface{}
	err = json.NewDecoder(reader).Decode(&imageInfoGot)
	require.Nilf(t, err, "decoding image-info output failed: %#v", err)

	err = cmd.Wait()
	require.Nilf(t, err, "running image-info failed: %#v", err)

	assert.Equal(t, imageInfoExpected, imageInfoGot)
}

type timeoutError struct{}

func (*timeoutError) Error() string { return "" }

// trySSHOnce tries to test the running image using ssh once
// It returns timeoutError if ssh command returns 255, if it runs for more
// that 10 seconds or if systemd-is-running returns starting.
// It returns nil if systemd-is-running returns running or degraded.
// It can also return other errors in other error cases.
func trySSHOnce(ns netNS) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := ns.NamespacedCommandContext(
		ctx,
		"ssh",
		"-p", "22",
		"-i", privateKeyPath,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"redhat@localhost",
		"systemctl --wait is-system-running",
	)
	output, err := cmd.Output()

	if ctx.Err() == context.DeadlineExceeded {
		return &timeoutError{}
	}

	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			if exitError.ExitCode() == 255 {
				return &timeoutError{}
			}
		} else {
			return fmt.Errorf("ssh command failed from unknown reason: %#v", err)
		}
	}

	outputString := strings.TrimSpace(string(output))
	switch outputString {
	case "running":
		return nil
	case "degraded":
		log.Print("ssh test passed, but the system is degraded")
		return nil
	case "starting":
		return &timeoutError{}
	default:
		return fmt.Errorf("ssh test failed, system status is: %s", outputString)
	}
}

// testSSH tests the running image using ssh.
// It tries 20 attempts before giving up. If a major error occurs, it might
// return earlier.
func testSSH(t *testing.T, ns netNS) {
	const attempts = 20
	for i := 0; i < attempts; i++ {
		err := trySSHOnce(ns)
		if err == nil {
			// pass the test
			return
		}

		// if any other error than the timeout one happened, fail the test immediately
		if _, ok := err.(*timeoutError); !ok {
			t.Fatal(err)
		}

		time.Sleep(10 * time.Second)
	}

	t.Errorf("ssh test failure, %d attempts were made", attempts)
}

// testBoot tests if the image is able to successfully boot
// Before the test it boots the image respecting the specified bootType.
// The test passes if the function is able to connect to the image via ssh
// in defined number of attempts and systemd-is-running returns running
// or degraded status.
func testBoot(t *testing.T, imagePath string, bootType string, outputID string) {
	err := withNetworkNamespace(func(ns netNS) error {
		switch bootType {
		case "qemu":
			fallthrough
		case "qemu-extract":
			return withBootedQemuImage(imagePath, ns, func() error {
				testSSH(t, ns)
				return nil
			})
		case "nspawn":
			return withBootedNspawnImage(imagePath, outputID, ns, func() error {
				testSSH(t, ns)
				return nil
			})
		case "nspawn-extract":
			return withExtractedTarArchive(imagePath, func(dir string) error {
				return withBootedNspawnDirectory(dir, outputID, ns, func() error {
					testSSH(t, ns)
					return nil
				})
			})
		default:
			panic("unknown boot type!")
		}
	})

	require.Nil(t, err)
}

// testImage performs a series of tests specified in the testcase
// on an image
func testImage(t *testing.T, testcase testcaseStruct, imagePath, outputID string) {
	if testcase.ImageInfo != nil {
		t.Run("image info", func(t *testing.T) {
			testImageInfo(t, imagePath, testcase.ImageInfo)
		})
	}

	if testcase.Boot != nil {
		t.Run("boot", func(t *testing.T) {
			testBoot(t, imagePath, testcase.Boot.Type, outputID)
		})
	}
}

// runTestcase builds the pipeline specified in the testcase and then it
// tests the result
func runTestcase(t *testing.T, testcase testcaseStruct) {
	store, err := ioutil.TempDir("/var/tmp", "osbuild-image-tests-")
	require.Nilf(t, err, "cannot create temporary store: %#v", err)

	defer func() {
		err := os.RemoveAll(store)
		if err != nil {
			log.Printf("cannot remove temporary store: %#v\n", err)
		}
	}()

	outputID, err := runOsbuild(testcase.Manifest, store)
	require.Nil(t, err)

	imagePath := fmt.Sprintf("%s/refs/%s/%s", store, outputID, testcase.ComposeRequest.Filename)

	// if the result is xz archive but not tar.xz archive, extract it
	base, ex := splitExtension(imagePath)
	if ex == ".xz" {
		_, ex = splitExtension(base)
		if ex != ".tar" {
			err := extractXZ(imagePath)
			require.Nil(t, err)
			imagePath = base
		}
	}

	testImage(t, testcase, imagePath, outputID)
}

// getAllCases returns paths to all testcases in the testcase directory
func getAllCases() ([]string, error) {
	cases, err := ioutil.ReadDir(testCasesDirectoryPath)
	if err != nil {
		return nil, fmt.Errorf("cannot list test cases: %#v", err)
	}

	casesPaths := []string{}
	for _, c := range cases {
		if c.IsDir() {
			continue
		}

		casePath := fmt.Sprintf("%s/%s", testCasesDirectoryPath, c.Name())
		casesPaths = append(casesPaths, casePath)
	}

	return casesPaths, nil
}

// runTests opens, parses and runs all the specified testcases
func runTests(t *testing.T, cases []string) {
	for _, path := range cases {
		t.Run(path, func(t *testing.T) {
			f, err := os.Open(path)
			require.Nilf(t, err, "%s: cannot open test case: %#v", path, err)

			var testcase testcaseStruct
			err = json.NewDecoder(f).Decode(&testcase)
			require.Nilf(t, err, "%s: cannot decode test case: %#v", path, err)

			currentArch := common.CurrentArch()
			if testcase.ComposeRequest.Arch != currentArch {
				t.Skipf("the required arch is %s, the current arch is %s", testcase.ComposeRequest.Arch, currentArch)
			}

			runTestcase(t, testcase)
		})

	}
}

func TestImages(t *testing.T) {
	cases := flag.Args()
	// if no cases were specified, run the default set
	if len(cases) == 0 {
		var err error
		cases, err = getAllCases()
		require.Nil(t, err)
	}

	runTests(t, cases)
}
