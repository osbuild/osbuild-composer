package container

import (
	"github.com/osbuild/image-builder/pkg/arch"
)

// A Spec is the specification of how to get a specific
// container from a Source and under what LocalName to
// store it in an image. The container is identified by
// at the Source via Digest and ImageID. The latter one
// should remain the same in the target image as well.
type Spec struct {
	Source       string // does not include the manifest digest
	Digest       string // digest of the manifest at the Source
	TLSVerify    *bool  // controls TLS verification
	ImageID      string // container image identifier
	LocalName    string // name to use inside the image
	ListDigest   string // digest of the list manifest at the Source (optional)
	LocalStorage bool

	Arch arch.Arch // the architecture of the image
}

// NewSpec creates a new Spec from the essential information.
func NewSpec(source string, digest, imageID string, tlsVerify *bool, listDigest string, localName string, localStorage bool) Spec {
	if localName == "" {
		localName = source
	}
	return Spec{
		Source:       source,
		Digest:       digest,
		TLSVerify:    tlsVerify,
		ImageID:      imageID,
		LocalName:    localName,
		ListDigest:   listDigest,
		LocalStorage: localStorage,
	}
}
