package manifestgen

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
)

const (
	defaultDepsolverSBOMType = sbom.StandardTypeSpdx
	defaultSBOMExt           = "spdx.json"

	defaultDepsolveCacheDir = "osbuild-depsolve-dnf"
)

var (
	ErrContainerArchMismatch = errors.New("requested container architecture does not match resolved container")
)

// Options contains the optional settings for the manifest generation.
// For unset values defaults will be used.
type Options struct {
	// Cachedir specified the rpmmd cache directory (rpm metadata)
	// If unset a default based on the xdgCacheHome is used
	// (e.g. /root/.cache/osbuild-store)
	Cachedir string

	// RpmDownloader allows to switch between the librepo and
	// libcurl download backends.
	RpmDownloader osbuild.RpmDownloader

	// SBOMWriter will be called for each generated SBOM the
	// filename contains the suggest filename string and the
	// content can be read
	SBOMWriter SBOMWriterFunc

	// WarningsOutput will receive any warnings that are part of
	// the manifest generation. If it is unset any warnings will
	// generate an error.
	WarningsOutput io.Writer

	// DepsolveWarningsOutput will receive any warnings that are
	// part of the depsolving step. If it is unset output ends up
	// on the default stdout/stderr.
	DepsolveWarningsOutput io.Writer

	// CustomSeed overrides the default rng seed, this is mostly
	// useful for testing
	CustomSeed *int64

	// OverrideRepos overrides the default repository selection.
	// This is mostly useful for testing
	OverrideRepos []rpmmd.RepoConfig

	// Custom "solver" functions, if unset the defaults will be
	// used. Only needed for specialized use-cases.
	Depsolve          DepsolveFunc
	ContainerResolver ContainerResolverFunc
	CommitResolver    CommitResolverFunc

	// Use the a bootstrap container to buildroot (useful for e.g.
	// cross-arch or cross-distro builds)
	UseBootstrapContainer bool
}

// Generator can generate an osbuild manifest from a given repository
// and options.
type Generator struct {
	cacheDir string

	depsolve               DepsolveFunc
	containerResolver      ContainerResolverFunc
	commitResolver         CommitResolverFunc
	sbomWriter             SBOMWriterFunc
	warningsOutput         io.Writer
	depsolveWarningsOutput io.Writer

	reporegistry *reporegistry.RepoRegistry

	rpmDownloader osbuild.RpmDownloader

	customSeed    *int64
	overrideRepos []rpmmd.RepoConfig

	useBootstrapContainer bool
}

// New will create a new manifest generator
func New(reporegistry *reporegistry.RepoRegistry, opts *Options) (*Generator, error) {
	if opts == nil {
		opts = &Options{}
	}
	mg := &Generator{
		reporegistry: reporegistry,

		cacheDir:               opts.Cachedir,
		depsolve:               opts.Depsolve,
		containerResolver:      opts.ContainerResolver,
		commitResolver:         opts.CommitResolver,
		rpmDownloader:          opts.RpmDownloader,
		sbomWriter:             opts.SBOMWriter,
		warningsOutput:         opts.WarningsOutput,
		depsolveWarningsOutput: opts.DepsolveWarningsOutput,
		customSeed:             opts.CustomSeed,
		overrideRepos:          opts.OverrideRepos,
		useBootstrapContainer:  opts.UseBootstrapContainer,
	}
	if mg.depsolve == nil {
		mg.depsolve = DefaultDepsolve
	}
	if mg.containerResolver == nil {
		mg.containerResolver = DefaultContainerResolver
	}
	if mg.commitResolver == nil {
		mg.commitResolver = DefaultCommitResolver
	}
	if mg.cacheDir == "" {
		xdgCacheHomeDir, err := xdgCacheHome()
		if err != nil {
			return nil, err
		}
		mg.cacheDir = filepath.Join(xdgCacheHomeDir, defaultDepsolveCacheDir)
	}

	return mg, nil
}

