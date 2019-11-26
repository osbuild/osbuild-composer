package target

type AzureTargetOptions struct {
	Account   string `json:"account"`
	AccessKey string `json:"accessKey"`
	Container string `json:"container"`
}

func (AzureTargetOptions) isTargetOptions() {}

func NewAzureTarget(options *AzureTargetOptions) *Target {
	return newTarget("org.osbuild.azure", options)
}
