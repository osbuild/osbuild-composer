package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// A LiveImgPipeline represents a raw image file which can be booted in a
// hypervisor. It is created from an existing OSPipeline.
type LiveImgPipeline struct {
	BasePipeline
	treePipeline *OSPipeline

	filename string
}

func NewLiveImgPipeline(buildPipeline *BuildPipeline, treePipeline *OSPipeline, filename string) LiveImgPipeline {
	return LiveImgPipeline{
		BasePipeline: NewBasePipeline("image", buildPipeline, nil),
		treePipeline: treePipeline,
		filename:     filename,
	}
}

func (p LiveImgPipeline) Filename() string {
	return p.filename
}

func (p LiveImgPipeline) serialize() osbuild2.Pipeline {
	pipeline := p.BasePipeline.serialize()

	pt := p.treePipeline.PartitionTable()
	if pt == nil {
		panic("no partition table in live image")
	}

	for _, stage := range osbuild2.GenImagePrepareStages(pt, p.Filename(), osbuild2.PTSfdisk) {
		pipeline.AddStage(stage)
	}

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := osbuild2.GenCopyFSTreeOptions(inputName, p.treePipeline.Name(), p.Filename(), pt)
	copyInputs := osbuild2.NewCopyStagePipelineTreeInputs(inputName, p.treePipeline.Name())
	pipeline.AddStage(osbuild2.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	for _, stage := range osbuild2.GenImageFinishStages(pt, p.Filename()) {
		pipeline.AddStage(stage)
	}

	switch p.treePipeline.BootLoader() {
	case BOOTLOADER_GRUB:
		if grubLegacy := p.treePipeline.GRUBLegacy(); grubLegacy != "" {
			pipeline.AddStage(osbuild2.NewGrub2InstStage(osbuild2.NewGrub2InstStageOption(p.Filename(), pt, grubLegacy)))
		}
	case BOOTLOADER_ZIPL:
		loopback := osbuild2.NewLoopbackDevice(&osbuild2.LoopbackDeviceOptions{Filename: p.Filename()})
		pipeline.AddStage(osbuild2.NewZiplInstStage(osbuild2.NewZiplInstStageOptions(p.treePipeline.KernelVer(), pt), loopback, copyDevices, copyMounts))
	}

	return pipeline
}
