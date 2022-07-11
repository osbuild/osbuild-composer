package manifest

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
)

const tarPath = "/liveimg.tar"

type tarTreeISOTreePayload struct {
	treePipelineName string
}

func NewTarISOTreePayload(treePipelineName string) ISOTreePayload {
	return tarTreeISOTreePayload{
		treePipelineName: treePipelineName,
	}
}

func (t tarTreeISOTreePayload) getBuildPackages() []string {
	return []string{"tar"}
}

func (t tarTreeISOTreePayload) getImageURL() string {
	return makeISORootPath(tarPath)
}

func (t tarTreeISOTreePayload) getOSTreeURL() string {
	return ""
}

func (t tarTreeISOTreePayload) getOSTreeRef() string {
	return ""
}

func (t tarTreeISOTreePayload) getOSTreeURLForKickstart() string {
	return ""
}

func (t tarTreeISOTreePayload) getOSTreeCommits() []osTreeCommit {
	return nil
}

func (t tarTreeISOTreePayload) getOSName() string {
	return ""
}

func (t tarTreeISOTreePayload) getPayloadStages() []*osbuild2.Stage {
	return []*osbuild2.Stage{osbuild2.NewTarStage(&osbuild2.TarStageOptions{Filename: tarPath}, osbuild2.NewTarStagePipelineTreeInputs(t.treePipelineName))}
}
