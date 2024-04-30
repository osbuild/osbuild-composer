package osbuild

import (
	"fmt"
	"regexp"
)

type serviceType string
type unitPath string

const (
	Simple              serviceType = "simple"
	Exec                serviceType = "exec"
	Forking             serviceType = "forking"
	Oneshot             serviceType = "oneshot"
	Dbus                serviceType = "dbus"
	Notify              serviceType = "notify"
	NotifyReloadservice serviceType = "notify-reload"
	Idle                serviceType = "idle"
	Etc                 unitPath    = "etc"
	Usr                 unitPath    = "usr"
)

type Unit struct {
	Description              string   `json:"Description,omitempty"`
	DefaultDependencies      bool     `json:"DefaultDependencies,omitempty"`
	ConditionPathExists      []string `json:"ConditionPathExists,omitempty"`
	ConditionPathIsDirectory []string `json:"ConditionPathIsDirectory,omitempty"`
	Requires                 []string `json:"Requires,omitempty"`
	Wants                    []string `json:"Wants,omitempty"`
	After                    []string `json:"After,omitempty"`
}

type Service struct {
	Type            serviceType           `json:"Type,omitempty"`
	RemainAfterExit bool                  `json:"RemainAfterExit,omitempty"`
	ExecStartPre    []string              `json:"ExecStartPre,omitempty"`
	ExecStopPost    []string              `json:"ExecStopPost,omitempty"`
	ExecStart       []string              `json:"ExecStart,omitempty"`
	Environment     []EnvironmentVariable `json:"Environment,omitempty"`
	EnvironmentFile []string              `json:"EnvironmentFile,omitempty"`
}

type Install struct {
	RequiredBy []string `json:"RequiredBy,omitempty"`
	WantedBy   []string `json:"WantedBy,omitempty"`
}

type SystemdServiceUnit struct {
	Unit    *Unit    `json:"Unit"`
	Service *Service `json:"Service"`
	Install *Install `json:"Install"`
}

type SystemdUnitCreateStageOptions struct {
	Filename string             `json:"filename"`
	UnitType unitType           `json:"unit-type,omitempty"` // unitType defined in ./systemd_unit_stage.go
	UnitPath unitPath           `json:"unit-path,omitempty"`
	Config   SystemdServiceUnit `json:"config"`
}

func (SystemdUnitCreateStageOptions) isStageOptions() {}

func (o *SystemdUnitCreateStageOptions) validate() error {
	fre := regexp.MustCompile(filenameRegex)
	if !fre.MatchString(o.Filename) {
		return fmt.Errorf("filename %q doesn't conform to schema (%s)", o.Filename, filenameRegex)
	}

	if o.Config.Install == nil {
		return fmt.Errorf("Install section of systemd unit is required")
	}

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

func NewSystemdUnitCreateStage(options *SystemdUnitCreateStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}
	return &Stage{
		Type:    "org.osbuild.systemd.unit.create",
		Options: options,
	}
}
