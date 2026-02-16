package generic

import (
	"errors"
	"fmt"
	"math/rand"
	"strings"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/cmdutil"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/bib/osinfo"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/disk/partition"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/bootc"
	"github.com/osbuild/images/pkg/distro/defs"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/pathpolicy"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/policies"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

var _ = distro.ImageType(&bootcImageType{})

type bootcImageType struct {
	defs.ImageTypeYAML

	arch *architecture
}

func (t *bootcImageType) Name() string {
	return t.ImageTypeYAML.Name()
}

func (t *bootcImageType) Aliases() []string {
	return t.ImageTypeYAML.NameAliases
}

func (t *bootcImageType) Arch() distro.Arch {
	return t.arch
}

func (t *bootcImageType) Filename() string {
	return t.ImageTypeYAML.Filename
}

func (t *bootcImageType) MIMEType() string {
	return t.ImageTypeYAML.MimeType
}

func (t *bootcImageType) OSTreeRef() string {
	return ""
}

func (t *bootcImageType) ISOLabel() (string, error) {
	return "", nil
}

func (t *bootcImageType) Size(size uint64) uint64 {
	if size == 0 {
		size = 1073741824
	}
	return size
}

func (t *bootcImageType) PartitionType() disk.PartitionTableType {
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

func (t *bootcImageType) BasePartitionTable() (*disk.PartitionTable, error) {
	bd := t.arch.distro.(*BootcDistro)
	return t.ImageTypeYAML.PartitionTable(bd.id, t.arch.arch.String())
}

func (t *bootcImageType) BootMode() platform.BootMode {
	// We really never want HYBRID or LEGACY on aarch64 platforms. In the future
	// it might be much nicer to take the same apporach as `Bootmode()` in the
	// generic distro but that's a bit more involved. Let's start here.
	if t.arch.arch == arch.ARCH_AARCH64 {
		return platform.BOOT_UEFI
	}

	return platform.BOOT_HYBRID
}

func (t *bootcImageType) PayloadPackageSets() []string {
	return nil
}

func (t *bootcImageType) Exports() []string {
	return t.ImageTypeYAML.Exports
}

func (t *bootcImageType) SupportedBlueprintOptions() []string {
	// The blueprint contains a few fields that are essentially metadata and
	// not configuration / customizations. These should always be implicitly
	// supported by all image types.
	return append(t.ImageTypeYAML.Blueprint.SupportedOptions, "name", "version", "description")
}

func (t *bootcImageType) RequiredBlueprintOptions() []string {
	return nil
}

// keep in sync with "generic/imagetype.go:checkOptions()"
func (t *bootcImageType) checkOptions(bp *blueprint.Blueprint) []string {
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

func (t *bootcImageType) Manifest(bp *blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig, seedp *int64) (*manifest.Manifest, []string, error) {
	validationWarnings := t.checkOptions(bp)

	mani, manifestWarnings, err := t.manifestWithoutValidation(bp, options)
	return mani, append(validationWarnings, manifestWarnings...), err
}

func (t *bootcImageType) manifestWithoutValidation(bp *blueprint.Blueprint, options distro.ImageOptions) (*manifest.Manifest, []string, error) {
	bd := t.arch.distro.(*BootcDistro)
	seed, err := cmdutil.SeedArgFor(nil, t.arch.Name(), bd.Name())
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

func (t *bootcImageType) manifestForDisk(bp *blueprint.Blueprint, options distro.ImageOptions, rng *rand.Rand) (*manifest.Manifest, []string, error) {
	bd := t.arch.distro.(*BootcDistro)
	if bd.imgref == "" {
		return nil, nil, fmt.Errorf("internal error: no base image defined")
	}
	containerSource := container.SourceSpec{
		Source: bd.imgref,
		Name:   bd.imgref,
		Local:  true,
	}
	buildContainerSource := container.SourceSpec{
		Source: bd.buildImgref,
		Name:   bd.buildImgref,
		Local:  true,
	}

	var customizations *blueprint.Customizations
	if bp != nil {
		customizations = bp.Customizations
	}

	platform := PlatformFor(t.arch.Name(), bd.sourceInfo.UEFIVendor)
	// For the bootc-disk image, the filename is the basename and
	// the extension is added automatically for each disk format
	filename := strings.Split(t.Filename(), ".")[0]

	img := image.NewBootcDiskImage(platform, filename, containerSource, buildContainerSource)
	if opts := buildOptions(t); opts != nil {
		img.BuildOptions = opts
	}
	img.OSCustomizations.Users = users.UsersFromBP(customizations.GetUsers())

	groups, err := customizations.GetGroups()
	if err != nil {
		return nil, nil, err
	}
	img.OSCustomizations.Groups = users.GroupsFromBP(groups)
	img.OSCustomizations.SELinux = bd.sourceInfo.SELinuxPolicy
	img.OSCustomizations.BuildSELinux = img.OSCustomizations.SELinux
	if bd.buildSourceInfo != nil {
		img.OSCustomizations.BuildSELinux = bd.buildSourceInfo.SELinuxPolicy
	}
	if bd.sourceInfo != nil && bd.sourceInfo.MountConfiguration != nil {
		img.OSCustomizations.MountConfiguration = *bd.sourceInfo.MountConfiguration
	}

	imageConfig := t.ImageTypeYAML.ImageConfig(bd.id, t.arch.Name())
	if imageConfig != nil {
		img.OSCustomizations.KernelOptionsAppend = imageConfig.KernelOptions
	}
	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.OSCustomizations.KernelOptionsAppend = append(img.OSCustomizations.KernelOptionsAppend, kopts.Append)
	}

	rootfsMinSize := max(bd.rootfsMinSize, options.Size)

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

func (t *bootcImageType) initAnacondaInstallerBaseFromSourceInfo(img *image.AnacondaInstallerBase, sourceInfo *osinfo.Info, customizations *blueprint.Customizations) error {
	img.RootfsCompression = "zstd"

	if t.arch.Name() == arch.ARCH_X86_64.String() {
		img.ISOCustomizations.BootType = manifest.Grub2ISOBoot
	}

	img.InstallerCustomizations.Product = sourceInfo.OSRelease.Name
	img.InstallerCustomizations.OSVersion = sourceInfo.OSRelease.VersionID
	img.ISOCustomizations.Label = bootc.LabelForISO(&sourceInfo.OSRelease, t.arch.Name())

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

func (t *bootcImageType) manifestForISO(bp *blueprint.Blueprint, options distro.ImageOptions, rng *rand.Rand) (*manifest.Manifest, []string, error) {
	bd := t.arch.distro.(*BootcDistro)
	if bd.imgref == "" {
		return nil, nil, fmt.Errorf("internal error in bootc iso: no base image defined")
	}
	if options.Bootc == nil || options.Bootc.InstallerPayloadRef == "" {
		return nil, nil, fmt.Errorf("no installer payload bootc ref set")
	}
	payloadRef := options.Bootc.InstallerPayloadRef
	imgref := bd.imgref
	containerSource := container.SourceSpec{
		Source: imgref,
		Name:   imgref,
		Local:  true,
	}
	sourceInfo := bd.sourceInfo
	// XXX: keep it simple for now, we may allow this in the future
	if bd.buildImgref != bd.imgref {
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
	if opts := buildOptions(t); opts != nil {
		img.BuildOptions = opts
	}
	img.ContainerRemoveSignatures = true
	// we auto-detect the lorax config from the source info
	img.InstallerCustomizations.LoraxTemplates = bootc.LoraxTemplates(sourceInfo.OSRelease)
	img.InstallerCustomizations.LoraxTemplatePackage = bootc.LoraxTemplatePackage(sourceInfo.OSRelease)

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

	installRootfsType, err := disk.NewFSType(bd.defaultFs)
	if err != nil {
		return nil, nil, err
	}
	img.InstallRootfsType = installRootfsType

	mf := manifest.New()

	foundDistro, foundRunner, err := bootc.GetDistroAndRunner(sourceInfo.OSRelease)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to infer distro and runner: %w", err)
	}
	mf.Distro = foundDistro

	_, err = img.InstantiateManifestFromContainer(&mf, []container.SourceSpec{containerSource}, foundRunner, rng)
	return &mf, nil, err
}

func (t *bootcImageType) manifestForGenericISO(options distro.ImageOptions, rng *rand.Rand) (*manifest.Manifest, []string, error) {
	bd := t.arch.distro.(*BootcDistro)
	if bd.imgref == "" {
		return nil, nil, fmt.Errorf("internal error: no base image defined")
	}

	containerSource := container.SourceSpec{
		Source: bd.imgref,
		Name:   bd.imgref,
		Local:  true,
	}

	platformi := PlatformFor(t.arch.Name(), bd.sourceInfo.UEFIVendor)
	platformi.ImageFormat = platform.FORMAT_ISO

	img := image.NewContainerBasedIso(platformi, t.Filename(), containerSource, nil)
	if options.Bootc != nil && options.Bootc.InstallerPayloadRef != "" {
		img.PayloadContainer = &container.SourceSpec{
			Source: options.Bootc.InstallerPayloadRef,
			Name:   options.Bootc.InstallerPayloadRef,
			Local:  true,
		}
	}
	img.RootfsCompression = "zstd"
	img.RootfsType = manifest.SquashfsRootfs
	img.KernelPath = fmt.Sprintf("lib/modules/%s/vmlinuz", bd.sourceInfo.KernelInfo.Version)
	img.InitramfsPath = fmt.Sprintf("lib/modules/%s/initramfs.img", bd.sourceInfo.KernelInfo.Version)
	img.Product = bd.sourceInfo.OSRelease.Name
	img.Version = bd.sourceInfo.OSRelease.VersionID
	img.Release = bd.sourceInfo.OSRelease.VersionID

	isoi := bd.sourceInfo.ISOInfo

	if isoi.Label != "" {
		img.ISOLabel = isoi.Label
	} else {
		img.ISOLabel = bootc.LabelForISO(&bd.sourceInfo.OSRelease, t.arch.Name())
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

	foundDistro, foundRunner, err := bootc.GetDistroAndRunner(bd.sourceInfo.OSRelease)
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

func (t *bootcImageType) manifestForLegacyISO(bp *blueprint.Blueprint, rng *rand.Rand) (*manifest.Manifest, []string, error) {
	bd := t.arch.distro.(*BootcDistro)
	if bd.imgref == "" {
		return nil, nil, fmt.Errorf("internal error in bootc legacy iso: no base image defined")
	}
	imgref := bd.imgref
	containerSource := container.SourceSpec{
		Source: imgref,
		Name:   imgref,
		Local:  true,
	}

	archStr := t.arch.Name()
	sourceInfo := bd.sourceInfo

	distroYAML, id, err := newDistroYAMLFrom(bd.sourceInfo)
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
	if opts := buildOptions(t); opts != nil {
		img.BuildOptions = opts
	}
	img.ContainerRemoveSignatures = true
	img.ExtraBasePackages = installerPkgSet
	// our installer customizations come from the distrodefs (unlike in manifestForISO)
	img.InstallerCustomizations.LoraxTemplates = installerConfig.LoraxTemplates
	if installerConfig.LoraxTemplatePackage != nil {
		img.InstallerCustomizations.LoraxTemplatePackage = *installerConfig.LoraxTemplatePackage
	}

	installRootfsType, err := disk.NewFSType(bd.defaultFs)
	if err != nil {
		return nil, nil, err
	}
	img.InstallRootfsType = installRootfsType

	mf := manifest.New()

	foundDistro, foundRunner, err := bootc.GetDistroAndRunner(sourceInfo.OSRelease)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to infer distro and runner: %w", err)
	}
	mf.Distro = foundDistro

	_, err = img.InstantiateManifest(&mf, nil, foundRunner, rng)
	return &mf, nil, err
}

// manifestForPXETar creates a PXE bootable bootc rootfs
func (t *bootcImageType) manifestForPXETar(bp *blueprint.Blueprint, options distro.ImageOptions, rng *rand.Rand) (*manifest.Manifest, []string, error) {
	bd := t.arch.distro.(*BootcDistro)
	if bd.imgref == "" {
		return nil, nil, fmt.Errorf("internal error: no base image defined")
	}
	containerSource := container.SourceSpec{
		Source: bd.imgref,
		Name:   bd.imgref,
		Local:  true,
	}
	buildContainerSource := container.SourceSpec{
		Source: bd.buildImgref,
		Name:   bd.buildImgref,
		Local:  true,
	}

	var customizations *blueprint.Customizations
	if bp != nil {
		customizations = bp.Customizations
	}

	platform := PlatformFor(t.arch.Name(), bd.sourceInfo.UEFIVendor)
	img := image.NewBootcPXEImage(platform, t.Filename(), containerSource, buildContainerSource)
	if opts := buildOptions(t); opts != nil {
		img.BuildOptions = opts
	}
	img.Compression = t.ImageTypeYAML.Compression
	img.OSCustomizations.Users = users.UsersFromBP(customizations.GetUsers())

	groups, err := customizations.GetGroups()
	if err != nil {
		return nil, nil, err
	}
	img.OSCustomizations.Groups = users.GroupsFromBP(groups)
	img.OSCustomizations.SELinux = bd.sourceInfo.SELinuxPolicy
	img.OSCustomizations.BuildSELinux = img.OSCustomizations.SELinux
	if bd.buildSourceInfo != nil {
		img.OSCustomizations.BuildSELinux = bd.buildSourceInfo.SELinuxPolicy
	}
	if bd.sourceInfo != nil && bd.sourceInfo.MountConfiguration != nil {
		img.OSCustomizations.MountConfiguration = *bd.sourceInfo.MountConfiguration
	}

	imageConfig := t.ImageTypeYAML.ImageConfig(bd.id, t.arch.Name())
	if imageConfig != nil {
		img.OSCustomizations.KernelOptionsAppend = imageConfig.KernelOptions
	}
	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.OSCustomizations.KernelOptionsAppend = append(img.OSCustomizations.KernelOptionsAppend, kopts.Append)
	}

	// NOTE: Only the / partition is needed since the final result is compressed
	//       filesystem. But the intermediate bootc filesystem install needs a size
	//       and partitions.
	rootfsMinSize := max(bd.rootfsMinSize, options.Size)
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
	img.KernelVersion = bd.sourceInfo.KernelInfo.Version

	mf := manifest.New()
	mf.Distro = manifest.DISTRO_FEDORA
	runner := &runner.Linux{}

	if err := img.InstantiateManifestFromContainers(&mf, []container.SourceSpec{containerSource}, runner, rng); err != nil {
		return nil, nil, err
	}

	return &mf, nil, nil

}

func PlatformFor(archStr, uefiVendor string) *platform.Data {
	archi := common.Must(arch.FromString(archStr))
	platform := &platform.Data{
		Arch:        archi,
		UEFIVendor:  uefiVendor,
		QCOW2Compat: "1.1",
	}
	switch archi {
	case arch.ARCH_X86_64:
		platform.BIOSPlatform = "i386-pc"
	case arch.ARCH_PPC64LE:
		platform.BIOSPlatform = "powerpc-ieee1275"
	case arch.ARCH_S390X:
		platform.ZiplSupport = true
	}
	return platform
}

var (
	// The mountpoint policy for bootc images is more restrictive than the
	// ostree mountpoint policy defined in osbuild/images. It only allows /
	// (for sizing the root partition) and custom mountpoints under /var but
	// not /var itself.

	// Since our policy library doesn't support denying a path while allowing
	// its subpaths (only the opposite), we augment the standard policy check
	// with a simple search through the custom mountpoints to deny /var
	// specifically.
	mountpointPolicy = pathpolicy.NewPathPolicies(map[string]pathpolicy.PathPolicy{
		// allow all existing mountpoints (but no subdirs) to support size customizations
		"/":     {Deny: false, Exact: true},
		"/boot": {Deny: false, Exact: true},

		// /var is not allowed, but we need to allow any subdirectories that
		// are not denied below, so we allow it initially and then check it
		// separately (in checkMountpoints())
		"/var": {Deny: false},

		// /var subdir denials
		"/var/home":     {Deny: true},
		"/var/lock":     {Deny: true}, // symlink to ../run/lock which is on tmpfs
		"/var/mail":     {Deny: true}, // symlink to spool/mail
		"/var/mnt":      {Deny: true},
		"/var/roothome": {Deny: true},
		"/var/run":      {Deny: true}, // symlink to ../run which is on tmpfs
		"/var/srv":      {Deny: true},
		"/var/usrlocal": {Deny: true},
	})

	mountpointMinimalPolicy = pathpolicy.NewPathPolicies(map[string]pathpolicy.PathPolicy{
		// allow all existing mountpoints to support size customizations
		"/":     {Deny: false, Exact: true},
		"/boot": {Deny: false, Exact: true},
	})
)

func (t *bootcImageType) basePartitionTable() (*disk.PartitionTable, error) {
	bd := t.arch.distro.(*BootcDistro)
	// base partition table can come from the container
	if bd.sourceInfo != nil && bd.sourceInfo.PartitionTable != nil {
		return bd.sourceInfo.PartitionTable, nil
	}
	// otherwise we use our YAML
	return t.ImageTypeYAML.PartitionTable(bd.id, t.arch.Name())
}

func (t *bootcImageType) genPartitionTable(customizations *blueprint.Customizations, rootfsMinSize uint64, rng *rand.Rand) (*disk.PartitionTable, error) {
	// XXX: much duplication with generic/imagetype.go:getPartitionTable()
	fsCust := customizations.GetFilesystems()
	diskCust, err := customizations.GetPartitioning()
	if err != nil {
		return nil, fmt.Errorf("error reading disk customizations: %w", err)
	}
	basept, err := t.basePartitionTable()
	if err != nil {
		return nil, err
	}

	bd := t.arch.distro.(*BootcDistro)

	// Embedded disk customization applies if there was no local customization
	if fsCust == nil && diskCust == nil && bd.sourceInfo != nil && bd.sourceInfo.ImageCustomization != nil {
		imageCustomizations := bd.sourceInfo.ImageCustomization

		fsCust = imageCustomizations.GetFilesystems()
		diskCust, err = imageCustomizations.GetPartitioning()
		if err != nil {
			return nil, fmt.Errorf("error reading disk customizations: %w", err)
		}
	}

	var partitionTable *disk.PartitionTable
	switch {
	case fsCust != nil && diskCust != nil:
		return nil, fmt.Errorf("cannot combine disk and filesystem customizations")
	case diskCust != nil:
		partitionTable, err = t.genPartitionTableDiskCust(basept, diskCust, rootfsMinSize, rng)
		if err != nil {
			return nil, err
		}
	default:
		partitionTable, err = t.genPartitionTableFsCust(basept, fsCust, rootfsMinSize, rng)
		if err != nil {
			return nil, err
		}
	}

	// XXX: make this generic/configurable
	// Ensure ext4 rootfs has fs-verity enabled
	rootfs := partitionTable.FindMountable("/")
	if rootfs != nil {
		switch elem := rootfs.(type) {
		case *disk.Filesystem:
			if elem.Type == "ext4" {
				elem.MkfsOptions.Verity = true
			}
		}
	}

	return partitionTable, nil
}

func (t *bootcImageType) genPartitionTableDiskCust(basept *disk.PartitionTable, diskCust *blueprint.DiskCustomization, rootfsMinSize uint64, rng *rand.Rand) (*disk.PartitionTable, error) {
	if err := diskCust.ValidateLayoutConstraints(); err != nil {
		return nil, fmt.Errorf("cannot use disk customization: %w", err)
	}

	diskCust.MinSize = max(diskCust.MinSize, rootfsMinSize)

	if basept == nil {
		return nil, fmt.Errorf("pipelines: no partition tables defined for %s", t.arch.Name())
	}
	bd := t.arch.distro.(*BootcDistro)
	defaultFSType, err := disk.NewFSType(bd.defaultFs)
	if err != nil {
		return nil, err
	}
	requiredMinSizes, err := calcRequiredDirectorySizes(diskCust, rootfsMinSize)
	if err != nil {
		return nil, err
	}
	partOptions := &disk.CustomPartitionTableOptions{
		PartitionTableType: basept.Type,
		// XXX: not setting/defaults will fail to boot with btrfs/lvm
		BootMode:         t.BootMode(),
		DefaultFSType:    defaultFSType,
		RequiredMinSizes: requiredMinSizes,
		Architecture:     t.arch.arch,
	}
	return disk.NewCustomPartitionTable(diskCust, partOptions, rng)
}

func (t *bootcImageType) genPartitionTableFsCust(basept *disk.PartitionTable, fsCust []blueprint.FilesystemCustomization, rootfsMinSize uint64, rng *rand.Rand) (*disk.PartitionTable, error) {
	if basept == nil {
		return nil, fmt.Errorf("pipelines: no partition tables defined for %s", t.arch.Name())
	}

	partitioningMode := partition.RawPartitioningMode
	bd := t.arch.distro.(*BootcDistro)
	if bd.defaultFs == "btrfs" {
		partitioningMode = partition.BtrfsPartitioningMode
	}
	if err := checkFilesystemCustomizations(fsCust, partitioningMode); err != nil {
		return nil, err
	}
	fsCustomizations := updateFilesystemSizes(fsCust, rootfsMinSize)

	imageSize := t.ImageTypeYAML.DefaultSize
	if basept.Size != 0 {
		imageSize = basept.Size
	}

	return disk.NewPartitionTable(basept, fsCustomizations, imageSize, partitioningMode, t.arch.arch, nil, bd.defaultFs, rng)
}

func checkMountpoints(filesystems []blueprint.FilesystemCustomization, policy *pathpolicy.PathPolicies) error {
	errs := []error{}
	for _, fs := range filesystems {
		if err := policy.Check(fs.Mountpoint); err != nil {
			errs = append(errs, err)
		}
		if fs.Mountpoint == "/var" {
			// this error message is consistent with the errors returned by policy.Check()
			// TODO: remove trailing space inside the quoted path when the function is fixed in osbuild/images.
			errs = append(errs, fmt.Errorf(`path "/var" is not allowed`))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("the following errors occurred while validating custom mountpoints:\n%w", errors.Join(errs...))
	}
	return nil
}

// calcRequiredDirectorySizes will calculate the minimum sizes for /
// for disk customizations. We need this because with advanced partitioning
// we never grow the rootfs to the size of the disk (unlike the tranditional
// filesystem customizations).
//
// So we need to go over the customizations and ensure the min-size for "/"
// is at least rootfsMinSize.
//
// Note that a custom "/usr" is not supported in image mode so splitting
// rootfsMinSize between / and /usr is not a concern.
func calcRequiredDirectorySizes(distCust *blueprint.DiskCustomization, rootfsMinSize uint64) (map[string]datasizes.Size, error) {
	// XXX: this has *way* too much low-level knowledge about the
	// inner workings of blueprint.DiskCustomizations plus when
	// a new type it needs to get added here too, think about
	// moving into "images" instead (at least partly)
	mounts := map[string]uint64{}
	for _, part := range distCust.Partitions {
		switch part.Type {
		case "", "plain":
			mounts[part.Mountpoint] = part.MinSize
		case "lvm":
			for _, lv := range part.LogicalVolumes {
				mounts[lv.Mountpoint] = part.MinSize
			}
		case "btrfs":
			for _, subvol := range part.Subvolumes {
				mounts[subvol.Mountpoint] = part.MinSize
			}
		default:
			return nil, fmt.Errorf("unknown disk customization type %q", part.Type)
		}
	}
	// ensure rootfsMinSize is respected
	return map[string]datasizes.Size{
		"/": datasizes.Size(max(rootfsMinSize, mounts["/"])),
	}, nil
}

func checkFilesystemCustomizations(fsCustomizations []blueprint.FilesystemCustomization, ptmode partition.PartitioningMode) error {
	var policy *pathpolicy.PathPolicies
	switch ptmode {
	case partition.BtrfsPartitioningMode:
		// btrfs subvolumes are not supported at build time yet, so we only
		// allow / and /boot to be customized when building a btrfs disk (the
		// minimal policy)
		policy = mountpointMinimalPolicy
	default:
		policy = mountpointPolicy
	}
	if err := checkMountpoints(fsCustomizations, policy); err != nil {
		return err
	}
	return nil
}

// updateFilesystemSizes updates the size of the root filesystem customization
// based on the minRootSize. The new min size whichever is larger between the
// existing size and the minRootSize. If the root filesystem is not already
// configured, a new customization is added.
func updateFilesystemSizes(fsCustomizations []blueprint.FilesystemCustomization, minRootSize uint64) []blueprint.FilesystemCustomization {
	updated := make([]blueprint.FilesystemCustomization, len(fsCustomizations), len(fsCustomizations)+1)
	hasRoot := false
	for idx, fsc := range fsCustomizations {
		updated[idx] = fsc
		if updated[idx].Mountpoint == "/" {
			updated[idx].MinSize = max(updated[idx].MinSize, minRootSize)
			hasRoot = true
		}
	}

	if !hasRoot {
		// no root customization found: add it
		updated = append(updated, blueprint.FilesystemCustomization{Mountpoint: "/", MinSize: minRootSize})
	}
	return updated
}
