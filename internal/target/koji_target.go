package target

const TargetNameKoji TargetName = "org.osbuild.koji"

type KojiTargetOptions struct {
	UploadDirectory string `json:"upload_directory"`
	Server          string `json:"server"`
}

func (KojiTargetOptions) isTargetOptions() {}

func NewKojiTarget(options *KojiTargetOptions) *Target {
	return newTarget(TargetNameKoji, options)
}

type KojiTargetResultOptions struct {
	ImageMD5  string `json:"image_md5"`
	ImageSize uint64 `json:"image_size"`
}

func (KojiTargetResultOptions) isTargetResultOptions() {}

func NewKojiTargetResult(options *KojiTargetResultOptions) *TargetResult {
	return newTargetResult(TargetNameKoji, options)
}
