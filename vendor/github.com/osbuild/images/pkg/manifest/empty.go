package manifest

import (
	"errors"
	"fmt"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rpmmd"
)

// A ContentTest can be used to define content sources without generating
// pipelines. It is useful for testing but not much else.
type ContentTest struct {
	Base

	// content sources
	packageSets []rpmmd.PackageSet
	containers  []container.SourceSpec
	commits     []ostree.SourceSpec

	// resolved content
	packageSpecs   []rpmmd.PackageSpec
	containerSpecs []container.Spec
	commitSpecs    []ostree.CommitSpec

	repos []rpmmd.RepoConfig

	// serialization flag
	serializing bool
}

// NewContentTest creates a new ContentTest pipeline with a given name and
// content sources.
func NewContentTest(m *Manifest, name string, packageSets []rpmmd.PackageSet, containers []container.SourceSpec, commits []ostree.SourceSpec) *ContentTest {
	pipeline := &ContentTest{
		Base:        NewBase(name, nil),
		packageSets: packageSets,
		containers:  containers,
		commits:     commits,
	}
	m.addPipeline(pipeline)
	return pipeline
}

func (p *ContentTest) getPackageSetChain(Distro) ([]rpmmd.PackageSet, error) {
	return p.packageSets, nil
}

func (p *ContentTest) getContainerSources() []container.SourceSpec {
	return p.containers
}

func (p *ContentTest) getOSTreeCommitSources() []ostree.SourceSpec {
	return p.commits
}

func (p *ContentTest) getPackageSpecs() []rpmmd.PackageSpec {
	return p.packageSpecs
}

func (p *ContentTest) getContainerSpecs() []container.Spec {
	return p.containerSpecs
}

func (p *ContentTest) getOSTreeCommits() []ostree.CommitSpec {
	return p.commitSpecs
}

func (p *ContentTest) serializeStart(inputs Inputs) error {
	if p.serializing {
		return errors.New("ContentTest: double call to serializeStart()")
	}
	p.packageSpecs = inputs.Depsolved.Packages
	p.containerSpecs = inputs.Containers
	p.commitSpecs = inputs.Commits
	p.repos = inputs.Depsolved.Repos

	p.serializing = true
	return nil
}

func (p *ContentTest) serializeEnd() {
	if !p.serializing {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.packageSpecs = nil
	p.containerSpecs = nil
	p.commitSpecs = nil

	p.serializing = false
}

func (p *ContentTest) serialize() (osbuild.Pipeline, error) {
	if !p.serializing {
		return osbuild.Pipeline{}, fmt.Errorf("ContentTest: serialization not started")
	}

	// no stages

	return osbuild.Pipeline{
		Name: p.name,
	}, nil
}
