package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type LiveImgPipeline struct {
	Pipeline
	PartitionTable disk.PartitionTable
	BootLoader     BootLoader
	GRUBLegacy     string
	treePipeline   *OSPipeline
	Filename       string
}

func NewLiveImgPipeline(buildPipeline *BuildPipeline, treePipeline *OSPipeline) LiveImgPipeline {
	return LiveImgPipeline{
		Pipeline:     New("image", buildPipeline, nil),
		treePipeline: treePipeline,
	}
}

func (p LiveImgPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	for _, stage := range osbuild2.GenImagePrepareStages(&p.PartitionTable, p.Filename, osbuild2.PTSfdisk) {
		pipeline.AddStage(stage)
	}

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := osbuild2.GenCopyFSTreeOptions(inputName, p.treePipeline.Name(), p.Filename, &p.PartitionTable)
	copyInputs := osbuild2.NewCopyStagePipelineTreeInputs(inputName, p.treePipeline.Name())
	pipeline.AddStage(osbuild2.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	for _, stage := range osbuild2.GenImageFinishStages(&p.PartitionTable, p.Filename) {
		pipeline.AddStage(stage)
	}

	switch p.BootLoader {
	case BOOTLOADER_GRUB:
		if p.GRUBLegacy != "" {
			pipeline.AddStage(osbuild2.NewGrub2InstStage(osbuild2.NewGrub2InstStageOption(p.Filename, &p.PartitionTable, p.GRUBLegacy)))
		}
	case BOOTLOADER_ZIPL:
		loopback := osbuild2.NewLoopbackDevice(&osbuild2.LoopbackDeviceOptions{Filename: p.Filename})
		pipeline.AddStage(osbuild2.NewZiplInstStage(osbuild2.NewZiplInstStageOptions(p.treePipeline.KernelVer(), &p.PartitionTable), loopback, copyDevices, copyMounts))
	}

	return pipeline
}
