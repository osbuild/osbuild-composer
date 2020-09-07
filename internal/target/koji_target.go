package target

type KojiTargetOptions struct {
	BuildID         uint64 `json:"build_id"`
	Token           string `json:"token"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	Release         string `json:"release"`
	Filename        string `json:"filename"`
	UploadDirectory string `json:"upload_directory"`
	Server          string `json:"server"`
}

func (KojiTargetOptions) isTargetOptions() {}

func NewKojiTarget(options *KojiTargetOptions) *Target {
	return newTarget("org.osbuild.koji", options)
}
