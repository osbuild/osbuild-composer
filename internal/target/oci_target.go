package target

type OCITargetOptions struct {
	User        string `json:"user"`
	Tenancy     string `json:"tenancy"`
	Region      string `json:"region"`
	FileName    string `json:"filename"`
	Fingerprint string `json:"fingerprint"`
	PrivateKey  string `json:"private_key"`
	Bucket      string `json:"bucket"`
	Namespace   string `json:"namespace"`
	Compartment string `json:"compartment_id"`
}

func (OCITargetOptions) isTargetOptions() {}

func NewOCITarget(options *OCITargetOptions) *Target {
	return newTarget("org.osbuild.oci", options)
}

type OCITargetResultOptions struct {
	Region  string `json:"region"`
	ImageID string `json:"image_id"`
}

func (OCITargetResultOptions) isTargetResultOptions() {}

func NewOCITargetResult(options *OCITargetResultOptions) *TargetResult {
	return newTargetResult("org.osbuild.oci", options)
}
