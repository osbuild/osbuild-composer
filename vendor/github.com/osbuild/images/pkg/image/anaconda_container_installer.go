package image

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/runner"
)

type AnacondaContainerInstaller struct {
	Base
	AnacondaInstallerBase

	ContainerSource           container.SourceSpec
	InstallerPayload          container.SourceSpec
	ContainerRemoveSignatures bool

	// Locale for the installer. This should be set to the same locale as the
	// ISO OS payload, if known.
	Locale string

	// Filesystem type for the installed system as opposed to that of the ISO.
	InstallRootfsType disk.FSType

	// KernelVer is needed so that dracut finds it files
	KernelVer string
	// {Kernel,Initramfs}Path is needed for grub2.iso
	KernelPath    string
	InitramfsPath string
	// bootc installer cannot use /root as installer home
	InstallerHome string
}

func NewAnacondaContainerInstaller(platform platform.Platform, filename string, container container.SourceSpec) *AnacondaContainerInstaller {
	return &AnacondaContainerInstaller{
		Base:            NewBase("bootc-installer", platform, filename),
		ContainerSource: container,
	}
}

func (img *AnacondaContainerInstaller) InstantiateManifestFromContainer(m *manifest.Manifest,
	containers []container.SourceSpec,
	runner runner.Runner,
	rng *rand.Rand) (*artifact.Artifact, error) {
	cnts := []container.SourceSpec{img.ContainerSource}
	buildPipeline := manifest.NewBuildFromContainer(m, runner, cnts,
		&manifest.BuildOptions{
			ContainerBuildable: true,
		})

	anacondaPipeline := manifest.NewAnacondaInstaller(
		manifest.AnacondaInstallerTypePayload,
		buildPipeline,
		img.platform,
		nil, // repos
		"kernel",
		img.InstallerCustomizations,
	)
	// with bootc we need different kernel/initramfs paths
	anacondaPipeline.BootcLivefsContainer = &img.ContainerSource
	anacondaPipeline.KernelPath = img.KernelPath
	anacondaPipeline.InitramfsPath = img.InitramfsPath
	anacondaPipeline.SetKernelVer(img.KernelVer)
	anacondaPipeline.InstallerHome = img.InstallerHome

	anacondaPipeline.Biosdevname = (img.platform.GetArch() == arch.ARCH_X86_64)
	anacondaPipeline.Checkpoint()

	if anacondaPipeline.InstallerCustomizations.FIPS {
		anacondaPipeline.InstallerCustomizations.EnabledAnacondaModules = append(
			anacondaPipeline.InstallerCustomizations.EnabledAnacondaModules,
			anaconda.ModuleSecurity,
		)
	}

	anacondaPipeline.Locale = img.Locale

	var rootfsImagePipeline *manifest.ISORootfsImg
	switch img.InstallerCustomizations.ISORootfsType {
	case manifest.SquashfsExt4Rootfs:
		rootfsImagePipeline = manifest.NewISORootfsImg(buildPipeline, anacondaPipeline)
		rootfsImagePipeline.Size = 4 * datasizes.GibiByte
	default:
	}

	bootTreePipeline := manifest.NewEFIBootTree(buildPipeline, img.InstallerCustomizations.Product, img.InstallerCustomizations.OSVersion)
	bootTreePipeline.Platform = img.platform
	bootTreePipeline.UEFIVendor = img.platform.GetUEFIVendor()
	bootTreePipeline.ISOLabel = img.InstallerCustomizations.ISOLabel

	if img.Kickstart == nil {
		img.Kickstart = &kickstart.Options{}
	}
	if img.Kickstart.Path == "" {
		img.Kickstart.Path = osbuild.KickstartPathOSBuild
	}

	kernelOpts := []string{
		fmt.Sprintf("inst.stage2=hd:LABEL=%s", img.InstallerCustomizations.ISOLabel),
		fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", img.InstallerCustomizations.ISOLabel, img.Kickstart.Path),
		"console=tty0",
		// XXX: we want the graphical installer eventually, just
		// need to figure out the dependencies
		"inst.text",
	}
	if anacondaPipeline.InstallerCustomizations.FIPS {
		kernelOpts = append(kernelOpts, "fips=1")
	}
	kernelOpts = append(kernelOpts, img.InstallerCustomizations.KernelOptionsAppend...)
	bootTreePipeline.KernelOpts = kernelOpts

	isoTreePipeline := manifest.NewAnacondaInstallerISOTree(buildPipeline, anacondaPipeline, rootfsImagePipeline, bootTreePipeline)
	initIsoTreePipeline(isoTreePipeline, &img.AnacondaInstallerBase, rng)

	// For ostree installers, always put the kickstart file in the root of the ISO
	isoTreePipeline.PayloadPath = "/container"
	isoTreePipeline.PayloadRemoveSignatures = img.ContainerRemoveSignatures
	isoTreePipeline.ContainerSource = &img.InstallerPayload
	isoTreePipeline.InstallRootfsType = img.InstallRootfsType

	isoPipeline := manifest.NewISO(buildPipeline, isoTreePipeline, img.InstallerCustomizations.ISOLabel)
	isoPipeline.SetFilename(img.filename)
	isoPipeline.ISOBoot = img.InstallerCustomizations.ISOBoot
	artifact := isoPipeline.Export()

	return artifact, nil
}
