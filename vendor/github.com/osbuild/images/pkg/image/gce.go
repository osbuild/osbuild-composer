package image

import (
	"fmt"
	"regexp"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
)

func newGCETarPipelineForImg(buildPipeline manifest.Build, inputPipeline manifest.FilePipeline, pipelinename string) *manifest.Tar {
	tarPipeline := manifest.NewTar(buildPipeline, inputPipeline, pipelinename)
	tarPipeline.Format = osbuild.TarArchiveFormatOldgnu
	tarPipeline.RootNode = osbuild.TarRootNodeOmit
	// these are required to successfully import the image to GCP
	tarPipeline.ACLs = common.ToPtr(false)
	tarPipeline.SELinux = common.ToPtr(false)
	tarPipeline.Xattrs = common.ToPtr(false)
	if inputPipeline.Filename() != "disk.raw" {
		tarPipeline.Transform = fmt.Sprintf(`s/%s/disk.raw/`, regexp.QuoteMeta(inputPipeline.Filename()))
	}
	return tarPipeline
}
