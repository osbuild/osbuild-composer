package target

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/common"
)

type Target struct {
	Uuid      uuid.UUID              `json:"uuid"`
	ImageName string                 `json:"image_name"` // Desired name of the image in the target environment
	Name      string                 `json:"name"`       // Name of the specific target type
	Created   time.Time              `json:"created"`
	Status    common.ImageBuildState `json:"status"`
	Options   TargetOptions          `json:"options"` // Target type specific options
}

func newTarget(name string, options TargetOptions) *Target {
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
	Name      string                 `json:"name"`
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

func UnmarshalTargetOptions(targetName string, rawOptions json.RawMessage) (TargetOptions, error) {
	var options TargetOptions
	switch targetName {
	case "org.osbuild.azure":
		options = new(AzureTargetOptions)
	case "org.osbuild.aws":
		options = new(AWSTargetOptions)
	case "org.osbuild.aws.s3":
		options = new(AWSS3TargetOptions)
	case "org.osbuild.gcp":
		options = new(GCPTargetOptions)
	case "org.osbuild.azure.image":
		options = new(AzureImageTargetOptions)
	case "org.osbuild.local":
		options = new(LocalTargetOptions)
	case "org.osbuild.koji":
		options = new(KojiTargetOptions)
	case "org.osbuild.vmware":
		options = new(VMWareTargetOptions)
	case "org.osbuild.oci":
		options = new(OCITargetOptions)
	case "org.osbuild.generic.s3":
		options = new(GenericS3TargetOptions)
	default:
		return nil, errors.New("unexpected target name")
	}
	err := json.Unmarshal(rawOptions, options)

	return options, err
}
