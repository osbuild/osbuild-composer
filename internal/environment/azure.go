package environment

type Azure struct {
	BaseEnvironment
}

func (p *Azure) GetPackages() []string {
	return []string{"WALinuxAgent"}
}

func (p *Azure) GetServices() []string {
	return []string{"waagent"}
}
