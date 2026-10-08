package manifest

import (
	"fmt"
	"path/filepath"

	"github.com/osbuild/image-builder/pkg/artifact"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/osbuild"
)

// FilePrep extracts a single file from an assembled raw disk image by
// mounting its filesystems and copying the file out. It runs after the
// full image (including the bootloader) has been assembled.
//
// The pipeline tree contains both the loop-mounted disk image and the
// extracted file. Use CopyFile (or a compression pipeline) to produce
// a clean export that contains only the extracted file.
type FilePrep struct {
	Base
	filename    string
	imgPipeline FilePipeline

	// Path is the absolute path inside the image's filesystem to extract.
	Path string

	// PartitionTable is used to set up loopback devices and mounts.
	PartitionTable *disk.PartitionTable
}

func NewFilePrep(buildPipeline Build, imgPipeline FilePipeline, path string, pt *disk.PartitionTable, name string) *FilePrep {
	p := &FilePrep{
		Base:           NewBase(name, buildPipeline),
		imgPipeline:    imgPipeline,
		filename:       filepath.Base(path),
		Path:           path,
		PartitionTable: pt,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *FilePrep) Filename() string {
	return p.filename
}

func (p *FilePrep) SetFilename(filename string) {
	p.filename = filename
}

func (p *FilePrep) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	// Copy the raw disk image from the input pipeline into this pipeline's
	// tree so it can be loop-mounted.
	imgFilename := "disk.img"
	copyImageOpts := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://image/%s", p.imgPipeline.Filename()),
				To:   fmt.Sprintf("tree:///%s", imgFilename),
			},
		},
	}
	copyImageInputs := osbuild.NewPipelineTreeInputs("image", p.imgPipeline.Name())
	pipeline.AddStage(osbuild.NewCopyStageSimple(copyImageOpts, copyImageInputs))

	// Mount the disk image and copy the file out.
	fsRootMntName, mounts, devices, err := osbuild.GenMountsDevicesFromPT(imgFilename, p.PartitionTable)
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	copyFileOpts := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("mount://%s%s", fsRootMntName, p.Path),
				To:   fmt.Sprintf("tree:///%s", p.filename),
			},
		},
	}
	pipeline.AddStage(osbuild.NewCopyStage(copyFileOpts, nil, devices, mounts))

	return pipeline, nil
}

func (p *FilePrep) getBuildPackages(Distro) ([]string, error) {
	return nil, nil
}

func (p *FilePrep) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename(), nil)
}

// CopyFile copies a single file from an input pipeline into a clean tree.
// It is used to produce an export that contains only the target file
// without any intermediate artifacts (such as the disk image that
// FilePrep keeps in its tree for loop-mounting).
type CopyFile struct {
	Base
	filename    string
	imgPipeline FilePipeline
}

func NewCopyFile(buildPipeline Build, imgPipeline FilePipeline, name string) *CopyFile {
	p := &CopyFile{
		Base:        NewBase(name, buildPipeline),
		imgPipeline: imgPipeline,
		filename:    imgPipeline.Filename(),
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *CopyFile) Filename() string {
	return p.filename
}

func (p *CopyFile) SetFilename(filename string) {
	p.filename = filename
}

func (p *CopyFile) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	inputName := "image"
	copyOpts := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/%s", inputName, p.imgPipeline.Filename()),
				To:   fmt.Sprintf("tree:///%s", p.filename),
			},
		},
	}
	copyInputs := osbuild.NewPipelineTreeInputs(inputName, p.imgPipeline.Name())
	pipeline.AddStage(osbuild.NewCopyStageSimple(copyOpts, copyInputs))

	return pipeline, nil
}

func (p *CopyFile) getBuildPackages(Distro) ([]string, error) {
	return nil, nil
}

func (p *CopyFile) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename(), nil)
}
