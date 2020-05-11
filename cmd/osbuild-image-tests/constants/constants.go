// +build integration,!travis

package constants

import "os/exec"

func GetOsbuildCommand(outputDirectory string) *exec.Cmd {
	return exec.Command(
		"osbuild",
		"--output-directory", outputDirectory,
		"--json",
		"-",
	)
}

var TestPaths = struct {
	ImageInfo               string
	PrivateKey              string
	TestCasesDirectory      string
	UserData                string
	MetaData                string
	AzureDeploymentTemplate string
}{
	ImageInfo:               "/usr/libexec/osbuild-composer/image-info",
	PrivateKey:              "/usr/share/tests/osbuild-composer/keyring/id_rsa",
	TestCasesDirectory:      "/usr/share/tests/osbuild-composer/cases",
	UserData:                "/usr/share/tests/osbuild-composer/cloud-init/user-data",
	MetaData:                "/usr/share/tests/osbuild-composer/cloud-init/meta-data",
	AzureDeploymentTemplate: "/usr/share/tests/osbuild-composer/azure-deployment-template.json",
}
