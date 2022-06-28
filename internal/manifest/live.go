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

func NewLiveImgPipeline(buildPipeline *BuildPipeline, treePipeline *OSPipeline, filename string) *LiveImgPipeline {
	p := &LiveImgPipeline{
		BasePipeline: NewBasePipeline("image", buildPipeline, nil),
		treePipeline: treePipeline,
		filename:     filename,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *LiveImgPipeline) serialize() osbuild2.Pipeline {
	pipeline := p.BasePipeline.serialize()

	pt := p.treePipeline.partitionTable
	if pt == nil {
		panic("no partition table in live image")
	}

	for _, stage := range osbuild2.GenImagePrepareStages(pt, p.filename, osbuild2.PTSfdisk) {
		pipeline.AddStage(stage)
	}

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := osbuild2.GenCopyFSTreeOptions(inputName, p.treePipeline.Name(), p.filename, pt)
	copyInputs := osbuild2.NewCopyStagePipelineTreeInputs(inputName, p.treePipeline.Name())
	pipeline.AddStage(osbuild2.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	for _, stage := range osbuild2.GenImageFinishStages(pt, p.filename) {
		pipeline.AddStage(stage)
	}

	switch p.treePipeline.bootLoader {
	case BOOTLOADER_GRUB:
		if grubLegacy := p.treePipeline.grubLegacy; grubLegacy != "" {
			pipeline.AddStage(osbuild2.NewGrub2InstStage(osbuild2.NewGrub2InstStageOption(p.filename, pt, grubLegacy)))
		}
	case BOOTLOADER_ZIPL:
		loopback := osbuild2.NewLoopbackDevice(&osbuild2.LoopbackDeviceOptions{Filename: p.filename})
		pipeline.AddStage(osbuild2.NewZiplInstStage(osbuild2.NewZiplInstStageOptions(p.treePipeline.kernelVer, pt), loopback, copyDevices, copyMounts))
	}

	return pipeline
}