// Generate will generate a new manifest for the given distro/imageType/arch
// combination.
func (mg *Generator) Generate(bp *blueprint.Blueprint, imgType distro.ImageType, imgOpts *distro.ImageOptions) ([]byte, error) {
	if imgOpts == nil {
		imgOpts = &distro.ImageOptions{}
	}
	imgOpts.UseBootstrapContainer = mg.useBootstrapContainer
	a := imgType.Arch()
	dist := a.Distro()

	var repos []rpmmd.RepoConfig
	if mg.overrideRepos != nil {
		repos = mg.overrideRepos
	} else {
		var err error
		repos, err = mg.reporegistry.ReposByImageTypeName(dist.Name(), a.Name(), imgType.Name())
		if err != nil {
			return nil, err
		}
	}
	// To support "user" a.k.a. "3rd party" repositories, these
	// will have to be added to the repos with
	// <repo_item>.PackageSets set to the "payload" pipeline names
	// for the given image type, see e.g. distro/rhel/imagetype.go:Manifest()
	preManifest, warnings, err := imgType.Manifest(bp, *imgOpts, repos, mg.customSeed)
	if err != nil {
		return nil, err
	}
	if len(warnings) > 0 {
		warn := strings.Join(warnings, "\n")
		if mg.warningsOutput != nil {
			fmt.Fprint(mg.warningsOutput, warn)
		} else {
			return nil, fmt.Errorf("Warnings during manifest creation:\n%v", warn)
		}
	}
	pkgSetChains, err := preManifest.GetPackageSetChains()
	if err != nil {
		return nil, err
	}
	solver := depsolvednf.NewSolver(dist.ModulePlatformID(), dist.Releasever(), a.Name(), dist.Name(), mg.cacheDir)
	if dd, ok := dist.(distro.CustomDepsolverDistro); ok {
		// XXX: it would be nice to have access to arch.Arch
		// from distro.Arch but we dont so we have to do without.
		archi := common.Must(arch.FromString(a.Name()))
		customSolver, cleanupFunc, err := dd.Depsolver(mg.cacheDir, archi)
		if err != nil {
			return nil, err
		}
		if customSolver != nil {
			solver = customSolver
		}
		defer func() {
			if err := cleanupFunc(); err != nil {
				fmt.Fprintf(mg.warningsOutput, "WARNING: cleanup failed: %v\n", err)
			}
		}()
	}
	depsolved, err := mg.depsolve(solver, mg.cacheDir, mg.depsolveWarningsOutput, pkgSetChains, dist, a.Name())
	if err != nil {
		return nil, err
	}
	containerSpecs, err := mg.containerResolver(preManifest.GetContainerSourceSpecs(), a.Name())
	if err != nil {
		return nil, err
	}
	for _, specs := range containerSpecs {
		for _, spec := range specs {
			if spec.Arch.String() != a.Name() {
				return nil, fmt.Errorf("%w: %q != %q", ErrContainerArchMismatch, spec.Arch, a.Name())
			}
		}
	}

	commitSpecs, err := mg.commitResolver(preManifest.GetOSTreeSourceSpecs())
	if err != nil {
		return nil, err
	}
	opts := &manifest.SerializeOptions{
		RpmDownloader: mg.rpmDownloader,
	}
	mf, err := preManifest.Serialize(depsolved, containerSpecs, commitSpecs, opts)
	if err != nil {
		return nil, err
	}

	if mg.sbomWriter != nil {
		// XXX: this is very similar to
		// osbuild-composer:jobimpl-osbuild.go, see if code
		// can be shared
		for plName, depsolvedPipeline := range depsolved {
			pipelinePurpose := "unknown"
			switch {
			case slices.Contains(preManifest.PayloadPipelines(), plName):
				pipelinePurpose = "image"
			case slices.Contains(preManifest.BuildPipelines(), plName):
				pipelinePurpose = "buildroot"
			}
			// XXX: sync with image-builder-cli:build.go name generation - can we have a shared helper?
			imageName := fmt.Sprintf("%s-%s-%s", dist.Name(), imgType.Name(), a.Name())
			sbomDocOutputFilename := fmt.Sprintf("%s.%s-%s.%s", imageName, pipelinePurpose, plName, defaultSBOMExt)

			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			if err := enc.Encode(depsolvedPipeline.SBOM.Document); err != nil {
				return nil, err
			}
			if err := mg.sbomWriter(sbomDocOutputFilename, &buf, depsolvedPipeline.SBOM.DocType); err != nil {
				return nil, err
			}
		}
	}

	return mf, nil
}

