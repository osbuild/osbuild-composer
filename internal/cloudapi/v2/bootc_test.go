package v2_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v2 "github.com/osbuild/osbuild-composer/internal/cloudapi/v2"
)

func TestBootcSupportedImageType(t *testing.T) {
	tests := []struct {
		name          string
		arch          string
		imageTypeName string
		expectErrMsg  string
	}{
		// Supported: API image types that map to bootc YAML entries
		{
			name:          "guest-image on x86_64",
			arch:          "x86_64",
			imageTypeName: v2.ImageTypeFromApiImageType(v2.ImageTypesGuestImage),
		},
		{
			name:          "aws on x86_64",
			arch:          "x86_64",
			imageTypeName: v2.ImageTypeFromApiImageType(v2.ImageTypesAws),
		},
		{
			name:          "azure on x86_64",
			arch:          "x86_64",
			imageTypeName: v2.ImageTypeFromApiImageType(v2.ImageTypesAzure),
		},
		{
			name:          "gcp on x86_64",
			arch:          "x86_64",
			imageTypeName: v2.ImageTypeFromApiImageType(v2.ImageTypesGcp),
		},
		{
			name:          "vsphere on x86_64",
			arch:          "x86_64",
			imageTypeName: v2.ImageTypeFromApiImageType(v2.ImageTypesVsphere),
		},
		{
			name:          "vsphere-ova on x86_64",
			arch:          "x86_64",
			imageTypeName: v2.ImageTypeFromApiImageType(v2.ImageTypesVsphereOva),
		},
		{
			name:          "pxe-tar-xz on x86_64",
			arch:          "x86_64",
			imageTypeName: v2.ImageTypeFromApiImageType(v2.ImageTypesPxeTarXz),
		},
		// Supported on aarch64
		{
			name:          "guest-image on aarch64",
			arch:          "aarch64",
			imageTypeName: v2.ImageTypeFromApiImageType(v2.ImageTypesGuestImage),
		},
		{
			name:          "aws on aarch64",
			arch:          "aarch64",
			imageTypeName: v2.ImageTypeFromApiImageType(v2.ImageTypesAws),
		},
		// Unsupported: internal names not in bootc YAML
		{
			name:          "minimal-raw not in bootc YAML",
			arch:          "x86_64",
			imageTypeName: v2.ImageTypeFromApiImageType(v2.ImageTypesMinimalRaw),
			expectErrMsg:  "unsupported image type \"minimal-raw\" for bootc composes on \"x86_64\"",
		},
		{
			name:          "image-installer not in bootc YAML",
			arch:          "x86_64",
			imageTypeName: "image-installer",
			expectErrMsg:  "unsupported image type \"image-installer\" for bootc composes on \"x86_64\"",
		},
		{
			name:          "ec2 (RHUI) not in bootc YAML",
			arch:          "x86_64",
			imageTypeName: "ec2",
			expectErrMsg:  "unsupported image type \"ec2\" for bootc composes on \"x86_64\"",
		},
		// Invalid architecture
		{
			name:          "invalid arch",
			arch:          "invalid-arch",
			imageTypeName: "qcow2",
			expectErrMsg:  "unsupported architecture \"invalid-arch\" for bootc composes",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v2.BootcSupportedImageType(tc.arch, tc.imageTypeName)
			if tc.expectErrMsg != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tc.expectErrMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
