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
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
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

	name          string
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

var _ = distro.ImageType(&BootcImageType{})

type BootcImageType struct {
	arch *BootcArch

	name     string
	export   string
	filename string
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
	return d.name
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
	return d.name
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

func (t *BootcImageType) Name() string {
	return t.name
}

func (t *BootcImageType) Aliases() []string {
	return nil
}

func (t *BootcImageType) Arch() distro.Arch {
	return t.arch
}

func (t *BootcImageType) Filename() string {
	return t.filename
}

func (t *BootcImageType) MIMEType() string {
	return "application/x-test"
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
	return disk.PT_NONE
}

func (t *BootcImageType) BasePartitionTable() (*disk.PartitionTable, error) {
	return nil, nil
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

func (t *BootcImageType) BuildPipelines() []string {
	return []string{"build"}
}

func (t *BootcImageType) PayloadPipelines() []string {
	return []string{""}
}

func (t *BootcImageType) PayloadPackageSets() []string {
	return nil
}

func (t *BootcImageType) Exports() []string {
	return []string{t.export}
}

func (t *BootcImageType) SupportedBlueprintOptions() []string {
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

	archi := common.Must(arch.FromString(t.arch.Name()))
	platform := &platform.Data{
		Arch:        archi,
		UEFIVendor:  t.arch.distro.sourceInfo.UEFIVendor,
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
	// For the bootc-disk image, the filename is the basename and
	// the extension is added automatically for each disk format
	filename := strings.Split(t.filename, ".")[0]

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

	img.OSCustomizations.KernelOptionsAppend = []string{
		"rw",
		// TODO: Drop this as we expect kargs to come from the container image,
		// xref https://github.com/CentOS/centos-bootc-layered/blob/main/cloud/usr/lib/bootc/install/05-cloud-kargs.toml
		"console=tty0",
		"console=ttyS0",
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

func newBootcDistroAfterIntrospect(archStr string, info *osinfo.Info, imgref, defaultFs string, cntSize uint64) (*BootcDistro, error) {
	nameVer := fmt.Sprintf("bootc-%s-%s", info.OSRelease.ID, info.OSRelease.VersionID)
	bd := &BootcDistro{
		name:          nameVer,
		releasever:    info.OSRelease.VersionID,
		defaultFs:     defaultFs,
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
	// TODO: add iso image types, see bootc-image-builder
	//
	// Note that the file extension is hardcoded in
	// pkg/image/bootc_disk.go, we have no way to access
	// it here so we need to duplicate it
	// XXX: find a way to avoid this duplication
	ba.addImageTypes(
		BootcImageType{
			name:     "ami",
			export:   "image",
			filename: "disk.raw",
		},
		BootcImageType{
			name:     "qcow2",
			export:   "qcow2",
			filename: "disk.qcow2",
		},
		BootcImageType{
			name:     "raw",
			export:   "image",
			filename: "disk.raw",
		},
		BootcImageType{
			name:     "vmdk",
			export:   "vmdk",
			filename: "disk.vmdk",
		},
		BootcImageType{
			name:     "vhd",
			export:   "bpc",
			filename: "disk.vhd",
		},
		BootcImageType{
			name:     "gce",
			export:   "gce",
			filename: "image.tar.gz",
		},
		BootcImageType{
			name:     "ova",
			export:   "archive",
			filename: "image.ova",
		},
	)
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
