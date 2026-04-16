package v2

import (
	"fmt"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/bootc"
	"github.com/osbuild/images/pkg/distro/generic"
)

// bootcSupportedImageType checks whether the given image type name is supported
// for bootc composes on the given architecture by constructing a dummy bootc
// distro and querying its image type registry.
//
// NOTE: This constructs a dummy bootc distro with placeholder values solely to
// query the set of supported image types from the bootc-generic YAML definitions
// in osbuild/images. The image type names and their availability are determined
// by the YAML and do not depend on the actual container metadata. The dummy
// values for Imgref, DefaultRootFs, Size, and OSInfo are required by the
// NewBootc constructor validation but are not used for image type listing.
//
// TODO: Consider adding a dedicated helper to the osbuild/images library
// (e.g. generic.BootcSupportedImageTypes) that returns the list of supported
// image type names without requiring a full bootc.Info. This would eliminate
// the need for dummy values and make the contract less fragile.
func bootcSupportedImageType(archName string, imageTypeName string) error {
	// Canonicalize the architecture name (e.g. "amd64" -> "x86_64")
	// before passing to NewBootc and GetArch.
	canonicalArch, err := arch.FromString(archName)
	if err != nil {
		return HTTPErrorWithDetails(
			ErrorUnsupportedArchitecture, nil,
			fmt.Sprintf("unsupported architecture %q for bootc composes", archName),
		)
	}
	canonicalArchName := canonicalArch.String()

	dummyInfo := &bootc.Info{
		Imgref:        "dummy",
		Arch:          canonicalArchName,
		DefaultRootFs: "ext4",
		Size:          1,
		OSInfo: &osinfo.Info{
			OSRelease: osinfo.OSRelease{
				ID:        "dummy",
				VersionID: "0",
			},
		},
	}

	bootcDistro, err := generic.NewBootc("bootc", dummyInfo)
	if err != nil {
		return HTTPErrorWithDetails(
			ErrorUnsupportedImageType, nil,
			fmt.Sprintf("failed to initialize bootc distro for arch %q: %v", canonicalArchName, err),
		)
	}

	archi, err := bootcDistro.GetArch(canonicalArchName)
	if err != nil {
		return HTTPErrorWithDetails(
			ErrorUnsupportedArchitecture, nil,
			fmt.Sprintf("internal error: architecture %q not available in bootc distro after successful construction", canonicalArchName),
		)
	}

	if _, err := archi.GetImageType(imageTypeName); err != nil {
		return HTTPErrorWithDetails(
			ErrorUnsupportedImageType, nil,
			fmt.Sprintf("unsupported image type %q for bootc composes on %q", imageTypeName, canonicalArchName),
		)
	}

	return nil
}
