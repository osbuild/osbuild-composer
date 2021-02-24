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
	"path"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/osbuild-composer/cmd/osbuild-image-tests/constants"
	"github.com/osbuild/osbuild-composer/internal/boot"
	"github.com/osbuild/osbuild-composer/internal/boot/azuretest"
	"github.com/osbuild/osbuild-composer/internal/boot/openstacktest"
	"github.com/osbuild/osbuild-composer/internal/boot/vmwaretest"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/test"
	"github.com/osbuild/osbuild-composer/internal/upload/vmware"
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

var disableLocalBoot = flag.Bool("disable-local-boot", false, "when this flag is given, no images are booted locally using qemu (this does not affect testing in clouds)")
var failLocalBoot = flag.Bool("fail-local-boot", true, "when this flag is on (default), local boot will fail. Usually indicates missing cloud credentials")

// runOsbuild runs osbuild with the specified manifest and output-directory.
func runOsbuild(manifest []byte, store, outputDirectory string, exports []string) error {
	cmd := constants.GetOsbuildCommand(store, outputDirectory, exports)

	cmd.Stdin = bytes.NewReader(manifest)
	var outBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stderr = &outBuffer

	err := cmd.Run()
	if err != nil {
		// Pretty print the osbuild error output.
		buf := new(bytes.Buffer)
		_ = json.Indent(buf, outBuffer.Bytes(), "", "    ")
		fmt.Println(buf)

		return fmt.Errorf("running osbuild failed: %v", err)
	}

	return nil
}

// testImageInfo runs image-info on image specified by imageImage and
// compares the result with expected image info
func testImageInfo(t *testing.T, imagePath string, rawImageInfoExpected []byte) {
	var imageInfoExpected interface{}
	err := json.Unmarshal(rawImageInfoExpected, &imageInfoExpected)
	require.NoErrorf(t, err, "cannot decode expected image info: %v", err)

	cmd := constants.GetImageInfoCommand(imagePath)
	cmd.Stderr = os.Stderr
	reader, writer := io.Pipe()
	cmd.Stdout = writer

	err = cmd.Start()
	require.NoErrorf(t, err, "image-info cannot start: %v", err)

	var imageInfoGot interface{}
	err = json.NewDecoder(reader).Decode(&imageInfoGot)
	require.NoErrorf(t, err, "decoding image-info output failed: %v", err)

	err = cmd.Wait()
	require.NoErrorf(t, err, "running image-info failed: %v", err)

	assert.Equal(t, imageInfoExpected, imageInfoGot)
}

type timeoutError struct{}

func (*timeoutError) Error() string { return "" }

// trySSHOnce tries to test the running image using ssh once
// It returns timeoutError if ssh command returns 255, if it runs for more
// that 10 seconds or if systemd-is-running returns starting.
// It returns nil if systemd-is-running returns running or degraded.
// It can also return other errors in other error cases.
func trySSHOnce(address string, privateKey string, ns *boot.NetNS) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmdName := "ssh"
	cmdArgs := []string{
		"-p", "22",
		"-i", privateKey,
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"redhat@" + address,
		"systemctl --wait is-system-running",
	}

	var cmd *exec.Cmd

	if ns != nil {
		cmd = ns.NamespacedCommandContext(ctx, cmdName, cmdArgs...)
	} else {
		cmd = exec.CommandContext(ctx, cmdName, cmdArgs...)
	}

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
func testSSH(t *testing.T, address string, privateKey string, ns *boot.NetNS) {
	const attempts = 20
	for i := 0; i < attempts; i++ {
		err := trySSHOnce(address, privateKey, ns)
		if err == nil {
			// pass the test
			return
		}

		// if any other error than the timeout one happened, fail the test immediately
		if _, ok := err.(*timeoutError); !ok {
			t.Fatal(err)
		}

		fmt.Println(err)
		time.Sleep(10 * time.Second)
	}

	t.Errorf("ssh test failure, %d attempts were made", attempts)
}

func testBootUsingQemu(t *testing.T, imagePath string) {
	if *failLocalBoot {
		t.Fatal("-fail-local-boot specified. Check missing cloud credentials!")
	}

	bootWithQemu(t, imagePath)
}

