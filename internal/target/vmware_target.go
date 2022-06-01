package target

const TargetNameVMWare TargetName = "org.osbuild.vmware"

type VMWareTargetOptions struct {
	Filename   string `json:"filename"`
	Host       string `json:"host"`
	Username   string `json:"username"`
	Password   string `json:"password"`
	Datacenter string `json:"datacenter"`
	Cluster    string `json:"cluster"`
	Datastore  string `json:"datastore"`
}

func (VMWareTargetOptions) isTargetOptions() {}

func NewVMWareTarget(options *VMWareTargetOptions) *Target {
	return newTarget(TargetNameVMWare, options)
}
