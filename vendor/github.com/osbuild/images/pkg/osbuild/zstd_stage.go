package osbuild

type ZstdStageOptions struct {
	// Filename for zstd archive
	Filename string `json:"filename"`
}

func (ZstdStageOptions) isStageOptions() {}

func NewZstdStageOptions(filename string) *ZstdStageOptions {
	return &ZstdStageOptions{
		Filename: filename,
	}
}

type ZstdStageInputs struct {
	File *FilesInput `json:"file"`
}

func (*ZstdStageInputs) isStageInputs() {}

func NewZstdStageInputs(references FilesInputRef) *ZstdStageInputs {
	return &ZstdStageInputs{
		File: NewFilesInput(references),
	}
}

// Compresses a file into a zstd archive.
func NewZstdStage(options *ZstdStageOptions, inputs *ZstdStageInputs) *Stage {
	var stageInputs Inputs
	if inputs != nil {
		stageInputs = inputs
	}

	return &Stage{
		Type:    "org.osbuild.zstd",
		Options: options,
		Inputs:  stageInputs,
	}
}
