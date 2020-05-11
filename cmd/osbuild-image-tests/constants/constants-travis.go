// +build travis

package constants

import "os/exec"

func GetOsbuildCommand(outputDirectory string) *exec.Cmd {
	cmd := exec.Command(
		"python3",
		"-m", "osbuild",
		"--libdir", ".",
		"--output-directory", outputDirectory,
		"--json",
		"-",
	)
	cmd.Dir = "osbuild"
	return cmd
}

var TestPaths = struct {
	ImageInfo               string
	PrivateKey              string
	TestCasesDirectory      string
	UserData                string
	MetaData                string
	AzureDeploymentTemplate string
}{
	ImageInfo:               "tools/image-info",
	PrivateKey:              "test/keyring/id_rsa",
	TestCasesDirectory:      "test/cases",
	UserData:                "test/cloud-init/user-data",
	MetaData:                "test/cloud-init/meta-data",
	AzureDeploymentTemplate: "test/azure-deployment-template.json",
}
