package bootc

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/cmdutil"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/policies"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

var _ = distro.ImageType(&imageType{})

type imageType struct {
	defs.ImageTypeYAML

	arch *Arch
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
	return ""
}

func (t *imageType) ISOLabel() (string, error) {
	return "", nil
}

func (t *imageType) Size(size uint64) uint64 {
	if size == 0 {
		size = 1073741824
	}
	return size
}

func (t *imageType) PartitionType() disk.PartitionTableType {
	// XXX: duplicated from generic/imagetype.go
	basePartitionTable, err := t.BasePartitionTable()
	if errors.Is(err, defs.ErrNoPartitionTableForImgType) {
		return disk.PT_NONE
	}
	if err != nil {
		panic(err)
	}

	return basePartitionTable.Type
}

func (t *imageType) BasePartitionTable() (*disk.PartitionTable, error) {
	return t.ImageTypeYAML.PartitionTable(t.arch.distro.id, t.arch.arch.String())
}

func (t *imageType) BootMode() platform.BootMode {
	// We really never want HYBRID or LEGACY on aarch64 platforms. In the future
	// it might be much nicer to take the same apporach as `Bootmode()` in the
	// generic distro but that's a bit more involved. Let's start here.
	if t.arch.arch == arch.ARCH_AARCH64 {
		return platform.BOOT_UEFI
	}

	return platform.BOOT_HYBRID
}

func (t *imageType) PayloadPackageSets() []string {
	return nil
}

func (t *imageType) Exports() []string {
	return t.ImageTypeYAML.Exports
}

func (t *imageType) SupportedBlueprintOptions() []string {
	// The blueprint contains a few fields that are essentially metadata and
	// not configuration / customizations. These should always be implicitly
	// supported by all image types.
	return append(t.ImageTypeYAML.Blueprint.SupportedOptions, "name", "version", "description")
}

func (t *imageType) RequiredBlueprintOptions() []string {
	return nil
}

// keep in sync with "generic/imagetype.go:checkOptions()"
func (t *imageType) checkOptions(bp *blueprint.Blueprint) []string {
	if bp == nil {
		return nil
	}

	if err := distro.ValidateConfig(t, *bp); err != nil {
		errPrefix := fmt.Sprintf("blueprint validation failed for image type %q", t.Name())
		// NOTE (validation-warnings): appending to warnings now, because this
		// is breaking a lot of things the service
		errAsWarning := fmt.Errorf("%s: %w", errPrefix, err)
		return []string{errAsWarning.Error()}
	}
	return nil
}

func (t *imageType) Manifest(bp *blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig, seedp *int64) (*manifest.Manifest, []string, error) {
	validationWarnings := t.checkOptions(bp)

	mani, manifestWarnings, err := t.manifestWithoutValidation(bp, options)
	return mani, append(validationWarnings, manifestWarnings...), err
}

func (t *imageType) manifestWithoutValidation(bp *blueprint.Blueprint, options distro.ImageOptions) (*manifest.Manifest, []string, error) {
	seed, err := cmdutil.SeedArgFor(nil, t.arch.Name(), t.arch.distro.Name())
	if err != nil {
		return nil, nil, err
	}
	//nolint:gosec
	rng := rand.New(rand.NewSource(seed))

	switch t.Image {
	case "bootc_legacy_iso":
		return t.manifestForLegacyISO(bp, rng)
	case "bootc_iso":
		return t.manifestForISO(bp, options, rng)
	case "bootc_generic_iso":
		return t.manifestForGenericISO(options, rng)
	case "bootc_disk":
		return t.manifestForDisk(bp, options, rng)
	case "pxe_tar":
		return t.manifestForPXETar(bp, options, rng)
	default:
		err := fmt.Errorf("unknown image func: %v for %v", t.Image, t.Name())
		panic(err)
	}
}

