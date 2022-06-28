package manifest

import (
	"encoding/json"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type osTreeCommit struct {
	checksum string
	url      string
}

// An OSBuildManifest is an opaque JSON object, which is a valid input to osbuild
// TODO: use this instead of distro.Manifest below
type OSBuildManifest []byte

type Manifest struct {
	pipelines     []Pipeline
	packageSpecs  []rpmmd.PackageSpec
	osTreeCommits []osTreeCommit
	inlineData    []string
}

func New() Manifest {
	return Manifest{
		pipelines:     make([]Pipeline, 0),
		packageSpecs:  make([]rpmmd.PackageSpec, 0),
		osTreeCommits: make([]osTreeCommit, 0),
		inlineData:    make([]string, 0),
	}
}

func (m *Manifest) AddPipeline(p Pipeline) {
	for _, pipeline := range m.pipelines {
		if pipeline.Name() == p.Name() {
			panic("duplicate pipeline name in manifest")
		}
	}
	m.pipelines = append(m.pipelines, p)
	m.addPackages(p.getPackages())
	m.addOSTreeCommits(p.getOSTreeCommits())
	m.addInline(p.getInline())
}

func (m *Manifest) addPackages(packages []rpmmd.PackageSpec) {
	m.packageSpecs = append(m.packageSpecs, packages...)
}

func (m *Manifest) addOSTreeCommits(commits []osTreeCommit) {
	m.osTreeCommits = append(m.osTreeCommits, commits...)
}

func (m *Manifest) addInline(data []string) {
	m.inlineData = append(m.inlineData, data...)
}

func (m Manifest) Serialize() (distro.Manifest, error) {
	var commits []ostree.CommitSource
	for _, commit := range m.osTreeCommits {
		commits = []ostree.CommitSource{
			{
				Checksum: commit.checksum, URL: commit.url,
			},
		}
	}

	pipelines := make([]osbuild2.Pipeline, 0)
	for _, pipeline := range m.pipelines {
		pipelines = append(pipelines, pipeline.serialize())
	}

	return json.Marshal(
		osbuild2.Manifest{
			Version:   "2",
			Pipelines: pipelines,
			Sources:   osbuild2.GenSources(m.packageSpecs, commits, m.inlineData),
		},
	)
}
