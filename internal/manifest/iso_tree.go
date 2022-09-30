package manifest

import (
	"fmt"
	"path"

	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/users"
)

// An ISOTree represents a tree containing the anaconda installer,
// configuration in terms of a kickstart file, as well as an embedded
// payload to be installed.
type ISOTree struct {
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

	KSPath   string
	isoLabel string

	OSTree *ostree.CommitSpec
}

func NewISOTree(m *Manifest,
	buildPipeline *Build,
	anacondaPipeline *Anaconda,
	rootfsPipeline *ISORootfsImg,
	bootTreePipeline *EFIBootTree,
	isoLabel string) *ISOTree {

	p := &ISOTree{
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

func (p *ISOTree) getOSTreeCommits() []osTreeCommit {
	var checksum, url string
	if p.OSTree != nil {
		checksum = p.OSTree.Checksum
		url = p.OSTree.URL

	}
	return []osTreeCommit{
		{
			checksum: checksum,
			url:      url,
		},
	}
}

func (p *ISOTree) getBuildPackages() []string {
	packages := []string{
		"rpm-ostree",
		"squashfs-tools",
	}
	return packages
}

func (p *ISOTree) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	ostreeRepoPath := "/ostree/repo"
	kernelOpts := fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", p.isoLabel, p.KSPath)

	pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.Path{
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
		Compression: osbuild.FSCompression{
			Method: "lz4",
		},
	}
	squashfsStage := osbuild.NewSquashfsStage(&squashfsOptions, p.rootfsPipeline.Name())
	pipeline.AddStage(squashfsStage)

	isoLinuxOptions := &osbuild.ISOLinuxStageOptions{
		Product: osbuild.ISOLinuxProduct{
			Name:    p.anacondaPipeline.product,
			Version: p.anacondaPipeline.version,
		},
		Kernel: osbuild.ISOLinuxKernel{
			Dir:  "/images/pxeboot",
			Opts: []string{kernelOpts},
		},
	}
	isoLinuxStage := osbuild.NewISOLinuxStage(isoLinuxOptions, p.anacondaPipeline.Name())
	pipeline.AddStage(isoLinuxStage)

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

	if p.OSTree == nil {
		panic("missing ostree parameters in ISO tree pipeline")
	}
	kickstartOptions, err := osbuild.NewKickstartStageOptions(p.KSPath, "", p.Users, p.Groups, makeISORootPath(ostreeRepoPath), p.OSTree.Ref, p.OSName)
	if err != nil {
		panic("password encryption failed")
	}

	pipeline.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: ostreeRepoPath}))
	pipeline.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: ostreeRepoPath},
		osbuild.NewOstreePullStageInputs("org.osbuild.source", p.OSTree.Checksum, p.OSTree.Ref),
	))

	pipeline.AddStage(osbuild.NewKickstartStage(kickstartOptions))
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
