package environment

import "github.com/osbuild/osbuild-composer/internal/rpmmd"

type Environment interface {
	GetPackages() []string
	GetRepos() []rpmmd.RepoConfig
	GetServices() []string
	GetKernelOptions() []string
}

type BaseEnvironment struct {
	Repos []rpmmd.RepoConfig
}

func (p BaseEnvironment) GetPackages() []string {
	return []string{}
}

func (p BaseEnvironment) GetRepos() []rpmmd.RepoConfig {
	return p.Repos
}

func (p BaseEnvironment) GetServices() []string {
	return []string{}
}

func (p BaseEnvironment) GetKernelOptions() []string {
	return []string{}
}
