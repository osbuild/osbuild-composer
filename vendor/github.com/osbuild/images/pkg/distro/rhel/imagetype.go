package rhel

import (
	"fmt"
	"math/rand"

	"slices"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/experimentalflags"
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

// Default directory size minimums for all image types.
var requiredDirectorySizes = map[string]uint64{
	"/":    1 * datasizes.GiB,
	"/usr": 2 * datasizes.GiB,
}

type ImageFunc func(workload workload.Workload, t *ImageType, customizations *blueprint.Customizations, options distro.ImageOptions, packageSets map[string]rpmmd.PackageSet, containers []container.SourceSpec, rng *rand.Rand) (image.ImageKind, error)

type PackageSetFunc func(t *ImageType) (rpmmd.PackageSet, error)

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
	KernelOptions          []string
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

func (t *ImageType) IsAlmaLinux() bool {
	return t.arch.distro.IsAlmaLinux()
}

func (t *ImageType) IsAlmaLinuxKitten() bool {
	return t.arch.distro.IsAlmaLinuxKitten()
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

func (t *ImageType) BootMode() platform.BootMode {
	if t.platform.GetUEFIVendor() != "" && t.platform.GetBIOSPlatform() != "" {
		return platform.BOOT_HYBRID
	} else if t.platform.GetUEFIVendor() != "" {
		return platform.BOOT_UEFI
	} else if t.platform.GetBIOSPlatform() != "" || t.platform.GetZiplSupport() {
		return platform.BOOT_LEGACY
	}
	return platform.BOOT_NONE
}

func (t *ImageType) BasePartitionTable() (*disk.PartitionTable, error) {
	// XXX: simplify once https://github.com/osbuild/images/pull/1372
	// (or something similar) went in, see pkg/distro/fedora, once
	// the yaml based loading is in we can drop from ImageType
	// "BasePartitionTables BasePartitionTableFunc"
	if t.BasePartitionTables == nil {
		return nil, nil
	}
	basePartitionTable, exists := t.BasePartitionTables(t)
	if !exists {
		return nil, nil
	}
	return &basePartitionTable, nil
}

func (t *ImageType) GetPartitionTable(
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	rng *rand.Rand,
) (*disk.PartitionTable, error) {
	archName := t.arch.Name()

	basePartitionTable, exists := t.BasePartitionTables(t)

	if !exists {
		return nil, fmt.Errorf("no partition table defined for architecture %q for image type %q", archName, t.Name())
	}

	imageSize := t.Size(options.Size)
	partitioning, err := customizations.GetPartitioning()
	if err != nil {
		return nil, err
	}
	if partitioning != nil {
		// Use the new custom partition table to create a PT fully based on the user's customizations.
		// This overrides FilesystemCustomizations, but we should never have both defined.
		if options.Size > 0 {
			// user specified a size on the command line, so let's override the
			// customization with the calculated/rounded imageSize
			partitioning.MinSize = imageSize
		}

		partOptions := &disk.CustomPartitionTableOptions{
			PartitionTableType: basePartitionTable.Type, // PT type is not customizable, it is determined by the base PT for an image type or architecture
			BootMode:           t.BootMode(),
			DefaultFSType:      disk.FS_XFS, // default fs type for RHEL
			RequiredMinSizes:   requiredDirectorySizes,
			Architecture:       t.platform.GetArch(),
		}
		return disk.NewCustomPartitionTable(partitioning, partOptions, rng)
	}

	return disk.NewPartitionTable(&basePartitionTable, customizations.GetFilesystems(), imageSize, options.PartitioningMode, t.platform.GetArch(), nil, rng)
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

func (t *ImageType) PartitionType() disk.PartitionTableType {
	if t.BasePartitionTables == nil {
		return disk.PT_NONE
	}

	basePartitionTable, exists := t.BasePartitionTables(t)
	if !exists {
		return disk.PT_NONE
	}

	return basePartitionTable.Type
}

func (t *ImageType) Manifest(bp *blueprint.Blueprint,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	seedp *int64) (*manifest.Manifest, []string, error) {
	seed := distro.SeedFrom(seedp)

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
		pkgSets, err := getter(t)
		if err != nil {
			return nil, nil, err
		}
		staticPackageSets[name] = pkgSets
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
		// XXX: this needs to get duplicaed in exactly the same
		// way in fedora/imagetype.go
		workloadRepos := payloadRepos
		customRepos, err := bp.Customizations.GetRepositories()
		if err != nil {
			return nil, nil, err
		}
		installFromRepos := blueprint.RepoCustomizationsInstallFromOnly(customRepos)
		workloadRepos = append(workloadRepos, installFromRepos...)

		cw := &workload.Custom{
			BaseWorkload: workload.BaseWorkload{
				Repos: workloadRepos,
			},
			Packages:       bp.GetPackagesEx(false),
			EnabledModules: bp.GetEnabledModules(),
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

	if experimentalflags.Bool("no-fstab") {
		if t.DefaultImageConfig == nil {
			t.DefaultImageConfig = &distro.ImageConfig{
				MountUnits: common.ToPtr(true),
			}
		} else {
			t.DefaultImageConfig.MountUnits = common.ToPtr(true)
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
	if options.UseBootstrapContainer {
		mf.DistroBootstrapRef = bootstrapContainerFor(t)
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
	if !t.RPMOSTree && options.OSTree != nil {
		return nil, fmt.Errorf("OSTree is not supported for %q", t.Name())
	}

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

// XXX: this will become part of the yaml distro definitions, i.e.
// the yaml will have a "bootstrap_ref" key for each distro/arch
func bootstrapContainerFor(t *ImageType) string {
	distro := t.arch.distro

	if distro.IsRHEL() {
		return fmt.Sprintf("registry.access.redhat.com/ubi%s/ubi:latest", distro.Releasever())
	} else {
		// we need the toolbox container because stock centos has
		// e.g. no mount util
		return "quay.io/toolbx-images/centos-toolbox:stream" + distro.Releasever()
	}
}
