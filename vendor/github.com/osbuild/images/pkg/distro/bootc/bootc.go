package bootc

import (
	"errors"
	"fmt"
	"maps"
	"math/rand"
	"slices"
	"sort"
	"strings"

	"github.com/osbuild/blueprint/pkg/blueprint"

	"github.com/osbuild/images/internal/cmdutil"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	bibcontainer "github.com/osbuild/images/pkg/bib/container"
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

var _ = distro.Distro(&BootcDistro{})

type BootcDistro struct {
	imgref          string
	buildImgref     string
	sourceInfo      *osinfo.Info
	buildSourceInfo *osinfo.Info

	id            distro.ID
	defaultFs     string
	releasever    string
	rootfsMinSize uint64

	arches map[string]distro.Arch
}

var _ = distro.Arch(&BootcArch{})

type BootcArch struct {
	distro *BootcDistro
	arch   arch.Arch

	imageTypes map[string]distro.ImageType
}

func (d *BootcDistro) SetBuildContainer(imgref string) (err error) {
	if imgref == "" {
		return nil
	}

	cnt, err := bibcontainer.New(imgref)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, cnt.Stop())
	}()

	info, err := osinfo.Load(cnt.Root())
	if err != nil {
		return err
	}
	return d.setBuildContainer(imgref, info)
}

func (d *BootcDistro) setBuildContainer(imgref string, info *osinfo.Info) error {
	d.buildImgref = imgref
	d.buildSourceInfo = info
	return nil
}

// SetBuildContainerForTesting should only be used for in tests
// please use "SetBuildContainer" instead
func (d *BootcDistro) SetBuildContainerForTesting(imgref string, info *osinfo.Info) error {
	return d.setBuildContainer(imgref, info)
}

func (d *BootcDistro) SetDefaultFs(defaultFs string) error {
	if defaultFs == "" {
		return nil
	}

	d.defaultFs = defaultFs
	return nil
}

func (d *BootcDistro) DefaultFs() string {
	return d.defaultFs
}

