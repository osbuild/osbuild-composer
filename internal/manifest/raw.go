package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/platform"
)

// A RawImage represents a raw image file which can be booted in a
// hypervisor. It is created from an existing OSPipeline.
type RawImage struct {
	Base
	treePipeline *OS
	Filename     string
}

func NewRawImage(m *Manifest,
	buildPipeline *Build,
	treePipeline *OS) *RawImage {
	p := &RawImage{
		Base:         NewBase(m, "image", buildPipeline),
		treePipeline: treePipeline,
		Filename:     "disk.img",
	}
	buildPipeline.addDependent(p)
	if treePipeline.Base.manifest != m {
		panic("tree pipeline from different manifest")
	}
	m.addPipeline(p)
	return p
}

func (p *RawImage) getBuildPackages() []string {
	return p.treePipeline.PartitionTable.GetBuildPackages()
}

func (p *RawImage) serialize() osbuild2.Pipeline {
	pipeline := p.Base.serialize()

	pt := p.treePipeline.PartitionTable
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

	switch p.treePipeline.platform.GetArch() {
	case platform.ARCH_S390X:
		loopback := osbuild2.NewLoopbackDevice(&osbuild2.LoopbackDeviceOptions{Filename: p.Filename})
		pipeline.AddStage(osbuild2.NewZiplInstStage(osbuild2.NewZiplInstStageOptions(p.treePipeline.kernelVer, pt), loopback, copyDevices, copyMounts))
	default:
		if grubLegacy := p.treePipeline.platform.GetBIOSPlatform(); grubLegacy != "" {
			pipeline.AddStage(osbuild2.NewGrub2InstStage(osbuild2.NewGrub2InstStageOption(p.Filename, pt, grubLegacy)))
		}
	}

	return pipeline
}
