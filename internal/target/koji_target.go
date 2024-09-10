package target

import (
	"encoding/json"
	"fmt"
)

const TargetNameKoji TargetName = "org.osbuild.koji"

type KojiTargetOptions struct {
	UploadDirectory string `json:"upload_directory"`
	Server          string `json:"server"`
}

func (KojiTargetOptions) isTargetOptions() {}

func NewKojiTarget(options *KojiTargetOptions) *Target {
	return newTarget(TargetNameKoji, options)
}

// ChecksumType represents the type of a checksum used for a KojiOutputInfo.
type ChecksumType string

const (
	ChecksumTypeMD5 ChecksumType = "md5"

	// Only MD5 is supported for now to enable backwards compatibility.
	// The reason is tha the old KojiTargetOptions contained only
	// ImageMD5 and ImageSize fields, which mandates the use of MD5.
	// TODO: uncomment the lines below when the backwards compatibility is no longer needed.
	//ChecksumTypeAdler32 ChecksumType = "adler32"
	//ChecksumTypeSHA256  ChecksumType = "sha256"
)

// KojiOutputInfo represents the information about any output file uploaded to Koji
// as part of the OSBuild job. This information is then used by the KojiFinalize
// job when importing files into Koji.
type KojiOutputInfo struct {
	Filename     string       `json:"filename"`
	ChecksumType ChecksumType `json:"checksum_type"`
	Checksum     string       `json:"checksum"`
	Size         uint64       `json:"size"`
}

type OSBuildComposerDepModule struct {
	Path    string                    `json:"path"`
	Version string                    `json:"version"`
	Replace *OSBuildComposerDepModule `json:"replace,omitempty"`
}

// ManifestInfo contains information about the environment in which
// the manifest was produced and which could affect its content.
type ManifestInfo struct {
	OSBuildComposerVersion string `json:"osbuild_composer_version"`
	// List of relevant modules used by osbuild-composer which
	// could affect the manifest content.
	OSBuildComposerDeps []*OSBuildComposerDepModule `json:"osbuild_composer_deps,omitempty"`
}

type KojiTargetResultOptions struct {
	Image               *KojiOutputInfo  `json:"image"`
	Log                 *KojiOutputInfo  `json:"log,omitempty"`
	OSBuildManifest     *KojiOutputInfo  `json:"osbuild_manifest,omitempty"`
	OSBuildManifestInfo *ManifestInfo    `json:"osbuild_manifest_info,omitempty"`
	SbomDocs            []KojiOutputInfo `json:"sbom_docs,omitempty"`
}

func (o *KojiTargetResultOptions) UnmarshalJSON(data []byte) error {
	type aliasType KojiTargetResultOptions
	if err := json.Unmarshal(data, (*aliasType)(o)); err != nil {
		return err
	}

	// compatType contains deprecated fields, which are being checked
	// for backwards compatibility.
	type compatType struct {
		// Deprecated: Use Image in KojiTargetOptions instead.
		// Kept for backwards compatibility.
		ImageMD5  string `json:"image_md5"`
		ImageSize uint64 `json:"image_size"`
	}

	var compat compatType
	if err := json.Unmarshal(data, &compat); err != nil {
		return err
	}

	// Check if the Image data in the new struct format are set.
	// If not, then the data are coming from an old composer.
	if o.Image == nil {
		// o.Image.Filename is kept empty, because the filename was previously
		// not set as there was always only the Image file. The KojiFinalize job
		// handles this case and takes the Image filename from the KojiFinalizeJob
		// options.

		o.Image = &KojiOutputInfo{
			ChecksumType: ChecksumTypeMD5,
			Checksum:     compat.ImageMD5,
			Size:         compat.ImageSize,
		}
	}

	return nil
}

func (o KojiTargetResultOptions) MarshalJSON() ([]byte, error) {
	type alias KojiTargetResultOptions
	// compatType is a super-set of the current KojiTargetResultOptions and
	// old version of it. It contains deprecated fields, which are being set
	// for backwards compatibility.
	type compatType struct {
		alias

		// Deprecated: Use Image in KojiTargetOptions instead.
		// Kept for backwards compatibility.
		ImageMD5  string `json:"image_md5"`
		ImageSize uint64 `json:"image_size"`
	}

	// Only MD5 is supported for now to enable backwards compatibility.
	// TODO: remove this block when the backwards compatibility is no longer needed.
	if o.Image.ChecksumType != ChecksumTypeMD5 {
		return nil, fmt.Errorf("unsupported checksum type: %s", o.Image.ChecksumType)
	}

	compat := compatType{
		alias:     (alias)(o),
		ImageMD5:  o.Image.Checksum,
		ImageSize: o.Image.Size,
	}

	return json.Marshal(compat)
}

func (KojiTargetResultOptions) isTargetResultOptions() {}

func NewKojiTargetResult(options *KojiTargetResultOptions, artifact *OsbuildArtifact) *TargetResult {
	return newTargetResult(TargetNameKoji, options, artifact)
}
