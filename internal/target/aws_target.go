package target

const (
	TargetNameAWS   TargetName = "org.osbuild.aws"
	TargetNameAWSS3 TargetName = "org.osbuild.aws.s3"
)

type AWSTargetOptions struct {
	Region            string   `json:"region"`
	AccessKeyID       string   `json:"accessKeyID"`
	SecretAccessKey   string   `json:"secretAccessKey"`
	SessionToken      string   `json:"sessionToken"`
	Bucket            string   `json:"bucket"`
	Key               string   `json:"key"`
	ShareWithAccounts []string `json:"shareWithAccounts"`
}

func (AWSTargetOptions) isTargetOptions() {}

func NewAWSTarget(options *AWSTargetOptions) *Target {
	return newTarget(TargetNameAWS, options)
}

type AWSTargetResultOptions struct {
	Ami    string `json:"ami"`
	Region string `json:"region"`
}

func (AWSTargetResultOptions) isTargetResultOptions() {}

func NewAWSTargetResult(options *AWSTargetResultOptions) *TargetResult {
	return newTargetResult(TargetNameAWS, options)
}

type AWSS3TargetOptions struct {
	Region              string `json:"region"`
	AccessKeyID         string `json:"accessKeyID"`
	SecretAccessKey     string `json:"secretAccessKey"`
	SessionToken        string `json:"sessionToken"`
	Bucket              string `json:"bucket"`
	Key                 string `json:"key"`
	Endpoint            string `json:"endpoint"`
	CABundle            string `json:"ca_bundle"`
	SkipSSLVerification bool   `json:"skip_ssl_verification"`
}

func (AWSS3TargetOptions) isTargetOptions() {}

func NewAWSS3Target(options *AWSS3TargetOptions) *Target {
	return newTarget(TargetNameAWSS3, options)
}

type AWSS3TargetResultOptions struct {
	URL string `json:"url"`
}

func (AWSS3TargetResultOptions) isTargetResultOptions() {}

func NewAWSS3TargetResult(options *AWSS3TargetResultOptions) *TargetResult {
	return newTargetResult(TargetNameAWSS3, options)
}
