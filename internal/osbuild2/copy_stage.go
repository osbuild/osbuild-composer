package osbuild2

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/osbuild/osbuild-composer/internal/disk"
)

// Stage to copy items from inputs to mount points or the tree. Multiple items
// can be copied. The source and destination is a URL.

type CopyStageOptions struct {
	Paths []CopyStagePath `json:"paths"`
}

type CopyStagePath struct {
	From string `json:"from"`
	To   string `json:"to"`
}

func (CopyStageOptions) isStageOptions() {}

type CopyStageInputs map[string]CopyStageInput

type CopyStageInput struct {
	inputCommon
	References CopyStageReferences `json:"references"`
}

func (CopyStageInputs) isStageInputs() {}

type CopyStageReferences []string

type CopyStageInputsNew interface {
	isCopyStageInputs()
}

func (CopyStageInputs) isCopyStageInputs() {}

func (CopyStageReferences) isReferences() {}

func NewCopyStage(options *CopyStageOptions, inputs CopyStageInputsNew, devices *Devices, mounts *Mounts) *Stage {
	var stageInputs Inputs
	if inputs != nil {
		stageInputs = inputs.(Inputs)
	}
	return &Stage{
		Type:    "org.osbuild.copy",
		Options: options,
		Inputs:  stageInputs,
		Devices: *devices,
		Mounts:  *mounts,
	}
}

func NewCopyStageSimple(options *CopyStageOptions, inputs CopyStageInputsNew) *Stage {
	var stageInputs Inputs
	if inputs != nil {
		stageInputs = inputs.(Inputs)
	}
	return &Stage{
		Type:    "org.osbuild.copy",
		Options: options,
		Inputs:  stageInputs,
	}
}

func NewCopyStagePipelineTreeInputs(inputName, inputPipeline string) *CopyStageInputs {
	treeInput := CopyStageInput{}
	treeInput.Type = "org.osbuild.tree"
	treeInput.Origin = "org.osbuild.pipeline"
	treeInput.References = []string{"name:" + inputPipeline}
	return &CopyStageInputs{inputName: treeInput}
}

// GenCopyFSTreeOptions creates the options, inputs, devices, and mounts properties
// for an org.osbuild.copy stage for a given source tree using a partition
// table description to define the mounts
func GenCopyFSTreeOptions(inputName, inputPipeline string, pt *disk.PartitionTable, device *Device) (
	*CopyStageOptions,
	*Devices,
	*Mounts,
) {
	// assume loopback device for simplicity since it's the only one currently supported
	// panic if the conversion fails
	devOptions, ok := device.Options.(*LoopbackDeviceOptions)
	if !ok {
		panic("GenCopyStageOptions: failed to convert device options to loopback options")
	}

	devices := make(map[string]Device, len(pt.Partitions))
	mounts := make([]Mount, 0, len(pt.Partitions))
	for _, p := range pt.Partitions {
		if p.Payload == nil {
			// no filesystem for partition (e.g., BIOS boot)
			continue
		}
		name := filepath.Base(p.Payload.Mountpoint)
		if name == "/" {
			name = "root"
		}
		devices[name] = *NewLoopbackDevice(
			&LoopbackDeviceOptions{
				Filename: devOptions.Filename,
				Start:    pt.BytesToSectors(p.Start),
				Size:     pt.BytesToSectors(p.Size),
			},
		)
		var mount *Mount
		switch p.Payload.Type {
		case "xfs":
			mount = NewXfsMount(name, name, p.Payload.Mountpoint)
		case "vfat":
			mount = NewFATMount(name, name, p.Payload.Mountpoint)
		case "ext4":
			mount = NewExt4Mount(name, name, p.Payload.Mountpoint)
		case "btrfs":
			mount = NewBtrfsMount(name, name, p.Payload.Mountpoint)
		default:
			panic("unknown fs type " + p.Type)
		}
		mounts = append(mounts, *mount)
	}

	// sort the mounts, using < should just work because:
	// - a parent directory should be always before its children:
	//   / < /boot
	// - the order of siblings doesn't matter
	sort.Slice(mounts, func(i, j int) bool {
		return mounts[i].Target < mounts[j].Target
	})

	stageMounts := Mounts(mounts)
	stageDevices := Devices(devices)

	options := CopyStageOptions{
		Paths: []CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/", inputName),
				To:   "mount://root/",
			},
		},
	}

	return &options, &stageDevices, &stageMounts
}
