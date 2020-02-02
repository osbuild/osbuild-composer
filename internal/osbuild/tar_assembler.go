package osbuild

// TarAssemblerOptions desrcibe how to assemble a tree into a tar ball.
//
// The assembler tars and optionally compresses the tree using the provided
// compression type, and stores the output with the given filename.
type TarAssemblerOptions struct {
	Filename    string `json:"filename"`
	Size        uint64 `json:"size"`
	Compression string `json:"compression,omitempty"`
}

func (TarAssemblerOptions) isAssemblerOptions() {}

// NewTarAssemblerOptions creates a new TarAssemblerOptions object, with the
// mandatory options set.
func NewTarAssemblerOptions(filename string, size uint64) *TarAssemblerOptions {
	return &TarAssemblerOptions{
		Filename: filename,
		Size:     size,
	}
}

// NewTarAssembler creates a new Tar Assembler object.
func NewTarAssembler(options *TarAssemblerOptions) *Assembler {
	return &Assembler{
		Name:    "org.osbuild.tar",
		Options: options,
	}
}

// SetCompression sets the compression type for a given TarAssemblerOptions
// object.
func (options *TarAssemblerOptions) SetCompression(compression string) {
	options.Compression = compression
}
