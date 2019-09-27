package pipeline

type TarAssemblerOptions struct {
	Filename    string `json:"filename"`
	Compression string `json:"compression,omitempty"`
}

func (TarAssemblerOptions) isAssemblerOptions() {}

func NewTarAssemblerOptions(filename string) *TarAssemblerOptions {
	return &TarAssemblerOptions{
		Filename: filename,
	}
}

func NewTarAssembler(options *TarAssemblerOptions) *Assembler {
	return &Assembler{
		Name:    "org.osbuild.tar",
		Options: options,
	}
}

func (options *TarAssemblerOptions) SetCompression(compression string) {
	options.Compression = compression
}
