package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"time"
)

// withNetworkNamespace provides the function f with a new network namespace
// which is deleted immediately after f returns
func withNetworkNamespace(f func(ns netNS) error) error {
	ns, err := newNetworkNamespace()
	if err != nil {
		return err
	}

	defer func() {
		err := ns.Delete()
		if err != nil {
			log.Printf("cannot delete network namespace: %#v", err)
		}
	}()

	return f(ns)
}

// withTempFile provides the function f with a new temporary file
// dir and pattern parameters have the same semantics as in ioutil.TempFile
func withTempFile(dir, pattern string, f func(file *os.File) error) error {
	tempFile, err := ioutil.TempFile(dir, pattern)
	if err != nil {
		return fmt.Errorf("cannot create the temporary file: %#v", err)
	}

	defer func() {
		err := os.Remove(tempFile.Name())
		if err != nil {
			log.Printf("cannot remove the temporary file: %#v", err)
		}
	}()

	return f(tempFile)
}

// writeCloudInitSO creates cloud-init iso from specified userData and
// metaData and writes it to the writer
func writeCloudInitISO(writer io.Writer, userData, metaData string) error {
	isoCmd := exec.Command(
		"genisoimage",
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
		return fmt.Errorf("cannot create cloud-init iso: %#v", err)
	}

	return nil
}

// withBootedQemuImage boots the specified image in the specified namespace
// using qemu. The VM is killed immediately after function returns.
func withBootedQemuImage(image string, ns netNS, f func() error) error {
	return withTempFile("", "osbuild-image-tests-cloudinit", func(cloudInitFile *os.File) error {
		err := writeCloudInitISO(
			cloudInitFile,
			userDataPath,
			metaDataPath,
		)
		if err != nil {
			return err
		}

		err = cloudInitFile.Close()
		if err != nil {
			return fmt.Errorf("cannot close temporary cloudinit file: %#v", err)
		}

		qemuCmd := ns.NamespacedCommand(
			"qemu-system-x86_64",
			"-m", "1024",
			"-snapshot",
			"-accel", "accel=kvm:hvf:tcg",
			"-cdrom", cloudInitFile.Name(),
			"-net", "nic,model=rtl8139", "-net", "user,hostfwd=tcp::22-:22",
			"-nographic",
			image,
		)

		err = qemuCmd.Start()
		if err != nil {
			return fmt.Errorf("cannot start the qemu process: %#v", err)
		}

		defer func() {
			err := killProcessCleanly(qemuCmd.Process, time.Second)
			if err != nil {
				log.Printf("cannot kill the qemu process: %#v", err)
			}
		}()

		return f()
	})
}

// withBootedNspawnImage boots the specified image in the specified namespace
// using nspawn. The VM is killed immediately after function returns.
func withBootedNspawnImage(image, name string, ns netNS, f func() error) error {
	cmd := exec.Command(
		"systemd-nspawn",
		"--boot", "--register=no",
		"-M", name,
		"--image", image,
		"--network-namespace-path", ns.Path(),
	)

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("cannot start the systemd-nspawn process: %#v", err)
	}

	defer func() {
		err := killProcessCleanly(cmd.Process, time.Second)
		if err != nil {
			log.Printf("cannot kill the systemd-nspawn process: %#v", err)
		}
	}()

	return f()
}

// withBootedNspawnImage boots the specified directory in the specified namespace
// using nspawn. The VM is killed immediately after function returns.
func withBootedNspawnDirectory(dir, name string, ns netNS, f func() error) error {
	cmd := exec.Command(
		"systemd-nspawn",
		"--boot", "--register=no",
		"-M", name,
		"--directory", dir,
		"--network-namespace-path", ns.Path(),
	)

	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("cannot start the systemd-nspawn process: %#v", err)
	}

	defer func() {
		err := killProcessCleanly(cmd.Process, time.Second)
		if err != nil {
			log.Printf("cannot kill the systemd-nspawn process: %#v", err)
		}
	}()

	return f()
}

// withExtractedTarArchive extracts the provided archive and passes
// a path to the result to the function f. The result is deleted
// immediately after the function returns.
func withExtractedTarArchive(archive string, f func(dir string) error) error {
	dir, err := ioutil.TempDir("", "tar-archive")
	if err != nil {
		return fmt.Errorf("cannot create a temporary dir: %#v", err)
	}

	defer func() {
		err := os.RemoveAll(dir)
		if err != nil {
			log.Printf("cannot remove the temporary dir: %#v", err)
		}
	}()

	cmd := exec.Command(
		"tar",
		"xf", archive,
		"-C", dir,
	)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("cannot untar the archive: %#v", err)
	}

	return f(dir)
}
