package generic

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/bootc"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/runner"
)

type BootcDistro struct {
	imgref          string
	imageID         string
	buildImgref     string
	buildImageID    string
	sourceInfo      *osinfo.Info
	buildSourceInfo *osinfo.Info

	id            distro.ID
	defaultFs     string
	releasever    string
	rootfsMinSize uint64

	arches map[string]distro.Arch
}

const (
	// As a baseline heuristic we double the size of the input container to
	// support in-place updates. We plan to make this configurable in the
	// future.
	containerSizeToDiskSizeMultiplier = 2
)

// NewBootc returns a distro initialised with the provided information. All
// required information must be defined. There are no restrictions or
// requirements for the name of the distro and it is only used to identify this
// particular instance of the distribution. The name bootc is commonly used,
// unless multiple instances are created.
// To generate the [github.com/osbuild/images/pkg/bootc.Info] from a container
// ref, use the [github.com/osbuild/images/pkg/bootc.Container] type and its
// methods.
func NewBootc(name string, cinfo *bootc.Info) (*BootcDistro, error) {
	if cinfo == nil {
		return nil, errors.New("failed to initialize bootc distro: container info is empty")
	}

	// verify required information
	var missing []string
	if cinfo.Imgref == "" {
		missing = append(missing, "Imgref")
	}
	// NOTE: Manifest generation for bootc-based images requires resolving the
	// container ID through the traditional, application container resolver,
	// and passed as a container spec to the serialize function. If we resolve
	// the ImageID here, we wont need to do that second container resolve and
	// we can keep the bootc-container information resolution in one place,
	// instead of needing to resolve most information using pkg/bootc and just
	// the image ID using pkg/container.
	// After being copied to the BootcDistro struct, the ImageID has no effect,
	// so we shouldn't require it, but we'll keep setting it until we need it
	// to replace the requirement for the separate resolve operation.
	// if cinfo.ImageID == "" {
	// 	missing = append(missing, "ImageID")
	// }
	if cinfo.Arch == "" {
		missing = append(missing, "Arch")
	}
	if cinfo.DefaultRootFs == "" {
		missing = append(missing, "DefaultRootFs")
	}
	if cinfo.Size == 0 {
		missing = append(missing, "Size")
	}

	// The following may not be strictly required, but they were used to define
	// the name of the distribution in the previous implementation of the bootc
	// distro. We set them to required and use them the same way for now, but
	// will likely drop this requirement and the naming behaviour later.
	if cinfo.OSInfo == nil {
		missing = append(missing, "OSInfo")
	} else {
		if cinfo.OSInfo.OSRelease.ID == "" {
			missing = append(missing, "OSInfo.OSRelease.ID")
		}
		if cinfo.OSInfo.OSRelease.VersionID == "" {
			missing = append(missing, "OSInfo.OSRelease.VersionID")
		}
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("failed to initialize bootc distro: missing required info: %s", strings.Join(missing, ", "))
	}

	osInfo := cinfo.OSInfo

	// Append the ID and version ID from the container to the distro name. This
	// is consistent with the previous implementation from bootc-image-builder.
	// It may be removed to allow the caller to set the ID directly based on
	// the name string.
	nameVer := fmt.Sprintf("%s-%s-%s", name, osInfo.OSRelease.ID, osInfo.OSRelease.VersionID)
	id, err := distro.ParseID(nameVer)
	if err != nil {
		return nil, err
	}

	d := &BootcDistro{
		// the ID is technically not allowed by the ID parser, as it doesn't
		// contain a version, but we will relax this requirement later
		id:              *id,
		imgref:          cinfo.Imgref,
		imageID:         cinfo.ImageID,
		buildImgref:     cinfo.Imgref, // default to using the same image for build for now
		sourceInfo:      osInfo,
		buildSourceInfo: osInfo,
		defaultFs:       cinfo.DefaultRootFs,
		releasever:      osInfo.OSRelease.VersionID,
		rootfsMinSize:   cinfo.Size * containerSizeToDiskSizeMultiplier,
	}

	// load image types from bootc-generic-1
	distroYAML, err := defs.LoadDistroWithoutImageTypes("bootc-generic-1")
	if err != nil {
		return nil, fmt.Errorf("failed to load bootc image types: %w", err)
	}
	defaultFs, err := disk.NewFSType(cinfo.DefaultRootFs)
	if err != nil {
		return nil, fmt.Errorf("failed to set default rootfs for bootc distro: %w", err)
	}
	distroYAML.DefaultFSType = defaultFs
	if err := distroYAML.LoadImageTypes(); err != nil {
		return nil, fmt.Errorf("failed to load bootc distro image types: %w", err)
	}

	// initialise a single architecture to match the architecture of the bootc
	// container
	archi, err := arch.FromString(cinfo.Arch)
	if err != nil {
		return nil, fmt.Errorf("failed to set bootc distro architecture: %w", err)
	}

	ba := &architecture{
		distro:     d,
		arch:       archi,
		imageTypes: map[string]distro.ImageType{},
	}
	for _, imgTypeYaml := range distroYAML.ImageTypes() {
		err := ba.addBootcImageType(bootcImageType{
			ImageTypeYAML: imgTypeYaml,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to add image type to bootc distro: %w", err)
		}
	}
	d.arches = map[string]distro.Arch{ba.Name(): ba}

	return d, nil
}

func (d *BootcDistro) Name() string {
	return d.id.String()
}

func (d *BootcDistro) ID() distro.ID {
	return d.id
}

func (d *BootcDistro) IDLike() manifest.Distro {
	// not applicable or needed for bootc
	return 0
}

func (d *BootcDistro) Codename() string {
	return ""
}

func (d *BootcDistro) Releasever() string {
	return d.releasever
}

func (d *BootcDistro) OsVersion() string {
	return d.releasever
}

func (d *BootcDistro) Product() string {
	return d.id.String()
}

func (d *BootcDistro) ModulePlatformID() string {
	return ""
}

func (d *BootcDistro) ListArches() []string {
	return slices.Sorted(maps.Keys(d.arches))
}

func (d *BootcDistro) GetArch(arch string) (distro.Arch, error) {
	a, exists := d.arches[arch]
	if !exists {
		return nil, fmt.Errorf("requested bootc arch %q does not match available arches %v", arch, slices.Collect(maps.Keys(d.arches)))
	}
	return a, nil
}

func (d *BootcDistro) GetTweaks() *distro.Tweaks {
	// The bootc distro does not require or support tweaks (yet)
	return nil
}

func (d *BootcDistro) Runner() runner.RunnerConf {
	// To get the bootc distro runner, use [bootc.GetDistroAndRunner]
	return runner.RunnerConf{}
}

func (d *BootcDistro) ImageConfig() *distro.ImageConfig {
	// not applicable for bootc distro
	return nil
}

// SetBuildContainer configures the build to use a separate container for the
// build root.
// To generate the [github.com/osbuild/images/pkg/bootc.Info] from a container
// ref, use the [github.com/osbuild/images/pkg/bootc.Container] type and its
// methods.
func (d *BootcDistro) SetBuildContainer(cinfo *bootc.Info) error {
	if cinfo == nil {
		return errors.New("failed to set build container for bootc distro: container info is empty")
	}

	// verify required information
	var missing []string
	if cinfo.Imgref == "" {
		missing = append(missing, "Imgref")
	}
	if cinfo.Arch == "" {
		missing = append(missing, "Arch")
	}
	if len(missing) > 0 {
		return fmt.Errorf("failed to set build container for bootc distro: missing required info: %s", strings.Join(missing, ", "))
	}

	// TODO: make ImageID a requirement when we start using it instead of
	// requiring a container spec in the serialize function (see note in
	// NewBootc()).

	// use the arch package to resolve architecture name aliases (amd64 ->
	// x86_64) and to verify that the architecture is supported
	buildArch, err := arch.FromString(cinfo.Arch)
	if err != nil {
		return fmt.Errorf("failed to determine architecture of build container for bootc distro: %w", err)
	}

	distroArches := d.ListArches()
	if len(distroArches) != 1 {
		// there should only ever be one architecture for a bootc distro
		return fmt.Errorf("found %d architectures for bootc distro while setting build container: bootc distro should have exactly 1 architecture", len(distroArches))
	}

	// build container arch must match the base container arch
	if _, err := d.GetArch(buildArch.String()); err != nil {
		baseArch := distroArches[0]
		return fmt.Errorf("failed to set build container for bootc distro: build container architecture %q does not match base container %q", buildArch, baseArch)
	}

	d.buildImgref = cinfo.Imgref
	d.buildImageID = cinfo.ImageID
	d.buildSourceInfo = cinfo.OSInfo

	return nil
}

func (d *BootcDistro) BootstrapContainer(a string) (string, error) {
	// NOTE: Return the build container ref instead and we will unify these two
	// concepts later.

	// verify that the architecture matches the bootc distro's architecture
	// (there should be only one)
	distroArches := d.ListArches()
	if len(distroArches) != 1 {
		// there should only ever be one architecture for a bootc distro
		return "", fmt.Errorf("found %d architectures for bootc distro while getting build container: bootc distro should have exactly 1 architecture", len(distroArches))
	}
	if _, err := d.GetArch(a); err != nil {
		baseArch := distroArches[0]
		return "", fmt.Errorf("failed to get build container for bootc distro: requested container architecture %q does not match base container %q", a, baseArch)
	}

	return d.buildImgref, nil
}
