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
	Devices Devices      `json:"devices,omitempty"`
	Mounts  Mounts       `json:"mounts,omitempty"`
}

// StageOptions specify the operations of a given stage-type.
type StageOptions interface {
	isStageOptions()
}

type rawStage struct {
	Type    string          `json:"type"`
	Options json.RawMessage `json:"options"`
	Inputs  json.RawMessage `json:"inputs"`
	Devices json.RawMessage `json:"devices"`
	Mounts  json.RawMessage `json:"mounts"`
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
	var devices Devices
	var mounts Mounts
	switch rawStage.Type {
	case "org.osbuild.authselect":
		options = new(AuthselectStageOptions)
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
	case "org.osbuild.cloud-init":
		options = new(CloudInitStageOptions)
	case "org.osbuild.chrony":
		options = new(ChronyStageOptions)
	case "org.osbuild.dracut":
		options = new(DracutStageOptions)
	case "org.osbuild.dracut.conf":
		options = new(DracutConfStageOptions)
	case "org.osbuild.keymap":
		options = new(KeymapStageOptions)
	case "org.osbuild.modprobe":
		options = new(ModprobeStageOptions)
	case "org.osbuild.firewall":
		options = new(FirewallStageOptions)
	case "org.osbuild.rhsm":
		options = new(RHSMStageOptions)
	case "org.osbuild.systemd":
		options = new(SystemdStageOptions)
	case "org.osbuild.systemd.unit":
		options = new(SystemdUnitStageOptions)
	case "org.osbuild.systemd-logind":
		options = new(SystemdLogindStageOptions)
	case "org.osbuild.script":
		options = new(ScriptStageOptions)
	case "org.osbuild.sysconfig":
		options = new(SysconfigStageOptions)
	case "org.osbuild.kernel-cmdline":
		options = new(KernelCmdlineStageOptions)
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
	case "org.osbuild.ostree.init":
		options = new(OSTreeInitStageOptions)
	case "org.osbuild.ostree.preptree":
		options = new(OSTreePrepTreeStageOptions)
	case "org.osbuild.truncate":
		options = new(TruncateStageOptions)
	case "org.osbuild.sfdisk":
		options = new(SfdiskStageOptions)
		devices = new(SfdiskStageDevices)
	case "org.osbuild.copy":
		options = new(CopyStageOptions)
		inputs = new(CopyStageInputs)
		devices = new(CopyStageDevices)
	case "org.osbuild.mkfs.btrfs":
		options = new(MkfsBtrfsStageOptions)
		devices = new(MkfsBtrfsStageDevices)
	case "org.osbuild.mkfs.ext4":
		options = new(MkfsExt4StageOptions)
		devices = new(MkfsExt4StageDevices)
	case "org.osbuild.mkfs.fat":
		options = new(MkfsFATStageOptions)
		devices = new(MkfsFATStageDevices)
	case "org.osbuild.mkfs.xfs":
		options = new(MkfsXfsStageOptions)
		devices = new(MkfsXfsStageDevices)
	case "org.osbuild.qemu":
		options = new(QEMUStageOptions)
		inputs = new(QEMUStageInputs)
	case "org.osbuild.xz":
		options = new(XzStageOptions)
		// TODO: Unmarshalling inputs should be moved to a separate method and struct should be determined by its Type
		// The stage accepts also source input, but we need to rework all inputs first to handle this nicely here.
		// Only files input is used by the XZ stage at this moment.
		inputs = new(FilesInputs)
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
	stage.Devices = devices
	stage.Mounts = mounts

	return nil
}
