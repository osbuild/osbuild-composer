package target

import (
	"encoding/json"
	"errors"
	"github.com/google/uuid"
	"github.com/osbuild/osbuild-composer/internal/common"
	"time"
)

type Target struct {
	Uuid      uuid.UUID              `json:"uuid"`
	ImageName string                 `json:"image_name"`
	Name      string                 `json:"name"`
	Created   time.Time              `json:"created"`
	Status    common.ImageBuildState `json:"status"`
	Options   TargetOptions          `json:"options"`
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
	case "org.osbuild.local":
		options = new(LocalTargetOptions)
	case "org.osbuild.koji":
		options = new(KojiTargetOptions)
	default:
		return nil, errors.New("unexpected target name")
	}
	err := json.Unmarshal(rawOptions, options)

	return options, err
}
