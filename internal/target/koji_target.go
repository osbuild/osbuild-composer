package target

type KojiTargetOptions struct {
	BuildID         uint64 `json:"build_id"`
	TaskID          uint64 `json:"task_id"`
	Token           string `json:"token"`
	Name            string `json:"name"`
	Version         string `json:"version"`
	Release         string `json:"release"`
	Filename        string `json:"filename"`
	UploadDirectory string `json:"upload_directory"`
	Server          string `json:"server"`
	KojiFilename    string `json:"koji_filename"`
}

func (KojiTargetOptions) isTargetOptions() {}

func NewKojiTarget(options *KojiTargetOptions) *Target {
	return newTarget("org.osbuild.koji", options)
}
