package osbuild1

// OSTreeCommitAssemblerOptions desrcibe how to assemble a tree into an OSTree commit.
type OSTreeCommitAssemblerOptions struct {
	Ref    string                          `json:"ref"`
	Parent string                          `json:"parent,omitempty"`
	Tar    OSTreeCommitAssemblerTarOptions `json:"tar"`
}

// OSTreeCommitAssemblerTarOptions desrcibes the output tarball
type OSTreeCommitAssemblerTarOptions struct {
	Filename string `json:"filename"`
}

func (OSTreeCommitAssemblerOptions) isAssemblerOptions() {}

// NewOSTreeCommitAssembler creates a new OSTree Commit Assembler object.
func NewOSTreeCommitAssembler(options *OSTreeCommitAssemblerOptions) *Assembler {
	return &Assembler{
		Name:    "org.osbuild.ostree.commit",
		Options: options,
	}
}
