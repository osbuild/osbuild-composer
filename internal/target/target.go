package target

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/common"
)

type TargetName string

type Target struct {
	Uuid      uuid.UUID              `json:"uuid"`
	ImageName string                 `json:"image_name"` // Desired name of the image in the target environment
	Name      TargetName             `json:"name"`       // Name of the specific target type
	Created   time.Time              `json:"created"`
	Status    common.ImageBuildState `json:"status"`
	Options   TargetOptions          `json:"options"` // Target type specific options
}

func newTarget(name TargetName, options TargetOptions) *Target {
	return &Target{
		Uuid:    uuid.New(),
		Name:    name,
		Created: time.Now(),
		Status:  common.IBWaiting,
		Options: options,
	}
}

type TargetOptions interface {
	isTargetOptions()
}

type rawTarget struct {
	Uuid      uuid.UUID              `json:"uuid"`
	ImageName string                 `json:"image_name"`
	Name      TargetName             `json:"name"`
	Created   time.Time              `json:"created"`
	Status    common.ImageBuildState `json:"status"`
	Options   json.RawMessage        `json:"options"`
}

func (target *Target) UnmarshalJSON(data []byte) error {
	var rawTarget rawTarget
	err := json.Unmarshal(data, &rawTarget)
	if err != nil {
		return err
	}
	options, err := UnmarshalTargetOptions(rawTarget.Name, rawTarget.Options)
	if err != nil {
		return err
	}

	target.Uuid = rawTarget.Uuid
	target.ImageName = rawTarget.ImageName
	target.Name = rawTarget.Name
	target.Created = rawTarget.Created
	target.Status = rawTarget.Status
	target.Options = options

	return nil
}

func UnmarshalTargetOptions(targetName TargetName, rawOptions json.RawMessage) (TargetOptions, error) {
	var options TargetOptions
	switch targetName {
	case TargetNameAzure:
		options = new(AzureTargetOptions)
	case TargetNameAWS:
		options = new(AWSTargetOptions)
	case TargetNameAWSS3:
		options = new(AWSS3TargetOptions)
	case TargetNameGCP:
		options = new(GCPTargetOptions)
	case TargetNameAzureImage:
		options = new(AzureImageTargetOptions)
	case TargetNameLocal:
		options = new(LocalTargetOptions)
	case TargetNameKoji:
		options = new(KojiTargetOptions)
	case TargetNameVMWare:
		options = new(VMWareTargetOptions)
	case TargetNameOCI:
		options = new(OCITargetOptions)
	case TargetNameContainer:
		options = new(ContainerTargetOptions)
	default:
		return nil, fmt.Errorf("unexpected target name: %s", targetName)
	}
	err := json.Unmarshal(rawOptions, options)

	return options, err
}
