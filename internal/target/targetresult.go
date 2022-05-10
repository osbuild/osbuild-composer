package target

import (
	"encoding/json"
	"fmt"
)

type TargetResult struct {
	Name    string              `json:"name"`
	Options TargetResultOptions `json:"options"`
}

func newTargetResult(name string, options TargetResultOptions) *TargetResult {
	return &TargetResult{
		Name:    name,
		Options: options,
	}
}

type TargetResultOptions interface {
	isTargetResultOptions()
}

type rawTargetResult struct {
	Name    string          `json:"name"`
	Options json.RawMessage `json:"options"`
}

func (targetResult *TargetResult) UnmarshalJSON(data []byte) error {
	var rawTR rawTargetResult
	err := json.Unmarshal(data, &rawTR)
	if err != nil {
		return err
	}
	options, err := UnmarshalTargetResultOptions(rawTR.Name, rawTR.Options)
	if err != nil {
		return err
	}

	targetResult.Name = rawTR.Name
	targetResult.Options = options
	return nil
}

func UnmarshalTargetResultOptions(trName string, rawOptions json.RawMessage) (TargetResultOptions, error) {
	var options TargetResultOptions
	switch trName {
	case "org.osbuild.aws":
		options = new(AWSTargetResultOptions)
	case "org.osbuild.aws.s3":
		options = new(AWSS3TargetResultOptions)
	case "org.osbuild.gcp":
		options = new(GCPTargetResultOptions)
	case "org.osbuild.azure.image":
		options = new(AzureImageTargetResultOptions)
	case "org.osbuild.koji":
		options = new(KojiTargetResultOptions)
	case "org.osbuild.oci":
		options = new(OCITargetResultOptions)
	default:
		return nil, fmt.Errorf("unexpected target result name: %s", trName)
	}
	err := json.Unmarshal(rawOptions, options)

	return options, err
}
