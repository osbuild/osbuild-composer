package target

type AzureTargetOptions struct {
	Filename         string `json:"filename"`
	StorageAccount   string `json:"storageAccount"`
	StorageAccessKey string `json:"storageAccessKey"`
	Container        string `json:"container"`
}

func (AzureTargetOptions) isTargetOptions() {}

func NewAzureTarget(options *AzureTargetOptions) *Target {
	return newTarget("org.osbuild.azure", options)
}
