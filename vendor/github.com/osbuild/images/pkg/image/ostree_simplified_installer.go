package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/customizations/fdo"
	"github.com/osbuild/images/pkg/customizations/ignition"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
)

type OSTreeSimplifiedInstaller struct {
	Base

	// Raw image that will be created and embedded
	rawImage *OSTreeDiskImage

	OSCustomizations        manifest.OSCustomizations
	Environment             environment.Environment
	InstallerCustomizations manifest.InstallerCustomizations
	ISOCustomizations       manifest.ISOCustomizations

	ExtraBasePackages rpmmd.PackageSet

	// ISO label template (architecture-free)
	ISOLabelTmpl string

	// OSName for ostree deployment
	OSName string

	installDevice string

	FDO *fdo.Options

	// Ignition firstboot configuration options
	IgnitionFirstBoot *ignition.FirstBootOptions

	// Ignition embedded configuration options
	IgnitionEmbedded *ignition.EmbeddedOptions
}

func NewOSTreeSimplifiedInstaller(platform platform.Platform, filename string, rawImage *OSTreeDiskImage, installDevice string) *OSTreeSimplifiedInstaller {
	return &OSTreeSimplifiedInstaller{
		Base:          NewBase("ostree-simplified-installer", platform, filename),
		rawImage:      rawImage,
		installDevice: installDevice,
	}
}

func (img *OSTreeSimplifiedInstaller) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := addBuildBootstrapPipelines(m, runner, repos, nil)
	buildPipeline.Checkpoint()

	imageFilename := "image.raw.xz"

	// image in simplified installer is always compressed
	compressedImage := manifest.NewXZ(buildPipeline, baseRawOstreeImage(img.rawImage, buildPipeline, nil))
	compressedImage.SetFilename(imageFilename)

	coiPipeline := manifest.NewCoreOSInstaller(
		buildPipeline,
		img.platform,
		repos,
		"kernel",
		img.InstallerCustomizations.Product,
		img.InstallerCustomizations.OSVersion,
	)
	coiPipeline.ExtraPackages = img.ExtraBasePackages.Include
	coiPipeline.ExcludePackages = img.ExtraBasePackages.Exclude
	coiPipeline.ExtraRepos = img.ExtraBasePackages.Repositories
	coiPipeline.FDO = img.FDO
	coiPipeline.Ignition = img.IgnitionEmbedded
	coiPipeline.Biosdevname = (img.platform.GetArch() == arch.ARCH_X86_64)
	coiPipeline.Variant = img.InstallerCustomizations.Variant
	coiPipeline.AdditionalDracutModules = img.InstallerCustomizations.AdditionalDracutModules
	coiPipeline.AdditionalDrivers = img.InstallerCustomizations.AdditionalDrivers

	if len(img.ISOCustomizations.Label) == 0 {
		img.ISOCustomizations.Label = fmt.Sprintf(img.ISOLabelTmpl, img.platform.GetArch())
	}

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
	bootTreePipeline.Platform = img.platform
	bootTreePipeline.UEFIVendor = img.platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = img.ISOCustomizations.Label

	// kernel options for EFI boot tree grub stage
	kernelOpts := []string{
		"rd.neednet=1",
		"coreos.inst.crypt_root=1",
		"coreos.inst.isoroot=" + img.ISOCustomizations.Label,
		"coreos.inst.install_dev=" + img.installDevice,
		fmt.Sprintf("coreos.inst.image_file=/run/media/iso/%s", imageFilename),
		"coreos.inst.insecure",
	}

	// extra FDO options for EFI boot tree grub stage
	if img.FDO != nil {
		kernelOpts = append(kernelOpts, "fdo.manufacturing_server_url="+img.FDO.ManufacturingServerURL)
		if img.FDO.DiunPubKeyInsecure != "" {
			kernelOpts = append(kernelOpts, "fdo.diun_pub_key_insecure="+img.FDO.DiunPubKeyInsecure)
		}
		if img.FDO.DiunPubKeyHash != "" {
			kernelOpts = append(kernelOpts, "fdo.diun_pub_key_hash="+img.FDO.DiunPubKeyHash)
		}
		if img.FDO.DiunPubKeyRootCerts != "" {
			kernelOpts = append(kernelOpts, "fdo.diun_pub_key_root_certs=/fdo_diun_pub_key_root_certs.pem")
		}
		if img.FDO.DiMfgStringTypeMacIface != "" {
			kernelOpts = append(kernelOpts, "fdo.di_mfg_string_type_mac_iface="+img.FDO.DiMfgStringTypeMacIface)
		}
	}
	// Note that we do not use the
	// InstallerCustomizations.KernelOptionsAppend here because
	// InstallerCustomizations.KernelOptionsAppend also picks up
	// the kernel options from the imageConfig but we only set
	// those in the rawImage.OSTreeDeploymentCustomizations and
	// not in the bootTreePipeline. Its unclear if we should change
	// this or not. Its inconsistent with the other installers but
	// then simplifiedInstaler is special.
	//
	// kernelOpts = append(kernelOpts, img.InstallerCustomizations.KernelOptionsAppend...)
	bootTreePipeline.KernelOpts = kernelOpts

	// enable ISOLinux on x86_64 only
	isoLinuxEnabled := img.platform.GetArch() == arch.ARCH_X86_64

	isoTreePipeline := manifest.NewCoreOSISOTree(buildPipeline, compressedImage, coiPipeline, bootTreePipeline)
	isoTreePipeline.KernelOpts = kernelOpts
	isoTreePipeline.PartitionTable = efiBootPartitionTable(rng)
	isoTreePipeline.OSName = img.OSName
	isoTreePipeline.PayloadPath = fmt.Sprintf("/%s", imageFilename)
	isoTreePipeline.ISOLinux = isoLinuxEnabled

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.ISOCustomizations)
	isoPipeline.SetFilename(img.filename)
	if isoLinuxEnabled {
		img.ISOCustomizations.BootType = manifest.SyslinuxISOBoot
	}

	artifact := isoPipeline.Export()
	return artifact, nil
}
