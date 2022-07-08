package manifest

import (
	"fmt"
	"path"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

// An ISOTree represents a tree containing the anaconda installer,
// configuration in terms of a kickstart file, as well as an embedded
// payload to be installed.
type ISOTree struct {
	Base
	// TODO: review optional and mandatory fields and their meaning
	UEFIVendor string
	OSName     string
	Release    string
	Users      []blueprint.UserCustomization
	Groups     []blueprint.GroupCustomization

	anacondaPipeline *Anaconda
	isoLabel         string
	osTreeCommit     string
	osTreeURL        string
	osTreeRef        string
}

func NewISOTree(m *Manifest,
	buildPipeline *Build,
	anacondaPipeline *Anaconda,
	osTreeCommit,
	osTreeURL,
	osTreeRef,
	isoLabelTmpl string) *ISOTree {
	// TODO: replace isoLabelTmpl with more high-level properties
	isoLabel := fmt.Sprintf(isoLabelTmpl, anacondaPipeline.platform.GetArch())

	p := &ISOTree{
		Base:             NewBase(m, "bootiso-tree", buildPipeline),
		anacondaPipeline: anacondaPipeline,
		isoLabel:         isoLabel,
		osTreeCommit:     osTreeCommit,
		osTreeURL:        osTreeURL,
		osTreeRef:        osTreeRef,
	}
	buildPipeline.addDependent(p)
	if anacondaPipeline.Base.manifest != m {
		panic("anaconda pipeline from different manifest")
	}
	m.addPipeline(p)
	return p
}

func (p *ISOTree) getOSTreeCommits() []osTreeCommit {
	return []osTreeCommit{
		{
			checksum: p.osTreeCommit,
			url:      p.osTreeURL,
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

func (p *ISOTree) serialize() osbuild2.Pipeline {
	pipeline := p.Base.serialize()

	kspath := "/osbuild.ks"
	ostreeRepoPath := "/ostree/repo"

	pipeline.AddStage(osbuild2.NewBootISOMonoStage(bootISOMonoStageOptions(p.anacondaPipeline.kernelVer,
		p.anacondaPipeline.platform.GetArch().String(),
		p.UEFIVendor,
		p.anacondaPipeline.product,
		p.anacondaPipeline.version,
		p.isoLabel,
		kspath),
		osbuild2.NewBootISOMonoStagePipelineTreeInputs(p.anacondaPipeline.Name())))

	kickstartOptions, err := osbuild2.NewKickstartStageOptions(kspath, "", p.Users, p.Groups, makeISORootPath(ostreeRepoPath), p.osTreeRef, p.OSName)
	if err != nil {
		panic("password encryption failed")
	}

	pipeline.AddStage(osbuild2.NewKickstartStage(kickstartOptions))
	pipeline.AddStage(osbuild2.NewDiscinfoStage(&osbuild2.DiscinfoStageOptions{
		BaseArch: p.anacondaPipeline.platform.GetArch().String(),
		Release:  p.Release,
	}))

	pipeline.AddStage(osbuild2.NewOSTreeInitStage(&osbuild2.OSTreeInitStageOptions{Path: ostreeRepoPath}))
	pipeline.AddStage(osbuild2.NewOSTreePullStage(
		&osbuild2.OSTreePullStageOptions{Repo: ostreeRepoPath},
		osbuild2.NewOstreePullStageInputs("org.osbuild.source", p.osTreeCommit, p.osTreeRef),
	))

	return pipeline
}

func bootISOMonoStageOptions(kernelVer, arch, vendor, product, osVersion, isolabel, kspath string) *osbuild2.BootISOMonoStageOptions {
	comprOptions := new(osbuild2.FSCompressionOptions)
	if bcj := osbuild2.BCJOption(arch); bcj != "" {
		comprOptions.BCJ = bcj
	}
	var architectures []string

	if arch == distro.X86_64ArchName {
		architectures = []string{"X64"}
	} else if arch == distro.Aarch64ArchName {
		architectures = []string{"AA64"}
	} else {
		panic("unsupported architecture")
	}

	return &osbuild2.BootISOMonoStageOptions{
		Product: osbuild2.Product{
			Name:    product,
			Version: osVersion,
		},
		ISOLabel:   isolabel,
		Kernel:     kernelVer,
		KernelOpts: fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", isolabel, kspath),
		EFI: osbuild2.EFI{
			Architectures: architectures,
			Vendor:        vendor,
		},
		ISOLinux: osbuild2.ISOLinux{
			Enabled: arch == distro.X86_64ArchName,
			Debug:   false,
		},
		Templates: "99-generic",
		RootFS: osbuild2.RootFS{
			Size: 9216,
			Compression: osbuild2.FSCompression{
				Method:  "xz",
				Options: comprOptions,
			},
		},
	}
}

//makeISORootPath return a path that can be used to address files and folders in
//the root of the iso
func makeISORootPath(p string) string {
	fullpath := path.Join("/run/install/repo", p)
	return fmt.Sprintf("file://%s", fullpath)
}
