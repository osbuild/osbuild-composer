package osbuild2

import (
	"encoding/json"
	"fmt"
)

// Single stage of a pipeline executing one step
type Stage struct {
	// Well-known name in reverse domain-name notation, uniquely identifying
	// the stage type.
	Type string `json:"type"`
	// Stage-type specific options fully determining the operations of the

	Inputs  Inputs       `json:"inputs,omitempty"`
	Options StageOptions `json:"options,omitempty"`
}

// Collection of Inputs for a Stage
type Inputs interface {
	isStageInputs()
}

// Single Input for a Stage
type Input interface {
	isInput()
}

// Fields shared between all Input types (should be embedded in each instance)
type inputCommon struct {
	Type string `json:"type"`
	// Origin should be either 'org.osbuild.source' or 'org.osbuild.pipeline'
	Origin string `json:"origin"`

	// References References `json:"references"`
}

type StageInput interface {
	isStageInput()
}

type References interface {
	isReferences()
}

// StageOptions specify the operations of a given stage-type.
type StageOptions interface {
	isStageOptions()
}

type InputOptions interface {
}

type rawStage struct {
	Type    string          `json:"type"`
	Options json.RawMessage `json:"options"`
	Inputs  json.RawMessage `json:"inputs"`
}

// UnmarshalJSON unmarshals JSON into a Stage object. Each type of stage has
// a custom unmarshaller for its options, selected based on the stage name.
func (stage *Stage) UnmarshalJSON(data []byte) error {
	var rawStage rawStage
	if err := json.Unmarshal(data, &rawStage); err != nil {
		return err
	}
	var options StageOptions
	var inputs Inputs
	switch rawStage.Type {
	case "org.osbuild.fix-bls":
		options = new(FixBLSStageOptions)
	case "org.osbuild.fstab":
		options = new(FSTabStageOptions)
	case "org.osbuild.grub2":
		options = new(GRUB2StageOptions)
	case "org.osbuild.locale":
		options = new(LocaleStageOptions)
	case "org.osbuild.selinux":
		options = new(SELinuxStageOptions)
	case "org.osbuild.hostname":
		options = new(HostnameStageOptions)
	case "org.osbuild.users":
		options = new(UsersStageOptions)
	case "org.osbuild.groups":
		options = new(GroupsStageOptions)
	case "org.osbuild.timezone":
		options = new(TimezoneStageOptions)
	case "org.osbuild.chrony":
		options = new(ChronyStageOptions)
	case "org.osbuild.keymap":
		options = new(KeymapStageOptions)
	case "org.osbuild.firewall":
		options = new(FirewallStageOptions)
	case "org.osbuild.rhsm":
		options = new(RHSMStageOptions)
	case "org.osbuild.systemd":
		options = new(SystemdStageOptions)
	case "org.osbuild.script":
		options = new(ScriptStageOptions)
	case "org.osbuild.rpm":
		options = new(RPMStageOptions)
		inputs = new(RPMStageInputs)
	case "org.osbuild.oci-archive":
		options = new(OCIArchiveStageOptions)
		inputs = new(OCIArchiveStageInputs)
	case "org.osbuild.ostree.commit":
		options = new(OSTreeCommitStageOptions)
		inputs = new(OSTreeCommitStageInputs)
	case "org.osbuild.ostree.pull":
		options = new(OSTreePullStageOptions)
		inputs = new(OSTreePullStageInputs)
	case "org.osbuild.ostree.preptree":
		options = new(RPMOSTreePrepTreeStageOptions)
	default:
		return fmt.Errorf("unexpected stage type: %s", rawStage.Type)
	}
	if err := json.Unmarshal(rawStage.Options, options); err != nil {
		return err
	}
	if inputs != nil && rawStage.Inputs != nil {
		if err := json.Unmarshal(rawStage.Inputs, inputs); err != nil {
			return err
		}
	}

	stage.Type = rawStage.Type
	stage.Options = options
	stage.Inputs = inputs

	return nil
}
