package runner

// RunnerConf implements the runner interface
var _ = Runner(&RunnerConf{})

type RunnerConf struct {
	Name          string   `yaml:"name"`
	BuildPackages []string `yaml:"build_packages"`
}

func (r *RunnerConf) String() string {
	return r.Name
}

func (r *RunnerConf) GetBuildPackages() []string {
	return r.BuildPackages
}
