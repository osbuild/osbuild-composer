package target

type AWSTargetOptions struct {
	Filename          string   `json:"filename"`
	Region            string   `json:"region"`
	AccessKeyID       string   `json:"accessKeyID"`
	SecretAccessKey   string   `json:"secretAccessKey"`
	Bucket            string   `json:"bucket"`
	Key               string   `json:"key"`
	ShareWithAccounts []string `json:"shareWithAccounts"`
}

func (AWSTargetOptions) isTargetOptions() {}

func NewAWSTarget(options *AWSTargetOptions) *Target {
	return newTarget("org.osbuild.aws", options)
}

type AWSTargetResultOptions struct {
	Ami    string `json:"ami"`
	Region string `json:"region"`
}

func (AWSTargetResultOptions) isTargetResultOptions() {}

func NewAWSTargetResult(options *AWSTargetResultOptions) *TargetResult {
	return newTargetResult("org.osbuild.aws", options)
}
