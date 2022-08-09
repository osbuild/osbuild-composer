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

type ContainerTargetResultOptions struct {
	URL    string `json:"url"`
	Digest string `json:"digest"`
}

func (ContainerTargetResultOptions) isTargetResultOptions() {}

func NewContainerTargetResult(options *ContainerTargetResultOptions) *TargetResult {
	return newTargetResult(TargetNameContainer, options)
}
