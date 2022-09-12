package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/platform"
)

// A RawOSTreeImage represents a raw ostree image file which can be booted in a
// hypervisor. It is created from an existing OSTreeDeployment.
type RawOSTreeImage struct {
	Base
	treePipeline *OSTreeDeployment
	Filename     string
	platform     platform.Platform
}

func NewRawOStreeImage(m *Manifest,
	buildPipeline *Build,
	platform platform.Platform,
	treePipeline *OSTreeDeployment) *RawOSTreeImage {
	p := &RawOSTreeImage{
		Base:         NewBase(m, "image", buildPipeline),
		treePipeline: treePipeline,
		Filename:     "disk.img",
		platform:     platform,
	}
	buildPipeline.addDependent(p)
	if treePipeline.Base.manifest != m {
		panic("tree pipeline from different manifest")
	}
	m.addPipeline(p)
	return p
}

func (p *RawOSTreeImage) getBuildPackages() []string {
	return p.treePipeline.PartitionTable.GetBuildPackages()
}

func (p *RawOSTreeImage) serialize() osbuild.Pipeline {
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

	if grubLegacy := p.treePipeline.platform.GetBIOSPlatform(); grubLegacy != "" {
		pipeline.AddStage(osbuild.NewGrub2InstStage(osbuild.NewGrub2InstStageOption(p.Filename, pt, grubLegacy)))
	}

	return pipeline
}

func (p *RawOSTreeImage) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename, nil)
}
