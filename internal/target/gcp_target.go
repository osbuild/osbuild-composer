package target

type GCPTargetOptions struct {
	Filename          string   `json:"filename"`
	Region            string   `json:"region"`
	Os                string   `json:"os"` // not exposed in cloudapi for now
	Bucket            string   `json:"bucket"`
	Object            string   `json:"object"`
	ShareWithAccounts []string `json:"shareWithAccounts"`
}

func (GCPTargetOptions) isTargetOptions() {}

func NewGCPTarget(options *GCPTargetOptions) *Target {
	return newTarget("org.osbuild.gcp", options)
}

type GCPTargetResultOptions struct {
	ImageName string `json:"image_name"`
	ProjectID string `json:"project_id"`
}

func (GCPTargetResultOptions) isTargetResultOptions() {}

func NewGCPTargetResult(options *GCPTargetResultOptions) *TargetResult {
	return newTargetResult("org.osbuild.gcp", options)
}
