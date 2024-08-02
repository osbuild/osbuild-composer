//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
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

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/osbuild-composer/cmd/osbuild-image-tests/constants"
	"github.com/osbuild/osbuild-composer/internal/boot"
	"github.com/osbuild/osbuild-composer/internal/boot/azuretest"
	"github.com/osbuild/osbuild-composer/internal/boot/openstacktest"
	"github.com/osbuild/osbuild-composer/internal/boot/vmwaretest"
	"github.com/osbuild/osbuild-composer/internal/test"
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

type strArrayFlag []string

func (a *strArrayFlag) String() string {
	return fmt.Sprintf("%+v", []string(*a))
}

func (a *strArrayFlag) Set(value string) error {
	*a = append(*a, value)
	return nil
}

var disableLocalBoot bool
var failLocalBoot bool
var skipSELinuxCtxCheck bool
var skipTmpfilesdPaths strArrayFlag

func init() {
	flag.BoolVar(&disableLocalBoot, "disable-local-boot", false, "when this flag is given, no images are booted locally using qemu (this does not affect testing in clouds)")
	flag.BoolVar(&failLocalBoot, "fail-local-boot", true, "when this flag is on (default), local boot will fail. Usually indicates missing cloud credentials")
	flag.BoolVar(&skipSELinuxCtxCheck, "skip-selinux-ctx-check", false, "when this flag is on, the 'selinux/context-mismatch' part is removed from the image-info report before it is checked.")
	flag.Var(&skipTmpfilesdPaths, "skip-tmpfilesd-path", "when this flag is given, the provided path is removed from the 'tmpfiles.d' section of the image-info report before it is checked.")
}

// runOsbuild runs osbuild with the specified manifest and output-directory.
func runOsbuild(manifest []byte, store, outputDirectory string, exports []string) error {
	cmd := constants.GetOsbuildCommand(store, outputDirectory, exports)

	cmd.Stdin = bytes.NewReader(manifest)
	var outBuffer, errBuffer bytes.Buffer
	cmd.Stdout = &outBuffer
	cmd.Stderr = &errBuffer

	err := cmd.Run()
	if err != nil {
		fmt.Println("stdout:")
		// stdout is json, indent it, otherwise we get a huge one-liner
		var formattedStdout bytes.Buffer
		indentErr := json.Indent(&formattedStdout, outBuffer.Bytes(), "", "  ")
		if indentErr == nil {
			fmt.Println(formattedStdout.String())
		} else {
			// fallback to raw output if json indent failed
			fmt.Println(outBuffer.String())
		}

		// stderr isn't structured, print it as is
		fmt.Printf("stderr:\n%s", errBuffer.String())

		return fmt.Errorf("running osbuild failed: %v", err)
	}

	return nil
}

// Delete the 'selinux/context-mismatch' part of the image-info report to
// workaround https://bugzilla.redhat.com/show_bug.cgi?id=1973754
func deleteSELinuxCtxFromImageInfoReport(imageInfoReport interface{}) {
	imageInfoMap := imageInfoReport.(map[string]interface{})
	selinuxReport, exists := imageInfoMap["selinux"]
	if exists {
		selinuxReportMap := selinuxReport.(map[string]interface{})
		delete(selinuxReportMap, "context-mismatch")
	}
}

// Delete the provided path form the 'tmpfiles.d' section of the image-info
// report. This is useful to workaround issues with non-deterministic content
// of dynamically generated tmpfiles.d configuration files present on the image.
func deleteTmpfilesdPathFromImageInfoReport(imageInfoReport interface{}, path string) {
	dir := filepath.Dir(path)
	file := filepath.Base(path)
	imageInfoMap := imageInfoReport.(map[string]interface{})
	tmpfilesdReport, exists := imageInfoMap["tmpfiles.d"]
	if exists {
		tmpfilesdReportMap := tmpfilesdReport.(map[string]interface{})
		tmpfilesdConfigDir, exists := tmpfilesdReportMap[dir]
		if exists {
			tmpfilesdConfigDirMap := tmpfilesdConfigDir.(map[string]interface{})
			delete(tmpfilesdConfigDirMap, file)
		}
	}
}