// will not fail even if -fail-local-boot is specified
func bootWithQemu(t *testing.T, imagePath string) {
	if *disableLocalBoot {
		t.Skip("local booting was disabled by -disable-local-boot, skipping")
	}
	err := boot.WithNetworkNamespace(func(ns boot.NetNS) error {
		return boot.WithBootedQemuImage(imagePath, ns, func() error {
			testSSH(t, "localhost", constants.TestPaths.PrivateKey, &ns)
			return nil
		})
	})
	require.NoError(t, err)
}

func testBootUsingNspawnImage(t *testing.T, imagePath string) {
	err := boot.WithNetworkNamespace(func(ns boot.NetNS) error {
		return boot.WithBootedNspawnImage(imagePath, ns, func() error {
			testSSH(t, "localhost", constants.TestPaths.PrivateKey, &ns)
			return nil
		})
	})
	require.NoError(t, err)
}

func testBootUsingNspawnDirectory(t *testing.T, imagePath string) {
	err := boot.WithNetworkNamespace(func(ns boot.NetNS) error {
		return boot.WithExtractedTarArchive(imagePath, func(dir string) error {
			return boot.WithBootedNspawnDirectory(dir, ns, func() error {
				testSSH(t, "localhost", constants.TestPaths.PrivateKey, &ns)
				return nil
			})
		})
	})
	require.NoError(t, err)
}

func testBootUsingAWS(t *testing.T, imagePath string) {
	creds, err := boot.GetAWSCredentialsFromEnv()
	require.NoError(t, err)

	// if no credentials are given, fall back to qemu
	if creds == nil {
		log.Print("no AWS credentials given, falling back to booting using qemu")
		testBootUsingQemu(t, imagePath)
		return

	}

	imageName, err := test.GenerateCIArtifactName("osbuild-image-tests-image-")
	require.NoError(t, err)

	e, err := boot.NewEC2(creds)
	require.NoError(t, err)

	// the following line should be done by osbuild-composer at some point
	err = boot.UploadImageToAWS(creds, imagePath, imageName)
	require.NoErrorf(t, err, "upload to amazon failed, resources could have been leaked")

	imageDesc, err := boot.DescribeEC2Image(e, imageName)
	require.NoErrorf(t, err, "cannot describe the ec2 image")

	// delete the image after the test is over
	defer func() {
		err = boot.DeleteEC2Image(e, imageDesc)
		require.NoErrorf(t, err, "cannot delete the ec2 image, resources could have been leaked")
	}()

	securityGroupName, err := test.GenerateCIArtifactName("osbuild-image-tests-security-group-")
	require.NoError(t, err)

	instanceTypeForArch := map[string]string{
		"x86_64":  "t3.micro",
		"aarch64": "t4g.micro",
	}

	instanceType, exists := instanceTypeForArch[common.CurrentArch()]
	if !exists {
		panic("unsupported AWS arch")
	}

	// boot the uploaded image and try to connect to it
	err = boot.WithSSHKeyPair(func(privateKey, publicKey string) error {
		return boot.WithBootedImageInEC2(e, securityGroupName, imageDesc, publicKey, instanceType, func(address string) error {
			testSSH(t, address, privateKey, nil)
			return nil
		})
	})
	require.NoError(t, err)
}

func testBootUsingAzure(t *testing.T, imagePath string) {
	creds, err := azuretest.GetAzureCredentialsFromEnv()
	require.NoError(t, err)

	// if no credentials are given, fall back to qemu
	if creds == nil {
		log.Print("no Azure credentials given, falling back to booting using qemu")
		testBootUsingQemu(t, imagePath)
		return
	}

	// create a random test id to name all the resources used in this test
	testId, err := test.GenerateCIArtifactName("")
	require.NoError(t, err)

	imageName := "image-" + testId + ".vhd"

	// the following line should be done by osbuild-composer at some point
	err = azuretest.UploadImageToAzure(creds, imagePath, imageName)
	require.NoErrorf(t, err, "upload to azure failed, resources could have been leaked")

	// delete the image after the test is over
	defer func() {
		err = azuretest.DeleteImageFromAzure(creds, imageName)
		require.NoErrorf(t, err, "cannot delete the azure image, resources could have been leaked")
	}()

	// boot the uploaded image and try to connect to it
	err = boot.WithSSHKeyPair(func(privateKey, publicKey string) error {
		return azuretest.WithBootedImageInAzure(creds, imageName, testId, publicKey, func(address string) error {
			testSSH(t, address, privateKey, nil)
			return nil
		})
	})
	require.NoError(t, err)
}

