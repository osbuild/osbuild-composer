package bootc

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/internal/cmdutil"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

var _ = distro.ImageType(&BootcAnacondaInstaller{})

// BootcAnacondaInstaller is an image-type for a bootc
// container based ISO installer.
type BootcAnacondaInstaller struct {
	arch *BootcArch

	name   string
	export string
}

func (t *BootcAnacondaInstaller) Name() string {
	return t.name
}

func (t *BootcAnacondaInstaller) Aliases() []string {
	return nil
}

func (t *BootcAnacondaInstaller) Arch() distro.Arch {
	return t.arch
}

func (t *BootcAnacondaInstaller) Filename() string {
	return "installer.iso"
}

func (t *BootcAnacondaInstaller) MIMEType() string {
	return "application/x-iso9660-image"
}

func (t *BootcAnacondaInstaller) OSTreeRef() string {
	return ""
}

func (t *BootcAnacondaInstaller) ISOLabel() (string, error) {
	return "Unknown", nil
}

func (t *BootcAnacondaInstaller) Size(size uint64) uint64 {
	return size
}

func (t *BootcAnacondaInstaller) PartitionType() disk.PartitionTableType {
	return disk.PT_NONE
}

func (t *BootcAnacondaInstaller) BasePartitionTable() (*disk.PartitionTable, error) {
	return nil, nil
}

func (t *BootcAnacondaInstaller) BootMode() platform.BootMode {
	return platform.BOOT_HYBRID
}

func (t *BootcAnacondaInstaller) BuildPipelines() []string {
	return []string{"build"}
}

func (t *BootcAnacondaInstaller) PayloadPipelines() []string {
	return []string{""}
}

func (t *BootcAnacondaInstaller) PayloadPackageSets() []string {
	return nil
}

func (t *BootcAnacondaInstaller) Exports() []string {
	return []string{t.export}
}

func (t *BootcAnacondaInstaller) SupportedBlueprintOptions() []string {
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
func (t *BootcAnacondaInstaller) RequiredBlueprintOptions() []string {
	return nil
}

// XXX: duplication with BootcImageType
func (t *BootcAnacondaInstaller) Manifest(bp *blueprint.Blueprint, options distro.ImageOptions, repos []rpmmd.RepoConfig, seedp *int64) (*manifest.Manifest, []string, error) {
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
	filename := "install.iso"

	// The ref is not needed and will be removed from the ctor later
	// in time
	img := image.NewAnacondaContainerInstaller(platformi, filename, containerSource, "")
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
