package environment

import "github.com/osbuild/images/pkg/rpmmd"

type Environment interface {
	GetPackages() []string
	GetRepos() []rpmmd.RepoConfig
	GetServices() []string
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

// EnvironmentConf is an environment that is fully defined via YAML
// and implements the "Environment" interface
type EnvironmentConf struct {
	Packages []string
	Repos    []rpmmd.RepoConfig
	Services []string
}

var _ = Environment(&EnvironmentConf{})

func (p *EnvironmentConf) GetPackages() []string {
	return p.Packages
}

func (p *EnvironmentConf) GetRepos() []rpmmd.RepoConfig {
	return p.Repos
}

func (p *EnvironmentConf) GetServices() []string {
	return p.Services
}