func testBootUsingOpenStack(t *testing.T, imagePath string) {
	creds, err := openstack.AuthOptionsFromEnv()

	// if no credentials are given, fall back to qemu
	if (creds == gophercloud.AuthOptions{}) {
		log.Print("No OpenStack credentials given, falling back to booting using qemu")
		testBootUsingQemu(t, imagePath)
		return
	}
	require.NoError(t, err)

	// provider is the top-level client that all OpenStack services derive from
	provider, err := openstack.AuthenticatedClient(creds)
	require.NoError(t, err)

	// create a random test id to name all the resources used in this test
	imageName, err := test.GenerateCIArtifactName("osbuild-image-tests-openstack-image-")
	require.NoError(t, err)

	// the following line should be done by osbuild-composer at some point
	image, err := openstacktest.UploadImageToOpenStack(provider, imagePath, imageName)
	require.NoErrorf(t, err, "Upload to OpenStack failed, resources could have been leaked")
	require.NotNil(t, image)

	// delete the image after the test is over
	defer func() {
		err = openstacktest.DeleteImageFromOpenStack(provider, image.ID)
		require.NoErrorf(t, err, "Cannot delete OpenStack image, resources could have been leaked")
	}()

	// boot the uploaded image and try to connect to it
	err = boot.WithSSHKeyPair(func(privateKey, publicKey string) error {
		userData, err := boot.CreateUserData(publicKey)
		require.NoErrorf(t, err, "Creating user data failed: %v", err)

		return openstacktest.WithBootedImageInOpenStack(provider, image.ID, userData, func(address string) error {
			testSSH(t, address, privateKey, nil)
			return nil
		})
	})
	require.NoError(t, err)
}

func testBootUsingVMware(t *testing.T, imagePath string) {
	creds, err := vmwaretest.AuthOptionsFromEnv()

	// if no credentials are given, fall back to qemu
	if creds == nil {
		log.Print("No vCenter credentials given, falling back to booting using qemu")
		log.Printf("Error=%v", err)
		testBootUsingQemu(t, imagePath)
		return
	}
	require.NoError(t, err)

	// convert to streamOptimized vmdk
	imageF, err := vmware.OpenAsStreamOptimizedVmdk(imagePath)
	require.NoError(t, err)
	// we don't need the file descriptor to be opened b/c import.vmdk operates
	// on the file path
	imageF.Close()
	imagePath = imageF.Name()
	require.NotEqual(t, "", imagePath)
	defer os.Remove(imagePath)

	// create a random test id to name all the resources used in this test
	imageName, err := test.GenerateCIArtifactName("osbuild-image-tests-vmware-image-")
	require.NoError(t, err)

	// the following line should be done by osbuild-composer at some point
	err = vmwaretest.ImportImage(creds, imagePath, imageName)
	require.NoErrorf(t, err, "Upload to vCenter failed, resources could have been leaked")

	// delete the image after the test is over
	defer func() {
		err = vmwaretest.DeleteImage(creds, imageName)
		require.NoErrorf(t, err, "Cannot delete image from vCenter, resources could have been leaked")
	}()

	// boot the uploaded image and try to connect to it
	err = vmwaretest.WithSSHKeyPair(func(privateKey, publicKey string) error {
		return vmwaretest.WithBootedImage(creds, imagePath, imageName, publicKey, func(address string) error {
			testSSH(t, address, privateKey, nil)
			return nil
		})
	})
	require.NoError(t, err)
}

