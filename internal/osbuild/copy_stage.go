package osbuild

import (
	"fmt"
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
func GenCopyFSTreeOptions(inputName, inputPipeline, filename string, pt *disk.PartitionTable) (
	*CopyStageOptions,
	*Devices,
	*Mounts,
) {

	devices := make(map[string]Device, len(pt.Partitions))
	mounts := make([]Mount, 0, len(pt.Partitions))
	genMounts := func(mnt disk.Mountable, path []disk.Entity) error {
		stageDevices, name := getDevices(path, filename, false)
		mountpoint := mnt.GetMountpoint()

		var mount *Mount
		t := mnt.GetFSType()
		switch t {
		case "xfs":
			mount = NewXfsMount(name, name, mountpoint)
		case "vfat":
			mount = NewFATMount(name, name, mountpoint)
		case "ext4":
			mount = NewExt4Mount(name, name, mountpoint)
		case "btrfs":
			mount = NewBtrfsMount(name, name, mountpoint)
		default:
			panic("unknown fs type " + t)
		}
		mounts = append(mounts, *mount)

		// update devices map with new elements from stageDevices
		for devName := range stageDevices {
			devices[devName] = stageDevices[devName]
		}
		return nil
	}

	_ = pt.ForEachMountable(genMounts)

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
