package osbuild

// The FilesSourceOptions specifies a custom script to run in the image
type FilesSource struct {
	URLs map[string]string `json:"urls"`
}

func (FilesSource) isSource() {}
