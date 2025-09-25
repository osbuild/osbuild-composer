package manifest

import (
	"embed"
	"fmt"
	"strings"

	"github.com/osbuild/images/data/files"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/osbuild"
)

var fileDataFS embed.FS = files.Data

type PXETree struct {
	Base
	RootfsCompression string
	RootfsType        ISORootfsType

	osPipeline *OS
	files      []*fsnode.File // grub template and README files
}

// NewPXETree creates a pipeline with a kernel, initrd, and compressed root filesystem
// suitable for use with PXE booting a system.
// Defaults to using xz compressed squashfs rootfs
func NewPXETree(buildPipeline Build, osPipeline *OS) *PXETree {
	p := &PXETree{
		Base:              NewBase("pxe-tree", buildPipeline),
		osPipeline:        osPipeline,
		RootfsCompression: "xz",
		RootfsType:        SquashfsRootfs,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *PXETree) getBuildPackages(Distro) ([]string, error) {
	switch p.RootfsType {
	case ErofsRootfs:
		return []string{"erofs-utils"}, nil
	default:
		return []string{"squashfs-tools"}, nil
	}
}

// Create a directory tree containing the kernel, initrd, and compressed rootfs
func (p *PXETree) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return pipeline, err
	}

	inputName := "tree"
	copyStageOptions := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/boot/vmlinuz-%s", inputName, p.osPipeline.kernelVer),
				To:   "tree:///vmlinuz",
			},
			{
				From: fmt.Sprintf("input://%s/boot/initramfs-%s.img", inputName, p.osPipeline.kernelVer),
				To:   "tree:///initrd.img",
			},
			{
				From: fmt.Sprintf("input://%s/boot/efi/EFI", inputName),
				To:   "tree:///EFI",
			},
		},
	}
	copyStageInputs := osbuild.NewPipelineTreeInputs(inputName, p.osPipeline.Name())
	copyStage := osbuild.NewCopyStageSimple(copyStageOptions, copyStageInputs)
	pipeline.AddStage(copyStage)

	// Compress the os tree
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
		pipeline.AddStage(osbuild.NewErofsStage(&erofsOptions, p.osPipeline.Name()))
	} else {
		var squashfsOptions osbuild.SquashfsStageOptions

		squashfsOptions.Filename = "rootfs.img"
		squashfsOptions.Compression.Method = "xz"

		if squashfsOptions.Compression.Method == "xz" {
			squashfsOptions.Compression.Options = &osbuild.FSCompressionOptions{
				BCJ: osbuild.BCJOption(p.osPipeline.platform.GetArch().String()),
			}
		}

		// TODO this is shared with the ISO, should it be?
		// Clean up the root filesystem's /boot to save space
		squashfsOptions.ExcludePaths = installerBootExcludePaths
		pipeline.AddStage(osbuild.NewSquashfsStage(&squashfsOptions, p.osPipeline.Name()))
	}

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

	// Make sure all the files are readable
	options := osbuild.ChmodStageOptions{
		Items: map[string]osbuild.ChmodStagePathOptions{
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
		},
	}
	pipeline.AddStage(osbuild.NewChmodStage(&options))
	return pipeline, nil
}

// dracutStageOptions returns the basic dracut setup for booting from a compressed
// root filesystem using root=live:... on the kernel cmdline.
func (p *PXETree) DracutConfStageOptions() *osbuild.DracutConfStageOptions {
	return &osbuild.DracutConfStageOptions{
		Filename: "40-pxe.conf",
		Config: osbuild.DracutConfigFile{
			EarlyMicrocode: common.ToPtr(false),
			AddModules:     []string{"qemu", "qemu-net", "livenet", "dmsquash-live"},
			Compress:       "xz",
		},
	}
}

// makeGrubConfig returns stages that creates an example grub config file
// It adds any kernel arguments from the blueprint to the cmdline in the template
func (p *PXETree) makeGrubConfig() ([]*osbuild.Stage, error) {
	grubTemplate, err := fileDataFS.ReadFile("pxetree/grub.cfg")
	if err != nil {
		return nil, err
	}

	template := strings.ReplaceAll(string(grubTemplate), "@CMDLINE@", strings.Join(p.osPipeline.OSCustomizations.KernelOptionsAppend, " "))
	f, err := fsnode.NewFile("/grub.cfg", nil, nil, nil, []byte(template))
	if err != nil {
		panic(err)
	}
	p.files = append(p.files, f)
	return osbuild.GenFileNodesStages([]*fsnode.File{f}), nil
}

// makeREADME returns a stage that creates a README file
func (p *PXETree) makeREADME() ([]*osbuild.Stage, error) {
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

func (p *PXETree) getInline() []string {
	inlineData := []string{}

	// inline data for custom files
	for _, file := range p.files {
		inlineData = append(inlineData, string(file.Data()))
	}

	return inlineData
}
