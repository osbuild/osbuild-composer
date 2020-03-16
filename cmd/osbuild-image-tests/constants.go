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

var imageInfoPath = "/usr/libexec/osbuild-composer/image-info"
var privateKeyPath = "/usr/share/tests/osbuild-composer/keyring/id_rsa"
var testCasesDirectoryPath = "/usr/share/tests/osbuild-composer/cases"
var userDataPath = "/usr/share/tests/osbuild-composer/cloud-init/user-data"
var metaDataPath = "/usr/share/tests/osbuild-composer/cloud-init/meta-data"
