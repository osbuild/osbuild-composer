package koji

import (
	"github.com/osbuild/osbuild-composer/internal/target"
)

// BUILD METADATA

// TypeInfoBuild is a map whose entries are the names of the build types
// used for the build, and the values are free-form maps containing
// type-specific information for the build.
type TypeInfoBuild struct {
	// Image holds extra metadata about all images built by the build.
	// It is a map whose keys are the filenames of the images, and
	// the values are the extra metadata for the image.
	// There can't be more than one image with the same filename.
	Image map[string]ImageExtraInfo `json:"image"`
}

// BuildExtra holds extra metadata associated with the build.
// It is a free-form map, but must contain at least the 'typeinfo' key.
type BuildExtra struct {
	TypeInfo TypeInfoBuild `json:"typeinfo"`
	// Manifest holds extra metadata about osbuild manifests attached to the build.
	// It is a map whose keys are the filenames of the manifests, and
	// the values are the extra metadata for the manifest.
	Manifest map[string]*ManifestExtraInfo `json:"osbuild_manifest,omitempty"`
}

// Build represents a Koji build and holds metadata about it.
type Build struct {
	BuildID   uint64 `json:"build_id"`
	TaskID    uint64 `json:"task_id"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Release   string `json:"release"`
	Source    string `json:"source"`
	StartTime int64  `json:"start_time"`
	EndTime   int64  `json:"end_time"`
	// NOTE: This is the struct that ends up shown in the buildinfo and webui in Koji.
	Extra BuildExtra `json:"extra"`
}

// BUIDROOT METADATA

// Host holds information about the host where the build was run.
type Host struct {
	Os   string `json:"os"`
	Arch string `json:"arch"`
}

// ContentGenerator holds information about the content generator which run the build.
type ContentGenerator struct {
	Name    string `json:"name"` // Must be 'osbuild'.
	Version string `json:"version"`
}

// Container holds information about the container in which the build was run.
type Container struct {
	// Type of the container that was used, e.g. 'none', 'chroot', 'kvm', 'docker', etc.
	Type string `json:"type"`
	Arch string `json:"arch"`
}

// Tool holds information about a tool used to run build.
type Tool struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// BuildRoot represents a buildroot used for the build.
type BuildRoot struct {
	ID               uint64           `json:"id"`
	Host             Host             `json:"host"`
	ContentGenerator ContentGenerator `json:"content_generator"`
	Container        Container        `json:"container"`
	Tools            []Tool           `json:"tools"`
	RPMs             []RPM            `json:"components"`
}

// OUTPUT METADATA

type ImageOutputTypeExtraInfo interface {
	isImageOutputTypeMD()
}

// ImageExtraInfo holds extra metadata about the image.
// This structure is shared for the Extra metadata of the output and the build.
type ImageExtraInfo struct {
	// Koji docs say: "should contain IDs that allow tracking the output back to the system in which it was generated"
	// TODO: we should probably add some ID here, probably the OSBuildJob UUID?

	Arch string `json:"arch"`
	// Boot mode of the image
	BootMode string `json:"boot_mode,omitempty"`
	// Configuration used to prouce this image using osbuild
	OSBuildArtifact *target.OsbuildArtifact `json:"osbuild_artifact,omitempty"`
	// Version of the osbuild binary used by the worker to build the image
	OSBuildVersion string `json:"osbuild_version,omitempty"`
	// Results from any upload targets associated with the image
	// except for the Koji target.
	UploadTargetResults []*target.TargetResult `json:"upload_target_results,omitempty"`
}

func (ImageExtraInfo) isImageOutputTypeMD() {}

type OSBuildComposerDepModule struct {
	Path    string                    `json:"path"`
	Version string                    `json:"version"`
	Replace *OSBuildComposerDepModule `json:"replace,omitempty"`
}

// ManifestInfo holds information about the environment in which
// the manifest was produced and which could affect its content.
type ManifestInfo struct {
	OSBuildComposerVersion string `json:"osbuild_composer_version"`
	// List of relevant modules used by osbuild-composer which
	// could affect the manifest content.
	OSBuildComposerDeps []*OSBuildComposerDepModule `json:"osbuild_composer_deps,omitempty"`
}

// ManifestExtraInfo holds extra metadata about the osbuild manifest.
type ManifestExtraInfo struct {
	Arch string        `json:"arch"`
	Info *ManifestInfo `json:"info,omitempty"`
}

func (ManifestExtraInfo) isImageOutputTypeMD() {}

type SbomDocExtraInfo struct {
	Arch string `json:"arch"`
}

func (SbomDocExtraInfo) isImageOutputTypeMD() {}

// BuildOutputExtra holds extra metadata associated with the build output.
type BuildOutputExtra struct {
	// ImageOutput holds extra metadata about a single "image" output.
	// "image" in this context is the "build type" in the Koji terminology,
	// not necessarily an actual image. It can and must be used also for
	// other supplementary files related to the image, such as osbuild manifest.
	// The only exception are logs, which do not need to specify any "typeinfo".
	ImageOutput ImageOutputTypeExtraInfo `json:"image"`
}

// BuildOutputType represents the type of a BuildOutput.
type BuildOutputType string

const (
	BuildOutputTypeImage    BuildOutputType = "image"
	BuildOutputTypeLog      BuildOutputType = "log"
	BuildOutputTypeManifest BuildOutputType = "osbuild-manifest"
	BuildOutputTypeSbomDoc  BuildOutputType = "sbom-doc"
)

// ChecksumType represents the type of a checksum used for a BuildOutput.
type ChecksumType string

const (
	ChecksumTypeMD5     ChecksumType = "md5"
	ChecksumTypeAdler32 ChecksumType = "adler32"
	ChecksumTypeSHA256  ChecksumType = "sha256"
)

// BuildOutput represents an output from the OSBuild content generator.
// The output can be a file of various types, which is imported to Koji.
// Examples of types are "image", "log" or other.
type BuildOutput struct {
	BuildRootID  uint64            `json:"buildroot_id"`
	Filename     string            `json:"filename"`
	FileSize     uint64            `json:"filesize"`
	Arch         string            `json:"arch"` // can be 'noarch' or a specific arch
	ChecksumType ChecksumType      `json:"checksum_type"`
	Checksum     string            `json:"checksum"`
	Type         BuildOutputType   `json:"type"`
	RPMs         []RPM             `json:"components,omitempty"`
	Extra        *BuildOutputExtra `json:"extra,omitempty"`
}

// CONTENT GENERATOR METADATA

// Metadata holds Koji Content Generator metadata.
// This is passed to the CGImport call.
// For more information, see https://docs.pagure.org/koji/content_generator_metadata/
type Metadata struct {
	MetadataVersion int           `json:"metadata_version"` // must be '0'
	Build           Build         `json:"build"`
	BuildRoots      []BuildRoot   `json:"buildroots"`
	Outputs         []BuildOutput `json:"output"`
}
