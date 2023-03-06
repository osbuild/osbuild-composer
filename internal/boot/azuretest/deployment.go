//go:build integration

package azuretest

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/osbuild/osbuild-composer/cmd/osbuild-image-tests/constants"
)

// loadDeploymentTemplate loads the deployment template from the specified
// path and return it as a "dynamically" typed object
func loadDeploymentTemplate() (interface{}, error) {
	f, err := os.Open(constants.TestPaths.AzureDeploymentTemplate)
	if err != nil {
		return nil, fmt.Errorf("cannot open the deployment file: %v", err)
	}

	defer f.Close()

	var result interface{}

	err = json.NewDecoder(f).Decode(&result)

	if err != nil {
		return nil, fmt.Errorf("cannot decode the deployment file: %v", err)
	}

	return result, nil
}

// struct for encoding a deployment parameter
type deploymentParameter struct {
	Value string `json:"value"`
}

func newDeploymentParameter(value string) deploymentParameter {
	return deploymentParameter{Value: value}
}

// struct for encoding deployment parameters
type DeploymentParameters struct {
	NetworkInterfaceName     deploymentParameter `json:"networkInterfaceName"`
	NetworkSecurityGroupName deploymentParameter `json:"networkSecurityGroupName"`
	VirtualNetworkName       deploymentParameter `json:"virtualNetworkName"`
	PublicIPAddressName      deploymentParameter `json:"publicIPAddressName"`
	VirtualMachineName       deploymentParameter `json:"virtualMachineName"`
	DiskName                 deploymentParameter `json:"diskName"`
	ImageName                deploymentParameter `json:"imageName"`
	Location                 deploymentParameter `json:"location"`
	ImagePath                deploymentParameter `json:"imagePath"`
	AdminUsername            deploymentParameter `json:"adminUsername"`
	AdminPublicKey           deploymentParameter `json:"adminPublicKey"`
}
