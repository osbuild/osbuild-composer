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

type AWSS3TargetOptions struct {
	Filename        string `json:"filename"`
	Region          string `json:"region"`
	AccessKeyID     string `json:"accessKeyID"`
	SecretAccessKey string `json:"secretAccessKey"`
	Bucket          string `json:"bucket"`
	Key             string `json:"key"`
}

func (AWSS3TargetOptions) isTargetOptions() {}

func NewAWSS3Target(options *AWSS3TargetOptions) *Target {
	return newTarget("org.osbuild.aws.s3", options)
}

type AWSS3TargetResultOptions struct {
	URL string `json:"url"`
}

func (AWSS3TargetResultOptions) isTargetResultOptions() {}

func NewAWSS3TargetResult(options *AWSS3TargetResultOptions) *TargetResult {
	return newTargetResult("org.osbuild.aws.s3", options)
}
