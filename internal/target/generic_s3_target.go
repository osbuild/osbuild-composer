package target

type GenericS3TargetOptions struct {
	AWSS3TargetOptions
	Endpoint            string `json:"endpoint"`
	CABundle            string `json:"ca_bundle"`
	SkipSSLVerification bool   `json:"skip_ssl_verification"`
}

func (GenericS3TargetOptions) isTargetOptions() {}

func NewGenericS3Target(options *GenericS3TargetOptions) *Target {
	return newTarget("org.osbuild.generic.s3", options)
}

type GenericS3TargetResultOptions AWSS3TargetResultOptions

func (GenericS3TargetResultOptions) isTargetResultOptions() {}

func NewGenericS3TargetResult(options *GenericS3TargetResultOptions) *TargetResult {
	return newTargetResult("org.osbuild.generic.s3", options)
}
