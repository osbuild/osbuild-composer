package manifest

import (
	"fmt"
	"path"

	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/users"
)

// An AnacondaISOTree represents a tree containing the anaconda installer,
// configuration in terms of a kickstart file, as well as an embedded
// payload to be installed, this payload can either be an ostree
// CommitSpec or OSPipeline for an OS.
type AnacondaISOTree struct {
	Base

	// TODO: review optional and mandatory fields and their meaning
	OSName  string
	Release string
	Users   []users.User
	Groups  []users.Group

	PartitionTable *disk.PartitionTable

	anacondaPipeline *Anaconda
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

	OSPipeline *OS
	OSTree     *ostree.CommitSpec

	KernelOpts []string

	// Enable ISOLinux stage
	ISOLinux bool
}

func NewAnacondaISOTree(m *Manifest,
	buildPipeline *Build,
	anacondaPipeline *Anaconda,
	rootfsPipeline *ISORootfsImg,
	bootTreePipeline *EFIBootTree,
	isoLabel string) *AnacondaISOTree {

	p := &AnacondaISOTree{
		Base:             NewBase(m, "bootiso-tree", buildPipeline),
		anacondaPipeline: anacondaPipeline,
		rootfsPipeline:   rootfsPipeline,
		bootTreePipeline: bootTreePipeline,
		isoLabel:         isoLabel,
	}
	buildPipeline.addDependent(p)
	if anacondaPipeline.Base.manifest != m {
		panic("anaconda pipeline from different manifest")
	}
	m.addPipeline(p)
	return p
}

func (p *AnacondaISOTree) getOSTreeCommits() []ostree.CommitSpec {
	if p.OSTree == nil {
		return nil
	}

	return []ostree.CommitSpec{
		*p.OSTree,
	}
}

func (p *AnacondaISOTree) getBuildPackages() []string {
	packages := []string{
		"squashfs-tools",
	}

	if p.OSTree != nil {
		packages = append(packages, "rpm-ostree")
	}

	if p.OSPipeline != nil {
		packages = append(packages, "tar")
	}

	return packages
}

func (p *AnacondaISOTree) serializeStart(_ []rpmmd.PackageSpec, _ []container.Spec, commits []ostree.CommitSpec) {
	if len(commits) == 0 {
		// nothing to do
		return
	}

	if len(commits) > 1 {
		panic("pipeline supports at most one ostree commit")
	}

	p.OSTree = &commits[0]
}

func (p *AnacondaISOTree) serializeEnd() {
	p.OSTree = nil
}

func (p *AnacondaISOTree) serialize() osbuild.Pipeline {
	// We need one of two payloads
	if p.OSTree == nil && p.OSPipeline == nil {
		panic("missing ostree or ospipeline parameters in ISO tree pipeline")
	}

	// But not both payloads
	if p.OSTree != nil && p.OSPipeline != nil {
		panic("got both ostree and ospipeline parameters in ISO tree pipeline")
	}

	pipeline := p.Base.serialize()

	kernelOpts := []string{}

	kernelOpts = append(kernelOpts, fmt.Sprintf("inst.stage2=hd:LABEL=%s", p.isoLabel))
	if p.KSPath != "" {
		kernelOpts = append(kernelOpts, fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", p.isoLabel, p.KSPath))
	}

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

	inputName := "tree"
	copyStageOptions := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/boot/vmlinuz-%s", inputName, p.anacondaPipeline.kernelVer),
				To:   "tree:///images/pxeboot/vmlinuz",
			},
			{
				From: fmt.Sprintf("input://%s/boot/initramfs-%s.img", inputName, p.anacondaPipeline.kernelVer),
				To:   "tree:///images/pxeboot/initrd.img",
			},
		},
	}
	copyStageInputs := osbuild.NewPipelineTreeInputs(inputName, p.anacondaPipeline.Name())
	copyStage := osbuild.NewCopyStageSimple(copyStageOptions, copyStageInputs)
	pipeline.AddStage(copyStage)

	squashfsOptions := osbuild.SquashfsStageOptions{
		Filename: "images/install.img",
	}

	if p.SquashfsCompression != "" {
		squashfsOptions.Compression.Method = p.SquashfsCompression
	} else {
		// default to xz if not specified
		squashfsOptions.Compression.Method = "xz"
	}

	if squashfsOptions.Compression.Method == "xz" {
		squashfsOptions.Compression.Options = &osbuild.FSCompressionOptions{
			BCJ: osbuild.BCJOption(p.anacondaPipeline.platform.GetArch().String()),
		}
	}

	squashfsStage := osbuild.NewSquashfsStage(&squashfsOptions, p.rootfsPipeline.Name())
	pipeline.AddStage(squashfsStage)

	if p.ISOLinux {
		isoLinuxOptions := &osbuild.ISOLinuxStageOptions{
			Product: osbuild.ISOLinuxProduct{
				Name:    p.anacondaPipeline.product,
				Version: p.anacondaPipeline.version,
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

	if p.OSTree != nil {
		// Set up the payload ostree repo
		pipeline.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: p.PayloadPath}))
		pipeline.AddStage(osbuild.NewOSTreePullStage(
			&osbuild.OSTreePullStageOptions{Repo: p.PayloadPath},
			osbuild.NewOstreePullStageInputs("org.osbuild.source", p.OSTree.Checksum, p.OSTree.Ref),
		))

		// Configure the kickstart file with the payload and any user options
		kickstartOptions, err := osbuild.NewKickstartStageOptions(p.KSPath, "", p.Users, p.Groups, makeISORootPath(p.PayloadPath), p.OSTree.Ref, p.OSName)

		if err != nil {
			panic("failed to create kickstartstage options")
		}

		pipeline.AddStage(osbuild.NewKickstartStage(kickstartOptions))
	}

	if p.OSPipeline != nil {
		// Create the payload tarball
		pipeline.AddStage(osbuild.NewTarStage(&osbuild.TarStageOptions{Filename: p.PayloadPath}, p.OSPipeline.name))

		// If the KSPath is set, we need to add the kickstart stage to this (bootiso-tree) pipeline.
		// If it's not specified here, it should have been added to the InteractiveDefaults in the anaconda-tree.
		if p.KSPath != "" {
			kickstartOptions, err := osbuild.NewKickstartStageOptions(p.KSPath, makeISORootPath(p.PayloadPath), p.Users, p.Groups, "", "", p.OSName)
			if err != nil {
				panic("failed to create kickstartstage options")
			}

			pipeline.AddStage(osbuild.NewKickstartStage(kickstartOptions))
		}
	}

	pipeline.AddStage(osbuild.NewDiscinfoStage(&osbuild.DiscinfoStageOptions{
		BaseArch: p.anacondaPipeline.platform.GetArch().String(),
		Release:  p.Release,
	}))

	return pipeline
}

// makeISORootPath return a path that can be used to address files and folders
// in the root of the iso
func makeISORootPath(p string) string {
	fullpath := path.Join("/run/install/repo", p)
	return fmt.Sprintf("file://%s", fullpath)
}
