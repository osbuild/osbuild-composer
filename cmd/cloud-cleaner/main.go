// +build integration

package main

import (
	"fmt"

	"github.com/Azure/go-autorest/autorest/azure/auth"

	"github.com/osbuild/osbuild-composer/internal/boot/azuretest"
	"github.com/osbuild/osbuild-composer/internal/test"
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



func main() {
	fmt.Println("Running a cloud cleanup")

	// Load Azure credentials
	creds, err := azuretest.GetAzureCredentialsFromEnv()
	panicErr(err)
	if creds == nil {
		panic("empty credentials")
	}
	// Get test ID
	testID, err := test.GenerateCIArtifactName("")
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
