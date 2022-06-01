package target

const TargetNameContainer TargetName = "org.osbuild.container"

type ContainerTargetOptions struct {
	Filename  string `json:"filename"`
	Reference string `json:"reference"`

	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	TlsVerify *bool `json:"tls_verify,omitempty"`
}

func (ContainerTargetOptions) isTargetOptions() {}

func NewContainerTarget(options *ContainerTargetOptions) *Target {
	return newTarget(TargetNameContainer, options)
}
