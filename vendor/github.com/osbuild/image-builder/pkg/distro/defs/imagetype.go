package defs

import (
	"bytes"
	"errors"
	"fmt"
	"math/rand"
	"slices"
	"text/template"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/image-builder/internal/environment"
	"github.com/osbuild/image-builder/pkg/container"
	"github.com/osbuild/image-builder/pkg/datasizes"
	"github.com/osbuild/image-builder/pkg/disk"
	"github.com/osbuild/image-builder/pkg/disk/partition"
	"github.com/osbuild/image-builder/pkg/distro"
	"github.com/osbuild/image-builder/pkg/image"
	"github.com/osbuild/image-builder/pkg/manifest"
	"github.com/osbuild/image-builder/pkg/platform"
	"github.com/osbuild/image-builder/pkg/rpmmd"
)

type imageFunc func(t *imageType, bp *blueprint.Blueprint, options distro.ImageOptions, packageSets map[string]rpmmd.PackageSet, payloadRepos []rpmmd.RepoConfig, containers []container.SourceSpec, rng *rand.Rand) (image.ImageKind, error)

// imageType implements the distro.ImageType interface
var _ = distro.ImageType(&imageType{})

type ostreeConfig struct {
	Name       string
	RemoteName string
	Ref        string
	URL        string
}

type blueprintOptions struct {
	SupportedOptions []string
	RequiredOptions  []string
}

type imageType struct {
	name        string
	nameAliases []string

	filename    string
	mimeType    string
	compression string

	packageSets map[string]rpmmd.PackageSet

	partitionTable *disk.PartitionTable

	imageConfig     distro.ImageConfig
	installerConfig distro.InstallerConfig
	isoConfig       distro.ISOConfig
	diskConfig      distro.DiskConfig

	environment environment.EnvironmentConf
	bootable    bool

	bootISO                 bool
	useLegacyAnacondaConfig bool

	isoLabel string
	variant  string

	ostree ostreeConfig

	useOstreeRemotes bool

	defaultSize            datasizes.Size
	exports                []string
	requiredPartitionSizes map[string]datasizes.Size

	installWeakDeps *bool

	diskImageVPCForceSize *bool
	diskImageGiBAligned   bool

	supportedPartitioningModes []partition.PartitioningMode

	blueprint blueprintOptions

	arch     *architecture
	platform platform.Platform

	image imageFunc

	ostreeRef string
}

