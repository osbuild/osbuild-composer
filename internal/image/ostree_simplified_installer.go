package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/artifact"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/environment"
	"github.com/osbuild/osbuild-composer/internal/fdo"
	"github.com/osbuild/osbuild-composer/internal/ignition"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/runner"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

type OSTreeSimplifiedInstaller struct {
	Base

	// Raw image that will be created and embedded
	rawImage *OSTreeRawImage

	Platform         platform.Platform
	OSCustomizations manifest.OSCustomizations
	Environment      environment.Environment
	Workload         workload.Workload

	ExtraBasePackages rpmmd.PackageSet

	// ISO label template (architecture-free)
	ISOLabelTempl string

	// Product string for ISO buildstamp
	Product string

	// OSVersion string for ISO buildstamp
	OSVersion string

	// Variant string for ISO buildstamp
	Variant string

	// OSName for ostree deployment
	OSName string

	installDevice string

	Filename string

	FDO *fdo.Options

	// Ignition firstboot configuration options
	IgnitionFirstBoot *ignition.FirstBootOptions

	// Ignition embedded configuration options
	IgnitionEmbedded *ignition.EmbeddedOptions

	AdditionalDracutModules []string
}

func NewOSTreeSimplifiedInstaller(rawImage *OSTreeRawImage, installDevice string) *OSTreeSimplifiedInstaller {
	return &OSTreeSimplifiedInstaller{
		Base:          NewBase("ostree-simplified-installer"),
		rawImage:      rawImage,
		installDevice: installDevice,
	}
}

func (img *OSTreeSimplifiedInstaller) InstantiateManifest(m *manifest.Manifest,
	repos []rpmmd.RepoConfig,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	buildPipeline := manifest.NewBuild(m, runner, repos)
	buildPipeline.Checkpoint()

	rawImageFilename := "image.raw.xz"

	// create the raw image
	img.rawImage.Filename = rawImageFilename
	rawImage := ostreeCompressedImagePipelines(img.rawImage, m, buildPipeline)

	coiPipeline := manifest.NewCoreOSInstaller(m,
		buildPipeline,
		img.Platform,
		repos,
		"kernel",
		img.Product,
		img.OSVersion)
	coiPipeline.ExtraPackages = img.ExtraBasePackages.Include
	coiPipeline.ExtraRepos = img.ExtraBasePackages.Repositories
	coiPipeline.FDO = img.FDO
	coiPipeline.Ignition = img.IgnitionEmbedded
	coiPipeline.Biosdevname = (img.Platform.GetArch() == platform.ARCH_X86_64)
	coiPipeline.Variant = img.Variant
	coiPipeline.AdditionalDracutModules = img.AdditionalDracutModules

	isoLabel := fmt.Sprintf(img.ISOLabelTempl, img.Platform.GetArch())

	bootTreePipeline := manifest.NewEFIBootTree(m, buildPipeline, img.Product, img.OSVersion)
	bootTreePipeline.Platform = img.Platform
	bootTreePipeline.UEFIVendor = img.Platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = isoLabel

	// kernel options for EFI boot tree grub stage
	kernelOpts := []string{
		"rd.neednet=1",
		"coreos.inst.crypt_root=1",
		"coreos.inst.isoroot=" + isoLabel,
		"coreos.inst.install_dev=" + img.installDevice,
		fmt.Sprintf("coreos.inst.image_file=/run/media/iso/%s", rawImageFilename),
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
	}

	// ignition firstboot options
	if img.IgnitionFirstBoot != nil {
		kernelOpts = append(kernelOpts, "coreos.inst.append ignition.config.url="+img.IgnitionFirstBoot.ProvisioningURL)
	}

	bootTreePipeline.KernelOpts = kernelOpts

	rootfsPartitionTable := &disk.PartitionTable{
		Size: 20 * common.MebiByte,
		Partitions: []disk.Partition{
			{
				Start: 0,
				Size:  20 * common.MebiByte,
				Payload: &disk.Filesystem{
					Type:       "vfat",
					Mountpoint: "/",
					UUID:       disk.NewVolIDFromRand(rng),
				},
			},
		},
	}

	isoTreePipeline := manifest.NewCoreOSISOTree(m,
		buildPipeline,
		rawImage,
		coiPipeline,
		bootTreePipeline,
		isoLabel)
	isoTreePipeline.PartitionTable = rootfsPartitionTable
	isoTreePipeline.OSName = img.OSName
	isoTreePipeline.PayloadPath = fmt.Sprintf("/%s", rawImageFilename)

	isoPipeline := manifest.NewISO(m, buildPipeline, isoTreePipeline, isoLabel)
	isoPipeline.Filename = img.Filename
	isoPipeline.ISOLinux = false

	artifact := isoPipeline.Export()
	return artifact, nil
}
