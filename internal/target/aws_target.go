package target

type AWSTargetOptions struct {
	Filename        string `json:"filename"`
	Region          string `json:"region"`
	AccessKeyID     string `json:"accessKeyID"`
	SecretAccessKey string `json:"secretAccessKey"`
	Bucket          string `json:"bucket"`
	Key             string `json:"key"`
}

func (AWSTargetOptions) isTargetOptions() {}

func NewAWSTarget(options *AWSTargetOptions) *Target {
	return newTarget("org.osbuild.aws", options)
}
