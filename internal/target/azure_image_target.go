package target

const TargetNameAzureImage TargetName = "org.osbuild.azure.image"

type HyperVGenerationType string

const (
	HyperVGenV1 HyperVGenerationType = "V1"
	HyperVGenV2 HyperVGenerationType = "V2"
)

type AzureImageTargetOptions struct {
	TenantID         string               `json:"tenant_id"`
	Location         string               `json:"location,omitempty"`
	SubscriptionID   string               `json:"subscription_id"`
	ResourceGroup    string               `json:"resource_group"`
	HyperVGeneration HyperVGenerationType `json:"hyperv_generation"`
}

func (AzureImageTargetOptions) isTargetOptions() {}

// NewAzureImageTarget creates org.osbuild.azure.image target
//
// This target uploads and registers an Azure Image. The image can be then
// immediately used to spin up a virtual machine.
//
// The target uses Azure OAuth credentials. In most cases you want to create
// a service principal for this purpose, see:
// https://docs.microsoft.com/en-us/azure/active-directory/develop/app-objects-and-service-principals
// The credentials are not passed in the target options, instead they are
// defined in the worker. If the worker doesn't have Azure credentials
// and gets a job with this target, the job will fail.
//
// The Tenant ID for the authorization process is specified in the target
// options. This means that this target can be used for multi-tenant
// applications.
//
// If you need to just upload a PageBlob into Azure Storage, see the
// org.osbuild.azure target.
func NewAzureImageTarget(options *AzureImageTargetOptions) *Target {
	return newTarget(TargetNameAzureImage, options)
}

type AzureImageTargetResultOptions struct {
	ImageName string `json:"image_name"`
}

func (AzureImageTargetResultOptions) isTargetResultOptions() {}

func NewAzureImageTargetResult(options *AzureImageTargetResultOptions, artifact *OsbuildArtifact) *TargetResult {
	return newTargetResult(TargetNameAzureImage, options, artifact)
}
