package target

type AWSTargetOptions struct {
	Region          string `json:"region"`
	AccessKeyID     string `json:"accessKeyID"`
	SecretAccessKey string `json:"secretAccessKey"`
	Bucket          string `json:"bucket"`
	Key             string `json:"key"`
}

func (AWSTargetOptions) isTargetOptions() {}

func NewAWSTarget(options *AWSTargetOptions) *Target {
	return &Target{
		Name:    "org.osbuild.aws",
		Options: options,
	}
}
