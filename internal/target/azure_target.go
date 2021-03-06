package target

type AzureTargetOptions struct {
	Filename         string `json:"filename"`
	StorageAccount   string `json:"storageAccount"`
	StorageAccessKey string `json:"storageAccessKey"`
	Container        string `json:"container"`
}

func (AzureTargetOptions) isTargetOptions() {}

// NewAzureTarget creates org.osbuild.azure target
//
// This target uploads a Page Blob to Azure Storage.
//
// The target uses Azure Storage keys for authentication, see:
// https://docs.microsoft.com/en-us/azure/storage/common/storage-account-keys-manage
// The credentials are defined inside the target options.
//
// If you need to upload an Azure Image instead, see the
// org.osbuild.azure.image target.
func NewAzureTarget(options *AzureTargetOptions) *Target {
	return newTarget("org.osbuild.azure", options)
}
