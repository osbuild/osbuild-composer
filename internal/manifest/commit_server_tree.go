package manifest

import (
	"path/filepath"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

// An OSTreeCommitServerTreePipeline contains an nginx server serving
// an embedded ostree commit.
type OSTreeCommitServerTreePipeline struct {
	BasePipeline
	// Packages to install in addition to the ones required by the
	// pipeline.
	ExtraPackages []string
	// Extra repositories to install packages from
	ExtraRepos []rpmmd.RepoConfig
	// TODO: should this be configurable?
	Language string

	repos           []rpmmd.RepoConfig
	packageSpecs    []rpmmd.PackageSpec
	commitPipeline  *OSTreeCommitPipeline
	nginxConfigPath string
	listenPort      string
}

// NewOSTreeCommitServerTreePipeline creates a new pipeline. The content
// is built from repos and packages, which must contain nginx. commitPipeline
// is a pipeline producing an ostree commit to be served. nginxConfigPath
// is the path to the main nginx config file and listenPort is the port
// nginx will be listening on.
func NewOSTreeCommitServerTreePipeline(m *Manifest,
	buildPipeline *BuildPipeline,
	repos []rpmmd.RepoConfig,
	commitPipeline *OSTreeCommitPipeline,
	nginxConfigPath,
	listenPort string) *OSTreeCommitServerTreePipeline {
	p := &OSTreeCommitServerTreePipeline{
		BasePipeline:    NewBasePipeline(m, "container-tree", buildPipeline, nil),
		repos:           repos,
		commitPipeline:  commitPipeline,
		nginxConfigPath: nginxConfigPath,
		listenPort:      listenPort,
		Language:        "en_US",
	}
	if commitPipeline.BasePipeline.manifest != m {
		panic("commit pipeline from different manifest")
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *OSTreeCommitServerTreePipeline) getPackageSetChain() []rpmmd.PackageSet {
	packages := []string{"nginx"}
	return []rpmmd.PackageSet{
		{
			Include:      append(packages, p.ExtraPackages...),
			Repositories: append(p.repos, p.ExtraRepos...),
		},
	}
}

func (p *OSTreeCommitServerTreePipeline) getBuildPackages() []string {
	packages := []string{
		"rpm-ostree",
	}
	return packages
}

func (p *OSTreeCommitServerTreePipeline) getPackageSpecs() []rpmmd.PackageSpec {
	return p.packageSpecs
}

func (p *OSTreeCommitServerTreePipeline) serializeStart(packages []rpmmd.PackageSpec) {
	if len(p.packageSpecs) > 0 {
		panic("double call to serializeStart()")
	}
	p.packageSpecs = packages
}

func (p *OSTreeCommitServerTreePipeline) serializeEnd() {
	if len(p.packageSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.packageSpecs = nil
}

func (p *OSTreeCommitServerTreePipeline) serialize() osbuild2.Pipeline {
	if len(p.packageSpecs) == 0 {
		panic("serialization not started")
	}
	pipeline := p.BasePipeline.serialize()

	pipeline.AddStage(osbuild2.NewRPMStage(osbuild2.NewRPMStageOptions(p.repos), osbuild2.NewRpmStageSourceFilesInputs(p.packageSpecs)))
	pipeline.AddStage(osbuild2.NewLocaleStage(&osbuild2.LocaleStageOptions{Language: p.Language}))

	htmlRoot := "/usr/share/nginx/html"
	repoPath := filepath.Join(htmlRoot, "repo")
	pipeline.AddStage(osbuild2.NewOSTreeInitStage(&osbuild2.OSTreeInitStageOptions{Path: repoPath}))

	pipeline.AddStage(osbuild2.NewOSTreePullStage(
		&osbuild2.OSTreePullStageOptions{Repo: repoPath},
		osbuild2.NewOstreePullStageInputs("org.osbuild.pipeline", "name:"+p.commitPipeline.Name(), p.commitPipeline.ref),
	))

	// make nginx log and lib directories world writeable, otherwise nginx can't start in
	// an unprivileged container
	pipeline.AddStage(osbuild2.NewChmodStage(chmodStageOptions("/var/log/nginx", "a+rwX", true)))
	pipeline.AddStage(osbuild2.NewChmodStage(chmodStageOptions("/var/lib/nginx", "a+rwX", true)))

	pipeline.AddStage(osbuild2.NewNginxConfigStage(nginxConfigStageOptions(p.nginxConfigPath, htmlRoot, p.listenPort)))

	return pipeline
}

func nginxConfigStageOptions(path, htmlRoot, listen string) *osbuild2.NginxConfigStageOptions {
	// configure nginx to work in an unprivileged container
	cfg := &osbuild2.NginxConfig{
		Listen: listen,
		Root:   htmlRoot,
		Daemon: common.BoolToPtr(false),
		PID:    "/tmp/nginx.pid",
	}
	return &osbuild2.NginxConfigStageOptions{
		Path:   path,
		Config: cfg,
	}
}

func chmodStageOptions(path, mode string, recursive bool) *osbuild2.ChmodStageOptions {
	return &osbuild2.ChmodStageOptions{
		Items: map[string]osbuild2.ChmodStagePathOptions{
			path: {Mode: mode, Recursive: recursive},
		},
	}
}
