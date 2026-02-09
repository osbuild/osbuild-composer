package manifest

import (
	"errors"
	"fmt"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/depsolvednf"
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

	depsolveResult *depsolvednf.DepsolveResult
	containerSpecs []container.Spec
	commitSpecs    []ostree.CommitSpec

	// serialization flag
	serializing bool
}

var _ Pipeline = (*ContentTest)(nil)

type ContentTestBuild struct {
	ContentTest

	dependents []Pipeline
}

var _ Build = (*ContentTestBuild)(nil)

// NewContentTest creates a new ContentTest pipeline with a given name and
// content sources.
func NewContentTest(name string, build Build, packageSets []rpmmd.PackageSet, containers []container.SourceSpec, commits []ostree.SourceSpec) *ContentTest {
	pipeline := &ContentTest{
		Base:        NewBase(name, build),
		packageSets: packageSets,
		containers:  containers,
		commits:     commits,
	}
	build.addDependent(pipeline)
	return pipeline
}

func NewContentTestBuild(m *Manifest, packageSets []rpmmd.PackageSet, containers []container.SourceSpec, commits []ostree.SourceSpec) Build {
	pipeline := &ContentTestBuild{
		ContentTest: ContentTest{
			Base:        NewBase("build", nil),
			packageSets: packageSets,
			containers:  containers,
			commits:     commits,
		},
		dependents: make([]Pipeline, 0),
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

func (p *ContentTest) getPackageSpecs() rpmmd.PackageList {
	if p.depsolveResult == nil {
		return nil
	}
	return p.depsolveResult.Transactions.AllPackages()
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
	p.depsolveResult = &inputs.Depsolved
	p.containerSpecs = inputs.Containers
	p.commitSpecs = inputs.Commits

	p.serializing = true
	return nil
}

func (p *ContentTest) serializeEnd() {
	if !p.serializing {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.depsolveResult = nil
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

func (p *ContentTestBuild) addDependent(dep Pipeline) {
	p.dependents = append(p.dependents, dep)
	man := p.Manifest()
	if man == nil {
		panic("cannot add build dependent without a manifest")
	}
	man.addPipeline(dep)
}
