package target

import (
	"encoding/json"
	"errors"
)

type Target struct {
	Name    string        `json:"name"`
	Options TargetOptions `json:"options"`
}

type TargetOptions interface {
	isTargetOptions()
}

type rawTarget struct {
	Name    string          `json:"name"`
	Options json.RawMessage `json:"options"`
}

func (target *Target) UnmarshalJSON(data []byte) error {
	var rawTarget rawTarget
	err := json.Unmarshal(data, &rawTarget)
	if err != nil {
		return err
	}
	var options TargetOptions
	switch rawTarget.Name {
	case "org.osbuild.local":
		options = new(LocalTargetOptions)
	default:
		return errors.New("unexpected target name")
	}
	err = json.Unmarshal(rawTarget.Options, options)
	if err != nil {
		return err
	}

	target.Name = rawTarget.Name
	target.Options = options

	return nil
}
