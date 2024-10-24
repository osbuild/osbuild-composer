package rhel

import (
	"fmt"
	"math/rand"

	"slices"

	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

const (
	// package set names

	// build package set name
	BuildPkgsKey = "build"

	// main/common os image package set name
	OSPkgsKey = "os"

	// container package set name
	ContainerPkgsKey = "container"

	// installer package set name
	InstallerPkgsKey = "installer"

	// blueprint package set name
	BlueprintPkgsKey = "blueprint"
)

type ImageFunc func(workload workload.Workload, t *ImageType, customizations *blueprint.Customizations, options distro.ImageOptions, packageSets map[string]rpmmd.PackageSet, containers []container.SourceSpec, rng *rand.Rand) (image.ImageKind, error)

type PackageSetFunc func(t *ImageType) rpmmd.PackageSet

type BasePartitionTableFunc func(t *ImageType) (disk.PartitionTable, bool)

type ISOLabelFunc func(t *ImageType) string

type CheckOptionsFunc func(t *ImageType, bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error)

type ImageType struct {
	// properties, which are part of the distro.ImageType interface or are used by all images
	name             string
	filename         string
	mimeType         string
	packageSets      map[string]PackageSetFunc
	buildPipelines   []string
	payloadPipelines []string
	exports          []string
	image            ImageFunc

	// properties which can't be set when defining the image type
	arch     *Architecture
	platform platform.Platform

	Environment            environment.Environment
	Workload               workload.Workload
	NameAliases            []string
	Compression            string // TODO: remove from image definition and make it a transport option
	DefaultImageConfig     *distro.ImageConfig
	DefaultInstallerConfig *distro.InstallerConfig
	KernelOptions          string
	DefaultSize            uint64

	// bootISO: installable ISO
	BootISO bool
	// rpmOstree: edge/ostree
	RPMOSTree bool
	// bootable image
	Bootable bool
	// List of valid arches for the image type
	BasePartitionTables BasePartitionTableFunc
	// Optional list of unsupported partitioning modes
	UnsupportedPartitioningModes []disk.PartitioningMode

	ISOLabelFn ISOLabelFunc

	// TODO: determine a better place for these options, but for now they are here
	DiskImagePartTool     *osbuild.PartTool
	DiskImageVPCForceSize *bool
}

func (t *ImageType) Name() string {
	return t.name
}

func (t *ImageType) Arch() distro.Arch {
	return t.arch
}

func (t *ImageType) Filename() string {
	return t.filename
}

func (t *ImageType) MIMEType() string {
	return t.mimeType
}

func (t *ImageType) OSTreeRef() string {
	d := t.arch.distro
	if t.RPMOSTree {
		return fmt.Sprintf(d.ostreeRefTmpl, t.Arch().Name())
	}
	return ""
}

// IsRHEL returns true if the image type is part of a RHEL distribution
//
// This is a convenience method, because external packages can't get the
// information from t.Arch().Distro(), since the distro.Distro interface
// does not have this method. And since the distro.Distro interface is
// distro-agnostic, it does not make much sense to have a method like this
// in the interface.
func (t *ImageType) IsRHEL() bool {
	return t.arch.distro.IsRHEL()
}

func (t *ImageType) ISOLabel() (string, error) {
	if !t.BootISO {
		return "", fmt.Errorf("image type %q is not an ISO", t.name)
	}

	if t.ISOLabelFn != nil {
		return t.ISOLabelFn(t), nil
	}

	return "", nil
}

func (t *ImageType) Size(size uint64) uint64 {
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.name == "vhd" && size%datasizes.MebiByte != 0 {
		size = (size/datasizes.MebiByte + 1) * datasizes.MebiByte
	}
	if size == 0 {
		size = t.DefaultSize
	}
	return size
}

func (t *ImageType) BuildPipelines() []string {
	return t.buildPipelines
}

func (t *ImageType) PayloadPipelines() []string {
	return t.payloadPipelines
}

func (t *ImageType) PayloadPackageSets() []string {
	return []string{BlueprintPkgsKey}
}

func (t *ImageType) Exports() []string {
	if len(t.exports) > 0 {
		return t.exports
	}
	return []string{"assembler"}
}

func (t *ImageType) BootMode() distro.BootMode {
	if t.platform.GetUEFIVendor() != "" && t.platform.GetBIOSPlatform() != "" {
		return distro.BOOT_HYBRID
	} else if t.platform.GetUEFIVendor() != "" {
		return distro.BOOT_UEFI
	} else if t.platform.GetBIOSPlatform() != "" || t.platform.GetZiplSupport() {
		return distro.BOOT_LEGACY
	}
	return distro.BOOT_NONE
}

func (t *ImageType) GetPartitionTable(
	mountpoints []blueprint.FilesystemCustomization,
	options distro.ImageOptions,
	rng *rand.Rand,
) (*disk.PartitionTable, error) {
	archName := t.arch.Name()

	basePartitionTable, exists := t.BasePartitionTables(t)

	if !exists {
		return nil, fmt.Errorf("no partition table defined for architecture %q for image type %q", archName, t.Name())
	}

	imageSize := t.Size(options.Size)

	return disk.NewPartitionTable(&basePartitionTable, mountpoints, imageSize, options.PartitioningMode, nil, rng)
}

func (t *ImageType) getDefaultImageConfig() *distro.ImageConfig {
	// ensure that image always returns non-nil default config
	imageConfig := t.DefaultImageConfig
	if imageConfig == nil {
		imageConfig = &distro.ImageConfig{}
	}
	return imageConfig.InheritFrom(t.arch.distro.GetDefaultImageConfig())

}

func (t *ImageType) getDefaultInstallerConfig() (*distro.InstallerConfig, error) {
	if !t.BootISO {
		return nil, fmt.Errorf("image type %q is not an ISO", t.name)
	}

	return t.DefaultInstallerConfig, nil
}

func (t *ImageType) PartitionType() string {
	basePartitionTable, exists := t.BasePartitionTables(t)
	if !exists {
		return ""
	}

	return basePartitionTable.Type
}

func (t *ImageType) Manifest(bp *blueprint.Blueprint,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	seed int64) (*manifest.Manifest, []string, error) {

	if t.Workload != nil {
		// For now, if an image type defines its own workload, don't allow any
		// user customizations.
		// Soon we will have more workflows and each will define its allowed
		// set of customizations.  The current set of customizations defined in
		// the blueprint spec corresponds to the Custom workflow.
		if bp.Customizations != nil {
			return nil, nil, fmt.Errorf(distro.NoCustomizationsAllowedError, t.Name())
		}
	}

	warnings, err := t.checkOptions(bp, options)
	if err != nil {
		return nil, nil, err
	}

	// merge package sets that appear in the image type with the package sets
	// of the same name from the distro and arch
	staticPackageSets := make(map[string]rpmmd.PackageSet)

	for name, getter := range t.packageSets {
		staticPackageSets[name] = getter(t)
	}

	// amend with repository information and collect payload repos
	payloadRepos := make([]rpmmd.RepoConfig, 0)
	for _, repo := range repos {
		if len(repo.PackageSets) > 0 {
			// only apply the repo to the listed package sets
			for _, psName := range repo.PackageSets {
				if slices.Contains(t.PayloadPackageSets(), psName) {
					payloadRepos = append(payloadRepos, repo)
				}
				ps := staticPackageSets[psName]
				ps.Repositories = append(ps.Repositories, repo)
				staticPackageSets[psName] = ps
			}
		}
	}

	w := t.Workload
	if w == nil {
		cw := &workload.Custom{
			BaseWorkload: workload.BaseWorkload{
				Repos: payloadRepos,
			},
			Packages: bp.GetPackagesEx(false),
		}
		if services := bp.Customizations.GetServices(); services != nil {
			cw.Services = services.Enabled
			cw.DisabledServices = services.Disabled
		}
		w = cw
	}

	containerSources := make([]container.SourceSpec, len(bp.Containers))
	for idx, cont := range bp.Containers {
		containerSources[idx] = container.SourceSpec{
			Source:    cont.Source,
			Name:      cont.Name,
			TLSVerify: cont.TLSVerify,
			Local:     cont.LocalStorage,
		}
	}

	source := rand.NewSource(seed)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	img, err := t.image(w, t, bp.Customizations, options, staticPackageSets, containerSources, rng)
	if err != nil {
		return nil, nil, err
	}
	mf := manifest.New()

	switch t.Arch().Distro().Releasever() {
	case "7":
		mf.Distro = manifest.DISTRO_EL7
	case "8":
		mf.Distro = manifest.DISTRO_EL8
	case "9":
		mf.Distro = manifest.DISTRO_EL9
	case "10":
		mf.Distro = manifest.DISTRO_EL10
	default:
		return nil, nil, fmt.Errorf("unsupported distro release version: %s", t.Arch().Distro().Releasever())
	}

	_, err = img.InstantiateManifest(&mf, repos, t.arch.distro.runner, rng)
	if err != nil {
		return nil, nil, err
	}

	return &mf, warnings, err
}

// checkOptions checks the validity and compatibility of options and customizations for the image type.
// Returns ([]string, error) where []string, if non-nil, will hold any generated warnings (e.g. deprecation notices).
func (t *ImageType) checkOptions(bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error) {
	if t.arch.distro.CheckOptions != nil {
		return t.arch.distro.CheckOptions(t, bp, options)
	}

	return nil, nil
}

func NewImageType(
	name, filename, mimeType string,
	pkgSets map[string]PackageSetFunc,
	imgFunc ImageFunc,
	buildPipelines, payloadPipelines, exports []string,
) *ImageType {
	return &ImageType{
		name:             name,
		filename:         filename,
		mimeType:         mimeType,
		packageSets:      pkgSets,
		image:            imgFunc,
		buildPipelines:   buildPipelines,
		payloadPipelines: payloadPipelines,
		exports:          exports,
	}
}
