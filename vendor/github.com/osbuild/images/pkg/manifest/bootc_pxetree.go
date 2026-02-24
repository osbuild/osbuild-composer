package manifest

import (
	"fmt"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
)

type BootcPXETree struct {
	Base
	RootfsCompression string
	RootfsType        ISORootfsType

	platform      platform.Platform
	bootcPipeline *RawBootcImage
	files         []*fsnode.File // grub template and README files
}

// NewBootcPXETree creates a pipeline with a kernel, initrd, and compressed root filesystem
// suitable for use with PXE booting a system.
// Defaults to using xz compressed squashfs rootfs
func NewBootcPXETree(buildPipeline Build, bootcPipeline *RawBootcImage, platform platform.Platform) *BootcPXETree {
	p := &BootcPXETree{
		Base:              NewBase("bootc-pxe-tree", buildPipeline),
		platform:          platform,
		bootcPipeline:     bootcPipeline,
		RootfsCompression: "xz",
		RootfsType:        SquashfsRootfs,
	}
	buildPipeline.addDependent(p)
	return p
}

// Create a directory tree containing the kernel, initrd, and compressed rootfs
func (p *BootcPXETree) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return pipeline, err
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
		erofsOptions := osbuild.ErofsStageOptions{
			Filename: "rootfs.img",
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

		// TODO this is shared with the ISO, should it be?
		// Clean up the root filesystem's /boot to save space
		erofsOptions.ExcludePaths = installerBootExcludePaths
		erofsOptions.Source = "mount://-/"
		erofsStage := osbuild.NewErofsWithMountsStage(&erofsOptions, nil, devices, mounts)
		pipeline.AddStage(erofsStage)
	} else {
		var squashfsOptions osbuild.SquashfsStageOptions

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

	// Mount the ostree disk image (not the deployment) and copy the EFI files from it
	copyStageOptions := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: "mount://-/boot/efi/EFI",
				To:   "tree:///EFI",
			},
		},
	}
	copyStage := osbuild.NewCopyStage(copyStageOptions, nil, devices, mounts)
	pipeline.AddStage(copyStage)

	// NOTE: After this point mounts includes the 'ostree.deployment' stage
	mounts = append(mounts, *osbuild.NewOSTreeDeploymentMountDefault("ostree.deployment", osbuild.OSTreeMountSourceMount))

	// Mount the ostree disk image and deployment and copy files from it
	copyStageOptions = &osbuild.CopyStageOptions{
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
	copyStage = osbuild.NewCopyStage(copyStageOptions, nil, devices, mounts)
	pipeline.AddStage(copyStage)

	// Make an example grub.cfg
	stages, err := p.makeGrubConfig()
	if err != nil {
		return pipeline, err
	}
	pipeline.AddStages(stages...)

	// Make a README file
	stages, err = p.makeREADME()
	if err != nil {
		return pipeline, err
	}
	pipeline.AddStages(stages...)

	// Update the grub.cfg with the ostree boot uuid
	ostreeStageOptions := &osbuild.OSTreeGrub2StageOptions{
		Filename: "grub.cfg",
		Source:   "mount://-/",
	}
	pipeline.AddStage(osbuild.NewOSTreeGrub2MountsStage(ostreeStageOptions, nil, devices, mounts))

	// Make sure all the files are readable
	options := osbuild.ChmodStageOptions{
		Items: p.getChmodFiles(),
	}
	pipeline.AddStage(osbuild.NewChmodStage(&options))
	return pipeline, nil
}

// makeGrubConfig returns stages that creates an example grub config file
// It adds any kernel arguments from the blueprint to the cmdline in the template
func (p *BootcPXETree) makeGrubConfig() ([]*osbuild.Stage, error) {
	grubTemplate, err := fileDataFS.ReadFile("pxetree/ostree-grub.cfg")
	if err != nil {
		return nil, err
	}

	template := strings.ReplaceAll(string(grubTemplate), "@CMDLINE@", strings.Join(p.bootcPipeline.OSCustomizations.KernelOptionsAppend, " "))
	f, err := fsnode.NewFile("/grub.cfg", nil, nil, nil, []byte(template))
	if err != nil {
		panic(err)
	}
	p.files = append(p.files, f)
	return osbuild.GenFileNodesStages([]*fsnode.File{f}), nil
}

// makeREADME returns a stage that creates a README file
func (p *BootcPXETree) makeREADME() ([]*osbuild.Stage, error) {
	readme, err := fileDataFS.ReadFile("pxetree/README")
	if err != nil {
		return nil, err
	}

	f, err := fsnode.NewFile("/README", nil, nil, nil, readme)
	if err != nil {
		return nil, err
	}
	p.files = append(p.files, f)
	return osbuild.GenFileNodesStages([]*fsnode.File{f}), nil
}

func (p *BootcPXETree) getInline() []string {
	inlineData := []string{}

	// inline data for custom files
	for _, file := range p.files {
		inlineData = append(inlineData, string(file.Data()))
	}

	return inlineData
}

// getChmodFiles returns a list of files and permissions that need to be updated at the
// end of the pipeline. Also used to list the files to include in the tar.
func (p *BootcPXETree) getChmodFiles() map[string]osbuild.ChmodStagePathOptions {
	return map[string]osbuild.ChmodStagePathOptions{
		"/EFI": {
			Mode:      "ugo+Xr",
			Recursive: true,
		},
		"/vmlinuz": {
			Mode: "0755",
		},
		"/initrd.img": {
			Mode: "0644",
		},
		"/rootfs.img": {
			Mode: "0644",
		},
		"/grub.cfg": {
			Mode: "0644",
		},
		"/README": {
			Mode: "0644",
		},
	}
}

// GetTarFiles returns the list of files in the tree to be included in a tar
func (p *BootcPXETree) GetTarFiles() []string {
	var files []string
	for f := range p.getChmodFiles() {
		// Tar needs relative paths
		files = append(files, strings.TrimPrefix(f, "/"))
	}
	return files
}