func (t *imageType) manifestForDisk(bp *blueprint.Blueprint, options distro.ImageOptions, rng *rand.Rand) (*manifest.Manifest, []string, error) {
	if t.arch.distro.imgref == "" {
		return nil, nil, fmt.Errorf("internal error: no base image defined")
	}
	containerSource := container.SourceSpec{
		Source: t.arch.distro.imgref,
		Name:   t.arch.distro.imgref,
		Local:  true,
	}
	buildContainerSource := container.SourceSpec{
		Source: t.arch.distro.buildImgref,
		Name:   t.arch.distro.buildImgref,
		Local:  true,
	}

	var customizations *blueprint.Customizations
	if bp != nil {
		customizations = bp.Customizations
	}

	platform := PlatformFor(t.arch.Name(), t.arch.distro.sourceInfo.UEFIVendor)
	// For the bootc-disk image, the filename is the basename and
	// the extension is added automatically for each disk format
	filename := strings.Split(t.Filename(), ".")[0]

	img := image.NewBootcDiskImage(platform, filename, containerSource, buildContainerSource)
	img.OSCustomizations.Users = users.UsersFromBP(customizations.GetUsers())

	groups, err := customizations.GetGroups()
	if err != nil {
		return nil, nil, err
	}
	img.OSCustomizations.Groups = users.GroupsFromBP(groups)
	img.OSCustomizations.SELinux = t.arch.distro.sourceInfo.SELinuxPolicy
	img.OSCustomizations.BuildSELinux = img.OSCustomizations.SELinux
	if t.arch.distro.buildSourceInfo != nil {
		img.OSCustomizations.BuildSELinux = t.arch.distro.buildSourceInfo.SELinuxPolicy
	}
	if t.arch.distro.sourceInfo != nil && t.arch.distro.sourceInfo.MountConfiguration != nil {
		img.OSCustomizations.MountConfiguration = *t.arch.distro.sourceInfo.MountConfiguration
	}

	imageConfig := t.ImageTypeYAML.ImageConfig(t.arch.distro.id, t.arch.Name())
	if imageConfig != nil {
		img.OSCustomizations.KernelOptionsAppend = imageConfig.KernelOptions
	}
	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.OSCustomizations.KernelOptionsAppend = append(img.OSCustomizations.KernelOptionsAppend, kopts.Append)
	}

	rootfsMinSize := max(t.arch.distro.rootfsMinSize, options.Size)

	pt, err := t.genPartitionTable(customizations, rootfsMinSize, rng)
	if err != nil {
		return nil, nil, err
	}
	img.PartitionTable = pt

	// Check Directory/File Customizations are valid
	dc := customizations.GetDirectories()
	fc := customizations.GetFiles()
	if err := blueprint.ValidateDirFileCustomizations(dc, fc); err != nil {
		return nil, nil, err
	}
	if err := blueprint.CheckDirectoryCustomizationsPolicy(dc, policies.OstreeCustomDirectoriesPolicies); err != nil {
		return nil, nil, err
	}
	if err := blueprint.CheckFileCustomizationsPolicy(fc, policies.OstreeCustomFilesPolicies); err != nil {
		return nil, nil, err
	}
	img.OSCustomizations.Files, err = blueprint.FileCustomizationsToFsNodeFiles(fc)
	if err != nil {
		return nil, nil, err
	}
	img.OSCustomizations.Directories, err = blueprint.DirectoryCustomizationsToFsNodeDirectories(dc)
	if err != nil {
		return nil, nil, err
	}

	mf := manifest.New()
	mf.Distro = manifest.DISTRO_FEDORA
	runner := &runner.Linux{}

	if err := img.InstantiateManifestFromContainers(&mf, []container.SourceSpec{containerSource}, runner, rng); err != nil {
		return nil, nil, err
	}

	return &mf, nil, nil
}

