package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type LiveImgPipeline struct {
	Pipeline
	BootLoader   BootLoader
	GRUBLegacy   string
	treePipeline *OSPipeline
	Filename     string
}

func NewLiveImgPipeline(buildPipeline *BuildPipeline, treePipeline *OSPipeline) LiveImgPipeline {
	return LiveImgPipeline{
		Pipeline:     New("image", buildPipeline, nil),
		treePipeline: treePipeline,
	}
}

func (p LiveImgPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	pt := p.treePipeline.PartitionTable()
	if pt == nil {
		panic("no partition table in live image")
	}

	for _, stage := range osbuild2.GenImagePrepareStages(pt, p.Filename, osbuild2.PTSfdisk) {
		pipeline.AddStage(stage)
	}

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := osbuild2.GenCopyFSTreeOptions(inputName, p.treePipeline.Name(), p.Filename, pt)
	copyInputs := osbuild2.NewCopyStagePipelineTreeInputs(inputName, p.treePipeline.Name())
	pipeline.AddStage(osbuild2.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	for _, stage := range osbuild2.GenImageFinishStages(pt, p.Filename) {
		pipeline.AddStage(stage)
	}

	switch p.BootLoader {
	case BOOTLOADER_GRUB:
		if p.GRUBLegacy != "" {
			pipeline.AddStage(osbuild2.NewGrub2InstStage(osbuild2.NewGrub2InstStageOption(p.Filename, pt, p.GRUBLegacy)))
		}
	case BOOTLOADER_ZIPL:
		loopback := osbuild2.NewLoopbackDevice(&osbuild2.LoopbackDeviceOptions{Filename: p.Filename})
		pipeline.AddStage(osbuild2.NewZiplInstStage(osbuild2.NewZiplInstStageOptions(p.treePipeline.KernelVer(), pt), loopback, copyDevices, copyMounts))
	}

	return pipeline
}
