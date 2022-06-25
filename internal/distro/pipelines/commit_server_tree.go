package pipeline

import (
	"path/filepath"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type OSTreeCommitServerTreePipeline struct {
	Pipeline
	commitPipeline  *OSTreeCommitPipeline
	Repos           []rpmmd.RepoConfig
	PackageSpecs    []rpmmd.PackageSpec
	Language        string
	NginxConfigPath string
	ListenPort      string
}

func NewOSTreeCommitServerTreePipeline(buildPipeline *BuildPipeline, commitPipeline *OSTreeCommitPipeline) OSTreeCommitServerTreePipeline {
	return OSTreeCommitServerTreePipeline{
		Pipeline:       New("container-tree", &buildPipeline.Pipeline),
		commitPipeline: commitPipeline,
		Language:       "en_US",
	}
}

func (p OSTreeCommitServerTreePipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	pipeline.AddStage(osbuild2.NewRPMStage(osbuild2.NewRPMStageOptions(p.Repos), osbuild2.NewRpmStageSourceFilesInputs(p.PackageSpecs)))
	pipeline.AddStage(osbuild2.NewLocaleStage(&osbuild2.LocaleStageOptions{Language: p.Language}))

	htmlRoot := "/usr/share/nginx/html"
	repoPath := filepath.Join(htmlRoot, "repo")
	pipeline.AddStage(osbuild2.NewOSTreeInitStage(&osbuild2.OSTreeInitStageOptions{Path: repoPath}))

	pipeline.AddStage(osbuild2.NewOSTreePullStage(
		&osbuild2.OSTreePullStageOptions{Repo: repoPath},
		osbuild2.NewOstreePullStageInputs("org.osbuild.pipeline", "name:"+p.commitPipeline.Name(), p.commitPipeline.Ref),
	))

	// make nginx log and lib directories world writeable, otherwise nginx can't start in
	// an unprivileged container
	pipeline.AddStage(osbuild2.NewChmodStage(chmodStageOptions("/var/log/nginx", "a+rwX", true)))
	pipeline.AddStage(osbuild2.NewChmodStage(chmodStageOptions("/var/lib/nginx", "a+rwX", true)))

	pipeline.AddStage(osbuild2.NewNginxConfigStage(nginxConfigStageOptions(p.NginxConfigPath, htmlRoot, p.ListenPort)))

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