func (t *imageType) initAnacondaInstallerBaseFromSourceInfo(img *image.AnacondaInstallerBase, sourceInfo *osinfo.Info, customizations *blueprint.Customizations) error {
	img.RootfsCompression = "zstd"

	if t.arch.Name() == arch.ARCH_X86_64.String() {
		img.ISOCustomizations.BootType = manifest.Grub2ISOBoot
	}

	img.InstallerCustomizations.Product = sourceInfo.OSRelease.Name
	img.InstallerCustomizations.OSVersion = sourceInfo.OSRelease.VersionID
	img.ISOCustomizations.Label = LabelForISO(&sourceInfo.OSRelease, t.arch.Name())

	img.InstallerCustomizations.FIPS = customizations.GetFIPS()
	var err error
	img.Kickstart, err = kickstart.New(customizations)
	if err != nil {
		return err
	}
	img.Kickstart.Path = osbuild.KickstartPathOSBuild
	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.Kickstart.KernelOptionsAppend = append(img.Kickstart.KernelOptionsAppend, kopts.Append)
	}
	img.Kickstart.NetworkOnBoot = true

	instCust, err := customizations.GetInstaller()
	if err != nil {
		return err
	}
	if instCust != nil && instCust.Modules != nil {
		img.InstallerCustomizations.EnabledAnacondaModules = append(img.InstallerCustomizations.EnabledAnacondaModules, instCust.Modules.Enable...)
		img.InstallerCustomizations.DisabledAnacondaModules = append(img.InstallerCustomizations.DisabledAnacondaModules, instCust.Modules.Disable...)
	}
	img.InstallerCustomizations.EnabledAnacondaModules = append(img.InstallerCustomizations.EnabledAnacondaModules,
		anaconda.ModuleUsers,
		anaconda.ModuleServices,
		anaconda.ModuleSecurity,
		// XXX: get from the imagedefs
		anaconda.ModuleNetwork,
		anaconda.ModulePayloads,
		anaconda.ModuleRuntime,
		anaconda.ModuleStorage,
	)
	if bpKernel := customizations.GetKernel(); bpKernel.Append != "" {
		img.InstallerCustomizations.KernelOptionsAppend = append(img.InstallerCustomizations.KernelOptionsAppend, bpKernel.Append)
	}

	img.Kickstart.OSTree = &kickstart.OSTree{
		OSName: "default",
	}

	// see https://github.com/osbuild/bootc-image-builder/issues/733
	img.ISOCustomizations.RootfsType = manifest.SquashfsRootfs

	// Enabled by default to keep backwards compatibility
	img.InstallerCustomizations.InstallWeakDeps = true

	return nil
}

func (t *imageType) manifestForISO(bp *blueprint.Blueprint, options distro.ImageOptions, rng *rand.Rand) (*manifest.Manifest, []string, error) {
	if t.arch.distro.imgref == "" {
		return nil, nil, fmt.Errorf("internal error in bootc iso: no base image defined")
	}
	if options.Bootc == nil || options.Bootc.InstallerPayloadRef == "" {
		return nil, nil, fmt.Errorf("no installer payload bootc ref set")
	}
	payloadRef := options.Bootc.InstallerPayloadRef
	imgref := t.arch.distro.imgref
	containerSource := container.SourceSpec{
		Source: imgref,
		Name:   imgref,
		Local:  true,
	}
	sourceInfo := t.arch.distro.sourceInfo
	// XXX: keep it simple for now, we may allow this in the future
	if t.arch.distro.buildImgref != t.arch.distro.imgref {
		return nil, nil, fmt.Errorf("cannot use build-containers with anaconda installer images")
	}

	var customizations *blueprint.Customizations
	if bp != nil {
		customizations = bp.Customizations
	}

	platformi := PlatformFor(t.arch.Name(), sourceInfo.UEFIVendor)
	platformi.ImageFormat = platform.FORMAT_ISO

	img := image.NewAnacondaContainerInstaller(platformi, t.Filename(), containerSource)
	if err := t.initAnacondaInstallerBaseFromSourceInfo(&img.AnacondaInstallerBase, sourceInfo, customizations); err != nil {
		return nil, nil, err
	}
	img.ContainerRemoveSignatures = true
	// we auto-detect the lorax config from the source info
	img.InstallerCustomizations.LoraxTemplates = LoraxTemplates(sourceInfo.OSRelease)
	img.InstallerCustomizations.LoraxTemplatePackage = LoraxTemplatePackage(sourceInfo.OSRelease)

	// kernelVer is used by dracut
	img.KernelVer = sourceInfo.KernelInfo.Version
	img.KernelPath = fmt.Sprintf("lib/modules/%s/vmlinuz", sourceInfo.KernelInfo.Version)
	img.InitramfsPath = fmt.Sprintf("lib/modules/%s/initramfs.img", sourceInfo.KernelInfo.Version)
	img.InstallerHome = "/var/roothome"
	payloadSource := container.SourceSpec{
		Source: payloadRef,
		Name:   payloadRef,
		Local:  true,
	}
	img.InstallerPayload = payloadSource

	installRootfsType, err := disk.NewFSType(t.arch.distro.defaultFs)
	if err != nil {
		return nil, nil, err
	}
	img.InstallRootfsType = installRootfsType

	mf := manifest.New()

	foundDistro, foundRunner, err := GetDistroAndRunner(sourceInfo.OSRelease)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to infer distro and runner: %w", err)
	}
	mf.Distro = foundDistro

	_, err = img.InstantiateManifestFromContainer(&mf, []container.SourceSpec{containerSource}, foundRunner, rng)
	return &mf, nil, err
}

