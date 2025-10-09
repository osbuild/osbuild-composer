package generic

import (
	"errors"
	"fmt"
	"math/rand"
	"slices"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/experimentalflags"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

type imageFunc func(t *imageType, bp *blueprint.Blueprint, options distro.ImageOptions, packageSets map[string]rpmmd.PackageSet, payloadRepos []rpmmd.RepoConfig, containers []container.SourceSpec, rng *rand.Rand) (image.ImageKind, error)

type isoLabelFunc func(t *imageType) string

// imageType implements the distro.ImageType interface
var _ = distro.ImageType(&imageType{})

type imageType struct {
	defs.ImageTypeYAML

	arch     *architecture
	platform platform.Platform

	image    imageFunc
	isoLabel isoLabelFunc
}

func newImageTypeFrom(d *distribution, ar *architecture, imgYAML defs.ImageTypeYAML) imageType {
	it := imageType{
		ImageTypeYAML: imgYAML,
		isoLabel:      d.getISOLabelFunc(imgYAML.ISOLabel),
	}

	switch imgYAML.Image {
	case "disk":
		it.image = diskImage
	case "container":
		it.image = containerImage
	case "image_installer":
		it.image = imageInstallerImage
	case "live_installer":
		it.image = liveInstallerImage
	case "bootable_container":
		it.image = bootableContainerImage
	case "iot":
		it.image = iotImage
	case "iot_commit":
		it.image = iotCommitImage
	case "iot_container":
		it.image = iotContainerImage
	case "iot_installer":
		it.image = iotInstallerImage
	case "iot_simplified_installer":
		it.image = iotSimplifiedInstallerImage
	case "tar":
		it.image = tarImage
	case "netinst":
		it.image = netinstImage
	case "pxe_tar":
		it.image = pxeTarImage
	default:
		err := fmt.Errorf("unknown image func: %v for %v", imgYAML.Image, imgYAML.Name())
		panic(err)
	}

	return it
}

func (t *imageType) Name() string {
	return t.ImageTypeYAML.Name()
}

func (t *imageType) Aliases() []string {
	return t.ImageTypeYAML.NameAliases
}

func (t *imageType) Arch() distro.Arch {
	return t.arch
}

func (t *imageType) Filename() string {
	return t.ImageTypeYAML.Filename
}

func (t *imageType) MIMEType() string {
	return t.ImageTypeYAML.MimeType
}

func (t *imageType) OSTreeRef() string {
	d := t.arch.distro
	if t.ImageTypeYAML.RPMOSTree {
		return fmt.Sprintf(d.OSTreeRef(), t.arch.Name())
	}
	return ""
}

func (t *imageType) ISOLabel() (string, error) {
	if !t.ImageTypeYAML.BootISO {
		return "", fmt.Errorf("image type %q is not an ISO", t.Name())
	}
	if t.isoLabel == nil {
		return "", fmt.Errorf("no iso label function for %q", t.Name())
	}

	return t.isoLabel(t), nil
}

func (t *imageType) Size(size uint64) uint64 {
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.ImageTypeYAML.Name() == "vhd" && size%datasizes.MebiByte != 0 {
		size = (size/datasizes.MebiByte + 1) * datasizes.MebiByte
	}
	if size == 0 {
		size = t.ImageTypeYAML.DefaultSize.Uint64()
	}
	return size
}

func (t *imageType) PayloadPackageSets() []string {
	return []string{blueprintPkgsKey}
}

func (t *imageType) Exports() []string {
	if len(t.ImageTypeYAML.Exports) > 0 {
		return t.ImageTypeYAML.Exports
	}
	return []string{"assembler"}
}

func (t *imageType) BootMode() platform.BootMode {
	if t.platform.GetUEFIVendor() != "" && t.platform.GetBIOSPlatform() != "" {
		return platform.BOOT_HYBRID
	} else if t.platform.GetUEFIVendor() != "" {
		return platform.BOOT_UEFI
	} else if t.platform.GetBIOSPlatform() != "" || t.platform.GetZiplSupport() {
		return platform.BOOT_LEGACY
	}
	return platform.BOOT_NONE
}

func (t *imageType) BasePartitionTable() (*disk.PartitionTable, error) {
	return t.ImageTypeYAML.PartitionTable(t.arch.distro.ID, t.arch.arch.String())
}

func (t *imageType) getPartitionTable(customizations *blueprint.Customizations, options distro.ImageOptions, rng *rand.Rand) (*disk.PartitionTable, error) {
	basePartitionTable, err := t.BasePartitionTable()
	if err != nil {
		return nil, err
	}

	imageSize := t.Size(options.Size)
	partitioning, err := customizations.GetPartitioning()
	if err != nil {
		return nil, err
	}

	defaultFsType := t.arch.distro.DefaultFSType
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
			DefaultFSType:      defaultFsType,
			RequiredMinSizes:   t.ImageTypeYAML.RequiredPartitionSizes,
			Architecture:       t.platform.GetArch(),
		}
		return disk.NewCustomPartitionTable(partitioning, partOptions, rng)
	}

	mountpoints := customizations.GetFilesystems()
	return disk.NewPartitionTable(basePartitionTable, mountpoints, datasizes.Size(imageSize), options.PartitioningMode, t.platform.GetArch(), t.ImageTypeYAML.RequiredPartitionSizes, defaultFsType.String(), rng)
}

