package workload

import "github.com/osbuild/images/pkg/rpmmd"

type Workload interface {
	GetPackages() []string
	GetEnabledModules() []string
	GetRepos() []rpmmd.RepoConfig
	GetServices() []string
	GetDisabledServices() []string
	GetMaskedServices() []string
}

type BaseWorkload struct {
	Repos []rpmmd.RepoConfig
}

func (p BaseWorkload) GetPackages() []string {
	return []string{}
}

func (p BaseWorkload) GetEnabledModules() []string {
	return []string{}
}

func (p BaseWorkload) GetRepos() []rpmmd.RepoConfig {
	return p.Repos
}

func (p BaseWorkload) GetServices() []string {
	return []string{}
}

func (p BaseWorkload) GetDisabledServices() []string {
	return []string{}
}

func (p BaseWorkload) GetMaskedServices() []string {
	return []string{}
}
