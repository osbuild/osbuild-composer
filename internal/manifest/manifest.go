// Package manifest is used to define an osbuild manifest as a series of
// pipelines with content. Typically, a Manifest is created using
// manifest.New() and pipelines are defined and added to it using the pipeline
// constructors (e.g., NewBuild()) with the manifest as the first argument. The
// pipelines are added in the order they are called.
//
// The package implements a standard set of osbuild pipelines. A pipeline
// conceptually represents a named filesystem tree, optionally generated
// in a provided build root (represented by another pipeline). All inputs
// to a pipeline must be explicitly specified, either in terms of another
// pipeline, in terms of content addressable inputs or in terms of static
// parameters to the inherited Pipeline structs.
package manifest

import (
	"encoding/json"

	"github.com/osbuild/osbuild-composer/internal/container"
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

// An OSBuildManifest is an opaque JSON object, which is a valid input to osbuild
type OSBuildManifest []byte

func (m OSBuildManifest) MarshalJSON() ([]byte, error) {
	return json.RawMessage(m).MarshalJSON()
}

func (m *OSBuildManifest) UnmarshalJSON(payload []byte) error {
	var raw json.RawMessage
	err := (&raw).UnmarshalJSON(payload)
	if err != nil {
		return err
	}
	*m = OSBuildManifest(raw)
	return nil
}

// Manifest represents a manifest initialised with all the information required
// to generate the pipelines but no content. The content included in the
// Content field must be resolved before serializing.
type Manifest struct {

	// pipelines describe the build process for an image.
	pipelines []Pipeline

	// Content for the image that will be built by the Manifest. Each content
	// type should be resolved before passing to the Serialize method.
	Content Content
}

// Content for the image that will be built by the Manifest. Each content type
// should be resolved before passing to the Serialize method.
type Content struct {
	// PackageSets are sequences of package sets, each set consisting of a list
	// of package names to include and exclude and a set of repositories to use
	// for resolving. Package set sequences (chains) should be depsolved in
	// separately and the result combined. Package set sequences (chains) are
	// keyed by the name of the Pipeline that will install them.
	PackageSets map[string][]rpmmd.PackageSet

	// Containers are source specifications for containers to embed in the image.
	Containers []container.SourceSpec

	// OSTreeCommits are source specifications for ostree commits to embed in
	// the image or use as parent commits when building a new one.
	OSTreeCommits []ostree.SourceSpec
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

func (m Manifest) Serialize(packageSets map[string][]rpmmd.PackageSpec) (OSBuildManifest, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	packages := make([]rpmmd.PackageSpec, 0)
	commits := make([]ostree.CommitSpec, 0)
	inline := make([]string, 0)
	containers := make([]container.Spec, 0)
	for _, pipeline := range m.pipelines {
		pipeline.serializeStart(packageSets[pipeline.Name()])
	}
	for _, pipeline := range m.pipelines {
		commits = append(commits, pipeline.getOSTreeCommits()...)
		pipelines = append(pipelines, pipeline.serialize())
		packages = append(packages, packageSets[pipeline.Name()]...)
		inline = append(inline, pipeline.getInline()...)
		containers = append(containers, pipeline.getContainerSpecs()...)
	}
	for _, pipeline := range m.pipelines {
		pipeline.serializeEnd()
	}

	return json.Marshal(
		osbuild.Manifest{
			Version:   "2",
			Pipelines: pipelines,
			Sources:   osbuild.GenSources(packages, commits, inline, containers),
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

// filterRepos returns a list of repositories that specify the given pipeline
// name in their PackageSets list in addition to any global repositories
// (global repositories are ones that do not specify any PackageSets).
func filterRepos(repos []rpmmd.RepoConfig, plName string) []rpmmd.RepoConfig {
	filtered := make([]rpmmd.RepoConfig, 0, len(repos))
	for _, repo := range repos {
		if len(repo.PackageSets) == 0 {
			filtered = append(filtered, repo)
			continue
		}
		for _, ps := range repo.PackageSets {
			if ps == plName {
				filtered = append(filtered, repo)
				continue
			}
		}
	}
	return filtered
}