// testBoot tests if the image is able to successfully boot
// Before the test it boots the image respecting the specified bootType.
// The test passes if the function is able to connect to the image via ssh
// in defined number of attempts and systemd-is-running returns running
// or degraded status.
func testBoot(t *testing.T, imagePath string, bootType string) {
	switch bootType {
	case "qemu":
		bootWithQemu(t, imagePath)

	case "nspawn":
		testBootUsingNspawnImage(t, imagePath)

	case "nspawn-extract":
		testBootUsingNspawnDirectory(t, imagePath)

	case "aws":
		testBootUsingAWS(t, imagePath)

	case "azure":
		testBootUsingAzure(t, imagePath)

	case "openstack":
		testBootUsingOpenStack(t, imagePath)

	case "vmware":
		testBootUsingVMware(t, imagePath)

	default:
		panic("unknown boot type!")
	}
}

// testImage performs a series of tests specified in the testcase
// on an image
func testImage(t *testing.T, testcase testcaseStruct, imagePath string) {
	if testcase.ImageInfo != nil {
		t.Run("image info", func(t *testing.T) {
			testImageInfo(t, imagePath, testcase.ImageInfo)
		})
	}

	if testcase.Boot != nil {
		t.Run("boot", func(t *testing.T) {
			testBoot(t, imagePath, testcase.Boot.Type)
		})
	}
}

// runTestcase builds the pipeline specified in the testcase and then it
// tests the result
func runTestcase(t *testing.T, testcase testcaseStruct, store string) {
	_ = os.Mkdir("/var/lib/osbuild-composer-tests", 0755)
	outputDirectory, err := ioutil.TempDir("/var/lib/osbuild-composer-tests", "osbuild-image-tests-*")
	require.NoError(t, err, "error creating temporary output directory")

	defer func() {
		err := os.RemoveAll(outputDirectory)
		require.NoError(t, err, "error removing temporary output directory")
	}()

	// NOTE(akoutsou) 1to2t: new v2 manifests name their last pipeline
	// "assembler" for compatibility with v1
	exports := []string{"assembler"}
	err = runOsbuild(testcase.Manifest, store, outputDirectory, exports)
	require.NoError(t, err)

	for _, export := range exports {
		imagePath := filepath.Join(outputDirectory, export, testcase.ComposeRequest.Filename)
		testImage(t, testcase, imagePath)
	}
}

// getAllCases returns paths to all testcases in the testcase directory
func getAllCases() ([]string, error) {
	cases, err := ioutil.ReadDir(constants.TestPaths.TestCasesDirectory)
	if err != nil {
		return nil, fmt.Errorf("cannot list test cases: %v", err)
	}

	casesPaths := []string{}
	for _, c := range cases {
		if c.IsDir() {
			continue
		}

		casePath := fmt.Sprintf("%s/%s", constants.TestPaths.TestCasesDirectory, c.Name())
		casesPaths = append(casesPaths, casePath)
	}

	return casesPaths, nil
}

// runTests opens, parses and runs all the specified testcases
func runTests(t *testing.T, cases []string) {
	_ = os.Mkdir("/var/lib/osbuild-composer-tests", 0755)
	store, err := ioutil.TempDir("/var/lib/osbuild-composer-tests", "osbuild-image-tests-*")
	require.NoError(t, err, "error creating temporary store")

	defer func() {
		err := os.RemoveAll(store)
		require.NoError(t, err, "error removing temporary store")
	}()

	for _, p := range cases {
		t.Run(path.Base(p), func(t *testing.T) {
			f, err := os.Open(p)
			if err != nil {
				t.Skipf("%s: cannot open test case: %v", p, err)
			}

			var testcase testcaseStruct
			err = json.NewDecoder(f).Decode(&testcase)
			require.NoErrorf(t, err, "%s: cannot decode test case", p)

			currentArch := common.CurrentArch()
			if testcase.ComposeRequest.Arch != currentArch {
				t.Skipf("the required arch is %s, the current arch is %s", testcase.ComposeRequest.Arch, currentArch)
			}

			runTestcase(t, testcase, store)
		})

	}
}

func TestImages(t *testing.T) {
	cases := flag.Args()
	// if no cases were specified, run the default set
	if len(cases) == 0 {
		var err error
		cases, err = getAllCases()
		require.NoError(t, err)
	}

	runTests(t, cases)
}