func (t *imageType) manifestForGenericISO(options distro.ImageOptions, rng *rand.Rand) (*manifest.Manifest, []string, error) {
	if t.arch.distro.imgref == "" {
		return nil, nil, fmt.Errorf("internal error: no base image defined")
	}

	containerSource := container.SourceSpec{
		Source: t.arch.distro.imgref,
		Name:   t.arch.distro.imgref,
		Local:  true,
	}

	platformi := PlatformFor(t.arch.Name(), t.arch.distro.sourceInfo.UEFIVendor)
	platformi.ImageFormat = platform.FORMAT_ISO

	img := image.NewContainerBasedIso(platformi, t.Filename(), containerSource)
	if options.Bootc != nil && options.Bootc.InstallerPayloadRef != "" {
		img.PayloadContainer = &container.SourceSpec{
			Source: options.Bootc.InstallerPayloadRef,
			Name:   options.Bootc.InstallerPayloadRef,
			Local:  true,
		}
	}
	img.RootfsCompression = "zstd"
	img.RootfsType = manifest.SquashfsRootfs
	img.KernelPath = fmt.Sprintf("lib/modules/%s/vmlinuz", t.arch.distro.sourceInfo.KernelInfo.Version)
	img.InitramfsPath = fmt.Sprintf("lib/modules/%s/initramfs.img", t.arch.distro.sourceInfo.KernelInfo.Version)
	img.Product = t.arch.distro.sourceInfo.OSRelease.Name
	img.Version = t.arch.distro.sourceInfo.OSRelease.VersionID
	img.Release = t.arch.distro.sourceInfo.OSRelease.VersionID

	isoi := t.arch.distro.sourceInfo.ISOInfo

	if isoi.Label != "" {
		img.ISOLabel = isoi.Label
	} else {
		img.ISOLabel = LabelForISO(&t.arch.distro.sourceInfo.OSRelease, t.arch.Name())
	}

	if len(isoi.KernelArgs) > 0 {
		img.KernelOpts = isoi.KernelArgs
	}

	img.Grub2MenuDefault = isoi.Grub2.Default
	img.Grub2MenuTimeout = isoi.Grub2.Timeout
	img.Grub2MenuEntries = []manifest.ISOGrub2MenuEntry{}

	for _, entry := range isoi.Grub2.Entries {
		img.Grub2MenuEntries = append(img.Grub2MenuEntries, manifest.ISOGrub2MenuEntry{
			Name:   entry.Name,
			Linux:  entry.Linux,
			Initrd: entry.Initrd,
		})
	}

	mf := manifest.New()

	foundDistro, foundRunner, err := GetDistroAndRunner(t.arch.distro.sourceInfo.OSRelease)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to infer distro and runner: %w", err)
	}
	mf.Distro = foundDistro

	_, err = img.InstantiateManifestFromContainer(&mf, []container.SourceSpec{containerSource}, foundRunner, rng)
	return &mf, nil, err
}