func newImageTypeFrom(d *distribution, ar *architecture, imgYAML ImageTypeYAML) (imageType, error) {
	it := imageType{
		name:                       imgYAML.Name(),
		nameAliases:                imgYAML.NameAliases,
		arch:                       ar,
		environment:                imgYAML.Environment,
		filename:                   imgYAML.Filename,
		mimeType:                   imgYAML.MimeType,
		compression:                imgYAML.Compression,
		bootable:                   imgYAML.Bootable,
		bootISO:                    imgYAML.BootISO,
		useLegacyAnacondaConfig:    imgYAML.UseLegacyAnacondaConfig,
		variant:                    imgYAML.Variant,
		useOstreeRemotes:           imgYAML.UseOstreeRemotes,
		defaultSize:                imgYAML.DefaultSize,
		exports:                    imgYAML.Exports,
		requiredPartitionSizes:     imgYAML.RequiredPartitionSizes,
		installWeakDeps:            imgYAML.InstallWeakDeps,
		diskImageVPCForceSize:      imgYAML.DiskImageVPCForceSize,
		diskImageGiBAligned:        imgYAML.DiskImageGiBAligned,
		supportedPartitioningModes: imgYAML.SupportedPartitioningModes,

		blueprint: blueprintOptions{
			// The blueprint contains a few fields that are essentially
			// metadata and not configuration / customizations. These should
			// always be implicitly supported by all image types.
			SupportedOptions: append(slices.Clone(imgYAML.Blueprint.SupportedOptions), "name", "version", "description"),
			RequiredOptions:  imgYAML.Blueprint.RequiredOptions,
		},

		ostree: ostreeConfig(imgYAML.OSTree),
	}

	isoLabel, err := d.resolveISOLabel(imgYAML.ISOLabel, ar.Name())
	if err != nil {
		return imageType{}, err
	}
	it.isoLabel = isoLabel

	imageConfig := imgYAML.ImageConfig(d.ID(), ar.Name()).InheritFrom(d.ImageConfig())
	if imageConfig == nil {
		return imageType{}, fmt.Errorf("image type initialisation failed: nil image config for %s %s %s", d.Name(), ar.Name(), imgYAML.Name())
	}
	it.imageConfig = *imageConfig

	installerConfig, err := imgYAML.InstallerConfig(d.ID(), ar.Name())
	if err != nil {
		return imageType{}, err
	}
	if installerConfig != nil {
		it.installerConfig = *installerConfig
	}

	isoConfig := imgYAML.ISOConfig(d.ID(), ar.Name())
	if isoConfig != nil {
		it.isoConfig = *isoConfig
	}

	diskConfig := imgYAML.DiskConfig(d.ID(), ar.Name())
	if diskConfig != nil {
		it.diskConfig = *diskConfig
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
	case "ostree_disk":
		it.image = ostreeDiskImage
	case "ostree_commit":
		it.image = ostreeCommitImage
	case "ostree_container":
		it.image = ostreeContainerImage
	case "ostree_installer":
		it.image = ostreeInstallerImage
	case "ostree_simplified_installer":
		it.image = ostreeSimplifiedInstallerImage
	case "tar":
		it.image = tarImage
	case "network-installer":
		it.image = networkInstallerImage
	case "pxe_tar":
		it.image = pxeTarImage
	default:
		return imageType{}, fmt.Errorf("unknown image func: %v for %v", imgYAML.Image, imgYAML.Name())
	}

	if err := it.expandOSTreeRefTemplate(ar, d.ID()); err != nil {
		return imageType{}, err
	}

	basePT, err := imgYAML.PartitionTable(d.ID(), ar.Name())
	// If the image type does not define a partition table, ignore the
	// error. If a partition table is required, the pipeline generator will
	// raise an error.
	if err != nil && !errors.Is(err, ErrNoPartitionTableForImgType) && !errors.Is(err, ErrNoPartitionTableForArch) {
		return imageType{}, err
	}

	it.partitionTable = basePT

	it.packageSets = imgYAML.PackageSets(d.ID(), ar.Name())

	return it, nil
}

func (t *imageType) Name() string {
	return t.name
}

func (t *imageType) Aliases() []string {
	return t.nameAliases
}

func (t *imageType) Arch() distro.Arch {
	return t.arch
}

func (t *imageType) Filename() string {
	return t.filename
}

func (t *imageType) MIMEType() string {
	return t.mimeType
}

func (t *imageType) OSTreeRef() string {
	return t.ostreeRef
}

func (t *imageType) OSTreeURL() string {
	return t.ostree.URL
}

func (t *imageType) ISOLabel() (string, error) {
	return t.isoLabel, nil
}

func (t *imageType) Size(size uint64) uint64 {
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.Name() == "vhd" && size%datasizes.MebiByte != 0 {
		size = (size/datasizes.MebiByte + 1) * datasizes.MebiByte
	}
	if size == 0 {
		size = t.defaultSize.Uint64()
	}
	if t.diskImageGiBAligned {
		size = (size + datasizes.GibiByte - 1) &^ (datasizes.GibiByte - 1)
	}
	return size
}

func (t *imageType) PayloadPackageSets() []string {
	return []string{blueprintPkgsKey}
}

