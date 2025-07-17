package manifest

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/osbuild"
)

type Vagrant struct {
	Base
	filename   string
	provider   osbuild.VagrantProvider
	macAddress string

	imgPipeline FilePipeline
}

// Create a randomized mac address for each build, but generated with a potentially seeded
// PRNG.
// See: https://github.com/mirror/vbox/blob/b9657cd5351cf17432b664009cc25bb480dc64c1/src/VBox/Main/src-server/HostImpl.cpp#L3258-L3269
// for where this implementation comes from.
func virtualboxMacAddress(prng *rand.Rand) string {
	manafacturer := "080027"
	serial := make([]byte, 3)

	prng.Read(serial)

	return fmt.Sprintf("%s%x", manafacturer, serial)
}

func (p Vagrant) Filename() string {
	return p.filename
}

func (p *Vagrant) SetFilename(filename string) {
	p.filename = filename
}

func NewVagrant(buildPipeline Build, imgPipeline FilePipeline, provider osbuild.VagrantProvider, prng *rand.Rand) *Vagrant {
	p := &Vagrant{
		Base:        NewBase("vagrant", buildPipeline),
		imgPipeline: imgPipeline,
		filename:    "image.box",
		provider:    provider,

		// macAddress is only required when the provider is virtualbox, we set it always so we don't have to
		// complicate flow in serialize
		macAddress: virtualboxMacAddress(prng),
	}

	if buildPipeline != nil {
		buildPipeline.addDependent(p)
	} else {
		imgPipeline.Manifest().addPipeline(p)
	}

	return p
}

func (p *Vagrant) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	vagrantOptions := osbuild.NewVagrantStageOptions(p.provider)

	// For the VirtualBox provider we need to inject the ovf stage as well
	if p.provider == osbuild.VagrantProviderVirtualBox {
		// TODO: find a way to avoid copying (by having the OVF stage take inputs?) as this can be
		// slow and increase disk usage
		inputName := "vmdk-tree"
		pipeline.AddStage(osbuild.NewCopyStageSimple(
			&osbuild.CopyStageOptions{
				Paths: []osbuild.CopyStagePath{
					{
						From: fmt.Sprintf("input://%s/%s", inputName, p.imgPipeline.Export().Filename()),
						To:   "tree:///",
					},
				},
			},
			osbuild.NewPipelineTreeInputs(inputName, p.imgPipeline.Name()),
		))

		vagrantOptions.SyncedFolders = map[string]*osbuild.VagrantSyncedFolderStageOptions{
			"/vagrant": &osbuild.VagrantSyncedFolderStageOptions{
				Type: osbuild.VagrantSyncedFolderTypeRsync,
			},
		}

		vagrantOptions.VirtualBox = &osbuild.VagrantVirtualBoxStageOptions{
			MacAddress: p.macAddress,
		}

		pipeline.AddStage(osbuild.NewOVFStage(&osbuild.OVFStageOptions{
			Vmdk: p.imgPipeline.Filename(),
		}))
	}

	pipeline.AddStage(osbuild.NewVagrantStage(
		vagrantOptions,
		osbuild.NewVagrantStagePipelineFilesInputs(p.imgPipeline.Name(), p.imgPipeline.Filename()),
	))

	return pipeline
}

func (p *Vagrant) getBuildPackages(Distro) []string {
	return []string{"qemu-img"}
}

func (p *Vagrant) Export() *artifact.Artifact {
	p.Base.export = true
	mimeType := "application/x-qemu-disk"
	return artifact.New(p.Name(), p.Filename(), &mimeType)
}
