package manifest

import (
	"fmt"
	"math"

	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

// PartitionImage extracts a single partition from a raw disk image using dd.
type PartitionImage struct {
	Base
	filename    string
	imgPipeline FilePipeline

	// Mountpoint identifies which partition to extract.
	Mountpoint string

	// PartitionTable is used to resolve the mountpoint to an offset and size.
	PartitionTable *disk.PartitionTable
}

func NewPartitionImage(buildPipeline Build, imgPipeline FilePipeline, mountpoint string, pt *disk.PartitionTable, name string) *PartitionImage {
	p := &PartitionImage{
		Base:           NewBase(name, buildPipeline),
		imgPipeline:    imgPipeline,
		filename:       fmt.Sprintf("%s.img", name),
		Mountpoint:     mountpoint,
		PartitionTable: pt,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *PartitionImage) Filename() string {
	return p.filename
}

func (p *PartitionImage) SetFilename(filename string) {
	p.filename = filename
}

func (p *PartitionImage) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	part := p.PartitionTable.FindPartitionForMountpoint(p.Mountpoint)
	if part == nil {
		return osbuild.Pipeline{}, fmt.Errorf("partition for mountpoint %q not found", p.Mountpoint)
	}

	if part.Start > math.MaxInt {
		return osbuild.Pipeline{}, fmt.Errorf("partition start exceeds addressable range: %d", part.Start)
	}
	if uint64(part.Size) > math.MaxInt {
		return osbuild.Pipeline{}, fmt.Errorf("partition size exceeds addressable range: %d", part.Size)
	}
	srcOffset := int(part.Start) // #nosec G115 - overflow guarded above
	opts := &osbuild.DDStageOptions{
		Src:       fmt.Sprintf("input://image/%s", p.imgPipeline.Filename()),
		Dst:       p.filename,
		SrcOffset: &srcOffset,
		Count:     int(part.Size), // #nosec G115 - overflow guarded above
	}

	inputs := osbuild.NewPipelineTreeInputs("image", p.imgPipeline.Name())
	pipeline.AddStage(osbuild.NewDDStage(opts, inputs))

	truncateOpts := &osbuild.TruncateStageOptions{
		Filename: p.filename,
		Size:     fmt.Sprintf("%d", part.Size),
	}
	pipeline.AddStage(osbuild.NewTruncateStage(truncateOpts))

	return pipeline, nil
}

func (p *PartitionImage) getBuildPackages(Distro) ([]string, error) {
	return nil, nil
}

func (p *PartitionImage) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename(), nil)
}