// Replace the UUID of the root LVM container with a static one.  We do not
// control this value and so it is not stable across image builds.
func replaceLVMUUID(imageInfoReport interface{}) {
	imageInfoMap := imageInfoReport.(map[string]interface{})
	partitions, exists := imageInfoMap["partitions"]
	if exists {
		for _, partition := range partitions.([]interface{}) {
			partitionInfoMap := partition.(map[string]interface{})
			if islvm, exists := partitionInfoMap["lvm"]; exists && islvm.(bool) == true {
				// replace UUID
				partitionInfoMap["uuid"] = "ffffff-ffff-ffff-ffff-ffff-ffff-ffffff"
			}
		}
	}
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

	if skipSELinuxCtxCheck {
		fmt.Println("ignoring 'selinux/context-mismatch' part of the image-info report")
		deleteSELinuxCtxFromImageInfoReport(imageInfoExpected)
		deleteSELinuxCtxFromImageInfoReport(imageInfoGot)
	}

	for _, path := range skipTmpfilesdPaths {
		fmt.Printf("ignoring %q path from the 'tmpfiles.d' part of the image-info report\n", path)
		deleteTmpfilesdPathFromImageInfoReport(imageInfoExpected, path)
		deleteTmpfilesdPathFromImageInfoReport(imageInfoGot, path)
	}

	replaceLVMUUID(imageInfoExpected)
	replaceLVMUUID(imageInfoGot)

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
	if failLocalBoot {
		t.Fatal("-fail-local-boot specified. Check missing cloud credentials!")
	}

	bootWithQemu(t, imagePath)
}

// will not fail even if -fail-local-boot is specified
func bootWithQemu(t *testing.T, imagePath string) {
	if disableLocalBoot {
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

	// the following line should be done by osbuild-composer at some point
	err = boot.UploadImageToAWS(creds, imagePath, imageName)
	require.NoErrorf(t, err, "upload to amazon failed, resources could have been leaked")

	imageDesc, err := boot.DescribeEC2Image(creds, imageName)
	require.NoErrorf(t, err, "cannot describe the ec2 image")

	// delete the image after the test is over
	defer func() {
		err = boot.DeleteEC2Image(creds, imageDesc)
		require.NoErrorf(t, err, "cannot delete the ec2 image, resources could have been leaked")
	}()

	securityGroupName, err := test.GenerateCIArtifactName("osbuild-image-tests-security-group-")
	require.NoError(t, err)

	instanceTypeForArch := map[string]string{
		"x86_64":  "t3.micro",
		"aarch64": "t4g.micro",
	}

	instanceType, exists := instanceTypeForArch[arch.Current().String()]
	if !exists {
		panic("unsupported AWS arch")
	}

	// boot the uploaded image and try to connect to it
	err = boot.WithSSHKeyPair(func(privateKey, publicKey string) error {
		return boot.WithBootedImageInEC2(creds, securityGroupName, imageDesc, publicKey, instanceType, func(address string) error {
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
	currentArch := arch.Current().String()

	// skip on aarch64 because we don't have aarch64 openstack or kvm machines
	if currentArch == arch.ARCH_AARCH64.String() {
		t.Skip("Openstack boot test is skipped on aarch64.")
		// if no credentials are given, fall back to qemu
	} else if (creds == gophercloud.AuthOptions{}) {
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

// guessPipelineToExport return a best-effort guess about which
// pipeline should be exported when running osbuild for the testcase
//
// If this function detects that this is a version 1 manifest, it
// always returns "assembler"
//
// For manifests version 2, the name of the last pipeline is returned.
func guessPipelineToExport(rawManifest json.RawMessage) string {
	const v1ManifestExportName = "assembler"
	var v2Manifest struct {
		Version   string `json:"version"`
		Pipelines []struct {
			Name string `json:"name,omitempty"`
		} `json:"pipelines"`
	}
	err := json.Unmarshal(rawManifest, &v2Manifest)
	if err != nil {
		// if we cannot unmarshal, let's just assume that it's a version 1 manifest
		return v1ManifestExportName
	}

	if v2Manifest.Version == "2" {
		return v2Manifest.Pipelines[len(v2Manifest.Pipelines)-1].Name
	}

	return v1ManifestExportName
}

// runTestcase builds the pipeline specified in the testcase and then it
// tests the result
func runTestcase(t *testing.T, testcase testcaseStruct, store string) {
	_ = os.Mkdir("/var/lib/osbuild-composer-tests", 0755)
	outputDirectory, err := os.MkdirTemp("/var/lib/osbuild-composer-tests", "osbuild-image-tests-*")
	require.NoError(t, err, "error creating temporary output directory")

	defer func() {
		err := os.RemoveAll(outputDirectory)
		require.NoError(t, err, "error removing temporary output directory")
	}()

	exports := []string{guessPipelineToExport(testcase.Manifest)}
	err = runOsbuild(testcase.Manifest, store, outputDirectory, exports)
	require.NoError(t, err)

	for _, export := range exports {
		imagePath := filepath.Join(outputDirectory, export, testcase.ComposeRequest.Filename)
		testImage(t, testcase, imagePath)
	}
}

// getAllCases returns paths to all testcases in the testcase directory
func getAllCases() ([]string, error) {
	cases, err := os.ReadDir(constants.TestPaths.TestCasesDirectory)
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
	store, err := os.MkdirTemp("/var/lib/osbuild-composer-tests", "osbuild-image-tests-*")
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

			currentArch := arch.Current().String()
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
