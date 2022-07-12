package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild"
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

func (p *RawImage) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pt := p.treePipeline.PartitionTable
	if pt == nil {
		panic("no partition table in live image")
	}

	for _, stage := range osbuild.GenImagePrepareStages(pt, p.Filename, osbuild.PTSfdisk) {
		pipeline.AddStage(stage)
	}

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := osbuild.GenCopyFSTreeOptions(inputName, p.treePipeline.Name(), p.Filename, pt)
	copyInputs := osbuild.NewCopyStagePipelineTreeInputs(inputName, p.treePipeline.Name())
	pipeline.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	for _, stage := range osbuild.GenImageFinishStages(pt, p.Filename) {
		pipeline.AddStage(stage)
	}

	switch p.treePipeline.platform.GetArch() {
	case platform.ARCH_S390X:
		loopback := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: p.Filename})
		pipeline.AddStage(osbuild.NewZiplInstStage(osbuild.NewZiplInstStageOptions(p.treePipeline.kernelVer, pt), loopback, copyDevices, copyMounts))
	default:
		if grubLegacy := p.treePipeline.platform.GetBIOSPlatform(); grubLegacy != "" {
			pipeline.AddStage(osbuild.NewGrub2InstStage(osbuild.NewGrub2InstStageOption(p.Filename, pt, grubLegacy)))
		}
	}

	return pipeline
}
