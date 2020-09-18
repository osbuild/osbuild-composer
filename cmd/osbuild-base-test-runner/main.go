// +build integration

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/osbuild/osbuild-composer/internal/boot"
	"github.com/osbuild/osbuild-composer/internal/upload/awsupload"
)

type timeoutError struct{}

func (*timeoutError) Error() string { return "Timeout exceeded" }

// panicErr panics on err != nil
func panicErr(err error) {
	if err != nil {
		panic(err)
	}
}

// GenerateCIArtifactName generates a new identifier for CI artifacts which is based
// on environment variables specified by Jenkins
// note: in case of migration to sth else like Github Actions, change it to whatever variables GH Action provides
func GenerateCIArtifactName(prefix string) (string, error) {
	distroCode := os.Getenv("DISTRO_CODE")
	branchName := os.Getenv("BRANCH_NAME")
	buildId := os.Getenv("BUILD_ID")
	if branchName == "" || buildId == "" || distroCode == "" {
		return "", fmt.Errorf("The environment variables must specify BRANCH_NAME, BUILD_ID, and DISTRO_CODE")
	}

	return fmt.Sprintf("%s%s-%s-%s", prefix, distroCode, branchName, buildId), nil
}

// sshDisableHostChecking disables host (=remote machine) key checking
// because the key of the fresh VM in AWS is not in the list of known_hosts.
// The function also uses an empty known_hosts file by setting it to /dev/null.
func sshDisableHostChecking() []string {
	return []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
	}
}

func runLocalCommand(name string, args ...string) int {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("Error: ", err)
		return 1
	}
	return 0
}

func runSSHCommand(address, privateKey, command string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmdName := "ssh"
	cmdArgs := []string{
		"-i", privateKey,
	}
	cmdArgs = append(cmdArgs, sshDisableHostChecking()...)
	cmdArgs = append(cmdArgs, "redhat@"+address, command)

	var cmd *exec.Cmd = exec.CommandContext(ctx, cmdName, cmdArgs...)

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
	fmt.Println(outputString)

	return nil
}

