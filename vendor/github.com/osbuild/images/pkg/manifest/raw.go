package manifest

import (
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
)

// A RawImage represents a raw image file which can be booted in a
// hypervisor. It is created from an existing OSPipeline.
type RawImage struct {
	Base
	treePipeline *OS
	Filename     string
	PartTool     osbuild.PartTool
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
	p.PartTool = osbuild.PTSfdisk // default; can be changed after initialisation
	m.addPipeline(p)
	return p
}

func (p *RawImage) getBuildPackages(d Distro) []string {
	pkgs := p.treePipeline.getBuildPackages(d)
	if p.PartTool == osbuild.PTSgdisk {
		pkgs = append(pkgs, "gdisk")
	}
	return pkgs
}

func (p *RawImage) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pt := p.treePipeline.PartitionTable
	if pt == nil {
		panic("no partition table in live image")
	}

	for _, stage := range osbuild.GenImagePrepareStages(pt, p.Filename, p.PartTool) {
		pipeline.AddStage(stage)
	}

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := osbuild.GenCopyFSTreeOptions(inputName, p.treePipeline.Name(), p.Filename, pt)
	copyInputs := osbuild.NewPipelineTreeInputs(inputName, p.treePipeline.Name())
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

func (p *RawImage) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename, nil)
}
