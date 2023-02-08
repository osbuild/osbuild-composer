package environment

type OCI struct {
	BaseEnvironment
}

func (p *OCI) GetPackages() []string {
	return []string{"iscsi-initiator-utils"}
}

func (p *OCI) GetKernelOptions() []string {
	return []string{
		"rd.iscsi.ibft=1",
		"rd.iscsi.firmware=1",
	}
}
