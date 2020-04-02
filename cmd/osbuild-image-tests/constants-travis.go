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

var testPaths = struct {
	imageInfo          string
	privateKey         string
	testCasesDirectory string
	userData           string
	metaData           string
}{
	imageInfo:          "tools/image-info",
	privateKey:         "test/keyring/id_rsa",
	testCasesDirectory: "test/cases",
	userData:           "test/cloud-init/user-data",
	metaData:           "test/cloud-init/meta-data",
}
