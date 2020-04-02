// +build integration,!travis

package main

import "os/exec"

func getOsbuildCommand(store string) *exec.Cmd {
	return exec.Command(
		"osbuild",
		"--store", store,
		"--json",
		"-",
	)
}

var testPaths = struct {
	imageInfo          string
	privateKey         string
	testCasesDirectory string
	userData           string
	metaData           string
}{
	imageInfo:          "/usr/libexec/osbuild-composer/image-info",
	privateKey:         "/usr/share/tests/osbuild-composer/keyring/id_rsa",
	testCasesDirectory: "/usr/share/tests/osbuild-composer/cases",
	userData:           "/usr/share/tests/osbuild-composer/cloud-init/user-data",
	metaData:           "/usr/share/tests/osbuild-composer/cloud-init/meta-data",
}
