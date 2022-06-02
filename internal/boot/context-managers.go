// +build integration

package boot

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/osbuild/osbuild-composer/cmd/osbuild-image-tests/constants"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
)

// WithNetworkNamespace provides the function f with a new network namespace
// which is deleted immediately after f returns
func WithNetworkNamespace(f func(ns NetNS) error) error {
	ns, err := newNetworkNamespace()
	if err != nil {
		return err
	}

	defer func() {
		err := ns.Delete()
		if err != nil {
			log.Printf("cannot delete network namespace: %v", err)
		}
	}()

	return f(ns)
}

// withTempFile provides the function f with a new temporary file
// dir and pattern parameters have the same semantics as in ioutil.TempFile
func withTempFile(dir, pattern string, f func(file *os.File) error) error {
	tempFile, err := ioutil.TempFile(dir, pattern)
	if err != nil {
		return fmt.Errorf("cannot create the temporary file: %v", err)
	}

	defer func() {
		err := os.Remove(tempFile.Name())
		if err != nil {
			log.Printf("cannot remove the temporary file: %v", err)
		}
	}()

	return f(tempFile)
}

func withTempDir(dir, pattern string, f func(dir string) error) error {
	tempDir, err := ioutil.TempDir(dir, pattern)
	if err != nil {
		return fmt.Errorf("cannot create the temporary directory %v", err)
	}

	defer func() {
		err := os.RemoveAll(tempDir)
		if err != nil {
			log.Printf("cannot remove the temporary directory: %v", err)
		}
	}()

	return f(tempDir)
}

// writeCloudInitSO creates cloud-init iso from specified userData and
// metaData and writes it to the writer
func writeCloudInitISO(writer io.Writer, userData, metaData string) error {
	isoCmd := exec.Command(
		"mkisofs",
		"-quiet",
		"-input-charset", "utf-8",
		"-volid", "cidata",
		"-joliet",
		"-rock",
		userData,
		metaData,
	)
	isoCmd.Stdout = writer
	isoCmd.Stderr = os.Stderr

	err := isoCmd.Run()
	if err != nil {
		return fmt.Errorf("cannot create cloud-init iso: %v", err)
	}

	return nil
}

// WithBootedQemuImage boots the specified image in the specified namespace
// using qemu. The VM is killed immediately after function returns.
func WithBootedQemuImage(image string, ns NetNS, f func() error) error {
	return withTempFile("", "osbuild-image-tests-cloudinit", func(cloudInitFile *os.File) error {
		err := writeCloudInitISO(
			cloudInitFile,
			constants.TestPaths.UserData,
			constants.TestPaths.MetaData,
		)
		if err != nil {
			return err
		}

		err = cloudInitFile.Close()
		if err != nil {
			return fmt.Errorf("cannot close temporary cloudinit file: %v", err)
		}

		var qemuCmd *exec.Cmd
		if common.CurrentArch() == "x86_64" {
			hostDistroName, _, _, err := common.GetHostDistroName()
			if err != nil {
				return fmt.Errorf("cannot determing the current distro: %v", err)
			}

			var qemuPath string
			if strings.HasPrefix(hostDistroName, "rhel") || strings.HasPrefix(hostDistroName, "centos") {
				qemuPath = "/usr/libexec/qemu-kvm"
			} else {
				qemuPath = "qemu-system-x86_64"
			}

			qemuCmd = ns.NamespacedCommand(
				qemuPath,
				"-cpu", "host",
				"-smp", strconv.Itoa(runtime.NumCPU()),
				"-m", "1024",
				"-snapshot",
				"-M", "accel=kvm",
				"-cdrom", cloudInitFile.Name(),
				"-net", "nic,model=rtl8139", "-net", "user,hostfwd=tcp::22-:22",
				"-nographic",
				image,
			)
		} else if common.CurrentArch() == distro.Aarch64ArchName {
			// This command does not use KVM as I was unable to make it work in Beaker,
			// once we have machines that can use KVM, enable it to make it faster
			qemuCmd = ns.NamespacedCommand(
				"qemu-system-aarch64",
				"-cpu", "host",
				"-M", "virt",
				"-m", "2048",
				// As opposed to x86_64, aarch64 uses UEFI, this one comes from edk2-aarch64 package on Fedora
				"-bios", "/usr/share/edk2/aarch64/QEMU_EFI.fd",
				"-boot", "efi",
				"-M", "accel=kvm",
				"-snapshot",
				"-cdrom", cloudInitFile.Name(),
				"-net", "nic,model=rtl8139", "-net", "user,hostfwd=tcp::22-:22",
				"-nographic",
				image,
			)
		} else {
			panic("Running on unknown architecture.")
		}

		err = qemuCmd.Start()
		if err != nil {
			return fmt.Errorf("cannot start the qemu process: %v", err)
		}

		defer func() {
			err := killProcessCleanly(qemuCmd.Process, time.Second)
			if err != nil {
				log.Printf("cannot kill the qemu process: %v", err)
			}
		}()

		return f()
	})
}

// WithBootedNspawnImage boots the specified image in the specified namespace
// using nspawn. The VM is killed immediately after function returns.
func WithBootedNspawnImage(image string, ns NetNS, f func() error) error {
	cmd := exec.Command(
		"systemd-nspawn",
		"--boot", "--register=no",
		"--image", image,
		"--network-namespace-path", ns.Path(),
	)

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("cannot start the systemd-nspawn process: %v", err)
	}

	defer func() {
		err := killProcessCleanly(cmd.Process, time.Second)
		if err != nil {
			log.Printf("cannot kill the systemd-nspawn process: %v", err)
		}
	}()

	return f()
}

// WithBootedNspawnImage boots the specified directory in the specified namespace
// using nspawn. The VM is killed immediately after function returns.
func WithBootedNspawnDirectory(dir string, ns NetNS, f func() error) error {
	cmd := exec.Command(
		"systemd-nspawn",
		"--boot", "--register=no",
		"--directory", dir,
		"--network-namespace-path", ns.Path(),
	)

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("cannot start the systemd-nspawn process: %v", err)
	}

	defer func() {
		err := killProcessCleanly(cmd.Process, time.Second)
		if err != nil {
			log.Printf("cannot kill the systemd-nspawn process: %v", err)
		}
	}()

	return f()
}

// WithExtractedTarArchive extracts the provided archive and passes
// a path to the result to the function f. The result is deleted
// immediately after the function returns.
func WithExtractedTarArchive(archive string, f func(dir string) error) error {
	return withTempDir("", "tar-archive", func(dir string) error {
		cmd := exec.Command(
			"tar",
			"xf", archive,
			"-C", dir,
		)
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout

		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("cannot untar the archive: %v", err)
		}

		return f(dir)
	})
}

// WithSSHKeyPair runs the function f with a newly generated
// ssh key-pair, they key-pair is deleted immediately after
// the function f returns
func WithSSHKeyPair(f func(privateKey, publicKey string) error) error {
	return withTempDir("", "keys", func(dir string) error {
		privateKey := dir + "/id_rsa"
		publicKey := dir + "/id_rsa.pub"
		cmd := exec.Command("ssh-keygen",
			"-N", "",
			"-f", privateKey,
		)

		err := cmd.Run()
		if err != nil {
			return fmt.Errorf("ssh-keygen failed: %v", err)
		}

		return f(privateKey, publicKey)
	})
}
