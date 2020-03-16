// +build travis

package main

import "os/exec"

func getOsbuildCommand(store string) *exec.Cmd {
	cmd := exec.Command(
		"python3",
		"-m", "osbuild",
		"--libdir", ".",
		"--store", store,
		"--json",
		"-",
	)
	cmd.Dir = "osbuild"
	return cmd
}

var imageInfoPath = "tools/image-info"
var privateKeyPath = "test/keyring/id_rsa"
var testCasesDirectoryPath = "test/cases"
var userDataPath = "test/cloud-init/user-data"
var metaDataPath = "test/cloud-init/meta-data"
