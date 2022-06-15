package target

type KojiTargetOptions struct {
	// Filename of the image as produced by osbuild for a given export
	Filename        string `json:"filename"`
	UploadDirectory string `json:"upload_directory"`
	Server          string `json:"server"`
}

func (KojiTargetOptions) isTargetOptions() {}

func NewKojiTarget(options *KojiTargetOptions) *Target {
	return newTarget("org.osbuild.koji", options)
}

type KojiTargetResultOptions struct {
	ImageMD5  string `json:"image_md5"`
	ImageSize uint64 `json:"image_size"`
}

func (KojiTargetResultOptions) isTargetResultOptions() {}

func NewKojiTargetResult(options *KojiTargetResultOptions) *TargetResult {
	return newTargetResult("org.osbuild.koji", options)
}
