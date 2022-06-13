package target

import (
	"encoding/json"
	"fmt"

	"github.com/osbuild/osbuild-composer/internal/worker/clienterrors"
)

type TargetResult struct {
	Name        TargetName          `json:"name"`
	Options     TargetResultOptions `json:"options"`
	TargetError *clienterrors.Error `json:"target_error,omitempty"`
}

func newTargetResult(name TargetName, options TargetResultOptions) *TargetResult {
	return &TargetResult{
		Name:    name,
		Options: options,
	}
}

type TargetResultOptions interface {
	isTargetResultOptions()
}

type rawTargetResult struct {
	Name        TargetName          `json:"name"`
	Options     json.RawMessage     `json:"options"`
	TargetError *clienterrors.Error `json:"target_error,omitempty"`
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
	targetResult.TargetError = rawTR.TargetError
	return nil
}

func UnmarshalTargetResultOptions(trName TargetName, rawOptions json.RawMessage) (TargetResultOptions, error) {
	var options TargetResultOptions
	switch trName {
	case TargetNameAWS:
		options = new(AWSTargetResultOptions)
	case TargetNameAWSS3:
		options = new(AWSS3TargetResultOptions)
	case TargetNameGCP:
		options = new(GCPTargetResultOptions)
	case TargetNameAzureImage:
		options = new(AzureImageTargetResultOptions)
	case TargetNameKoji:
		options = new(KojiTargetResultOptions)
	case TargetNameOCI:
		options = new(OCITargetResultOptions)
	default:
		return nil, fmt.Errorf("unexpected target result name: %s", trName)
	}
	err := json.Unmarshal(rawOptions, options)

	return options, err
}
