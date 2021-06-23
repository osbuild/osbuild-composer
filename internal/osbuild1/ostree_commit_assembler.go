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

type OSTreeCommitStageMetadata struct {
	Compose OSTreeCommitStageMetadataCompose `json:"compose"`
}

type OSTreeCommitStageMetadataCompose struct {
	Ref                       string `json:"ref"`
	OSTreeNMetadataTotal      int    `json:"ostree-n-metadata-total"`
	OSTreeNMetadataWritten    int    `json:"ostree-n-metadata-written"`
	OSTreeNContentTotal       int    `json:"ostree-n-content-total"`
	OSTreeNContentWritten     int    `json:"ostree-n-content-written"`
	OSTreeNCacheHits          int    `json:"ostree-n-cache-hits"`
	OSTreeContentBytesWritten int    `json:"ostree-content-bytes-written"`
	OSTreeCommit              string `json:"ostree-commit"`
	OSTreeContentChecksum     string `json:"ostree-content-checksum"`
	OSTreeTimestamp           string `json:"ostree-timestamp"`
	RPMOSTreeInputHash        string `json:"rpm-ostree-inputhash"`
}

func (OSTreeCommitStageMetadata) isStageMetadata() {}
