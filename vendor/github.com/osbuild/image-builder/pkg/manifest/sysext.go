package manifest

import (
	"errors"
	"fmt"

	"github.com/osbuild/image-builder/pkg/depsolvednf"
	"github.com/osbuild/image-builder/pkg/osbuild"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/rpmmd"
)

// SysextCustomizations holds options for a sysext image. It is shared across
// the various Sysext* pipeline generators and might be used in different places.
type SysextCustomizations struct {
	PackageSet       rpmmd.PackageSet
	Paths            []string
	ExcludePaths     []string
	ExtensionRelease osbuild.OSReleaseStageOptions
	BaseRPMOptions   *osbuild.RPMStageOptions
	SELinux          string
}

// SysextTree installs packages into a tree that serves as the overlay for a
// systemd system extension.
type SysextTree struct {
	Base

	// Necessary plumbing values
	depsolveRepos  []rpmmd.RepoConfig
	depsolveResult *depsolvednf.DepsolveResult

	platform platform.Platform

	// When non-nil, this pipeline's package sets are merged with the
	// reference's sets so that shared dependencies resolve to the same
	// versions as the base OS.
	depsolveReferencePipeline Pipeline

	Customizations SysextCustomizations
}

func NewSysextTree(buildPipeline Build, platform platform.Platform, repos []rpmmd.RepoConfig, name string) *SysextTree {
	pipelineName := "sysext-" + name + "-tree"
	p := &SysextTree{
		Base:          NewBase(pipelineName, buildPipeline),
		depsolveRepos: filterRepos(repos, pipelineName),
		platform:      platform,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *SysextTree) serializeStart(inputs Inputs) error {
	if p.depsolveResult != nil {
		return errors.New("SysextTree: double call to serializeStart()")
	}

	p.depsolveResult = &inputs.Depsolved

	return nil
}

func (p *SysextTree) serializeEnd() {
	if p.depsolveResult == nil {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.depsolveResult = nil
}

func (p *SysextTree) serialize() (osbuild.Pipeline, error) {
	if p.depsolveResult == nil {
		return osbuild.Pipeline{}, fmt.Errorf("serialization not started")
	}

	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	// Do not copy the reference pipeline tree here; it causes issues with the RPM database and package selections.
	rpmStages, err := osbuild.GenRPMStagesFromTransactions(p.depsolveResult.Transactions, p.Customizations.BaseRPMOptions)
	if err != nil {
		return osbuild.Pipeline{}, err
	}
	pipeline.AddStages(rpmStages...)

	return pipeline, nil
}

func (p *SysextTree) getPackageSetChain(d Distro) ([]rpmmd.PackageSet, error) {
	pkgSet := p.Customizations.PackageSet
	if len(pkgSet.Include) == 0 && len(pkgSet.Exclude) == 0 {
		return nil, nil
	}

	if p.depsolveReferencePipeline != nil {
		refChain, err := p.depsolveReferencePipeline.getPackageSetChain(d)
		if err != nil {
			return nil, fmt.Errorf("getting reference package set chain: %w", err)
		}
		for _, refSet := range refChain {
			pkgSet = pkgSet.Append(refSet)
			pkgSet.Repositories = append(pkgSet.Repositories, refSet.Repositories...)
		}
	}

	pkgSet.Repositories = append(pkgSet.Repositories, p.depsolveRepos...)
	pkgSet.InstallWeakDeps = false
	return []rpmmd.PackageSet{pkgSet}, nil
}

// SysextPrep computes the file-level difference between a reference tree
// (typically the base OS) and an overlay tree (the SysextTree output), then
// writes the extension-release metadata. The result contains only files that
// were added or changed plus the sysext metadata.
type SysextPrep struct {
	Base

	Customizations SysextCustomizations

	name string

	filesystemReferencePipeline Pipeline
	overlayPipeline             Pipeline
}

func NewSysextPrep(
	buildPipeline Build,
	filesystemReferencePipeline Pipeline,
	overlayPipeline Pipeline,
	name string,
) *SysextPrep {
	p := &SysextPrep{
		Base:                        NewBase("sysext-"+name+"-prep", buildPipeline),
		name:                        name,
		filesystemReferencePipeline: filesystemReferencePipeline,
		overlayPipeline:             overlayPipeline,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *SysextPrep) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	opts := &osbuild.TreeDeltaStageOptions{
		Paths:        p.Customizations.Paths,
		ExcludePaths: p.Customizations.ExcludePaths,
	}
	pipeline.AddStage(osbuild.NewTreeDeltaStage(opts, p.filesystemReferencePipeline.Name(), p.overlayPipeline.Name()))

	pipeline = p.writeExtensionRelease(pipeline)

	if p.Customizations.SELinux != "" {
		inputName := "tree"
		fileContextsPath := fmt.Sprintf("input://%s/etc/selinux/%s/contexts/files/file_contexts",
			inputName, p.Customizations.SELinux)
		selinuxStage := osbuild.NewSELinuxStage(&osbuild.SELinuxStageOptions{
			FileContexts: fileContextsPath,
		})
		selinuxStage.Inputs = osbuild.NewPipelineTreeInputs(inputName, p.filesystemReferencePipeline.Name())
		pipeline.AddStage(selinuxStage)
	}

	return pipeline, nil
}

func (p *SysextPrep) writeExtensionRelease(pipeline osbuild.Pipeline) osbuild.Pipeline {
	opts := p.Customizations.ExtensionRelease
	opts.Path = "usr/lib/extension-release.d/extension-release." + p.name
	pipeline.AddStage(osbuild.NewOSReleaseStage(&opts))
	return pipeline
}

// SysextPipelines groups the two pipelines that together produce a sysext:
// Tree (install packages), Prep (diff against the OS + extension-release metadata).
type SysextPipelines struct {
	Tree *SysextTree
	Prep *SysextPrep
}

// NewSysextPipelines creates the full Tree -> Prep chain for a named sysext.
// filesystemReferencePipeline is used by the Prep stage to compute the
// tree-delta and is always required. depsolveReferencePipeline is optional:
// when non-nil, its package sets are merged into the tree's depsolve so that
// shared dependencies resolve to the same versions as the base OS.
func NewSysextPipelines(buildPipeline Build, platform platform.Platform, repos []rpmmd.RepoConfig, filesystemReferencePipeline Pipeline, depsolveReferencePipeline Pipeline, name string) SysextPipelines {
	tree := NewSysextTree(buildPipeline, platform, repos, name)
	tree.depsolveReferencePipeline = depsolveReferencePipeline
	prep := NewSysextPrep(buildPipeline, filesystemReferencePipeline, tree, name)
	return SysextPipelines{
		Tree: tree,
		Prep: prep,
	}
}
