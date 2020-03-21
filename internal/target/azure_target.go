package target

type AzureTargetOptions struct {
	Filename  string `json:"filename"`
	Account   string `json:"account"`
	AccessKey string `json:"accessKey"`
	Container string `json:"container"`
}

func (AzureTargetOptions) isTargetOptions() {}

func NewAzureTarget(options *AzureTargetOptions) *Target {
	return newTarget("org.osbuild.azure", options)
}
