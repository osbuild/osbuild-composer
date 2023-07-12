package manifest

import (
	"fmt"
	//"path"

	"github.com/osbuild/images/internal/users"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

// An LiveTree represents a tree containing the anaconda installer,
// configuration in terms of a kickstart file, as well as an embedded
// payload to be installed, this payload can either be an ostree
// CommitSpec or OSPipeline for an OS.
type LiveTree struct {
	Base

	// TODO: review optional and mandatory fields and their meaning
	OSName  string
	Release string
	Users   []users.User
	Groups  []users.Group

	PartitionTable *disk.PartitionTable

	anacondaPipeline Tree
	rootfsPipeline   *ISORootfsImg
	bootTreePipeline *EFIBootTree

	// The location of the kickstart file, if it will be added to the
	// bootiso-tree.
	// Otherwise, it should be defined in the interactive defaults of the
	// Anaconda pipeline.
	KSPath string

	// The path where the payload (tarball or ostree repo) will be stored.
	PayloadPath string

	isoLabel string

	SquashfsCompression string

	//OSPipeline         *OS
	//OSTreeCommitSource *ostree.SourceSpec
	//ostreeCommitSpec *ostree.CommitSpec

	KernelOpts []string

	// Enable ISOLinux stage
	ISOLinux bool

	Product string
	Version string
}

func NewLiveTree(m *Manifest,
	buildPipeline *Build,
	anacondaPipeline Tree,
	rootfsPipeline *ISORootfsImg,
	bootTreePipeline *EFIBootTree,
	isoLabel string) *LiveTree {

	p := &LiveTree{
		Base:             NewBase(m, "bootiso-tree", buildPipeline),
		anacondaPipeline: anacondaPipeline,
		rootfsPipeline:   rootfsPipeline,
		bootTreePipeline: bootTreePipeline,
		isoLabel:         isoLabel,
	}
	buildPipeline.addDependent(p)
	//if anacondaPipeline.Base.manifest != m {
	//	panic("anaconda pipeline from different manifest")
	//}
	m.addPipeline(p)
	return p
}

/*
func (p *LiveTree) getOSTreeCommitSources() []ostree.SourceSpec {
	if p.OSTreeCommitSource == nil {
		return nil
	}

	return []ostree.SourceSpec{
		*p.OSTreeCommitSource,
	}
}

func (p *LiveTree) getOSTreeCommits() []ostree.CommitSpec {
	if p.ostreeCommitSpec == nil {
		return nil
	}
	return []ostree.CommitSpec{*p.ostreeCommitSpec}
}
*/

func (p *LiveTree) getBuildPackages(_ Distro) []string {
	packages := []string{
		"squashfs-tools",
	}

	/*
		if p.OSTreeCommitSource != nil {
			packages = append(packages, "rpm-ostree")
		}

		if p.OSPipeline != nil {
			packages = append(packages, "tar")
		}
	*/

	if p.anacondaPipeline.GetPlatform().GetArch().String() == platform.ARCH_X86_64.String() {
		packages = append(packages, "syslinux-nonlinux")
	}

	packages = append(packages, p.PartitionTable.GetBuildPackages()...)

	return packages
}

func (p *LiveTree) serializeStart(_ []rpmmd.PackageSpec, _ []container.Spec, commits []ostree.CommitSpec) {
	if len(commits) == 0 {
		// nothing to do
		return
	}

	if len(commits) > 1 {
		panic("pipeline supports at most one ostree commit")
	}

	// p.ostreeCommitSpec = &commits[0]
}

func (p *LiveTree) serializeEnd() {
	//p.ostreeCommitSpec = nil
}

func (p *LiveTree) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	kernelOpts := []string{}

	if len(p.KernelOpts) > 0 {
		kernelOpts = append(kernelOpts, p.KernelOpts...)
	}

	pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{
			{
				Path: "images",
			},
			{
				Path: "images/pxeboot",
			},
		},
	}))

	// extra files
	pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{
			{
				Path: "LiveOS",
			},
		},
	}))

	inputName := "tree"
	copyStageOptions := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/boot/vmlinuz-%s", inputName, p.anacondaPipeline.GetKernelVersion()),
				To:   "tree:///images/pxeboot/vmlinuz",
			},
			{
				From: fmt.Sprintf("input://%s/boot/initramfs-%s.img", inputName, p.anacondaPipeline.GetKernelVersion()),
				To:   "tree:///images/pxeboot/initrd.img",
			},
		},
	}
	copyStageInputs := osbuild.NewPipelineTreeInputs(inputName, p.anacondaPipeline.Name())
	copyStage := osbuild.NewCopyStageSimple(copyStageOptions, copyStageInputs)
	pipeline.AddStage(copyStage)

	var squashfsOptions = osbuild.SquashfsStageOptions{
		Filename: "LiveOS/squashfs.img",
	}

	if p.SquashfsCompression != "" {
		squashfsOptions.Compression.Method = p.SquashfsCompression
	} else {
		// default to xz if not specified
		squashfsOptions.Compression.Method = "xz"
	}

	if squashfsOptions.Compression.Method == "xz" {
		squashfsOptions.Compression.Options = &osbuild.FSCompressionOptions{
			BCJ: osbuild.BCJOption(p.anacondaPipeline.GetPlatform().GetArch().String()),
		}
	}

	squashfsStage := osbuild.NewSquashfsStage(&squashfsOptions, p.rootfsPipeline.Name())
	pipeline.AddStage(squashfsStage)

	if p.ISOLinux {
		isoLinuxOptions := &osbuild.ISOLinuxStageOptions{
			Product: osbuild.ISOLinuxProduct{
				Name:    p.Product,
				Version: p.Version,
			},
			Kernel: osbuild.ISOLinuxKernel{
				Dir:  "/images/pxeboot",
				Opts: kernelOpts,
			},
		}

		isoLinuxStage := osbuild.NewISOLinuxStage(isoLinuxOptions, p.anacondaPipeline.Name())
		pipeline.AddStage(isoLinuxStage)
	}

	filename := "images/efiboot.img"
	pipeline.AddStage(osbuild.NewTruncateStage(&osbuild.TruncateStageOptions{
		Filename: filename,
		Size:     fmt.Sprintf("%d", p.PartitionTable.Size),
	}))

	efibootDevice := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: filename})
	for _, stage := range osbuild.GenMkfsStages(p.PartitionTable, efibootDevice) {
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

	pipeline.AddStage(osbuild.NewDiscinfoStage(&osbuild.DiscinfoStageOptions{
		BaseArch: p.anacondaPipeline.GetPlatform().GetArch().String(),
		Release:  p.Release,
	}))

	return pipeline
}
