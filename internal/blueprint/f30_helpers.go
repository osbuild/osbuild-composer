package blueprint

import (
	"github.com/osbuild/osbuild-composer/internal/pipeline"

	"github.com/google/uuid"
)

func getF30Repository() *pipeline.DNFRepository {
	repo := pipeline.NewDNFRepository("https://mirrors.fedoraproject.org/metalink?repo=fedora-$releasever&arch=$basearch", "", "")
	repo.SetChecksum("sha256:9f596e18f585bee30ac41c11fb11a83ed6b11d5b341c1cb56ca4015d7717cb97")
	repo.SetGPGKey("F1D8 EC98 F241 AAF2 0DF6  9420 EF3C 111F CFC6 59B9")
	return repo
}

func getF30BuildPipeline() *pipeline.Pipeline {
	p := &pipeline.Pipeline{}
	options := &pipeline.DNFStageOptions{
		ReleaseVersion:   "30",
		BaseArchitecture: "x86_64",
	}
	options.AddRepository(getF30Repository())
	options.AddPackage("dnf")
	options.AddPackage("e2fsprogs")
	options.AddPackage("policycoreutils")
	options.AddPackage("qemu-img")
	options.AddPackage("systemd")
	options.AddPackage("grub2-pc")
	options.AddPackage("tar")
	p.AddStage(pipeline.NewDNFStage(options))
	return p
}

func getF30Pipeline() *pipeline.Pipeline {
	p := &pipeline.Pipeline{
		BuildPipeline: getF30BuildPipeline(),
	}
	options := &pipeline.DNFStageOptions{
		ReleaseVersion:   "30",
		BaseArchitecture: "x86_64",
	}
	options.AddRepository(getF30Repository())
	options.AddPackage("@Core")
	options.AddPackage("chrony")
	options.AddPackage("kernel")
	options.AddPackage("selinux-policy-targeted")
	options.AddPackage("grub2-pc")
	options.AddPackage("spice-vdagent")
	options.AddPackage("qemu-guest-agent")
	options.AddPackage("xen-libs")
	options.AddPackage("langpacks-en")
	p.AddStage(pipeline.NewDNFStage(options))
	p.AddStage(pipeline.NewFixBLSStage())
	p.AddStage(pipeline.NewLocaleStage(
		&pipeline.LocaleStageOptions{
			Language: "en_US",
		}))

	return p
}

func getCustomF30PackageSet(packages []string, excludedPackages []string) *pipeline.Pipeline {
	p := &pipeline.Pipeline{
		BuildPipeline: getF30BuildPipeline(),
	}
	options := &pipeline.DNFStageOptions{
		ReleaseVersion:   "30",
		BaseArchitecture: "x86_64",
	}
	options.AddRepository(getF30Repository())
	for _, pkg := range packages {
		options.AddPackage(pkg)
	}
	for _, pkg := range excludedPackages {
		options.ExcludePackage(pkg)
	}
	p.AddStage(pipeline.NewDNFStage(options))
	return p
}

func addF30GRUB2Stage(p *pipeline.Pipeline) {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}
	p.AddStage(pipeline.NewGRUB2Stage(
		&pipeline.GRUB2StageOptions{
			RootFilesystemUUID: id,
			KernelOptions:      "ro biosdevname=0 net.ifnames=0",
		},
	))
}

func addF30FSTabStage(p *pipeline.Pipeline) {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}
	options := &pipeline.FSTabStageOptions{}
	options.AddFilesystem(id, "ext4", "/", "defaults", 1, 1)
	p.AddStage(pipeline.NewFSTabStage(options))
}

func addF30SELinuxStage(p *pipeline.Pipeline) {
	p.AddStage(pipeline.NewSELinuxStage(
		&pipeline.SELinuxStageOptions{
			FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
		}))
}

func addF30LocaleStage(p *pipeline.Pipeline) {
	p.AddStage(pipeline.NewLocaleStage(
		&pipeline.LocaleStageOptions{
			Language: "en_US",
		}))
}

func addF30FixBlsStage(p *pipeline.Pipeline) {
	p.AddStage(pipeline.NewFixBLSStage())
}

func addF30QemuAssembler(p *pipeline.Pipeline, format string, filename string) {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}
	p.Assembler = pipeline.NewQEMUAssembler(
		&pipeline.QEMUAssemblerOptions{
			Format:             format,
			Filename:           filename,
			PTUUID:             "0x14fc63d2",
			RootFilesystemUUDI: id,
			Size:               3221225472,
		})
}

func addF30TarAssembler(p *pipeline.Pipeline, filename string) {
	p.Assembler = pipeline.NewTarAssembler(
		&pipeline.TarAssemblerOptions{
			Filename: filename,
		})
}

func addF30RawFSAssembler(p *pipeline.Pipeline, filename string) {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}
	p.Assembler = pipeline.NewRawFSAssembler(
		&pipeline.RawFSAssemblerOptions{
			Filename:           filename,
			RootFilesystemUUDI: id,
			Size:               3221225472,
		})
}
