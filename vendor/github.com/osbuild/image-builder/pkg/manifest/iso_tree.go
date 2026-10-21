package manifest

import (
	"fmt"

	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

// ISOTree represents a simplified ISO tree that supports squashfs and erofs
// root filesystems.
type ISOTree struct {
	Base

	Release string
	Product string
	Version string

	PartitionTable *disk.PartitionTable
	bootloaders    []ISOBootloader
	files          []*fsnode.File

	treePipeline TreePipeline

	RootfsCompression string
	RootfsType        ISORootfsType
	RootfsExcludes    []string
	// set only when RootfsType is erofs
	ErofsOptions *osbuild.ErofsStageOptions `yaml:"erofs_options,omitempty"`

	// Kernel options for the ISO image
	KernelOpts []string

	// Kernel and initramfs paths in the tree pipeline
	KernelPath    string
	InitramfsPath string
}

func NewISOTree(buildPipeline Build, treePipeline TreePipeline, bootloaders []ISOBootloader) *ISOTree {
	// the pipelines should all belong to the same manifest
	for _, b := range bootloaders {
		if b.Manifest() != nil {
			if b.Manifest() != treePipeline.Manifest() {
				panic("pipelines from different manifests")
			}
		}
	}

	p := &ISOTree{
		Base:         NewBase("bootiso-tree", buildPipeline),
		treePipeline: treePipeline,
		bootloaders:  bootloaders,
	}
	buildPipeline.addDependent(p)
	return p
}

// NewSquashfsStage returns an osbuild stage configured to build
// the squashfs root filesystem for the ISO.
func (p *ISOTree) NewSquashfsStage() (*osbuild.Stage, error) {
	// TODO: We should somehow make this customizable, because non-live anaconda ISOs
	// use images/install.img
	squashfsOptions := osbuild.SquashfsStageOptions{
		Filename: "LiveOS/squashfs.img",
	}

	if p.RootfsCompression != "" {
		squashfsOptions.Compression.Method = p.RootfsCompression
	} else {
		// default to xz if not specified
		squashfsOptions.Compression.Method = "xz"
	}

	if squashfsOptions.Compression.Method == "xz" {
		arch := p.treePipeline.Platform().GetArch().String()
		squashfsOptions.Compression.Options = &osbuild.FSCompressionOptions{
			BCJ: osbuild.BCJOption(arch),
		}
	}

	// Clean up the root filesystem's /boot to save space
	squashfsOptions.ExcludePaths = p.RootfsExcludes

	return osbuild.NewSquashfsStage(&squashfsOptions, p.treePipeline.Name()), nil
}

// NewErofsStage returns an osbuild stage configured to build
// the erofs root filesystem for the ISO.
func (p *ISOTree) NewErofsStage() (*osbuild.Stage, error) {
	// TODO: We should somehow make this customizable, because non-live anaconda ISOs
	// use images/install.img
	if p.RootfsType != ErofsRootfs || p.ErofsOptions == nil {
		return nil, fmt.Errorf("Rootfs not set to Erofs or options set to nil for %s pipeline, can not create erofs stage", p.name)
	}

	erofsOptions := *p.ErofsOptions
	erofsOptions.Filename = "LiveOS/squashfs.img"

	// Clean up the root filesystem's /boot to save space
	erofsOptions.ExcludePaths = p.RootfsExcludes

	return osbuild.NewErofsStage(erofsOptions, p.treePipeline.Name()), nil
}

func (p *ISOTree) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{
			{
				Path: "/images",
			},
			{
				Path: "/images/pxeboot",
			},
			{
				Path: "/LiveOS",
			},
		},
	}))

	// Copy kernel and initramfs
	if p.KernelPath == "" || p.InitramfsPath == "" {
		return osbuild.Pipeline{}, fmt.Errorf("kernel and initramfs paths must be set")
	}

	inputName := "tree"
	copyStageOptions := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/%s", inputName, p.KernelPath),
				To:   "tree:///images/pxeboot/vmlinuz",
			},
			{
				From: fmt.Sprintf("input://%s/%s", inputName, p.InitramfsPath),
				To:   "tree:///images/pxeboot/initrd.img",
			},
		},
	}
	copyStageInputs := osbuild.NewPipelineTreeInputs(inputName, p.treePipeline.Name())
	copyStage := osbuild.NewCopyStageSimple(copyStageOptions, copyStageInputs)
	pipeline.AddStage(copyStage)

	pipeline.AddStage(osbuild.NewChmodStage(&osbuild.ChmodStageOptions{
		Items: map[string]osbuild.ChmodStagePathOptions{
			"/images/pxeboot/vmlinuz":    {Mode: "0644"},
			"/images/pxeboot/initrd.img": {Mode: "0644"},
		},
	}))

	// Add the selected rootfs stage
	switch p.RootfsType {
	case SquashfsRootfs:
		stage, err := p.NewSquashfsStage()
		if err != nil {
			return osbuild.Pipeline{}, fmt.Errorf("cannot create squashfs stage: %w", err)
		}
		pipeline.AddStage(stage)
	case ErofsRootfs:
		stage, err := p.NewErofsStage()
		if err != nil {
			return osbuild.Pipeline{}, fmt.Errorf("cannot create erofs stage: %w", err)
		}
		pipeline.AddStage(stage)
	default:
		return osbuild.Pipeline{}, fmt.Errorf("unsupported rootfs type: %v (only SquashfsRootfs and ErofsRootfs are supported)", p.RootfsType)
	}

	// Add bootloaders
	for _, loader := range p.bootloaders {
		stages, files, err := loader.GetISOBootStages(p.treePipeline.Name(), p.PartitionTable)
		if err != nil {
			return osbuild.Pipeline{}, fmt.Errorf("cannot add ISO bootloader: %w", err)
		}
		pipeline.AddStages(stages...)
		p.files = append(p.files, files...)
	}

	pipeline.AddStage(osbuild.NewDiscinfoStage(&osbuild.DiscinfoStageOptions{
		BaseArch: p.treePipeline.Platform().GetArch().String(),
		Release:  p.Release,
	}))

	return pipeline, nil
}

func (p *ISOTree) getInline() []string {
	inlineData := []string{}

	// inline data for custom files
	for _, file := range p.files {
		inlineData = append(inlineData, string(file.Data()))
	}

	return inlineData
}
