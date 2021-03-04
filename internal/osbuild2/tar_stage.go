package osbuild2

type TarStageOptions struct {
	// Filename for tar archive
	Filename string `json:"filename"`

	// Enable support for POSIX ACLs
	ACLs bool `json:"acls,omitempty"`

	// Enable support for SELinux contexts
	SELinux bool `json:"selinux,omitempty"`

	// Enable support for extended attributes
	Xattrs bool `json:"xattrs,omitempty"`
}

func (TarStageOptions) isStageOptions() {}

// Assembles a tree into a tar archive. Compression is determined by the suffix
// (i.e., --auto-compress is used).
func NewTarAssembler(options *TarStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.tar",
		Options: options,
	}
}
