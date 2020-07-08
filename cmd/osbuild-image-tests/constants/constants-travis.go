// +build travis

package constants

import (
	"os"
	"os/exec"
)

func GetOsbuildCommand(store, outputDirectory string) *exec.Cmd {
	cmd := exec.Command(
		"python3",
		"-m", "osbuild",
		"--libdir", ".",
		"--store", store,
		"--output-directory", outputDirectory,
		"--json",
		"-",
	)
	cmd.Dir = "osbuild"
	return cmd
}

func GetImageInfoCommand(imagePath string) *exec.Cmd {
	cmd := exec.Command(
		"tools/image-info",
		imagePath,
	)
	cmd.Env = append(os.Environ(), "PYTHONPATH=osbuild")
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
