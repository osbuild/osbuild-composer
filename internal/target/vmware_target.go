package target

const TargetNameVMWare TargetName = "org.osbuild.vmware"

type VMWareTargetOptions struct {
	Host       string `json:"host"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Datacenter string `json:"datacenter"`
	Cluster    string `json:"cluster"`
	Datastore  string `json:"datastore"`
	Folder     string `json:"folder"`
}

func (VMWareTargetOptions) isTargetOptions() {}

func NewVMWareTarget(options *VMWareTargetOptions) *Target {
	return newTarget(TargetNameVMWare, options)
}

func NewVMWareTargetResult(artifact *OsbuildArtifact) *TargetResult {
	return newTargetResult(TargetNameVMWare, nil, artifact)
}
