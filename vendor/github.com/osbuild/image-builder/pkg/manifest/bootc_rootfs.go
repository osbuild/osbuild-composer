package manifest

import (
	"fmt"

	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
)

type BootcRootFS struct {
	Base
	RootfsCompression string
	RootfsType        ISORootfsType
	RootfsExcludes    []string
	ErofsOptions      osbuild.ErofsStageOptions // set only when RootfsType is erofs

	platform      platform.Platform
	bootcPipeline *RawBootcImage
}

// NewBootcRootFS creates a pipeline with a kernel, initrd, and compressed root filesystem
// suitable for use to boot the system. Used for PXE and ISO images.
// The bootcPipeline MUST contain an installed disk image and partition mouting info
// Defaults to using xz compressed squashfs rootfs
func NewBootcRootFS(buildPipeline Build, bootcPipeline *RawBootcImage, platform platform.Platform) *BootcRootFS {
	p := &BootcRootFS{
		Base:              NewBase("bootc-rootfs", buildPipeline),
		platform:          platform,
		bootcPipeline:     bootcPipeline,
		RootfsCompression: "xz",
		RootfsType:        SquashfsRootfs,
	}
	buildPipeline.addDependent(p)
	return p
}

// Platform satisfies TreePipeline interface for BootcRootFS
func (p *BootcRootFS) Platform() platform.Platform {
	return p.platform
}

// Create a directory tree containing the kernel, initrd, and compressed rootfs
func (p *BootcRootFS) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return pipeline, err
	}

	// Make sure KernelVersion has been set
	if p.bootcPipeline.KernelVersion == "" {
		return pipeline, fmt.Errorf("NewBootcRootFS requires bootcPipeline.KernelVersion to be set: %s", p.bootcPipeline.KernelVersion)
	}

	// Copy the disk.raw from the bootc image pipeline
	inputName := "tree"
	copyOptions := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/%s", inputName, p.bootcPipeline.Filename()),
				To:   "tree:///",
			},
		},
	}
	copyInputs := osbuild.NewPipelineTreeInputs(inputName, p.bootcPipeline.Name())
	stage := osbuild.NewCopyStageSimple(copyOptions, copyInputs)
	pipeline.AddStage(stage)

	// Setup the device and mounts needed to access the ostree filesystem
	devices, mounts, err := osbuild.GenBootupdDevicesMounts(p.bootcPipeline.Filename(), p.bootcPipeline.PartitionTable, p.platform)
	if err != nil {
		return osbuild.Pipeline{}, fmt.Errorf("gen devices stage failed %w", err)
	}

	// Create the compressed rootfs.img of the bare ostree filesystem
	// NOTE: mounts MUST NOT include the 'ostree.deployment' stage
	if p.RootfsType == ErofsRootfs {
		// Settings come from the YAML iso_config.erofs_options section
		erofsOptions := p.ErofsOptions
		erofsOptions.Filename = "rootfs.img"

		// TODO this is shared with the ISO, should it be?
		// Clean up the root filesystem's /boot to save space
		erofsOptions.ExcludePaths = p.RootfsExcludes
		erofsOptions.Source = "mount://-/"
		erofsStage := osbuild.NewErofsWithMountsStage(&erofsOptions, nil, devices, mounts)
		pipeline.AddStage(erofsStage)
	} else {
		var squashfsOptions osbuild.SquashfsStageOptions

		// TODO this should come from the YAML
		squashfsOptions.Filename = "rootfs.img"
		squashfsOptions.Compression.Method = "xz"

		if squashfsOptions.Compression.Method == "xz" {
			squashfsOptions.Compression.Options = &osbuild.FSCompressionOptions{
				BCJ: osbuild.BCJOption(p.platform.GetArch().String()),
			}
		}

		// Compress the mounted ostree filesystem instead of a pipeline tree
		squashfsOptions.Source = "mount://-/"
		squashfsStage := osbuild.NewSquashfsWithMountsStage(&squashfsOptions, nil, devices, mounts)
		pipeline.AddStage(squashfsStage)
	}

	// NOTE: After this point mounts includes the 'ostree.deployment' stage
	mounts = append(mounts, *osbuild.NewOSTreeDeploymentMountDefault("ostree.deployment", osbuild.OSTreeMountSourceMount))

	// Mount the ostree disk image and deployment and copy files from it
	copyStageOptions := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("mount://-/usr/lib/modules/%s/vmlinuz", p.bootcPipeline.KernelVersion),
				To:   "tree:///vmlinuz",
			},
			{
				From: fmt.Sprintf("mount://-/usr/lib/modules/%s/initramfs.img", p.bootcPipeline.KernelVersion),
				To:   "tree:///initrd.img",
			},
		},
	}
	copyStage := osbuild.NewCopyStage(copyStageOptions, nil, devices, mounts)
	pipeline.AddStage(copyStage)

	// Make sure all the files are readable
	options := osbuild.ChmodStageOptions{
		Items: p.getChmodFiles(),
	}
	pipeline.AddStage(osbuild.NewChmodStage(&options))
	return pipeline, nil
}

// getChmodFiles returns a list of files and permissions that need to be updated at the
// end of the pipeline. Also used to list the files to include in the tar.
func (p *BootcRootFS) getChmodFiles() map[string]osbuild.ChmodStagePathOptions {
	return map[string]osbuild.ChmodStagePathOptions{
		"/vmlinuz": {
			Mode: "0755",
		},
		"/initrd.img": {
			Mode: "0644",
		},
		"/rootfs.img": {
			Mode: "0644",
		},
	}
}
