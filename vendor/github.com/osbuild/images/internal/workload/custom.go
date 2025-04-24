package workload

type Custom struct {
	BaseWorkload
	Packages         []string
	EnabledModules   []string
	Services         []string
	DisabledServices []string
	MaskedServices   []string
}

func (p *Custom) GetPackages() []string {
	return p.Packages
}

func (p *Custom) GetEnabledModules() []string {
	return p.EnabledModules
}

func (p *Custom) GetServices() []string {
	return p.Services
}

// TODO: Do these belong here? What kind of workload requires
// services to be disabled or masked?
func (p *Custom) GetDisabledServices() []string {
	return p.DisabledServices
}

func (p *Custom) GetMaskedServices() []string {
	return p.MaskedServices
}
