package manifest

import (
	"fmt"

	"github.com/osbuild/image-builder/pkg/arch"
	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
)

// A RawImage represents a raw image file which can be booted in a
// hypervisor. It is created from an existing OSPipeline.
type RawImage struct {
	Base
	treePipeline       *OS
	filename           string
	DiskCustomizations DiskCustomizations
}

func (p RawImage) Filename() string {
	return p.filename
}

func (p *RawImage) SetFilename(filename string) {
	p.filename = filename
}

func NewRawImage(buildPipeline Build, treePipeline *OS, diskCustomizations DiskCustomizations) *RawImage {
	p := &RawImage{
		Base:               NewBase("image", buildPipeline),
		treePipeline:       treePipeline,
		filename:           "disk.img",
		DiskCustomizations: diskCustomizations,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *RawImage) getBuildPackages(d Distro) ([]string, error) {
	pkgs, err := p.treePipeline.getBuildPackages(d)
	if err != nil {
		return nil, fmt.Errorf("cannget get build packages from %q: %w", p.treePipeline.Name(), err)
	}
	if p.DiskCustomizations.PartitioningTool == osbuild.PTSgdisk {
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

	for _, stage := range osbuild.GenImagePrepareStages(pt, p.Filename(), p.DiskCustomizations.PartitioningTool, p.treePipeline.Name()) {
		pipeline.AddStage(stage)
	}

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := osbuild.GenCopyFSTreeOptions(inputName, p.treePipeline.Name(), p.Filename(), pt)
	copyInputs := osbuild.NewPipelineTreeInputs(inputName, p.treePipeline.Name())
	pipeline.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	treeBootFiles, buildBootFiles := splitBootFiles(p.treePipeline.platform.GetBootFiles())
	if len(treeBootFiles) > 0 {
		treeInputName := "root-tree"
		bootCopyInputs := osbuild.NewPipelineTreeInputs(treeInputName, p.treePipeline.Name())
		stage, err := bootFilesCopyStage(treeBootFiles, bootCopyInputs, treeInputName, p.Filename(), pt)
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

	switch p.treePipeline.platform.GetArch() {
	case arch.ARCH_S390X:
		loopback := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: p.Filename()})
		pipeline.AddStage(osbuild.NewZiplInstStage(osbuild.NewZiplInstStageOptions(p.treePipeline.kernelVer, pt), loopback, copyDevices, copyMounts))
	default:
		if grubLegacy := p.treePipeline.platform.GetBIOSPlatform(); grubLegacy != "" {
			pipeline.AddStage(osbuild.NewGrub2InstStage(osbuild.NewGrub2InstStageOption(p.Filename(), pt, grubLegacy)))
		}
	}

	if p.treePipeline.platform.GetBootloader() == platform.BOOTLOADER_SYSTEMD {
		_, bootctlDevices, bootctlMounts := osbuild.GenCopyFSTreeOptions(inputName, p.treePipeline.Name(), p.Filename(), pt)

		opts := &osbuild.BootctlInstallRootStageOptions{
			Root: "mount://-/",
		}

		// ESP is required
		espMountpoint, err := findESPMountpoint(p.treePipeline.PartitionTable)
		if err != nil {
			return osbuild.Pipeline{}, err
		}
		opts.ESPPath = espMountpoint

		// XBOOTLDR is optional so we don't need to check the error, if there's
		// an error it's empty and the stage options will omit it
		opts.BootPath, _ = findXBootLDRMountpoint(p.treePipeline.PartitionTable)

		if cfg := p.treePipeline.OSCustomizations.SystemdBoot; cfg != nil {
			opts.RandomSeed = cfg.RandomSeed
			opts.MakeEntryDirectory = cfg.MakeEntryDirectory
			opts.EntryToken = cfg.EntryToken
		}

		pipeline.AddStage(osbuild.NewBootctlInstallRootStage(opts, bootctlDevices, bootctlMounts))
	}

	return pipeline, nil
}

func splitBootFiles(bootFiles []platform.BootFile) (tree, build []platform.BootFile) {
	for _, bf := range bootFiles {
		if bf.FromBuild {
			build = append(build, bf)
		} else {
			tree = append(tree, bf)
		}
	}
	return tree, build
}

func bootFilesCopyStage(
	bootFiles []platform.BootFile,
	inputs osbuild.Inputs,
	srcPrefix, filename string,
	pt *disk.PartitionTable,
) (*osbuild.Stage, error) {
	_, devices, mounts := osbuild.GenCopyFSTreeOptions("", "", filename, pt)

	var fsRootMntName string
	for _, mnt := range mounts {
		if mnt.Target == "/" {
			fsRootMntName = mnt.Name
			break
		}
	}
	if fsRootMntName == "" {
		return nil, fmt.Errorf("no mount found for the filesystem root")
	}

	options := &osbuild.CopyStageOptions{}
	for _, bf := range bootFiles {
		options.Paths = append(options.Paths, osbuild.CopyStagePath{
			From: fmt.Sprintf("input://%s%s", srcPrefix, bf.Src),
			To:   fmt.Sprintf("mount://%s%s", fsRootMntName, bf.Dst),
		})
	}

	return osbuild.NewCopyStage(options, inputs, devices, mounts), nil
}

func (p *RawImage) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename(), nil)
}
