package manifest

import (
	"fmt"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

// A RawImage represents a raw image file which can be booted in a
// hypervisor. It is created from an existing OSPipeline.
type RawImage struct {
	Base
	treePipeline *OS
	filename     string
	PartTool     osbuild.PartTool
}

func (p RawImage) Filename() string {
	return p.filename
}

func (p *RawImage) SetFilename(filename string) {
	p.filename = filename
}

func NewRawImage(buildPipeline Build, treePipeline *OS) *RawImage {
	p := &RawImage{
		Base:         NewBase("image", buildPipeline),
		treePipeline: treePipeline,
		filename:     "disk.img",
	}
	buildPipeline.addDependent(p)
	p.PartTool = osbuild.PTSfdisk // default; can be changed after initialisation
	return p
}

func (p *RawImage) getBuildPackages(d Distro) ([]string, error) {
	pkgs, err := p.treePipeline.getBuildPackages(d)
	if err != nil {
		return nil, fmt.Errorf("cannget get build packages from %q: %w", p.treePipeline.Name(), err)
	}
	if p.PartTool == osbuild.PTSgdisk {
		pkgs = append(pkgs, "gdisk")
	}
	return pkgs, nil
}

func (p *RawImage) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	pt := p.treePipeline.PartitionTable
	if pt == nil {
		return osbuild.Pipeline{}, fmt.Errorf("no partition table in live image")
	}

	for _, stage := range osbuild.GenImagePrepareStages(pt, p.Filename(), p.PartTool, p.treePipeline.Name()) {
		pipeline.AddStage(stage)
	}

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := osbuild.GenCopyFSTreeOptions(inputName, p.treePipeline.Name(), p.Filename(), pt)
	copyInputs := osbuild.NewPipelineTreeInputs(inputName, p.treePipeline.Name())
	pipeline.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	bootFiles := p.treePipeline.platform.GetBootFiles()
	if len(bootFiles) > 0 {
		// we ignore the bootcopyoptions as they contain a full tree copy instead we make our own, we *do* still want all the other
		// information such as mountpoints and devices
		_, bootCopyDevices, bootCopyMounts := osbuild.GenCopyFSTreeOptions(inputName, p.treePipeline.Name(), p.Filename(), pt)
		bootCopyOptions := &osbuild.CopyStageOptions{}
		bootCopyInputs := osbuild.NewPipelineTreeInputs(inputName, p.treePipeline.Name())

		// Find the FS root mount name to use as the destination root
		// for the target when copying the boot files.
		var fsRootMntName string
		for _, mnt := range bootCopyMounts {
			if mnt.Target == "/" {
				fsRootMntName = mnt.Name
				break
			}
		}

		if fsRootMntName == "" {
			return osbuild.Pipeline{}, fmt.Errorf("no mount found for the filesystem root")
		}

		for _, paths := range bootFiles {
			bootCopyOptions.Paths = append(bootCopyOptions.Paths, osbuild.CopyStagePath{
				From: fmt.Sprintf("input://root-tree%s", paths[0]),
				To:   fmt.Sprintf("mount://%s%s", fsRootMntName, paths[1]),
			})
		}

		pipeline.AddStage(osbuild.NewCopyStage(bootCopyOptions, bootCopyInputs, bootCopyDevices, bootCopyMounts))
	}

	for _, stage := range osbuild.GenImageFinishStages(pt, p.Filename()) {
		pipeline.AddStage(stage)
	}

	switch p.treePipeline.platform.GetArch() {
	case arch.ARCH_S390X:
		loopback := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: p.Filename()})
		pipeline.AddStage(osbuild.NewZiplInstStage(osbuild.NewZiplInstStageOptions(p.treePipeline.kernelVer, pt), loopback, copyDevices, copyMounts))
	default:
		if grubLegacy := p.treePipeline.platform.GetBIOSPlatform(); grubLegacy != "" {
			pipeline.AddStage(osbuild.NewGrub2InstStage(osbuild.NewGrub2InstStageOption(p.Filename(), pt, grubLegacy)))
		}
	}

	return pipeline, nil
}

func (p *RawImage) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename(), nil)
}