func (d *BootcDistro) Name() string {
	return d.id.String()
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

func (d *BootcDistro) OSTreeRef() string {
	return ""
}

func (d *BootcDistro) ListArches() []string {
	archs := make([]string, 0, len(d.arches))
	for name := range d.arches {
		archs = append(archs, name)
	}
	sort.Strings(archs)
	return archs
}

func (d *BootcDistro) GetArch(arch string) (distro.Arch, error) {
	a, exists := d.arches[arch]
	if !exists {
		return nil, fmt.Errorf("requested bootc arch %q does not match available arches %v", arch, slices.Collect(maps.Keys(d.arches)))
	}
	return a, nil
}

func (d *BootcDistro) addArches(arches ...*BootcArch) {
	if d.arches == nil {
		d.arches = map[string]distro.Arch{}
	}

	for _, a := range arches {
		a.distro = d
		d.arches[a.Name()] = a
	}
}

func (a *BootcArch) Name() string {
	return a.arch.String()
}

func (a *BootcArch) Distro() distro.Distro {
	return a.distro
}

func (a *BootcArch) ListImageTypes() []string {
	formats := make([]string, 0, len(a.imageTypes))
	for name := range a.imageTypes {
		formats = append(formats, name)
	}
	sort.Strings(formats)
	return formats
}

func (a *BootcArch) GetImageType(imageType string) (distro.ImageType, error) {
	t, exists := a.imageTypes[imageType]
	if !exists {
		return nil, errors.New("invalid image type: " + imageType)
	}

	return t, nil
}

func (a *BootcArch) addImageTypes(imageTypes ...BootcImageType) {
	if a.imageTypes == nil {
		a.imageTypes = map[string]distro.ImageType{}
	}
	for idx := range imageTypes {
		it := imageTypes[idx]
		it.arch = a
		a.imageTypes[it.Name()] = &it
	}
}

var _ = distro.ImageType(&BootcImageType{})

type BootcImageType struct {
	defs.ImageTypeYAML

	arch *BootcArch
}

func (t *BootcImageType) Name() string {
	return t.ImageTypeYAML.Name()
}

func (t *BootcImageType) Aliases() []string {
	return t.ImageTypeYAML.NameAliases
}

func (t *BootcImageType) Arch() distro.Arch {
	return t.arch
}

func (t *BootcImageType) Filename() string {
	return t.ImageTypeYAML.Filename
}

func (t *BootcImageType) MIMEType() string {
	return t.ImageTypeYAML.MimeType
}

func (t *BootcImageType) OSTreeRef() string {
	return ""
}

func (t *BootcImageType) ISOLabel() (string, error) {
	return "", nil
}

func (t *BootcImageType) Size(size uint64) uint64 {
	if size == 0 {
		size = 1073741824
	}
	return size
}

func (t *BootcImageType) PartitionType() disk.PartitionTableType {
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

func (t *BootcImageType) BasePartitionTable() (*disk.PartitionTable, error) {
	return t.ImageTypeYAML.PartitionTable(t.arch.distro.id, t.arch.arch.String())
}

func (t *BootcImageType) BootMode() platform.BootMode {
	// We really never want HYBRID or LEGACY on aarch64 platforms. In the future
	// it might be much nicer to take the same apporach as `Bootmode()` in the
	// generic distro but that's a bit more involved. Let's start here.
	if t.arch.arch == arch.ARCH_AARCH64 {
		return platform.BOOT_UEFI
	}

	return platform.BOOT_HYBRID
}

func (t *BootcImageType) PayloadPackageSets() []string {
	return nil
}

func (t *BootcImageType) Exports() []string {
	return t.ImageTypeYAML.Exports
}

func (t *BootcImageType) SupportedBlueprintOptions() []string {
	if t.BootISO {
		// XXX: this is probably too minimal but lets start small
		// and expand
		return []string{
			"customizations.fips",
			"customizations.group",
			"customizations.installer",
			"customizations.kernel.append",
			"customizations.user",
		}
	}
	return []string{
		"customizations.directories",
		"customizations.disk",
		"customizations.files",
		"customizations.filesystem",
		"customizations.group",
		"customizations.kernel",
		"customizations.user",
	}
}
func (t *BootcImageType) RequiredBlueprintOptions() []string {
	return nil
}

func (t *BootcImageType) Manifest(bp *blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig, seedp *int64) (*manifest.Manifest, []string, error) {
	if t.BootISO {
		return t.manifestForISO(bp, options, repos, seedp)
	}
	return t.manifestForDisk(bp, options, repos, seedp)
}

func (t *BootcImageType) manifestForDisk(bp *blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig, seedp *int64) (*manifest.Manifest, []string, error) {
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
	seed, err := cmdutil.SeedArgFor(nil, t.Name(), t.arch.Name(), t.arch.distro.Name())
	if err != nil {
		return nil, nil, err
	}
	//nolint:gosec
	rng := rand.New(rand.NewSource(seed))

	platform := PlatformFor(t.arch.Name(), t.arch.distro.sourceInfo.UEFIVendor)
	// For the bootc-disk image, the filename is the basename and
	// the extension is added automatically for each disk format
	filename := strings.Split(t.Filename(), ".")[0]

	img := image.NewBootcDiskImage(platform, filename, containerSource, buildContainerSource)
	img.OSCustomizations.Users = users.UsersFromBP(customizations.GetUsers())
	img.OSCustomizations.Groups = users.GroupsFromBP(customizations.GetGroups())
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

func (t *BootcImageType) manifestForISO(bp *blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig, seedp *int64) (*manifest.Manifest, []string, error) {
	if t.arch.distro.imgref == "" {
		return nil, nil, fmt.Errorf("internal error: no base image defined")
	}
	if options.Bootc == nil || options.Bootc.InstallerPayloadRef == "" {
		return nil, nil, fmt.Errorf("no installer payload bootc ref set")
	}
	payloadRef := options.Bootc.InstallerPayloadRef

	containerSource := container.SourceSpec{
		Source: t.arch.distro.imgref,
		Name:   t.arch.distro.imgref,
		Local:  true,
	}
	// XXX: keep it simple for now, we may allow this in the future
	if t.arch.distro.buildImgref != t.arch.distro.imgref {
		return nil, nil, fmt.Errorf("cannot use build-containers with anaconda installer images")
	}

	var customizations *blueprint.Customizations
	if bp != nil {
		customizations = bp.Customizations
	}
	seed, err := cmdutil.SeedArgFor(nil, t.Name(), t.arch.Name(), t.arch.distro.Name())
	if err != nil {
		return nil, nil, err
	}
	//nolint:gosec
	rng := rand.New(rand.NewSource(seed))

	platformi := PlatformFor(t.arch.Name(), t.arch.distro.sourceInfo.UEFIVendor)
	platformi.ImageFormat = platform.FORMAT_ISO

	// XXX: tons of copied code from
	// bootc-image-builder:‎bib/cmd/bootc-image-builder/legacy_iso.go
	// but sharing is hard because AnacondaContainerInstaller and
	// AnacondaContainerInstallerLegacy are different types so
	// a shared helper to set the fields won't work (unless
	// reflection urgh).

	// The ref is not needed and will be removed from the ctor later
	// in time
	img := image.NewAnacondaContainerInstaller(platformi, t.Filename(), containerSource, "")
	img.ContainerRemoveSignatures = true
	img.RootfsCompression = "zstd"
	// kernelVer is used by dracut
	img.KernelVer = t.arch.distro.sourceInfo.KernelInfo.Version
	img.KernelPath = fmt.Sprintf("lib/modules/%s/vmlinuz", t.arch.distro.sourceInfo.KernelInfo.Version)
	img.InitramfsPath = fmt.Sprintf("lib/modules/%s/initramfs.img", t.arch.distro.sourceInfo.KernelInfo.Version)
	img.InstallerHome = "/var/roothome"
	payloadSource := container.SourceSpec{
		Source: payloadRef,
		Name:   payloadRef,
		Local:  true,
	}
	img.InstallerPayload = payloadSource

	if t.arch.Name() == arch.ARCH_X86_64.String() {
		img.InstallerCustomizations.ISOBoot = manifest.Grub2ISOBoot
	}

	img.InstallerCustomizations.Product = t.arch.distro.sourceInfo.OSRelease.Name
	img.InstallerCustomizations.OSVersion = t.arch.distro.sourceInfo.OSRelease.VersionID
	img.InstallerCustomizations.ISOLabel = LabelForISO(&t.arch.distro.sourceInfo.OSRelease, t.arch.Name())

	img.InstallerCustomizations.FIPS = customizations.GetFIPS()
	img.Kickstart, err = kickstart.New(customizations)
	if err != nil {
		return nil, nil, err
	}
	img.Kickstart.Path = osbuild.KickstartPathOSBuild
	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.Kickstart.KernelOptionsAppend = append(img.Kickstart.KernelOptionsAppend, kopts.Append)
	}
	img.Kickstart.NetworkOnBoot = true

	instCust, err := customizations.GetInstaller()
	if err != nil {
		return nil, nil, err
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
	img.InstallerCustomizations.LoraxTemplates = LoraxTemplates(t.arch.distro.sourceInfo.OSRelease)
	img.InstallerCustomizations.LoraxTemplatePackage = LoraxTemplatePackage(t.arch.distro.sourceInfo.OSRelease)

	// see https://github.com/osbuild/bootc-image-builder/issues/733
	img.InstallerCustomizations.ISORootfsType = manifest.SquashfsRootfs

	installRootfsType, err := disk.NewFSType(t.arch.distro.defaultFs)
	if err != nil {
		return nil, nil, err
	}
	img.InstallRootfsType = installRootfsType

	mf := manifest.New()

	foundDistro, foundRunner, err := GetDistroAndRunner(t.arch.distro.sourceInfo.OSRelease)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to infer distro and runner: %w", err)
	}
	mf.Distro = foundDistro

	_, err = img.InstantiateManifestFromContainer(&mf, []container.SourceSpec{containerSource}, foundRunner, rng)
	return &mf, nil, err
}

// newBootcDistro returns a new instance of BootcDistro
// from the given url
func NewBootcDistro(imgref string) (*BootcDistro, error) {
	cnt, err := bibcontainer.New(imgref)
	if err != nil {
		return nil, err
	}
	defer func() {
		err = errors.Join(err, cnt.Stop())
	}()

	info, err := osinfo.Load(cnt.Root())
	if err != nil {
		return nil, err
	}
	// XXX: provide a way to set defaultfs (needed for bib)
	defaultFs, err := cnt.DefaultRootfsType()
	if err != nil {
		return nil, err
	}
	cntSize, err := getContainerSize(imgref)
	if err != nil {
		return nil, fmt.Errorf("cannot get container size: %w", err)
	}
	return newBootcDistroAfterIntrospect(cnt.Arch(), info, imgref, defaultFs, cntSize)
}

func newBootcDistroAfterIntrospect(archStr string, info *osinfo.Info, imgref, defaultFsStr string, cntSize uint64) (*BootcDistro, error) {
	nameVer := fmt.Sprintf("bootc-%s-%s", info.OSRelease.ID, info.OSRelease.VersionID)
	id, err := distro.ParseID(nameVer)
	if err != nil {
		return nil, err
	}
	bd := &BootcDistro{
		id:            *id,
		releasever:    info.OSRelease.VersionID,
		defaultFs:     defaultFsStr,
		rootfsMinSize: cntSize * containerSizeToDiskSizeMultiplier,

		imgref:     imgref,
		sourceInfo: info,
		// default buildref/info to regular container, this can
		// be overriden with SetBuildContainer()
		buildImgref:     imgref,
		buildSourceInfo: info,
	}

	archi, err := arch.FromString(archStr)
	if err != nil {
		return nil, err
	}
	ba := &BootcArch{
		arch: archi,
	}

	distroYAML, err := defs.LoadDistroWithoutImageTypes("bootc-generic-1")
	if err != nil {
		return nil, err
	}
	defaultFs, err := disk.NewFSType(defaultFsStr)
	if err != nil {
		return nil, err
	}
	distroYAML.DefaultFSType = defaultFs
	if err := distroYAML.LoadImageTypes(); err != nil {
		return nil, err
	}
	for _, imgTypeYaml := range distroYAML.ImageTypes() {
		ba.addImageTypes(BootcImageType{
			ImageTypeYAML: imgTypeYaml,
		})
	}
	bd.addArches(ba)

	return bd, nil
}

// NewBootcDistroForTesting can be used to generate test manifests.
// The container introspection is skipped. Do not use this for
// anything but tests.
var NewBootcDistroForTesting = newBootcDistroAfterIntrospect

func DistroFactory(idStr string) distro.Distro {
	l := strings.SplitN(idStr, ":", 2)
	if l[0] != "bootc" {
		return nil
	}
	imgRef := l[1]

	return common.Must(NewBootcDistro(imgRef))
}