// newDistroYAMLFrom() returns the distroYAML for the given sourceInfo,
// if no direct match can be found it will it will use the ID_LIKE.
// This should ensure we work on every bootc image that puts a correct
// ID_LIKE= in /etc/os-release
func newDistroYAMLFrom(sourceInfo *osinfo.Info) (*defs.DistroYAML, *distro.ID, error) {
	for _, distroID := range append([]string{sourceInfo.OSRelease.ID}, sourceInfo.OSRelease.IDLike...) {
		nameVer := fmt.Sprintf("%s-%s", distroID, sourceInfo.OSRelease.VersionID)
		id, err := distro.ParseID(nameVer)
		if err != nil {
			return nil, nil, err
		}
		distroYAML, err := defs.NewDistroYAML(nameVer)
		if err != nil {
			return nil, nil, err
		}
		if distroYAML != nil {
			return distroYAML, id, nil
		}
	}
	return nil, nil, fmt.Errorf("cannot load distro definitions for %s-%s or any of %v", sourceInfo.OSRelease.ID, sourceInfo.OSRelease.VersionID, sourceInfo.OSRelease.IDLike)
}

func (t *imageType) manifestForLegacyISO(bp *blueprint.Blueprint, rng *rand.Rand) (*manifest.Manifest, []string, error) {
	if t.arch.distro.imgref == "" {
		return nil, nil, fmt.Errorf("internal error in bootc legacy iso: no base image defined")
	}
	imgref := t.arch.distro.imgref
	containerSource := container.SourceSpec{
		Source: imgref,
		Name:   imgref,
		Local:  true,
	}

	archStr := t.arch.Name()
	sourceInfo := t.arch.distro.sourceInfo

	distroYAML, id, err := newDistroYAMLFrom(t.arch.distro.sourceInfo)
	if err != nil {
		return nil, nil, err
	}

	// XXX: or "bootc-legacy-installer"?
	installerImgTypeName := "bootc-rpm-installer"
	imgType, ok := distroYAML.ImageTypes()[installerImgTypeName]
	if !ok {
		return nil, nil, fmt.Errorf("cannot find image definition for %v", installerImgTypeName)
	}
	installerPkgSet, ok := imgType.PackageSets(*id, archStr)["installer"]
	if !ok {
		return nil, nil, fmt.Errorf("cannot find installer package set for %v", installerImgTypeName)
	}
	installerConfig := imgType.InstallerConfig(*id, archStr)
	if installerConfig == nil {
		return nil, nil, fmt.Errorf("empty installer config for %s", installerImgTypeName)
	}
	var customizations *blueprint.Customizations
	if bp != nil {
		customizations = bp.Customizations
	}

	platformi := PlatformFor(archStr, sourceInfo.UEFIVendor)
	platformi.ImageFormat = platform.FORMAT_ISO

	img := image.NewAnacondaContainerInstallerLegacy(platformi, t.Filename(), containerSource)
	if err := t.initAnacondaInstallerBaseFromSourceInfo(&img.AnacondaInstallerBase, sourceInfo, customizations); err != nil {
		return nil, nil, err
	}
	img.ContainerRemoveSignatures = true
	img.ExtraBasePackages = installerPkgSet
	// our installer customizations come from the distrodefs (unlike in manifestForISO)
	img.InstallerCustomizations.LoraxTemplates = installerConfig.LoraxTemplates
	if installerConfig.LoraxTemplatePackage != nil {
		img.InstallerCustomizations.LoraxTemplatePackage = *installerConfig.LoraxTemplatePackage
	}

	installRootfsType, err := disk.NewFSType(t.arch.distro.defaultFs)
	if err != nil {
		return nil, nil, err
	}
	img.InstallRootfsType = installRootfsType

	mf := manifest.New()

	foundDistro, foundRunner, err := GetDistroAndRunner(sourceInfo.OSRelease)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to infer distro and runner: %w", err)
	}
	mf.Distro = foundDistro

	_, err = img.InstantiateManifest(&mf, nil, foundRunner, rng)
	return &mf, nil, err
}

