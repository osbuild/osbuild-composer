package osbuild

type Secret struct {
	Name string `json:"name,omitempty"`
}
type FileSource struct {
	URL     string  `json:"url"`
	Secrets *Secret `json:"secrets,omitempty"`
}

// The FilesSourceOptions specifies a custom script to run in the image
type FilesSource struct {
	URLs map[string]FileSource `json:"urls"`
}

func (FilesSource) isSource() {}
