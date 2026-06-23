package manifest

import (
	"fmt"

	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
)

// A RawOSTreeImage represents a raw ostree image file which can be booted in a
// hypervisor. It is created from an existing OSTreeDeployment.
type RawOSTreeImage struct {
	Base
	treePipeline *OSTreeDeployment
	filename     string
	platform     platform.Platform
}

func (p RawOSTreeImage) Filename() string {
	return p.filename
}

func (p *RawOSTreeImage) SetFilename(filename string) {
	p.filename = filename
}

func NewRawOStreeImage(buildPipeline Build, treePipeline *OSTreeDeployment, platform platform.Platform) *RawOSTreeImage {
	p := &RawOSTreeImage{
		Base:         NewBase("image", buildPipeline),
		treePipeline: treePipeline,
		filename:     "disk.img",
		platform:     platform,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *RawOSTreeImage) getBuildPackages(Distro) ([]string, error) {
	packages := p.platform.GetBuildPackages()
	packages = append(packages, p.platform.GetPackages()...)
	packages = append(packages, p.treePipeline.PartitionTable.GetBuildPackages()...)
	packages = append(packages,
		"rpm-ostree",

		// these should be defined on the platform
		"dracut-config-generic",
		"efibootmgr",
	)
	return packages, nil
}

func (p *RawOSTreeImage) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	pt := p.treePipeline.PartitionTable
	if pt == nil {
		return osbuild.Pipeline{}, fmt.Errorf("no partition table in live image")
	}

	for _, stage := range osbuild.GenImagePrepareStages(pt, p.Filename(), osbuild.PTSfdisk, p.treePipeline.Name()) {
		pipeline.AddStage(stage)
	}

	inputName := "root-tree"
	treeCopyOptions, treeCopyDevices, treeCopyMounts := osbuild.GenCopyFSTreeOptions(inputName, p.treePipeline.Name(), p.Filename(), pt)
	treeCopyInputs := osbuild.NewPipelineTreeInputs(inputName, p.treePipeline.Name())

	pipeline.AddStage(osbuild.NewCopyStage(treeCopyOptions, treeCopyInputs, treeCopyDevices, treeCopyMounts))

	treeBootFiles, buildBootFiles := splitBootFiles(p.platform.GetBootFiles())
	if len(treeBootFiles) > 0 {
		commit := p.treePipeline.ostreeSpec
		commitChecksum := commit.Checksum
		bootCopyInputs := osbuild.OSTreeCheckoutInputs{
			"ostree-tree": *osbuild.NewOSTreeCheckoutInput("org.osbuild.source", commitChecksum),
		}
		srcPrefix := fmt.Sprintf("ostree-tree/%s", commitChecksum)
		stage, err := bootFilesCopyStage(treeBootFiles, bootCopyInputs, srcPrefix, p.Filename(), pt)
		if err != nil {
			return osbuild.Pipeline{}, err
		}
		pipeline.AddStage(stage)
	}
	if len(buildBootFiles) > 0 {
		buildInputName := "build-tree"
		buildCopyInputs := osbuild.NewPipelineTreeInputs(buildInputName, p.build.Name())
		stage, err := bootFilesCopyStage(buildBootFiles, buildCopyInputs, buildInputName, p.Filename(), pt)
		if err != nil {
			return osbuild.Pipeline{}, err
		}
		pipeline.AddStage(stage)
	}

	for _, stage := range osbuild.GenImageFinishStages(pt, p.Filename()) {
		pipeline.AddStage(stage)
	}

	if p.treePipeline.UseBootupd {
		if err := p.addBootupdStage(&pipeline); err != nil {
			return osbuild.Pipeline{}, err
		}
	} else {
		p.maybeAddGrubInstStage(&pipeline)
	}

	return pipeline, nil
}

func (p *RawOSTreeImage) addBootupdStage(pipeline *osbuild.Pipeline) error {
	pt := p.treePipeline.PartitionTable

	treeBootupdDevices, treeBootupdMounts, err := osbuild.GenBootupdDevicesMounts(p.Filename(), pt, p.platform)
	if err != nil {
		return err
	}
	opts := &osbuild.BootupdStageOptions{
		Deployment: &osbuild.OSTreeDeployment{
			OSName: p.treePipeline.osName,
			Ref:    p.treePipeline.ref,
		},
		StaticConfigs: true,
	}
	if legacyBios := p.treePipeline.platform.GetBIOSPlatform(); legacyBios != "" {
		opts.Bios = &osbuild.BootupdStageOptionsBios{
			Device: "disk",
		}
	}
	bootupd, err := osbuild.NewBootupdStage(opts, treeBootupdDevices, treeBootupdMounts, p.platform)
	if err != nil {
		return err
	}

	pipeline.AddStage(bootupd)
	return nil
}

func (p *RawOSTreeImage) maybeAddGrubInstStage(pipeline *osbuild.Pipeline) {
	pt := p.treePipeline.PartitionTable

	if grubLegacy := p.treePipeline.platform.GetBIOSPlatform(); grubLegacy != "" {
		pipeline.AddStage(osbuild.NewGrub2InstStage(osbuild.NewGrub2InstStageOption(p.Filename(), pt, grubLegacy)))
	}
}

func (p *RawOSTreeImage) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename(), nil)
}
