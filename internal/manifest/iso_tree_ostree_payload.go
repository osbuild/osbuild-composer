package manifest

import "github.com/osbuild/osbuild-composer/internal/osbuild2"

const ostreeRepoPath = "/ostree/repo"

type osTreeISOTreePayload struct {
	osTreeURL    string
	osTreeRef    string
	osTreeCommit string
	osName       string
}

func NewOSTreeISOTreePayload(osTreeURL, osTreeRef, osTreeCommit, osName string) ISOTreePayload {
	return osTreeISOTreePayload{
		osTreeURL:    osTreeURL,
		osTreeRef:    osTreeRef,
		osTreeCommit: osTreeCommit,
		osName:       osName,
	}
}

func (o osTreeISOTreePayload) getBuildPackages() []string {
	return []string{"rpm-ostree"}
}

func (o osTreeISOTreePayload) getImageURL() string {
	return ""
}

func (o osTreeISOTreePayload) getOSTreeURL() string {
	return o.osTreeURL
}

func (o osTreeISOTreePayload) getOSTreeRef() string {
	return o.osTreeRef
}

func (o osTreeISOTreePayload) getOSTreeURLForKickstart() string {
	return makeISORootPath(ostreeRepoPath)
}

func (o osTreeISOTreePayload) getOSTreeCommits() []osTreeCommit {
	return []osTreeCommit{
		{
			checksum: o.osTreeCommit,
			url:      o.osTreeURL,
		},
	}
}

func (o osTreeISOTreePayload) getOSName() string {
	return o.osName
}

func (o osTreeISOTreePayload) getPayloadStages() []*osbuild2.Stage {
	return []*osbuild2.Stage{
		osbuild2.NewOSTreeInitStage(&osbuild2.OSTreeInitStageOptions{Path: ostreeRepoPath}),
		osbuild2.NewOSTreePullStage(
			&osbuild2.OSTreePullStageOptions{Repo: ostreeRepoPath},
			osbuild2.NewOstreePullStageInputs("org.osbuild.source", o.osTreeCommit, o.osTreeRef),
		),
	}
}
