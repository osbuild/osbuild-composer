// +build integration

package main

import (
	"fmt"
	"os"

	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/osbuild/osbuild-composer/internal/boot/azuretest"
)

func panicErr(err error) {
	if err != nil {
		panic(err)
	}
}

func printErr(err error) {
	if err != nil {
		fmt.Println(err)
	}
}

// GenerateCIArtifactName generates a new identifier for CI artifacts which is based
// on environment variables specified by Jenkins
// note: in case of migration to sth else like Github Actions, change it to whatever variables GH Action provides
func GenerateCIArtifactName(prefix string) (string, error) {
	distroCode := os.Getenv("DISTRO_CODE")
	branchName := os.Getenv("BRANCH_NAME")
	buildId := os.Getenv("BUILD_ID")
	if branchName == "" || buildId == "" || distroCode == "" {
		return "", fmt.Errorf("The environment variables must specify BRANCH_NAME, BUILD_ID, and DISTRO_CODE")
	}

	return fmt.Sprintf("%s%s-%s-%s", prefix, distroCode, branchName, buildId), nil
}

func main() {
	fmt.Println("Running a cloud cleanup")

	// Load Azure credentials
	creds, err := azuretest.GetAzureCredentialsFromEnv()
	panicErr(err)
	if creds == nil {
		panic("empty credentials")
	}
	// Get test ID
	testID, err := GenerateCIArtifactName("")
	panicErr(err)
	// Delete the vhd image
	imageName := "image-" + testID + ".vhd"
	fmt.Println("Running delete image from Azure, this should fail if the test succedded")
	err = azuretest.DeleteImageFromAzure(creds, imageName)
	printErr(err)

	// Delete all remaining resources (see the full list in the CleanUpBootedVM function)
	fmt.Println("Running clean up booted VM, this should fail if the test succedded")
	parameters := azuretest.NewDeploymentParameters(creds, imageName, testID, "")
	clientCredentialsConfig := auth.NewClientCredentialsConfig(creds.ClientID, creds.ClientSecret, creds.TenantID)
	authorizer, err := clientCredentialsConfig.Authorizer()
	panicErr(err)
	err = azuretest.CleanUpBootedVM(creds, parameters, authorizer, testID)
	printErr(err)
}
