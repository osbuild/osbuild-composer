package manifest

import (
	"errors"
	"fmt"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

// A Build represents the build environment for other pipelines. As a
// general rule, tools required to build pipelines are used from the build
// environment, rather than from the pipeline itself. Without a specified
// build environment, the build host's root filesystem would be used, which
// is not predictable nor reproducible. For the purposes of building the
// build pipeline, we do use the build host's filesystem, this means we should
// make minimal assumptions about what's available there.
//
// Any pipeline that implements the "Build" interface will be returned
// from "Manifest.BuildPipeline()"
type Build interface {
	Name() string
	Checkpoint()
	Manifest() *Manifest

	addDependent(dep Pipeline)
}

type BuildrootFromPackages struct {
	Base

	runner     runner.Runner
	dependents []Pipeline

	// depsolveRepos holds the repository configuration used by
	// getPackageSetChain() for depsolving. After depsolving, use
	// depsolveResult.Repos which contains only repos that provided packages.
	depsolveRepos  []rpmmd.RepoConfig
	depsolveResult *depsolvednf.DepsolveResult

	containerBuildable bool

	// XXX: disableSelinux controlls if selinux should be disabled
	// for the given buildroot. This is currently needed for
	// bootstraped buildroot containers because most of the
	// boostrap containers do not include setfiles(8). A
	// reasonable fix is to teach osbuild to chroot into the
	// buildroot itself when running setfiles. Once osbuild has
	// this then this option would become "useChrootSetfiles"
	disableSelinux bool

	selinuxPolicy string

	rpmStageIgnoreGPGImportFailures bool
}

type BuildOptions struct {
	// ContainerBuildable tweaks the buildroot to be container friendly,
	// i.e. to not rely on an installed osbuild-selinux
	ContainerBuildable bool

	// DisableSELinux disables SELinux, this is not advised, but is
	// currently needed when using (experimental) cross-arch building.
	DisableSELinux bool

	// The SELinux policy to use in the buildroot, defaults to 'targeted' if not specified
	SELinuxPolicy string

	// BootstrapPipeline add the given bootstrap pipeline to the
	// build pipeline. This is only needed when doing cross-arch
	// building
	BootstrapPipeline Build

	// In some cases we have multiple build pipelines
	PipelineName string

	// Copy in files from other pipeline
	CopyFilesFrom map[string][]string

	// Ensure directories exist
	EnsureDirs []*fsnode.Directory

	// Ignore gpg import failures
	RPMStageIgnoreGPGImportFailures bool
}

// policy or default returns the selinuxPolicy or (if unset) the
// default policy
func policyOrDefault(selinuxPolicy string) string {
	if selinuxPolicy != "" {
		return selinuxPolicy
	}
	return "targeted"
}

// NewBuild creates a new build pipeline from the repositories in repos
// and the specified packages.
func NewBuild(m *Manifest, runner runner.Runner, repos []rpmmd.RepoConfig, opts *BuildOptions) Build {
	if opts == nil {
		opts = &BuildOptions{}
	}

	name := "build"
	if opts.PipelineName != "" {
		name = opts.PipelineName
	}
	pipeline := &BuildrootFromPackages{
		Base:                            NewBase(name, opts.BootstrapPipeline),
		runner:                          runner,
		dependents:                      make([]Pipeline, 0),
		depsolveRepos:                   filterRepos(repos, name),
		containerBuildable:              opts.ContainerBuildable,
		disableSelinux:                  opts.DisableSELinux,
		selinuxPolicy:                   policyOrDefault(opts.SELinuxPolicy),
		rpmStageIgnoreGPGImportFailures: opts.RPMStageIgnoreGPGImportFailures,
	}

	m.addPipeline(pipeline)
	return pipeline
}

func (p *BuildrootFromPackages) addDependent(dep Pipeline) {
	p.dependents = append(p.dependents, dep)
	man := p.Manifest()
	if man == nil {
		panic("cannot add build dependent without a manifest")
	}
	man.addPipeline(dep)
}

func (p *BuildrootFromPackages) getPackageSetChain(distro Distro) ([]rpmmd.PackageSet, error) {
	// TODO: make the /usr/bin/cp dependency conditional
	// TODO: make the /usr/bin/xz dependency conditional
	policyPackage := fmt.Sprintf("selinux-policy-%s", p.selinuxPolicy)
	packages := []string{
		policyPackage, // needed to build the build pipeline
		"coreutils",   // /usr/bin/cp - used all over
		"xz",          // usage unclear
	}

	packages = append(packages, p.runner.GetBuildPackages()...)

	for _, pipeline := range p.dependents {
		pipelineBuildPackages, err := pipeline.getBuildPackages(distro)
		if err != nil {
			return nil, fmt.Errorf("cannot get build packages for %s: %w", distro, err)
		}
		packages = append(packages, pipelineBuildPackages...)
	}

	return []rpmmd.PackageSet{
		{
			Include:         packages,
			Repositories:    p.depsolveRepos,
			InstallWeakDeps: true,
		},
	}, nil
}

func (p *BuildrootFromPackages) getPackageSpecs() rpmmd.PackageList {
	if p.depsolveResult == nil {
		return nil
	}
	return p.depsolveResult.Transactions.AllPackages()
}

func (p *BuildrootFromPackages) serializeStart(inputs Inputs) error {
	if p.depsolveResult != nil {
		return errors.New("BuildrootFromPackages: double call to serializeStart()")
	}
	p.depsolveResult = &inputs.Depsolved
	return nil
}

func (p *BuildrootFromPackages) serializeEnd() {
	if p.depsolveResult == nil {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.depsolveResult = nil
}

func (p *BuildrootFromPackages) serialize() (osbuild.Pipeline, error) {
	if p.depsolveResult == nil {
		return osbuild.Pipeline{}, fmt.Errorf("BuildrootFromPackages: serialization not started")
	}
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	pipeline.Runner = p.runner.String()

	baseOptions := osbuild.RPMStageOptions{}
	if p.rpmStageIgnoreGPGImportFailures {
		baseOptions.RPMKeys = &osbuild.RPMKeys{
			IgnoreImportFailures: true,
		}
	}
	rpmStages, err := osbuild.GenRPMStagesFromTransactions(p.depsolveResult.Transactions, &baseOptions)
	if err != nil {
		return osbuild.Pipeline{}, err
	}
	pipeline.AddStages(rpmStages...)

	if !p.disableSelinux {
		pipeline.AddStage(osbuild.NewSELinuxStage(&osbuild.SELinuxStageOptions{
			FileContexts: fmt.Sprintf("etc/selinux/%s/contexts/files/file_contexts", p.selinuxPolicy),
			Labels:       p.getSELinuxLabels(),
		},
		))
	}

	return pipeline, nil
}

// Returns a map of paths to labels for the SELinux stage based on specific
// packages found in the pipeline.
func (p *BuildrootFromPackages) getSELinuxLabels() map[string]string {
	labels := make(map[string]string)
	for _, pkg := range p.getPackageSpecs() {
		switch pkg.Name {
		case "coreutils":
			labels["/usr/bin/cp"] = "system_u:object_r:install_exec_t:s0"
			if p.containerBuildable {
				labels["/usr/bin/mount"] = "system_u:object_r:install_exec_t:s0"
				labels["/usr/bin/umount"] = "system_u:object_r:install_exec_t:s0"
			}
		case "tar":
			labels["/usr/bin/tar"] = "system_u:object_r:install_exec_t:s0"
		}
	}
	return labels
}

type BuildrootFromContainer struct {
	Base

	runner     runner.Runner
	dependents []Pipeline

	containers     []container.SourceSpec
	containerSpecs []container.Spec

	containerBuildable bool
	disableSelinux     bool
	selinuxPolicy      string

	copyFilesFrom map[string][]string
	ensureDirs    []*fsnode.Directory
}

// NewBuildFromContainer creates a new build pipeline from the given
// containers specs
func NewBuildFromContainer(m *Manifest, runner runner.Runner, containerSources []container.SourceSpec, opts *BuildOptions) Build {
	if opts == nil {
		opts = &BuildOptions{}
	}

	name := "build"
	if opts.PipelineName != "" {
		name = opts.PipelineName
	}
	pipeline := &BuildrootFromContainer{
		Base:       NewBase(name, opts.BootstrapPipeline),
		runner:     runner,
		dependents: make([]Pipeline, 0),
		containers: containerSources,

		containerBuildable: opts.ContainerBuildable,
		disableSelinux:     opts.DisableSELinux,
		selinuxPolicy:      policyOrDefault(opts.SELinuxPolicy),

		copyFilesFrom: opts.CopyFilesFrom,
		ensureDirs:    opts.EnsureDirs,
	}
	m.addPipeline(pipeline)
	return pipeline
}

func (p *BuildrootFromContainer) addDependent(dep Pipeline) {
	p.dependents = append(p.dependents, dep)
	man := p.Manifest()
	if man == nil {
		panic("cannot add build dependent without a manifest")
	}
	man.addPipeline(dep)
}

func (p *BuildrootFromContainer) getContainerSources() []container.SourceSpec {
	return p.containers
}

func (p *BuildrootFromContainer) getContainerSpecs() []container.Spec {
	return p.containerSpecs
}

func (p *BuildrootFromContainer) serializeStart(inputs Inputs) error {
	if len(p.containerSpecs) > 0 {
		return errors.New("BuildrootFromContainer: double call to serializeStart()")
	}
	p.containerSpecs = inputs.Containers
	return nil
}

func (p *BuildrootFromContainer) serializeEnd() {
	if len(p.containerSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.containerSpecs = nil
}

func (p *BuildrootFromContainer) getSELinuxLabels() map[string]string {
	if p.disableSelinux {
		return nil
	}

	labels := map[string]string{
		"/usr/bin/ostree": "system_u:object_r:install_exec_t:s0",
	}
	if p.containerBuildable {
		labels["/usr/bin/mount"] = "system_u:object_r:install_exec_t:s0"
		labels["/usr/bin/umount"] = "system_u:object_r:install_exec_t:s0"
	}
	return labels
}

func (p *BuildrootFromContainer) serialize() (osbuild.Pipeline, error) {
	if len(p.containerSpecs) == 0 {
		return osbuild.Pipeline{}, fmt.Errorf("BuildrootFromContainer: serialization not started")
	}
	if len(p.containerSpecs) != 1 {
		return osbuild.Pipeline{}, fmt.Errorf("BuildrootFromContainer expectes exactly one container input, got: %v", p.containerSpecs)
	}

	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	pipeline.Runner = p.runner.String()

	image := osbuild.NewContainersInputForSingleSource(p.containerSpecs[0])
	// Make skopeo copy to remove the signatures of signed containers by default to workaround
	// build failures until https://github.com/containers/image/issues/2599 is implemented
	stage, err := osbuild.NewContainerDeployStage(image, &osbuild.ContainerDeployOptions{RemoveSignatures: true})
	if err != nil {
		return osbuild.Pipeline{}, err
	}
	pipeline.AddStage(stage)

	for _, stage := range osbuild.GenDirectoryNodesStages(p.ensureDirs) {
		pipeline.AddStage(stage)
	}

	for copyFilesFrom, copyFiles := range p.copyFilesFrom {
		inputName := "copy-tree"
		paths := []osbuild.CopyStagePath{}
		for _, copyPath := range copyFiles {
			paths = append(paths, osbuild.CopyStagePath{
				From: fmt.Sprintf("input://%s%s", inputName, copyPath),
				To:   fmt.Sprintf("tree://%s", copyPath),
			})
		}

		pipeline.AddStage(osbuild.NewCopyStageSimple(
			&osbuild.CopyStageOptions{Paths: paths},
			osbuild.NewPipelineTreeInputs(inputName, copyFilesFrom),
		))
	}

	if !p.disableSelinux {
		pipeline.AddStage(osbuild.NewSELinuxStage(
			&osbuild.SELinuxStageOptions{
				FileContexts: fmt.Sprintf("etc/selinux/%s/contexts/files/file_contexts", p.selinuxPolicy),
				ExcludePaths: []string{"/sysroot"},
				Labels:       p.getSELinuxLabels(),
			},
		))
	}

	return pipeline, nil
}

// NewBootstrap creates a new bootstrap build pipeline from the given
// containers specs
func NewBootstrap(m *Manifest, containerSources []container.SourceSpec) Build {
	name := "bootstrap-buildroot"
	pipeline := &BuildrootFromContainer{
		Base: NewBase(name, nil),
		// use the most minimal runner as we cannot make assumption
		// about our environment
		runner:             &runner.Linux{},
		dependents:         make([]Pipeline, 0),
		containers:         containerSources,
		containerBuildable: true,
		// XXX: we only disable selinux currently in the buildroot
		// because selinux requies the bootstrap container to have the
		// "setfiles" binary. this is typcially not shiped in a
		// container so we disable it. the easiest fix is to make
		// osbuld support running setfiles inside the buildroot chroot
		// (which has setfiles installed)
		disableSelinux: true,
	}
	m.addPipeline(pipeline)
	return pipeline
}
