package osbuild

type OSTreeCommitStageOptions struct {
	// OStree ref to create for the commit
	Ref string `json:"ref"`

	// Set the version of the OS as commit metadata
	OSVersion string `json:"os_version,omitempty"`

	// Commit ID of the parent commit
	Parent string `json:"parent,omitempty"`
}

func (OSTreeCommitStageOptions) isStageOptions() {}

type OSTreeCommitStageInput struct {
	inputCommon
	References OSTreeCommitStageReferences `json:"references"`
}

func (OSTreeCommitStageInput) isStageInput() {}

type OSTreeCommitStageInputs struct {
	Tree *OSTreeCommitStageInput `json:"tree"`
}

func (OSTreeCommitStageInputs) isStageInputs() {}

type OSTreeCommitStageReferences []string

func (OSTreeCommitStageReferences) isReferences() {}

// The OSTreeCommitStage (org.osbuild.ostree.commit) describes how to assemble
// a tree into an OSTree commit.
func NewOSTreeCommitStage(options *OSTreeCommitStageOptions, inputs *OSTreeCommitStageInputs) *Stage {
	return &Stage{
		Type:    "org.osbuild.ostree.commit",
		Options: options,
		Inputs:  inputs,
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