// manifestForPXETar creates a PXE bootable bootc rootfs
func (t *imageType) manifestForPXETar(bp *blueprint.Blueprint, options distro.ImageOptions, rng *rand.Rand) (*manifest.Manifest, []string, error) {
	if t.arch.distro.imgref == "" {
		return nil, nil, fmt.Errorf("internal error: no base image defined")
	}
	containerSource := container.SourceSpec{
		Source: t.arch.distro.imgref,
		Name:   t.arch.distro.imgref,
		Local:  true,
	}
	buildContainerSource := container.SourceSpec{
		Source: t.arch.distro.buildImgref,
		Name:   t.arch.distro.buildImgref,
		Local:  true,
	}

	var customizations *blueprint.Customizations
	if bp != nil {
		customizations = bp.Customizations
	}

	platform := PlatformFor(t.arch.Name(), t.arch.distro.sourceInfo.UEFIVendor)
	img := image.NewBootcPXEImage(platform, t.Filename(), containerSource, buildContainerSource)
	img.Compression = t.ImageTypeYAML.Compression
	img.OSCustomizations.Users = users.UsersFromBP(customizations.GetUsers())

	groups, err := customizations.GetGroups()
	if err != nil {
		return nil, nil, err
	}
	img.OSCustomizations.Groups = users.GroupsFromBP(groups)
	img.OSCustomizations.SELinux = t.arch.distro.sourceInfo.SELinuxPolicy
	img.OSCustomizations.BuildSELinux = img.OSCustomizations.SELinux
	if t.arch.distro.buildSourceInfo != nil {
		img.OSCustomizations.BuildSELinux = t.arch.distro.buildSourceInfo.SELinuxPolicy
	}
	if t.arch.distro.sourceInfo != nil && t.arch.distro.sourceInfo.MountConfiguration != nil {
		img.OSCustomizations.MountConfiguration = *t.arch.distro.sourceInfo.MountConfiguration
	}

	imageConfig := t.ImageTypeYAML.ImageConfig(t.arch.distro.id, t.arch.Name())
	if imageConfig != nil {
		img.OSCustomizations.KernelOptionsAppend = imageConfig.KernelOptions
	}
	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.OSCustomizations.KernelOptionsAppend = append(img.OSCustomizations.KernelOptionsAppend, kopts.Append)
	}

	// NOTE: Only the / partition is needed since the final result is compressed
	//       filesystem. But the intermediate bootc filesystem install needs a size
	//       and partitions.
	rootfsMinSize := max(t.arch.distro.rootfsMinSize, options.Size)
	pt, err := t.genPartitionTable(customizations, rootfsMinSize, rng)
	if err != nil {
		return nil, nil, err
	}
	img.PartitionTable = pt

	// Check Directory/File Customizations are valid
	dc := customizations.GetDirectories()
	fc := customizations.GetFiles()
	if err := blueprint.ValidateDirFileCustomizations(dc, fc); err != nil {
		return nil, nil, err
	}
	if err := blueprint.CheckDirectoryCustomizationsPolicy(dc, policies.OstreeCustomDirectoriesPolicies); err != nil {
		return nil, nil, err
	}
	if err := blueprint.CheckFileCustomizationsPolicy(fc, policies.OstreeCustomFilesPolicies); err != nil {
		return nil, nil, err
	}
	img.OSCustomizations.Files, err = blueprint.FileCustomizationsToFsNodeFiles(fc)
	if err != nil {
		return nil, nil, err
	}
	img.OSCustomizations.Directories, err = blueprint.DirectoryCustomizationsToFsNodeDirectories(dc)
	if err != nil {
		return nil, nil, err
	}

	// Used when dracut rebuilds the initramfs in the bootc pipeline
	img.KernelVersion = t.arch.distro.sourceInfo.KernelInfo.Version

	mf := manifest.New()
	mf.Distro = manifest.DISTRO_FEDORA
	runner := &runner.Linux{}

	if err := img.InstantiateManifestFromContainers(&mf, []container.SourceSpec{containerSource}, runner, rng); err != nil {
		return nil, nil, err
	}

	return &mf, nil, nil

}
