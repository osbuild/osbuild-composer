package target

const TargetNameContainer TargetName = "org.osbuild.container"

type ContainerTargetOptions struct {
	Reference string `json:"reference"`

	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`

	TlsVerify *bool `json:"tls_verify,omitempty"`
}

func (ContainerTargetOptions) isTargetOptions() {}

func NewContainerTarget(options *ContainerTargetOptions) *Target {
	return newTarget(TargetNameContainer, options)
}

func NewContainerTargetResult() *TargetResult {
	return newTargetResult(TargetNameContainer, nil)
}
