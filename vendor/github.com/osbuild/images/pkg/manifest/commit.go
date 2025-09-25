package manifest

import (
	"fmt"

	"github.com/osbuild/images/pkg/osbuild"
)

// OSTreeCommit represents an ostree with one commit.
type OSTreeCommit struct {
	Base
	OSVersion string

	treePipeline *OS
	ref          string
}

// NewOSTreeCommit creates a new OSTree commit pipeline. The
// treePipeline is the tree representing the content of the commit.
// ref is the ref to create the commit under.
func NewOSTreeCommit(buildPipeline Build, treePipeline *OS, ref string) *OSTreeCommit {
	p := &OSTreeCommit{
		Base:         NewBase("ostree-commit", buildPipeline),
		treePipeline: treePipeline,
		ref:          ref,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *OSTreeCommit) getBuildPackages(Distro) ([]string, error) {
	packages := []string{
		"rpm-ostree",
	}
	return packages, nil
}

func (p *OSTreeCommit) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	if p.treePipeline.OSTreeRef == "" {
		return osbuild.Pipeline{}, fmt.Errorf("tree is not ostree")
	}

	pipeline.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: "/repo"}))

	var parentID string
	treeCommits := p.treePipeline.getOSTreeCommits()
	if len(treeCommits) > 0 {
		if len(treeCommits) > 1 {
			return osbuild.Pipeline{}, fmt.Errorf("multiple ostree commit specs found; this is a programming error")
		}
		parentCommit := &treeCommits[0]
		parentID = parentCommit.Checksum
	}

	pipeline.AddStage(osbuild.NewOSTreeCommitStage(
		&osbuild.OSTreeCommitStageOptions{
			Ref:       p.ref,
			OSVersion: p.OSVersion,
			Parent:    parentID,
		},
		p.treePipeline.Name()),
	)

	return pipeline, nil
}
