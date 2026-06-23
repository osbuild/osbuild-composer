package manifest

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/osbuild/image-builder/pkg/customizations/fsnode"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
)

type BootcPXETree struct {
	Base
	RootfsType ISORootfsType

	platform     platform.Platform
	treePipeline TreePipeline
	files        []*fsnode.File // grub template and README files

	KernelOptionsAppend []string
	KernelPath          string
	InitramfsPath       string
	RootfsPath          string
}

// NewBootcPXETree creates a pipeline with a kernel, initrd, and compressed root filesystem
// suitable for use with PXE booting a system.
// Defaults to using xz compressed squashfs rootfs
func NewBootcPXETree(buildPipeline Build, treePipeline TreePipeline, platform platform.Platform) *BootcPXETree {
	p := &BootcPXETree{
		Base:         NewBase("bootc-pxe-tree", buildPipeline),
		platform:     platform,
		treePipeline: treePipeline,
		RootfsType:   SquashfsRootfs,
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

	// Copy kernel and initramfs
	if p.KernelPath == "" || p.InitramfsPath == "" || p.RootfsPath == "" {
		return osbuild.Pipeline{}, fmt.Errorf("kernel, initramfs, and rootfs paths must be set")
	}

	inputName := "tree"
	copyStageOptions := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/%s", inputName, p.KernelPath),
				To:   "tree:///vmlinuz",
			},
			{
				From: fmt.Sprintf("input://%s/%s", inputName, p.InitramfsPath),
				To:   "tree:///initrd.img",
			},
			{
				From: fmt.Sprintf("input://%s/%s", inputName, p.RootfsPath),
				To:   "tree:///rootfs.img",
			},
		},
	}
	copyStageInputs := osbuild.NewPipelineTreeInputs(inputName, p.treePipeline.Name())
	copyStage := osbuild.NewCopyStageSimple(copyStageOptions, copyStageInputs)
	pipeline.AddStage(copyStage)

	lodevice := osbuild.NewLoopbackDevice(
		&osbuild.LoopbackDeviceOptions{
			Filename: "rootfs.img",
		},
	)
	devices := map[string]osbuild.Device{"disk": *lodevice}
	var mounts []osbuild.Mount
	switch p.RootfsType {
	case ErofsRootfs:
		mounts = []osbuild.Mount{*osbuild.NewErofsMount("-", "disk", "/")}
	case SquashfsRootfs:
		mounts = []osbuild.Mount{*osbuild.NewSquashfsMount("-", "disk", "/")}
	default:
		return osbuild.Pipeline{}, fmt.Errorf("Unknown ISOTree rootfs type: %v", p.RootfsType)
	}

	// Copy the EFI boot files
	copyStageOptions = &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: "mount://-/boot/efi/EFI",
				To:   "tree:///EFI",
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

	// Update the grub.cfg with the ostree boot uuid
	ostreeStageOptions := &osbuild.OSTreeGrub2StageOptions{
		Filename: "grub.cfg",
		Source:   "mount://-/",
	}
	pipeline.AddStage(osbuild.NewOSTreeGrub2MountsStage(ostreeStageOptions, nil, devices, mounts))

	// Make a README file
	stages, err = p.makeREADME()
	if err != nil {
		return pipeline, err
	}
	pipeline.AddStages(stages...)

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

	template := strings.ReplaceAll(string(grubTemplate), "@CMDLINE@", strings.Join(p.KernelOptionsAppend, " "))
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
	for _, f := range slices.Sorted(maps.Keys(p.getChmodFiles())) {
		// Tar needs relative paths
		files = append(files, strings.TrimPrefix(f, "/"))
	}
	return files
}
