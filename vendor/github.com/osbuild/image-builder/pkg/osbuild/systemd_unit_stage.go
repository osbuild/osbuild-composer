package osbuild

import (
	"fmt"
	"regexp"
)

type unitType string

const (
	SystemUnitType unitType = "system"
	GlobalUnitType unitType = "global"
)

type SystemdUnitStageOptions struct {
	Unit     string                   `json:"unit" yaml:"unit"`
	Dropin   string                   `json:"dropin" yaml:"dropin"`
	Config   SystemdServiceUnitDropin `json:"config" yaml:"config"`
	UnitType unitType                 `json:"unit-type,omitempty" yaml:"unit-type,omitempty"`
}

func (SystemdUnitStageOptions) isStageOptions() {}

func (o *SystemdUnitStageOptions) validate() error {
	vre := regexp.MustCompile(envVarRegex)
	if service := o.Config.Service; service != nil {
		for _, envVar := range service.Environment {
			if !vre.MatchString(envVar.Key) {
				return fmt.Errorf("variable name %q doesn't conform to schema (%s)", envVar.Key, envVarRegex)
			}
		}
	}
	return nil
}

func NewSystemdUnitStage(options *SystemdUnitStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}
	return &Stage{
		Type:    "org.osbuild.systemd.unit",
		Options: options,
	}
}

// Drop-in configuration for a '.service' unit
type SystemdServiceUnitDropin struct {
	Service *SystemdUnitServiceSection `json:"Service,omitempty"`
	Unit    *SystemdUnitSection        `json:"Unit,omitempty" yaml:"Unit,omitempty"`
}

// 'Service' configuration section of a unit file
type SystemdUnitServiceSection struct {
	// Sets environment variables for executed process
	Environment     []EnvironmentVariable `json:"Environment,omitempty"`
	EnvironmentFile []string              `json:"EnvironmentFile,omitempty"`
}

// 'Unit' configuration section of a unit file
type SystemdUnitSection struct {
	// Sets condition to to check if file exits
	FileExists string `json:"ConditionPathExists,omitempty" yaml:"ConditionPathExists,omitempty"`
}
