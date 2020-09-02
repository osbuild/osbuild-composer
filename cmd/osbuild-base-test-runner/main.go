// +build integration

package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/osbuild/osbuild-composer/internal/boot"
)

type timeoutError struct{}

func (*timeoutError) Error() string { return "Timeout exceeded" }

// GenerateCIArtifactName generates a new identifier for CI artifacts which is based
// on environment variables specified by Jenkins
// note: in case of migration to sth else like Github Actions, change it to whatever variables GH Action provides
func GenerateCIArtifactName(prefix string) (string, error) {
	distroCode := os.Getenv("DISTRO_CODE")
	changeId := os.Getenv("CHANGE_ID")
	buildId := os.Getenv("BUILD_ID")
	if changeId == "" || buildId == "" || distroCode == "" {
		return "", fmt.Errorf("The environment variables must specify CHANGE_ID, BUILD_ID, and DISTRO_CODE")
	}

	return fmt.Sprintf("%s%s-%s-%s", prefix, distroCode, changeId, buildId), nil
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

func runLocalCommand(name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("Error: ", err)
	}
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

func main() {
	fmt.Println("Getting AWS credentials")
	creds, err := boot.GetAWSCredentialsFromEnv()
	if err != nil {
		panic("AWS credentials unavailable")
	}
	if creds == nil {
		panic("Empty AWS credentials")
	}
<<<<<<< HEAD
=======

	// Get environment variables defining the name of CI artifacts
>>>>>>> 1fc50d4... amend introduce
	fmt.Println("Getting change and build IDs")
	changeId := os.Getenv("CHANGE_ID")
	buildId := os.Getenv("BUILD_ID")
	if changeId == "" || buildId == "" {
		panic("The environment variables must specify CHANGE_ID and BUILD_ID")
	}
	imageName := fmt.Sprintf("osbuild-composer-base-test-%s-%s", changeId, buildId)
<<<<<<< HEAD
	fmt.Println("Getting the EC2 image description")
=======

	securityGroupName, err := GenerateCIArtifactName("osbuild-image-tests-security-group-")
	if err != nil {
		panic(err)
	}

>>>>>>> 1fc50d4... amend introduce
	e, err := boot.NewEC2(creds)
	if err != nil {
		panic("Failed to obtain credentials for EC2")
	}
	imageDesc, err := boot.DescribeEC2Image(e, imageName)
	if err != nil {
		panic("Failed to describe EC2 image")
	}
	// delete the image after the test is over
	defer func() {
		err = boot.DeregisterEC2Image(e, imageDesc)
		if err != nil {
			fmt.Println("Cannot delete the ec2 image, resources could have been leaked")
		}
	}()
	fmt.Println("Booting the image")
	// boot the uploaded image and try to connect to it
	err = boot.WithSSHKeyPair(func(privateKey, publicKey string) error {
		return boot.WithBootedImageInEC2(e, securityGroupName, imageDesc, publicKey, func(address string) error {
			testSSH(address, privateKey)
			err = runSSHCommand(address, privateKey, "cat /etc/os-release")
			if err != nil {
				panic(err)
			}
			err = runSSHCommand(address, privateKey, "sudo chmod go+rw /run/weldr/api.socket")
			if err != nil {
				panic(err)
			}
			runLocalCommand("sudo", "mkdir", "/run/weldr")
			runLocalCommand("sudo", "ssh", "-o", "StrictHostKeyChecking=no", "-o", "UserKnownHostsFile=/dev/null", "-i", privateKey, "-fN", "-L", "/run/weldr/api.socket:/run/weldr/api.socket", fmt.Sprintf("redhat@%s", address))
			fmt.Println("Running test: ", os.Args[1])
			runLocalCommand(os.Args[1])
			return nil
		})
	})
}
