package main

import (
	"bytes"
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
}

// runOsbuild runs osbuild with the specified manifest and store.
func runOsbuild(manifest []byte, store string) (string, error) {
	cmd := exec.Command(
		"osbuild",
		"--store", store,
		"--json",
		"-",
	)

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

	cmd := exec.Command("/usr/libexec/osbuild-composer/image-info", imagePath)
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

// testImage performs a series of tests specified in the testcase
// on an image
func testImage(testcase testcaseStruct, imagePath string) error {
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

	// if the result is xz archive, extract it
	base, ex := splitExtension(imagePath)
	if ex == ".xz" {
		if err := extractXZ(imagePath); err != nil {
			return err
		}
		imagePath = base
	}

	return testImage(testcase, imagePath)
}

// getAllCases returns paths to all testcases in the testcase directory
func getAllCases() ([]string, error) {
	const casesDirectory = "/usr/share/tests/osbuild-composer/cases"
	cases, err := ioutil.ReadDir(casesDirectory)
	if err != nil {
		return nil, fmt.Errorf("cannot list test cases: %v", err)
	}

	casesPaths := []string{}
	for _, c := range cases {
		if c.IsDir() {
			continue
		}

		casePath := fmt.Sprintf("%s/%s", casesDirectory, c.Name())
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