func (t *imageType) Exports() []string {
	return t.exports
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
	// TODO: make this a direct accessor that just returns the partitionTable
	// property even if it's nil. The caller can decide if not having one (nil)
	// is an error or not.
	if t.partitionTable == nil {
		return nil, fmt.Errorf("%w: %q", ErrNoPartitionTableForImgType, t.name)
	}
	return t.partitionTable, nil
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

	d, convOk := t.arch.distro.(*distribution)
	if !convOk {
		return nil, fmt.Errorf("failed to cast image type distribution %T to *distribution: this is a programming error", t.arch.distro)
	}
	defaultFsType := d.DefaultFSType
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
			RequiredMinSizes:   t.requiredPartitionSizes,
			Architecture:       t.platform.GetArch(),
			// the ESP size is not customizable either, so keep the one the
			// image type defines instead of falling back to a generic default
			ESPSize: basePartitionTable.ESPSize(),
		}
		return disk.NewCustomPartitionTable(partitioning, partOptions, nil, rng)
	}

	mountpoints := customizations.GetFilesystems()
	return disk.NewPartitionTable(basePartitionTable, mountpoints, datasizes.Size(imageSize), options.PartitioningMode, t.platform.GetArch(), t.requiredPartitionSizes, defaultFsType.String(), rng)
}

func (t *imageType) getDefaultImageConfig() *distro.ImageConfig {
	return &t.imageConfig
}

func (t *imageType) getDefaultInstallerConfig() *distro.InstallerConfig {
	return &t.installerConfig
}

func (t *imageType) getDefaultISOConfig() *distro.ISOConfig {
	return &t.isoConfig
}

func (t *imageType) getDefaultDiskConfig() *distro.DiskConfig {
	return &t.diskConfig
}

func (t *imageType) PartitionType() disk.PartitionTableType {
	basePartitionTable, err := t.BasePartitionTable()
	if errors.Is(err, ErrNoPartitionTableForImgType) {
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

	d := t.Arch().Distro()
	pkgSets := t.packageSets
	for name, pkgSet := range pkgSets {
		staticPackageSets[name] = pkgSet
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
	mf.Distro = d.IDLike()
	if mf.Distro == manifest.DISTRO_NULL {
		return nil, nil, fmt.Errorf("no distro_like set in yaml for %q", d.Name())
	}
	if options.UseBootstrapContainer {
		bootstrapPkgs, err := t.Arch().Distro().BootstrapPackages(t.arch.Name())
		if err != nil {
			return nil, nil, err
		}
		if len(bootstrapPkgs) > 0 {
			mf.Bootstrap = &manifest.BootstrapConfig{
				Packages: bootstrapPkgs,
			}
		} else {
			bootstrapContainerRef, err := t.Arch().Distro().BootstrapContainer(t.arch.Name())
			if err != nil {
				return nil, nil, err
			}
			if bootstrapContainerRef != "" {
				mf.Bootstrap = &manifest.BootstrapConfig{
					ContainerRef: bootstrapContainerRef,
				}
			}
		}
	}
	runner := d.Runner()
	_, err = img.InstantiateManifest(&mf, repos, &runner, rng)
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

	d := t.Arch().Distro()
	switch idLike := d.IDLike(); idLike {
	case manifest.DISTRO_FEDORA, manifest.DISTRO_ELN, manifest.DISTRO_EL7, manifest.DISTRO_EL10:
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
	return t.blueprint.RequiredOptions
}

func (t *imageType) SupportedBlueprintOptions() []string {
	return t.blueprint.SupportedOptions
}

func (t *imageType) expandOSTreeRefTemplate(ar *architecture, id distro.ID) error {
	if !t.isOSTreeBasedImageType() {
		return nil
	}

	subs := struct {
		Arch   string
		Distro distro.ID
	}{
		Arch:   ar.Name(),
		Distro: id,
	}

	var buf bytes.Buffer

	tmpl, err := template.New("ostree-ref").Parse(t.ostree.Ref)
	if err != nil {
		return err
	}

	if err := tmpl.Execute(&buf, subs); err != nil {
		return err
	}

	t.ostreeRef = buf.String()

	// if we're empty after templating that's an error as we can't
	// have an empty commit
	if t.ostreeRef == "" {
		return fmt.Errorf("empty ostree ref after expansion")
	}

	return nil
}

func (t *imageType) isOSTreeBasedImageType() bool {
	return t.ostree.Name != "" || t.ostree.RemoteName != "" || t.ostree.Ref != "" || t.ostree.URL != ""
}
