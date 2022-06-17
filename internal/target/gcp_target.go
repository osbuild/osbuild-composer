package target

const TargetNameGCP TargetName = "org.osbuild.gcp"

type GCPTargetOptions struct {
	Region            string   `json:"region"`
	Os                string   `json:"os"` // not exposed in cloudapi for now
	Bucket            string   `json:"bucket"`
	Object            string   `json:"object"`
	ShareWithAccounts []string `json:"shareWithAccounts,omitempty"`

	// If provided, these credentials are used by the worker to import the image
	// to GCP. If not provided, the worker will try to authenticate using the
	// credentials from worker's configuration.
	Credentials []byte `json:"credentials,omitempty"`
}

func (GCPTargetOptions) isTargetOptions() {}

func NewGCPTarget(options *GCPTargetOptions) *Target {
	return newTarget(TargetNameGCP, options)
}

type GCPTargetResultOptions struct {
	ImageName string `json:"image_name"`
	ProjectID string `json:"project_id"`
}

func (GCPTargetResultOptions) isTargetResultOptions() {}

func NewGCPTargetResult(options *GCPTargetResultOptions) *TargetResult {
	return newTargetResult(TargetNameGCP, options)
}