func xdgCacheHome() (string, error) {
	xdgCacheHome := os.Getenv("XDG_CACHE_HOME")
	if xdgCacheHome != "" {
		return xdgCacheHome, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache"), nil
}

// DefaultDepsolve provides a default implementation for depsolving.
// It should rarely be necessary to use it directly and will be used
// by default by manifestgen (unless overriden)
//
// The customSolver argument can be nil
func DefaultDepsolve(solver *depsolvednf.Solver, cacheDir string, depsolveWarningsOutput io.Writer, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]depsolvednf.DepsolveResult, error) {
	if solver == nil {
		return nil, fmt.Errorf("need a valid solver, got nil")
	}
	if depsolveWarningsOutput != nil {
		solver.Stderr = depsolveWarningsOutput
	}

	depsolvedSets := make(map[string]depsolvednf.DepsolveResult)
	for name, pkgSet := range packageSets {
		// Always generate Spdx SBOMs for now, this makes the
		// default depsolve slightly slower but it means we
		// need no extra argument here to select the SBOM
		// type. Once we have more types than Spdx of course
		// we need to add a option to select the type.
		res, err := solver.Depsolve(pkgSet, sbom.StandardTypeSpdx)
		if err != nil {
			return nil, fmt.Errorf("error depsolving: %w", err)
		}
		depsolvedSets[name] = *res
	}
	return depsolvedSets, nil
}

func resolveContainers(containers []container.SourceSpec, archName string) ([]container.Spec, error) {
	resolver := container.NewBlockingResolver(archName)

	for _, c := range containers {
		resolver.Add(c)
	}

	return resolver.Finish()
}

// DefaultContainersResolve provides a default implementation for
// container resolving.
// It should rarely be necessary to use it directly and will be used
// by default by manifestgen (unless overriden)
func DefaultContainerResolver(containerSources map[string][]container.SourceSpec, archName string) (map[string][]container.Spec, error) {
	containerSpecs := make(map[string][]container.Spec, len(containerSources))
	for plName, sourceSpecs := range containerSources {
		specs, err := resolveContainers(sourceSpecs, archName)
		if err != nil {
			return nil, fmt.Errorf("error container resolving: %w", err)
		}
		containerSpecs[plName] = specs
	}
	return containerSpecs, nil
}

// DefaultCommitResolver provides a default implementation for
// ostree commit resolving.
// It should rarely be necessary to use it directly and will be used
// by default by manifestgen (unless overriden)
func DefaultCommitResolver(commitSources map[string][]ostree.SourceSpec) (map[string][]ostree.CommitSpec, error) {
	commits := make(map[string][]ostree.CommitSpec, len(commitSources))
	for name, commitSources := range commitSources {
		commitSpecs := make([]ostree.CommitSpec, len(commitSources))
		for idx, commitSource := range commitSources {
			var err error
			commitSpecs[idx], err = ostree.Resolve(commitSource)
			if err != nil {
				return nil, fmt.Errorf("error ostree commit resolving: %w", err)
			}
		}
		commits[name] = commitSpecs
	}
	return commits, nil
}

type (
	DepsolveFunc func(solver *depsolvednf.Solver, cacheDir string, depsolveWarningsOutput io.Writer, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]depsolvednf.DepsolveResult, error)

	ContainerResolverFunc func(containerSources map[string][]container.SourceSpec, archName string) (map[string][]container.Spec, error)

	CommitResolverFunc func(commitSources map[string][]ostree.SourceSpec) (map[string][]ostree.CommitSpec, error)

	SBOMWriterFunc func(filename string, content io.Reader, docType sbom.StandardType) error
)
