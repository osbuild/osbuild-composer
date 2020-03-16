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
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
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

	log.Print("[osbuild] running")
	err := cmd.Run()

	if err != nil {
		log.Print("[osbuild] failed")
		if _, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("running osbuild failed: %s", outBuffer.String())
		}
		return "", fmt.Errorf("running osbuild failed from an unexpected reason: %v", err)
	}

	log.Print("[osbuild] succeeded")

	var result struct {
		OutputID string `json:"output_id"`
	}

	err = json.NewDecoder(&outBuffer).Decode(&result)
	if err != nil {
		return "", fmt.Errorf("cannot decode osbuild output: %v", err)
	}

	return result.OutputID, nil
}

// extractXZ extracts an xz archive, it's just a simple wrapper around unxz(1).
func extractXZ(archivePath string) error {
	cmd := exec.Command("unxz", archivePath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cannot extract xz archive: %v", err)
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
func testImageInfo(imagePath string, rawImageInfoExpected []byte) error {
	var imageInfoExpected interface{}
	err := json.Unmarshal(rawImageInfoExpected, &imageInfoExpected)
	if err != nil {
		return fmt.Errorf("cannot decode expected image info: %v", err)
	}

	cmd := exec.Command(imageInfoPath, imagePath)
	cmd.Stderr = os.Stderr
	reader, writer := io.Pipe()
	cmd.Stdout = writer

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("image-info cannot start: %v", err)
	}

	var imageInfoGot interface{}
	err = json.NewDecoder(reader).Decode(&imageInfoGot)
	if err != nil {
		return fmt.Errorf("decoding image-info output failed: %v", err)
	}

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("running image-info failed: %v", err)
	}

	if diff := cmp.Diff(imageInfoExpected, imageInfoGot); diff != "" {
		return fmt.Errorf("image info differs:\n%s", diff)
	}

	return nil
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
			return fmt.Errorf("ssh command failed from unknown reason: %v", err)
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
func testSSH(ns netNS) error {
	const attempts = 20
	for i := 0; i < attempts; i++ {
		err := trySSHOnce(ns)
		if err == nil {
			return nil
		}
		if _, ok := err.(*timeoutError); !ok {
			return err
		}

		time.Sleep(10 * time.Second)
	}

	return fmt.Errorf("ssh test failure, %d attempts were made", attempts)
}

// testBoot tests if the image is able to successfully boot
// Before the test it boots the image respecting the specified bootType.
// The test passes if the function is able to connect to the image via ssh
// in defined number of attempts and systemd-is-running returns running
// or degraded status.
func testBoot(imagePath string, bootType string, outputID string) error {
	return withNetworkNamespace(func(ns netNS) error {
		switch bootType {
		case "qemu":
			fallthrough
		case "qemu-extract":
			return withBootedQemuImage(imagePath, ns, func() error {
				return testSSH(ns)
			})
		case "nspawn":
			return withBootedNspawnImage(imagePath, outputID, ns, func() error {
				return testSSH(ns)
			})
		case "nspawn-extract":
			return withExtractedTarArchive(imagePath, func(dir string) error {
				return withBootedNspawnDirectory(dir, outputID, ns, func() error {
					return testSSH(ns)
				})
			})
		default:
			panic("unknown boot type!")
		}
	})
}

// testImage performs a series of tests specified in the testcase
// on an image
func testImage(testcase testcaseStruct, imagePath, outputID string) error {
	if testcase.ImageInfo != nil {
		log.Print("[image info sub-test] running")
		err := testImageInfo(imagePath, testcase.ImageInfo)
		if err != nil {
			log.Print("[image info sub-test] failed")
			return err
		}
		log.Print("[image info sub-test] succeeded")
	} else {
		log.Print("[image info sub-test] not defined, skipping")
	}

	if testcase.Boot != nil {
		log.Print("[boot sub-test] running")
		err := testBoot(imagePath, testcase.Boot.Type, outputID)
		if err != nil {
			log.Print("[boot sub-test] failed")
			return err
		}
		log.Print("[boot sub-test] succeeded")
	} else {
		log.Print("[boot sub-test] not defined, skipping")
	}

	return nil
}

// runTestcase builds the pipeline specified in the testcase and then it
// tests the result
func runTestcase(testcase testcaseStruct) error {
	store, err := ioutil.TempDir("/var/tmp", "osbuild-image-tests-")
	if err != nil {
		return fmt.Errorf("cannot create temporary store: %v", err)
	}
	defer func() {
		err := os.RemoveAll(store)
		if err != nil {
			log.Printf("cannot remove temporary store: %v\n", err)
		}
	}()

	outputID, err := runOsbuild(testcase.Manifest, store)
	if err != nil {
		return err
	}

	imagePath := fmt.Sprintf("%s/refs/%s/%s", store, outputID, testcase.ComposeRequest.Filename)

	// if the result is xz archive but not tar.xz archive, extract it
	base, ex := splitExtension(imagePath)
	if ex == ".xz" {
		_, ex = splitExtension(base)
		if ex != ".tar" {
			if err := extractXZ(imagePath); err != nil {
				return err
			}
			imagePath = base
		}
	}

	return testImage(testcase, imagePath, outputID)
}

// getAllCases returns paths to all testcases in the testcase directory
func getAllCases() ([]string, error) {
	cases, err := ioutil.ReadDir(testCasesDirectoryPath)
	if err != nil {
		return nil, fmt.Errorf("cannot list test cases: %v", err)
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
func runTests(cases []string) error {
	for _, path := range cases {
		f, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("%s: cannot open test case: %v", path, err)
		}

		var testcase testcaseStruct
		err = json.NewDecoder(f).Decode(&testcase)
		if err != nil {
			return fmt.Errorf("%s: cannot decode test case: %v", path, err)
		}

		currentArch := common.CurrentArch()
		if testcase.ComposeRequest.Arch != currentArch {
			log.Printf("%s: skipping, the required arch is %s, the current arch is %s", path, testcase.ComposeRequest.Arch, currentArch)
			continue
		}

		hostDistroName, err := distro.GetHostDistroName()
		if err != nil {
			return fmt.Errorf("cannot get host distro name: %v", err)
		}

		// TODO: forge distro name for now
		if strings.HasPrefix(hostDistroName, "fedora") {
			hostDistroName = "fedora-30"
		}

		if testcase.ComposeRequest.Distro != hostDistroName {
			log.Printf("%s: skipping, the required distro is %s, the host distro is %s", path, testcase.ComposeRequest.Distro, hostDistroName)
			continue
		}

		log.Printf("%s: RUNNING", path)

		err = runTestcase(testcase)
		if err != nil {
			log.Printf("%s: FAILURE\nReason: %v", path, err)
		} else {
			log.Printf("%s: SUCCESS", path)
		}

	}

	return nil
}

func main() {
	flag.Parse()
	cases := flag.Args()

	// if no cases were specified, run the default set
	if len(cases) == 0 {
		var err error
		cases, err = getAllCases()
		if err != nil {
			log.Fatalf("searching for testcases failed: %v", err)
		}
	}

	err := runTests(cases)

	if err != nil {
		log.Fatalf("error occured while running tests: %v", err)
	}
}
