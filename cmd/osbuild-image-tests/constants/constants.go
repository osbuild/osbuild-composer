//go:build integration

package constants

import "os/exec"

func GetOsbuildCommand(store, outputDirectory string, exports []string) *exec.Cmd {
	cmd := exec.Command(
		"osbuild",
		"--store", store,
		"--output-directory", outputDirectory,
		"--checkpoint", "build",
		"--json",
		"-",
	)
	for _, export := range exports {
		cmd.Args = append(cmd.Args, "--export", export)
	}
	return cmd
}

func GetImageInfoCommand(imagePath string) *exec.Cmd {
	return exec.Command(
		"osbuild-image-info",
		imagePath,
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
	ImageInfo:               "osbuild-image-info",
	PrivateKey:              "/usr/share/tests/osbuild-composer/keyring/id_rsa",
	TestCasesDirectory:      "/usr/share/tests/osbuild-composer/manifests",
	UserData:                "/usr/share/tests/osbuild-composer/cloud-init/user-data",
	MetaData:                "/usr/share/tests/osbuild-composer/cloud-init/meta-data",
	AzureDeploymentTemplate: "/usr/share/tests/osbuild-composer/azure/deployment-template.json",
}
