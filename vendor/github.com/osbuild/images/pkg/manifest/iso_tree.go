package manifest

import (
	"fmt"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
)

// ISOTree represents a simplified ISO tree that supports squashfs and erofs
// root filesystems.
type ISOTree struct {
	Base

	Release string
	Product string
	Version string

	PartitionTable *disk.PartitionTable

	treePipeline     Pipeline
	bootTreePipeline *EFIBootTree

	isoLabel string

	RootfsCompression string
	RootfsType        ISORootfsType

	// Kernel options for the ISO image
	KernelOpts []string

	// Kernel and initramfs paths in the tree pipeline
	KernelPath    string
	InitramfsPath string
}

func NewISOTree(buildPipeline Build, treePipeline Pipeline, bootTreePipeline *EFIBootTree) *ISOTree {
	// the pipelines should all belong to the same manifest
	if treePipeline.Manifest() != bootTreePipeline.Manifest() {
		panic("pipelines from different manifests")
	}
	p := &ISOTree{
		Base:             NewBase("bootiso-tree", buildPipeline),
		treePipeline:     treePipeline,
		bootTreePipeline: bootTreePipeline,
		isoLabel:         bootTreePipeline.ISOLabel,
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
		// Try to get architecture from bootTreePipeline if available
		arch := "x86_64" // default
		if p.bootTreePipeline.Platform != nil {
			arch = p.bootTreePipeline.Platform.GetArch().String()
		}
		squashfsOptions.Compression.Options = &osbuild.FSCompressionOptions{
			BCJ: osbuild.BCJOption(arch),
		}
	}

	// Clean up the root filesystem's /boot to save space
	squashfsOptions.ExcludePaths = installerBootExcludePaths

	return osbuild.NewSquashfsStage(&squashfsOptions, p.treePipeline.Name()), nil
}

// NewErofsStage returns an osbuild stage configured to build
// the erofs root filesystem for the ISO.
func (p *ISOTree) NewErofsStage() (*osbuild.Stage, error) {
	// TODO: We should somehow make this customizable, because non-live anaconda ISOs
	// use images/install.img
	erofsOptions := osbuild.ErofsStageOptions{
		Filename: "LiveOS/squashfs.img",
	}

	var compression osbuild.ErofsCompression
	if p.RootfsCompression != "" {
		compression.Method = p.RootfsCompression
	} else {
		// default to zstd if not specified
		compression.Method = "zstd"
	}
	compression.Level = common.ToPtr(8)
	erofsOptions.Compression = &compression
	erofsOptions.ExtendedOptions = []string{"all-fragments", "dedupe"}
	erofsOptions.ClusterSize = common.ToPtr(131072)

	// Clean up the root filesystem's /boot to save space
	erofsOptions.ExcludePaths = installerBootExcludePaths

	return osbuild.NewErofsStage(&erofsOptions, p.treePipeline.Name()), nil
}

func (p *ISOTree) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	kernelOpts := []string{}
	if len(p.KernelOpts) > 0 {
		kernelOpts = append(kernelOpts, p.KernelOpts...)
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

	// Add grub2 boot configuration
	product := p.Product
	if product == "" {
		product = "OS"
	}
	version := p.Version
	if version == "" {
		version = p.Release
	}

	var grub2config *osbuild.Grub2Config
	var disableTestEntry bool
	var disableTroubleshootingEntry bool

	if p.bootTreePipeline != nil {
		if p.bootTreePipeline.DefaultMenu > 0 {
			grub2config = &osbuild.Grub2Config{
				Default: p.bootTreePipeline.DefaultMenu,
			}
		}

		if p.bootTreePipeline.MenuTimeout != nil {
			if grub2config == nil {
				grub2config = &osbuild.Grub2Config{
					Timeout: *p.bootTreePipeline.MenuTimeout,
				}
			} else {
				grub2config.Timeout = *p.bootTreePipeline.MenuTimeout
			}
		}

		disableTestEntry = p.bootTreePipeline.DisableTestEntry
		disableTroubleshootingEntry = p.bootTreePipeline.DisableTroubleshootingEntry
	}
	options := &osbuild.Grub2ISOLegacyStageOptions{
		Product: osbuild.Product{
			Name:    product,
			Version: version,
		},
		Kernel: osbuild.ISOKernel{
			Dir:  "/images/pxeboot",
			Opts: kernelOpts,
		},
		ISOLabel:        p.isoLabel,
		FIPS:            false,
		Install:         true,
		Test:            !disableTestEntry,
		Troubleshooting: !disableTroubleshootingEntry,
		Config:          grub2config,
	}

	// If any menu entries are defined we turn off all default
	// entries and instead append our own
	// entries only
	if len(p.bootTreePipeline.MenuEntries) > 0 {
		options.Troubleshooting = false
		options.Test = false
		options.Install = false

		for _, entry := range p.bootTreePipeline.MenuEntries {
			options.Custom = append(options.Custom, osbuild.Grub2ISOLegacyCustomEntryOptions{
				Name:   entry.Name,
				Linux:  entry.Linux,
				Initrd: entry.Initrd,
			})
		}
	}

	if p.bootTreePipeline != nil && p.bootTreePipeline.Platform != nil {
		options.FIPS = p.bootTreePipeline.Platform.GetFIPSMenu()
	}

	stage := osbuild.NewGrub2ISOLegacyStage(options)
	pipeline.AddStage(stage)

	// Add a stage to create the eltorito.img file for grub2 BIOS boot support
	pipeline.AddStage(osbuild.NewGrub2InstStage(osbuild.NewGrub2InstISO9660StageOption("images/eltorito.img", "/boot/grub2")))

	// Create EFI boot partition
	filename := "images/efiboot.img"
	pipeline.AddStage(osbuild.NewTruncateStage(&osbuild.TruncateStageOptions{
		Filename: filename,
		Size:     fmt.Sprintf("%d", p.PartitionTable.Size),
	}))

	for _, stage := range osbuild.GenFsStages(p.PartitionTable, filename, p.treePipeline.Name()) {
		pipeline.AddStage(stage)
	}

	inputName = "root-tree"
	copyInputs := osbuild.NewPipelineTreeInputs(inputName, p.bootTreePipeline.Name())
	copyOptions, copyDevices, copyMounts := osbuild.GenCopyFSTreeOptions(inputName, p.bootTreePipeline.Name(), filename, p.PartitionTable)
	pipeline.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	copyInputs = osbuild.NewPipelineTreeInputs(inputName, p.bootTreePipeline.Name())
	pipeline.AddStage(osbuild.NewCopyStageSimple(
		&osbuild.CopyStageOptions{
			Paths: []osbuild.CopyStagePath{
				{
					From: fmt.Sprintf("input://%s/EFI", inputName),
					To:   "tree:///",
				},
			},
		},
		copyInputs,
	))

	// Determine architecture for discinfo
	arch := "x86_64" // default
	if p.bootTreePipeline != nil && p.bootTreePipeline.Platform != nil {
		arch = p.bootTreePipeline.Platform.GetArch().String()
	}

	pipeline.AddStage(osbuild.NewDiscinfoStage(&osbuild.DiscinfoStageOptions{
		BaseArch: arch,
		Release:  p.Release,
	}))

	return pipeline, nil
}
