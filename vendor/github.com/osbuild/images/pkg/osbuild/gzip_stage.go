package osbuild

type GzipStageOptions struct {
	// Filename for gz archive
	Filename string `json:"filename"`
}

func (GzipStageOptions) isStageOptions() {}

func NewGzipStageOptions(filename string) *GzipStageOptions {
	return &GzipStageOptions{
		Filename: filename,
	}
}

type GzipStageInputs struct {
	File *FilesInput `json:"file"`
}

func (*GzipStageInputs) isStageInputs() {}

func NewGzipStageInputs(references FilesInputRef) *GzipStageInputs {
	return &GzipStageInputs{
		File: NewFilesInput(references),
	}
}

// Compresses a file into a gzip archive.
func NewGzipStage(options *GzipStageOptions, inputs *GzipStageInputs) *Stage {
	var stageInputs Inputs
	if inputs != nil {
		stageInputs = inputs
	}

	return &Stage{
		Type:    "org.osbuild.gzip",
		Options: options,
		Inputs:  stageInputs,
	}
}
