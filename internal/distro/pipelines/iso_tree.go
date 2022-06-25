package pipeline

import (
	"fmt"
	"path"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

type ISOTreePipeline struct {
	Pipeline
	treePipeline *Pipeline
	KernelVer    string
	Arch         string
	Release      string
	Vendor       string
	Product      string
	OSName       string
	OSVersion    string
	ISOLabel     string
	Users        []blueprint.UserCustomization
	Groups       []blueprint.GroupCustomization
	OSTreeParent string
	OSTreeRef    string
}

func NewISOTreePipeline(buildPipeline *BuildPipeline, treePipeline *Pipeline) ISOTreePipeline {
	return ISOTreePipeline{
		Pipeline:     New("bootiso-tree", &buildPipeline.Pipeline),
		treePipeline: treePipeline,
	}
}

func (p ISOTreePipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	kspath := "/osbuild.ks"
	ostreeRepoPath := "/ostree/repo"

	pipeline.AddStage(osbuild2.NewBootISOMonoStage(bootISOMonoStageOptions(p.KernelVer, p.Arch, p.Vendor, p.Product, p.OSVersion, p.ISOLabel, kspath), osbuild2.NewBootISOMonoStagePipelineTreeInputs(p.treePipeline.Name())))

	kickstartOptions, err := osbuild2.NewKickstartStageOptions(kspath, "", p.Users, p.Groups, makeISORootPath(ostreeRepoPath), p.OSTreeRef, p.OSName)
	if err != nil {
		panic("password encryption failed")
	}

	pipeline.AddStage(osbuild2.NewKickstartStage(kickstartOptions))
	pipeline.AddStage(osbuild2.NewDiscinfoStage(&osbuild2.DiscinfoStageOptions{
		BaseArch: p.Arch,
		Release:  p.Release,
	}))

	pipeline.AddStage(osbuild2.NewOSTreeInitStage(&osbuild2.OSTreeInitStageOptions{Path: ostreeRepoPath}))
	pipeline.AddStage(osbuild2.NewOSTreePullStage(
		&osbuild2.OSTreePullStageOptions{Repo: ostreeRepoPath},
		osbuild2.NewOstreePullStageInputs("org.osbuild.source", p.OSTreeParent, p.OSTreeRef),
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