func (t *imageType) getDefaultImageConfig() *distro.ImageConfig {
	imageConfig := t.ImageConfig(t.arch.distro.ID, t.arch.arch.String())
	return imageConfig.InheritFrom(t.arch.distro.ImageConfig())

}

func (t *imageType) getDefaultInstallerConfig() (*distro.InstallerConfig, error) {
	if !t.ImageTypeYAML.BootISO {
		return nil, fmt.Errorf("image type %q is not an ISO", t.Name())
	}
	return t.InstallerConfig(t.arch.distro.ID, t.arch.arch.String()), nil
}

func (t *imageType) PartitionType() disk.PartitionTableType {
	basePartitionTable, err := t.BasePartitionTable()
	if errors.Is(err, defs.ErrNoPartitionTableForImgType) {
		return disk.PT_NONE
	}
	if err != nil {
		panic(err)
	}

	return basePartitionTable.Type
}

func (t *imageType) Manifest(bp *blueprint.Blueprint,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	seedp *int64) (*manifest.Manifest, []string, error) {
	seed := distro.SeedFrom(seedp)

	warnings, err := t.checkOptions(bp, options)
	if err != nil {
		return nil, nil, err
	}

	// merge package sets that appear in the image type with the package sets
	// of the same name from the distro and arch
	staticPackageSets := make(map[string]rpmmd.PackageSet)

	// don't add any static packages if Minimal was selected
	if !bp.Minimal {
		pkgSets := t.ImageTypeYAML.PackageSets(t.arch.distro.ID, t.arch.arch.String())
		for name, pkgSet := range pkgSets {
			staticPackageSets[name] = pkgSet
		}
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

	customRepos, err := bp.Customizations.GetRepositories()
	if err != nil {
		return nil, nil, err
	}
	installFromRepos := blueprint.RepoCustomizationsInstallFromOnly(customRepos)
	payloadRepos = append(payloadRepos, installFromRepos...)

	if experimentalflags.Bool("no-fstab") {
		if t.ImageConfigYAML.ImageConfig != nil {
			t.ImageConfigYAML.ImageConfig = &distro.ImageConfig{}
		}
		t.ImageConfigYAML.ImageConfig.MountUnits = common.ToPtr(true)
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

	img, err := t.image(t, bp, options, staticPackageSets, payloadRepos, containerSources, rng)
	if err != nil {
		return nil, nil, err
	}
	mf := manifest.New()
	// TODO: remove the need for this entirely, the manifest has a
	// bunch of code that checks the distro currently, ideally all
	// would just be encoded in the YAML
	mf.Distro = t.arch.distro.DistroYAML.DistroLike
	if mf.Distro == manifest.DISTRO_NULL {
		return nil, nil, fmt.Errorf("no distro_like set in yaml for %q", t.arch.distro.Name())
	}
	if options.UseBootstrapContainer {
		mf.DistroBootstrapRef = bootstrapContainerFor(t)
	}
	_, err = img.InstantiateManifest(&mf, repos, &t.arch.distro.DistroYAML.Runner, rng)
	if err != nil {
		return nil, nil, err
	}

	return &mf, warnings, err
}

// checkOptions checks the validity and compatibility of options and customizations for the image type.
// Returns ([]string, error) where []string, if non-nil, will hold any generated warnings (e.g. deprecation notices).
func (t *imageType) checkOptions(bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error) {

	warnings, err := checkOptionsCommon(t, bp, options)
	if err != nil {
		return warnings, err
	}

	switch idLike := t.arch.distro.DistroYAML.DistroLike; idLike {
	case manifest.DISTRO_FEDORA, manifest.DISTRO_EL7, manifest.DISTRO_EL10:
		// no specific options checkers
	case manifest.DISTRO_EL8:
		if err := checkOptionsRhel8(t, bp); err != nil {
			return warnings, err
		}
	case manifest.DISTRO_EL9:
		if err := checkOptionsRhel9(t, bp); err != nil {
			return warnings, err
		}
	default:
		return nil, fmt.Errorf("checkOptions called with unknown distro-like %v", idLike)
	}

	return warnings, nil
}

func (t *imageType) RequiredBlueprintOptions() []string {
	return t.ImageTypeYAML.RequiredBlueprintOptions
}

func (t *imageType) SupportedBlueprintOptions() []string {
	// The blueprint contains a few fields that are essentially metadata and
	// not configuration / customizations. These should always be implicitly
	// supported by all image types.
	return append(t.ImageTypeYAML.SupportedBlueprintOptions, "name", "version", "description")
}

func bootstrapContainerFor(t *imageType) string {
	return t.arch.distro.DistroYAML.BootstrapContainers[t.arch.arch]
}
