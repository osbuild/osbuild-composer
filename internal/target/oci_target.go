package target

const TargetNameOCI TargetName = "org.osbuild.oci"

type OCITargetOptions struct {
	User        string `json:"user"`
	Tenancy     string `json:"tenancy"`
	Region      string `json:"region"`
	Fingerprint string `json:"fingerprint"`
	PrivateKey  string `json:"private_key"`
	Bucket      string `json:"bucket"`
	Namespace   string `json:"namespace"`
	Compartment string `json:"compartment_id"`
}

func (OCITargetOptions) isTargetOptions() {}

func NewOCITarget(options *OCITargetOptions) *Target {
	return newTarget(TargetNameOCI, options)
}

type OCITargetResultOptions struct {
	Region  string `json:"region"`
	ImageID string `json:"image_id"`
}

func (OCITargetResultOptions) isTargetResultOptions() {}

func NewOCITargetResult(options *OCITargetResultOptions) *TargetResult {
	return newTargetResult(TargetNameOCI, options)
}

const TargetNameOCIObjectStorage TargetName = "org.osbuild.oci.objectstorage"

func NewOCIObjectStorageTarget(options *OCIObjectStorageTargetOptions) *Target {
	return newTarget(TargetNameOCIObjectStorage, options)
}

type OCIObjectStorageTargetOptions struct {
	User        string `json:"user"`
	Tenancy     string `json:"tenancy"`
	Region      string `json:"region"`
	Fingerprint string `json:"fingerprint"`
	PrivateKey  string `json:"private_key"`
	Bucket      string `json:"bucket"`
	Namespace   string `json:"namespace"`
	Compartment string `json:"compartment_id"`
}

func (OCIObjectStorageTargetOptions) isTargetOptions() {}

type OCIObjectStorageTargetResultOptions struct {
	URL string `json:"url"`
}

func (OCIObjectStorageTargetResultOptions) isTargetResultOptions() {}

func NewOCIObjectStorageTargetResult(options *OCIObjectStorageTargetResultOptions) *TargetResult {
	return newTargetResult(TargetNameOCIObjectStorage, options)
}
