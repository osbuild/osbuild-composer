package image

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/environment"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

type OSTreeContainer struct {
	Base
	Platform               platform.Platform
	OSCustomizations       manifest.OSCustomizations
	Environment            environment.Environment
	Workload               workload.Workload
	OSTreeParent           manifest.OSTree
	OSTreeRef              string // TODO: merge into the above
	OSVersion              string
	ExtraContainerPackages rpmmd.PackageSet
	ContainerLanguage      string
	Filename               string
}

func NewOSTreeContainer() *OSTreeContainer {
	return &OSTreeContainer{
		Base: NewBase("ostree-container"),
	}
}

func (img *OSTreeContainer) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos)
	buildPipeline.Checkpoint()

	osPipeline := manifest.NewOS(m, buildPipeline, img.Platform, repos)
	osPipeline.OSCustomizations = img.OSCustomizations
	osPipeline.Environment = img.Environment
	osPipeline.Workload = img.Workload
	osPipeline.OSTree = &img.OSTreeParent

	commitPipeline := manifest.NewOSTreeCommit(m, buildPipeline, osPipeline, img.OSTreeRef)
	commitPipeline.OSVersion = img.OSVersion

	nginxConfigPath := "/etc/nginx.conf"
	listenPort := "8080"

	serverPipeline := manifest.NewOSTreeCommitServer(m,
		buildPipeline,
		img.Platform,
		repos,
		commitPipeline,
		nginxConfigPath,
		listenPort)
	serverPipeline.Language = img.ContainerLanguage

	containerPipeline := manifest.NewOCIContainer(m, buildPipeline, serverPipeline)
	containerPipeline.Cmd = []string{"nginx", "-c", nginxConfigPath}
	containerPipeline.ExposedPorts = []string{listenPort}
	containerPipeline.Filename = img.Filename
	artifact := containerPipeline.Export()

	return artifact, nil
}