// testSSHOnce tries to test the running image using ssh once
// It returns timeoutError if ssh command returns 255, if it runs for more
// that 10 seconds or if systemd-is-running returns starting.
// It returns nil if systemd-is-running returns running or degraded.
// It can also return other errors in other error cases.
func testSSHOnce(address string, privateKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmdName := "ssh"
	cmdArgs := []string{
		"-i", privateKey,
	}
	cmdArgs = append(cmdArgs, sshDisableHostChecking()...)
	cmdArgs = append(cmdArgs, "redhat@"+address, "systemctl --wait is-system-running")

	var cmd *exec.Cmd = exec.CommandContext(ctx, cmdName, cmdArgs...)

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
		fmt.Println("ssh test passed")
		return nil
	case "degraded":
		fmt.Println("ssh test passed, but the system is degraded")
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
func testSSH(address string, privateKey string) {
	const attempts = 20
	for i := 0; i < attempts; i++ {
		err := testSSHOnce(address, privateKey)
		if err == nil {
			// pass the test
			return
		}

		// if any other error than the timeout one happened, fail the test immediately
		if _, ok := err.(*timeoutError); !ok {
			panic(err)
		}

		time.Sleep(10 * time.Second)
	}

	panic(fmt.Sprintf("ssh test failure, %d attempts were made", attempts))
}

func runTestOverSSH(address, privateKey, testCommand string) int {
	// Get details about the machine
	err := runSSHCommand(address, privateKey, "cat /etc/os-release")
	panicErr(err)

	// Prepare the socket for forwarding because it is not owned by the redhat user
	err = runSSHCommand(address, privateKey, "sudo chmod go+rw /run/weldr/api.socket")
	panicErr(err)

	// Forward the socket to the local machine
	runLocalCommand("sudo", "mkdir", "/run/weldr")
	forwardSocketCommand := []string{"ssh"}
	forwardSocketCommand = append(forwardSocketCommand, sshDisableHostChecking()...)
	// Use this private key as an identity
	forwardSocketCommand = append(forwardSocketCommand, "-i", privateKey)
	// -f = go to background
	// -N = do not execute a remote command, only forward the port
	// -L = forward the port
	forwardSocketCommand = append(forwardSocketCommand, "-fN", "-L", "/run/weldr/api.socket:/run/weldr/api.socket", fmt.Sprintf("redhat@%s", address))
	runLocalCommand("sudo", forwardSocketCommand...)

	fmt.Println("Running test: ", testCommand)
	return runLocalCommand(testCommand)
}

func cleanupImageAndSnapshot(e *ec2.EC2, imageName string) {
	fmt.Println("Getting the EC2 image description")
	imageDesc, err := boot.DescribeEC2Image(e, imageName)
	if err != nil {
		fmt.Println("Could not get image description, this is expected.")
	} else {
		fmt.Println("Image exists, this is unexpected, but deleting it anyway.")
		fmt.Println("[AWS] 🧹 Deregistering image", imageName)
		err = boot.DeregisterEC2Image(e, imageDesc.Id)
		if err != nil {
			fmt.Println("Cannot delete the ec2 image, resources could have been leaked")
		}
	}
	fmt.Println("[AWS] 🧹 Deregistering snapshot", imageName)
	sid, err := boot.DescribeEC2Snapshot(e, imageName)
	panicErr(err)
	err = boot.DeleteEC2Snapshot(e, sid)
	panicErr(err)
	fmt.Println("[AWS] 🧹 Successfully wiped all images and snapshots associated with this PR.")
}

func runTest(e *ec2.EC2, imageName, securityGroupName string) int {
	// Start by registering the image from a stapshot if it is not already registered
	fmt.Println("Getting the EC2 image description")
	imageDesc, err := boot.DescribeEC2Image(e, imageName)
	if err != nil {
		fmt.Println("Failed to describe EC2 image, trying to register it from a snapshot.")
		sid, err := boot.DescribeEC2Snapshot(e, imageName)
		panicErr(err)
		fmt.Printf("[AWS] 📋 Registering AMI from imported snapshot: %s", *sid)
		imgId, err := awsupload.RegisterEC2Snapshot(e, sid, imageName)
		panicErr(err)
		imageDesc = new(boot.ImageDescription)
		imageDesc.Id = imgId
		imageDesc.SnapshotId = sid
	}

	// Deregister the image after the test is over
	defer func() {
		fmt.Println("[AWS] 🧹 Deregistering image", imageName)
		err = boot.DeregisterEC2Image(e, imageDesc.Id)
		if err != nil {
			fmt.Println("Cannot delete the ec2 image, resources could have been leaked")
		}
	}()

	// Boot the image and try to connect to it. In case it works, run the test.
	fmt.Println("[AWS] 🚀 Booting the image")
	var returnCode int = 1
	err = boot.WithSSHKeyPair(func(privateKey, publicKey string) error {
		return boot.WithBootedImageInEC2(e, securityGroupName, imageDesc, publicKey, func(address string) error {
			// First make sure it works
			testSSH(address, privateKey)
			// Now run the test
			returnCode = runTestOverSSH(address, privateKey, os.Args[1])
			return nil
		})
	})
	return returnCode
}

func main() {
	// Parse cmdline arguments if there are any
	cleanup := false
	if len(os.Args) > 1 {
		if os.Args[1] == "-h" || os.Args[1] == "-help" {
			fmt.Printf("Usage: %s [-h | -help] [-c | -cleanup] [TEST]", os.Args[0])
			return
		}
		if os.Args[1] == "-c" || os.Args[1] == "-cleanup" {
			fmt.Println("Running AWS cleanup")
			cleanup = true
		}
	}

	// Get AWS credentials and Jenkins job variables which will identify the image in AWS
	fmt.Println("Getting AWS credentials")
	creds, err := boot.GetAWSCredentialsFromEnv()
	panicErr(err)
	if creds == nil {
		panic("Empty AWS credentials")
	}

	// Generate names for CI artifacts
	imageName, err := GenerateCIArtifactName("osbuild-composer-base-test-")
	panicErr(err)
	securityGroupName, err := GenerateCIArtifactName("osbuild-image-tests-security-group-")
	panicErr(err)

	// Get credentials for EC2
	e, err := boot.NewEC2(creds)
	panicErr(err)

	// Cleanup mode:
	// Try to wipe the image and snapshot associated with this PR
	if cleanup {
		cleanupImageAndSnapshot(e, imageName)
		return
	}

	// "Regular" mode:
	// Spin up a VM in AWS and run the test there
	returnCode := runTest(e, imageName, securityGroupName)

	os.Exit(returnCode)
}
