package manifest

import (
	"encoding/json"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type Arch uint64

const (
	ARCH_X86_64 Arch = iota
	ARCH_AARCH64
	ARCH_S390X
	ARCH_PPC64LE
)

type osTreeCommit struct {
	checksum string
	url      string
}

// An OSBuildManifest is an opaque JSON object, which is a valid input to osbuild
// TODO: use this instead of distro.Manifest below
type OSBuildManifest []byte

type Manifest struct {
	pipelines []Pipeline
}

func New() Manifest {
	return Manifest{
		pipelines: make([]Pipeline, 0),
	}
}

func (m *Manifest) addPipeline(p Pipeline) {
	for _, pipeline := range m.pipelines {
		if pipeline.Name() == p.Name() {
			panic("duplicate pipeline name in manifest")
		}
	}
	m.pipelines = append(m.pipelines, p)
}

func (m Manifest) GetPackageSetChains() map[string][]rpmmd.PackageSet {
	chains := make(map[string][]rpmmd.PackageSet)

	for _, pipeline := range m.pipelines {
		if chain := pipeline.getPackageSetChain(); chain != nil {
			chains[pipeline.Name()] = chain
		}
	}

	return chains
}

func (m Manifest) Serialize(packageSets map[string][]rpmmd.PackageSpec) (distro.Manifest, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	packages := make([]rpmmd.PackageSpec, 0)
	commits := make([]ostree.CommitSpec, 0)
	inline := make([]string, 0)
	for _, pipeline := range m.pipelines {
		pipeline.serializeStart(packageSets[pipeline.Name()])
	}
	for _, pipeline := range m.pipelines {
		for _, commit := range pipeline.getOSTreeCommits() {
			commits = append(commits, ostree.CommitSpec{
				Checksum: commit.checksum, URL: commit.url,
			})
		}
		pipelines = append(pipelines, pipeline.serialize())
		packages = append(packages, packageSets[pipeline.Name()]...)
		inline = append(inline, pipeline.getInline()...)
	}
	for _, pipeline := range m.pipelines {
		pipeline.serializeEnd()
	}

	return json.Marshal(
		osbuild.Manifest{
			Version:   "2",
			Pipelines: pipelines,
			Sources:   osbuild.GenSources(packages, commits, inline, nil),
		},
	)
}

func (m Manifest) GetCheckpoints() []string {
	checkpoints := []string{}
	for _, p := range m.pipelines {
		if p.getCheckpoint() {
			checkpoints = append(checkpoints, p.Name())
		}
	}
	return checkpoints
}

func (m Manifest) GetExports() []string {
	exports := []string{}
	for _, p := range m.pipelines {
		if p.getExport() {
			exports = append(exports, p.Name())
		}
	}
	return exports
}
