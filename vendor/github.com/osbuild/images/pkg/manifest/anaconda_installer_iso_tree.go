package manifest

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
)

type ISORootfsType uint64

// These constants are used by the ISO images to control the style of the root filesystem
const ( // Rootfs type enum
	SquashfsExt4Rootfs ISORootfsType = iota // Create an EXT4 rootfs compressed by Squashfs
	SquashfsRootfs                          // Create a plain squashfs rootfs
	ErofsRootfs                             // Create a plain erofs rootfs
)

func (r *ISORootfsType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "squashfs-ext4", "":
		*r = SquashfsExt4Rootfs
	case "squashfs":
		*r = SquashfsRootfs
	case "erofs":
		*r = ErofsRootfs
	default:
		return fmt.Errorf("unknown ISORootfsType: %q", s)
	}

	return nil
}

func (r *ISORootfsType) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(r, unmarshal)
}

type ISOBootType uint64

// These constants are used by the ISO images to control the type of bootable iso
const ( // ISOBoot type enum
	Grub2UEFIOnlyISOBoot ISOBootType = iota // Only boot with grub2 UEFI
	SyslinuxISOBoot                         // Boot with grub2 UEFI and syslinux/isolinux BIOS
	Grub2ISOBoot                            // Boot with grub2 UEFI and grub2 BIOS
)

func (r *ISOBootType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "grub2-uefi", "":
		*r = Grub2UEFIOnlyISOBoot
	case "syslinux":
		*r = SyslinuxISOBoot
	case "grub2":
		*r = Grub2ISOBoot
	default:
		return fmt.Errorf("unknown ISOBootType: %q", s)
	}

	return nil
}

func (r *ISOBootType) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(r, unmarshal)
}

// An AnacondaInstallerISOTree represents a tree containing the anaconda installer,
// configuration in terms of a kickstart file, as well as an embedded
// payload to be installed, this payload can either be an ostree
// CommitSpec or OSPipeline for an OS.
type AnacondaInstallerISOTree struct {
	Base

	// TODO: review optional and mandatory fields and their meaning
	Release string

	PartitionTable *disk.PartitionTable

	anacondaPipeline *AnacondaInstaller
	rootfsPipeline   *ISORootfsImg // May be nil for plain squashfs rootfs
	bootTreePipeline *EFIBootTree

	// The path where the payload (tarball, ostree repo, or container) will be stored.
	PayloadPath string

	// If set the skopeo stage will remove signatures during copy
	PayloadRemoveSignatures bool

	isoLabel string

	RootfsCompression string
	RootfsType        ISORootfsType

	OSPipeline         *OS
	OSTreeCommitSource *ostree.SourceSpec

	ostreeCommitSpec *ostree.CommitSpec
	ContainerSource  *container.SourceSpec
	containerSpec    *container.Spec

	// Kernel options for the ISO image
	KernelOpts []string

	// ISOBoot selects the type of boot support on the iso
	ISOBoot ISOBootType

	Kickstart *kickstart.Options

	Files []*fsnode.File

	// Pipeline object where subscription-related files are created for copying
	// onto the ISO.
	SubscriptionPipeline *Subscription

	InstallRootfsType disk.FSType
}

func NewAnacondaInstallerISOTree(buildPipeline Build, anacondaPipeline *AnacondaInstaller, rootfsPipeline *ISORootfsImg, bootTreePipeline *EFIBootTree) *AnacondaInstallerISOTree {

	// the three pipelines should all belong to the same manifest
	if (rootfsPipeline != nil && anacondaPipeline.Manifest() != rootfsPipeline.Manifest()) ||
		anacondaPipeline.Manifest() != bootTreePipeline.Manifest() {
		panic("pipelines from different manifests")
	}
	p := &AnacondaInstallerISOTree{
		Base:             NewBase("bootiso-tree", buildPipeline),
		anacondaPipeline: anacondaPipeline,
		rootfsPipeline:   rootfsPipeline,
		bootTreePipeline: bootTreePipeline,
		isoLabel:         bootTreePipeline.ISOLabel,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *AnacondaInstallerISOTree) getOSTreeCommitSources() []ostree.SourceSpec {
	if p.OSTreeCommitSource == nil {
		return nil
	}

	return []ostree.SourceSpec{
		*p.OSTreeCommitSource,
	}
}

func (p *AnacondaInstallerISOTree) getOSTreeCommits() []ostree.CommitSpec {
	if p.ostreeCommitSpec == nil {
		return nil
	}
	return []ostree.CommitSpec{*p.ostreeCommitSpec}
}

func (p *AnacondaInstallerISOTree) getContainerSpecs() []container.Spec {
	if p.containerSpec == nil {
		return []container.Spec{}
	}
	return []container.Spec{*p.containerSpec}
}

func (p *AnacondaInstallerISOTree) getContainerSources() []container.SourceSpec {
	if p.ContainerSource == nil {
		return []container.SourceSpec{}
	}
	return []container.SourceSpec{
		*p.ContainerSource,
	}
}

func (p *AnacondaInstallerISOTree) getInline() []string {
	inlineData := []string{}

	// inline data for custom files
	for _, file := range p.Files {
		inlineData = append(inlineData, string(file.Data()))
	}

	return inlineData
}
func (p *AnacondaInstallerISOTree) getBuildPackages(_ Distro) ([]string, error) {
	var packages []string
	switch p.RootfsType {
	case SquashfsExt4Rootfs, SquashfsRootfs:
		packages = []string{"squashfs-tools"}
	case ErofsRootfs:
		packages = []string{"erofs-utils"}
	default:
		return nil, fmt.Errorf("unknown rootfs type: %q", p.RootfsType)
	}

	if p.ISOBoot == Grub2ISOBoot {
		// Needed for the i386-pc directory of modules needed by grub2 BIOS booting
		packages = append(packages, "grub2-pc-modules")
	}

	if p.OSTreeCommitSource != nil {
		packages = append(packages, "rpm-ostree")
	}

	if p.ContainerSource != nil {
		packages = append(packages, "skopeo")
	}

	if p.OSPipeline != nil {
		packages = append(packages, "tar")
	}

	return packages, nil
}

// Exclude most of the /boot files inside the rootfs to save space
// These are not needed on the running system
// The kernel and kernel .hmac are left for use with FIPS systems
// Used by NewSquashfsStage and NewErofsStage
var installerBootExcludePaths = []string{
	"boot/efi/.*",
	"boot/grub2/.*",
	"boot/config-.*",
	"boot/initramfs-.*",
	"boot/loader/.*",
	"boot/symvers-.*",
	"boot/System.map-.*",
	"usr/lib/sysimage/rpm/.*",
	"var/lib/rpm/.*",
	"var/lib/yum/.*",
	"var/lib/dnf/.*",
}

// NewSquashfsStage returns an osbuild stage configured to build
// the squashfs root filesystem for the ISO.
func (p *AnacondaInstallerISOTree) NewSquashfsStage() (*osbuild.Stage, error) {
	var squashfsOptions osbuild.SquashfsStageOptions

	switch p.anacondaPipeline.Type {
	case AnacondaInstallerTypePayload, AnacondaInstallerTypeNetinst:
		squashfsOptions = osbuild.SquashfsStageOptions{
			Filename: "images/install.img",
		}
	case AnacondaInstallerTypeLive:
		squashfsOptions = osbuild.SquashfsStageOptions{
			Filename: "LiveOS/squashfs.img",
		}
	default:
		// Shouldn't be possible, but catch it anyway
		return nil, fmt.Errorf("unknown AnacondaInstallerType %v in NewSquashfsStage", p.anacondaPipeline.Type)
	}

	if p.RootfsCompression != "" {
		squashfsOptions.Compression.Method = p.RootfsCompression
	} else {
		// default to xz if not specified
		squashfsOptions.Compression.Method = "xz"
	}

	if squashfsOptions.Compression.Method == "xz" {
		squashfsOptions.Compression.Options = &osbuild.FSCompressionOptions{
			BCJ: osbuild.BCJOption(p.anacondaPipeline.platform.GetArch().String()),
		}
	}

	// Clean up the root filesystem's /boot to save space
	squashfsOptions.ExcludePaths = installerBootExcludePaths

	// The iso's rootfs can either be an ext4 filesystem compressed with squashfs, or
	// a squashfs of the plain directory tree
	if p.RootfsType == SquashfsExt4Rootfs && p.rootfsPipeline != nil {
		return osbuild.NewSquashfsStage(&squashfsOptions, p.rootfsPipeline.Name()), nil
	}
	return osbuild.NewSquashfsStage(&squashfsOptions, p.anacondaPipeline.Name()), nil
}

// NewErofsStage returns an osbuild stage configured to build
// the erofs root filesystem for the ISO.
func (p *AnacondaInstallerISOTree) NewErofsStage() (*osbuild.Stage, error) {
	var erofsOptions osbuild.ErofsStageOptions

	switch p.anacondaPipeline.Type {
	case AnacondaInstallerTypePayload, AnacondaInstallerTypeNetinst:
		erofsOptions = osbuild.ErofsStageOptions{
			Filename: "images/install.img",
		}
	case AnacondaInstallerTypeLive:
		erofsOptions = osbuild.ErofsStageOptions{
			Filename: "LiveOS/squashfs.img",
		}
	default:
		// Shouldn't be possible, but catch it anyway
		return nil, fmt.Errorf("unknown AnacondaInstallerType %v in NewErofsStage", p.anacondaPipeline.Type)
	}

	var compression osbuild.ErofsCompression
	if p.RootfsCompression != "" {
		compression.Method = p.RootfsCompression
	} else {
		// default to zstd if not specified
		compression.Method = "zstd"
	}
	compression.Level = common.ToPtr(8)
	erofsOptions.Compression = &compression
	erofsOptions.ExtendedOptions = []string{"all-fragments", "dedupe"}
	erofsOptions.ClusterSize = common.ToPtr(131072)

	// Clean up the root filesystem's /boot to save space
	erofsOptions.ExcludePaths = installerBootExcludePaths

	return osbuild.NewErofsStage(&erofsOptions, p.anacondaPipeline.Name()), nil
}

func (p *AnacondaInstallerISOTree) serializeStart(inputs Inputs) error {
	if p.ostreeCommitSpec != nil || p.containerSpec != nil {
		return errors.New("AnacondaInstallerISOTree: double call to serializeStart()")
	}

	if len(inputs.Commits) > 1 {
		return errors.New("AnacondaInstallerISOTree: pipeline supports at most one ostree commit")
	}

	if len(inputs.Containers) > 1 {
		return errors.New("AnacondaInstallerISOTree: pipeline supports at most one container")
	}

	if len(inputs.Commits) > 0 {
		p.ostreeCommitSpec = &inputs.Commits[0]
	}

	if len(inputs.Containers) > 0 {
		p.containerSpec = &inputs.Containers[0]
	}

	return nil
}

func (p *AnacondaInstallerISOTree) serializeEnd() {
	p.ostreeCommitSpec = nil
	p.containerSpec = nil
}

func (p *AnacondaInstallerISOTree) serialize() (osbuild.Pipeline, error) {
	// If the anaconda pipeline is a payload then we need one of three payload types
	if p.anacondaPipeline.Type == AnacondaInstallerTypePayload {
		count := 0

		if p.ostreeCommitSpec != nil {
			count++
		}

		if p.containerSpec != nil {
			count++
		}

		if p.OSPipeline != nil {
			count++
		}

		if count == 0 {
			return osbuild.Pipeline{}, fmt.Errorf("missing ostree, container, or ospipeline parameters in ISO tree pipeline")
		}

		// But not more than one payloads
		if count > 1 {
			return osbuild.Pipeline{}, fmt.Errorf("got multiple payloads in ISO tree pipeline")
		}
	}

	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	kernelOpts := []string{}

	if p.anacondaPipeline.Type == AnacondaInstallerTypePayload ||
		p.anacondaPipeline.Type == AnacondaInstallerTypeNetinst {
		kernelOpts = append(kernelOpts, fmt.Sprintf("inst.stage2=hd:LABEL=%s", p.isoLabel))
		if p.Kickstart != nil && p.Kickstart.Path != "" {
			kernelOpts = append(kernelOpts, fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", p.isoLabel, p.Kickstart.Path))
		}
	}

	if len(p.KernelOpts) > 0 {
		kernelOpts = append(kernelOpts, p.KernelOpts...)
	}

	pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{
			{
				Path: "/images",
			},
			{
				Path: "/images/pxeboot",
			},
		},
	}))

	if p.anacondaPipeline.Type == AnacondaInstallerTypeLive {
		pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
			Paths: []osbuild.MkdirStagePath{
				{
					Path: "/LiveOS",
				},
			},
		}))
	}

	inputName := "tree"
	copyStageOptions := &osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/boot/vmlinuz-%s", inputName, p.anacondaPipeline.kernelVer),
				To:   "tree:///images/pxeboot/vmlinuz",
			},
			{
				From: fmt.Sprintf("input://%s/boot/initramfs-%s.img", inputName, p.anacondaPipeline.kernelVer),
				To:   "tree:///images/pxeboot/initrd.img",
			},
		},
	}
	copyStageInputs := osbuild.NewPipelineTreeInputs(inputName, p.anacondaPipeline.Name())
	copyStage := osbuild.NewCopyStageSimple(copyStageOptions, copyStageInputs)
	pipeline.AddStage(copyStage)

	// Add the selected roofs stage
	switch p.RootfsType {
	case SquashfsExt4Rootfs, SquashfsRootfs:
		stage, err := p.NewSquashfsStage()
		if err != nil {
			return osbuild.Pipeline{}, fmt.Errorf("cannot create squashfs stage: %w", err)
		}
		pipeline.AddStage(stage)
	case ErofsRootfs:
		stage, err := p.NewErofsStage()
		if err != nil {
			return osbuild.Pipeline{}, fmt.Errorf("cannot create erofs stage: %w", err)
		}
		pipeline.AddStage(stage)
	default:
	}

	switch p.ISOBoot {
	case SyslinuxISOBoot:
		options := &osbuild.ISOLinuxStageOptions{
			Product: osbuild.ISOLinuxProduct{
				Name:    p.anacondaPipeline.InstallerCustomizations.Product,
				Version: p.anacondaPipeline.InstallerCustomizations.OSVersion,
			},
			Kernel: osbuild.ISOLinuxKernel{
				Dir:  "/images/pxeboot",
				Opts: kernelOpts,
			},
			FIPS: p.anacondaPipeline.platform.GetFIPSMenu(),
		}

		stage := osbuild.NewISOLinuxStage(options, p.anacondaPipeline.Name())
		pipeline.AddStage(stage)
	case Grub2ISOBoot:
		var grub2config *osbuild.Grub2Config
		if p.anacondaPipeline.InstallerCustomizations.DefaultMenu > 0 {
			grub2config = &osbuild.Grub2Config{
				Default: p.anacondaPipeline.InstallerCustomizations.DefaultMenu,
			}
		}
		options := &osbuild.Grub2ISOLegacyStageOptions{
			Product: osbuild.Product{
				Name:    p.anacondaPipeline.InstallerCustomizations.Product,
				Version: p.anacondaPipeline.InstallerCustomizations.OSVersion,
			},
			Kernel: osbuild.ISOKernel{
				Dir:  "/images/pxeboot",
				Opts: kernelOpts,
			},
			ISOLabel: p.isoLabel,
			FIPS:     p.anacondaPipeline.platform.GetFIPSMenu(),
			Config:   grub2config,
		}

		stage := osbuild.NewGrub2ISOLegacyStage(options)
		pipeline.AddStage(stage)

		// Add a stage to create the eltorito.img file for grub2 BIOS boot support
		pipeline.AddStage(osbuild.NewGrub2InstStage(osbuild.NewGrub2InstISO9660StageOption("images/eltorito.img", "/boot/grub2")))
	}

	filename := "images/efiboot.img"
	pipeline.AddStage(osbuild.NewTruncateStage(&osbuild.TruncateStageOptions{
		Filename: filename,
		Size:     fmt.Sprintf("%d", p.PartitionTable.Size),
	}))

	for _, stage := range osbuild.GenFsStages(p.PartitionTable, filename, p.anacondaPipeline.Name()) {
		pipeline.AddStage(stage)
	}

	inputName = "root-tree"
	copyInputs := osbuild.NewPipelineTreeInputs(inputName, p.bootTreePipeline.Name())
	copyOptions, copyDevices, copyMounts := osbuild.GenCopyFSTreeOptions(inputName, p.bootTreePipeline.Name(), filename, p.PartitionTable)
	pipeline.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	copyInputs = osbuild.NewPipelineTreeInputs(inputName, p.bootTreePipeline.Name())
	pipeline.AddStage(osbuild.NewCopyStageSimple(
		&osbuild.CopyStageOptions{
			Paths: []osbuild.CopyStagePath{
				{
					From: fmt.Sprintf("input://%s/EFI", inputName),
					To:   "tree:///",
				},
			},
		},
		copyInputs,
	))

	if p.anacondaPipeline.Type == AnacondaInstallerTypePayload {
		// the following pipelines are only relevant for payload installers
		switch {
		case p.ostreeCommitSpec != nil:
			ostreeCommitStages, err := p.ostreeCommitStages()
			if err != nil {
				return osbuild.Pipeline{}, fmt.Errorf("cannot create ostree commit stages: %w", err)
			}
			pipeline.AddStages(ostreeCommitStages...)
		case p.containerSpec != nil:
			ostreeContainerStages, err := p.ostreeContainerStages()
			if err != nil {
				return osbuild.Pipeline{}, fmt.Errorf("cannot create ostree container stages: %w", err)
			}
			pipeline.AddStages(ostreeContainerStages...)
		case p.OSPipeline != nil:
			tarPayloadStages, err := p.tarPayloadStages()
			if err != nil {
				return osbuild.Pipeline{}, fmt.Errorf("cannot create tar payload stages: %w", err)
			}
			pipeline.AddStages(tarPayloadStages...)
		default:
			// this should have been caught at the top of the function, but
			// let's check again in case we refactor the function.
			return osbuild.Pipeline{}, fmt.Errorf("missing ostree, container, or ospipeline parameters in ISO tree pipeline")
		}
	}

	pipeline.AddStage(osbuild.NewDiscinfoStage(&osbuild.DiscinfoStageOptions{
		BaseArch: p.anacondaPipeline.platform.GetArch().String(),
		Release:  p.Release,
	}))

	return pipeline, nil
}

func (p *AnacondaInstallerISOTree) ostreeCommitStages() ([]*osbuild.Stage, error) {
	stages := make([]*osbuild.Stage, 0)

	// Set up the payload ostree repo
	stages = append(stages, osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: p.PayloadPath}))
	stages = append(stages, osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: p.PayloadPath},
		osbuild.NewOstreePullStageInputs("org.osbuild.source", p.ostreeCommitSpec.Checksum, p.ostreeCommitSpec.Ref),
	))

	if p.Kickstart == nil {
		return nil, fmt.Errorf("Kickstart options not set for %s pipeline", p.name)
	}

	if p.Kickstart.OSTree == nil {
		return nil, fmt.Errorf("Kickstart ostree options not set for %s pipeline", p.name)
	}
	// Configure the kickstart file with the payload and any user options
	kickstartOptions, err := osbuild.NewKickstartStageOptionsWithOSTreeCommit(
		p.Kickstart.Path,
		p.Kickstart.Users,
		p.Kickstart.Groups,
		makeISORootPath(p.PayloadPath),
		p.ostreeCommitSpec.Ref,
		p.Kickstart.OSTree.Remote,
		p.Kickstart.OSTree.OSName)

	if err != nil {
		return nil, fmt.Errorf("failed to create kickstart stage options: %w", err)
	}

	kickstartStages, err := p.makeKickstartStages(kickstartOptions)
	if err != nil {
		return nil, fmt.Errorf("cannot create kickstart stages: %w", err)
	}
	stages = append(stages, kickstartStages...)

	return stages, nil
}

func (p *AnacondaInstallerISOTree) ostreeContainerStages() ([]*osbuild.Stage, error) {
	stages := make([]*osbuild.Stage, 0)

	image := osbuild.NewContainersInputForSingleSource(*p.containerSpec)

	stages = append(stages, osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{
			{
				Path: p.PayloadPath,
			},
		},
	}))

	// copy the container in
	skopeoStage := osbuild.NewSkopeoStageWithOCI(
		p.PayloadPath,
		image,
		nil)
	if p.PayloadRemoveSignatures {
		opts := skopeoStage.Options.(*osbuild.SkopeoStageOptions)
		opts.RemoveSignatures = common.ToPtr(true)
	}
	stages = append(stages, skopeoStage)

	kickstartStages, err := p.bootcInstallerKickstartStages()
	if err != nil {
		return nil, fmt.Errorf("cannot generate bootc installer kickstart stages: %w", err)
	}
	stages = append(stages, kickstartStages...)
	return stages, nil
}

// bootcInstallerKickstartStages sets up kickstart-related stages for Anaconda
// ISOs that install a bootc bootable container.
func (p *AnacondaInstallerISOTree) bootcInstallerKickstartStages() ([]*osbuild.Stage, error) {
	if p.Kickstart == nil {
		return nil, fmt.Errorf("Kickstart options not set for %s pipeline", p.name)
	}

	stages := make([]*osbuild.Stage, 0)

	// do what we can in our kickstart stage
	kickstartOptions, err := osbuild.NewKickstartStageOptionsWithOSTreeContainer(
		p.Kickstart.Path,
		p.Kickstart.Users,
		p.Kickstart.Groups,
		path.Join("/run/install/repo", p.PayloadPath),
		"oci",
		"",
		"")
	if err != nil {
		return nil, fmt.Errorf("failed to create kickstart stage options: %w", err)
	}

	// Workaround for lack of --target-imgref in Anaconda, xref https://github.com/osbuild/images/issues/380
	kickstartOptions.Post = append(kickstartOptions.Post, osbuild.PostOptions{
		ErrorOnFail: true,
		Commands: []string{
			fmt.Sprintf("bootc switch --mutate-in-place --transport registry %s", p.containerSpec.LocalName),
			"# used during automatic image testing as finished marker",
			"if [ -c /dev/ttyS0 ]; then",
			"  # continue on errors here, because we used to omit --erroronfail",
			`  echo "Install finished" > /dev/ttyS0 || true`,
			"fi",
		},
	})

	// kickstart.New() already validates the options but they may have been
	// modified since then, so validate them before we create the stages
	if err := p.Kickstart.Validate(); err != nil {
		return nil, err
	}

	if p.Kickstart.UserFile != nil {

		// when a user defines their own kickstart, we create a kickstart that
		// takes care of the installation and let the user kickstart handle
		// everything else
		stages = append(stages, osbuild.NewKickstartStage(kickstartOptions))
		kickstartFile, err := kickstartOptions.IncludeRaw(p.Kickstart.UserFile.Contents)
		if err != nil {
			return nil, err
		}
		p.Files = append(p.Files, kickstartFile)
		return append(stages, osbuild.GenFileNodesStages(p.Files)...), nil
	}

	// create a fully unattended/automated kickstart

	// NOTE: these are similar to the unattended kickstart options in the
	// other two payload configurations but partitioning is different and
	// we need to add that separately, so we can't use makeKickstartStage
	kickstartOptions.RootPassword = &osbuild.RootPasswordOptions{
		Lock: true,
	}

	// NOTE: These were decided somewhat arbitrarily for the BIB installer. We
	// might want to drop them here and move them into the bib code as
	// project-specific defaults.

	// TODO: unify with other ostree variants and allow overrides from customizations
	kickstartOptions.Lang = "en_US.UTF-8"
	kickstartOptions.Keyboard = "us"
	kickstartOptions.Timezone = "UTC"
	kickstartOptions.ClearPart = &osbuild.ClearPartOptions{
		All: true,
	}

	if len(p.Kickstart.KernelOptionsAppend) > 0 {
		kickstartOptions.Bootloader = &osbuild.BootloaderOptions{
			// We currently leaves quoting to the
			// user. This is generally ok - to do better
			// we will have to mimic the kernel arg
			// parser, see
			// https://www.kernel.org/doc/html/latest/admin-guide/kernel-parameters.html
			// and lib/cmdline.c in the kernel source
			Append: strings.Join(p.Kickstart.KernelOptionsAppend, " "),
		}
	}
	if p.Kickstart.NetworkOnBoot {
		kickstartOptions.Network = []osbuild.NetworkOptions{
			{BootProto: "dhcp", Device: "link", Activate: common.ToPtr(true), OnBoot: "on"},
		}
	}

	stages = append(stages, osbuild.NewKickstartStage(kickstartOptions))

	// Because osbuild core only supports a subset of options, we append to the
	// base here with some more hardcoded defaults
	// that should very likely become configurable.
	var hardcodedKickstartBits string

	// using `autopart` because  `part / --fstype=btrfs` didn't work
	rootFsType := p.InstallRootfsType
	if rootFsType == disk.FS_NONE {
		// if the rootfs type is not set, we default to ext4
		rootFsType = disk.FS_EXT4
	}
	switch rootFsType {
	case disk.FS_BTRFS:
		hardcodedKickstartBits = `
autopart --nohome --type=btrfs
`
	default:
		hardcodedKickstartBits = fmt.Sprintf(`
autopart --nohome --type=plain --fstype=%s
`, rootFsType.String())
	}

	hardcodedKickstartBits += `
reboot --eject
`

	kickstartFile, err := kickstartOptions.IncludeRaw(hardcodedKickstartBits)
	if err != nil {
		return nil, err
	}

	p.Files = append(p.Files, kickstartFile)
	return append(stages, osbuild.GenFileNodesStages(p.Files)...), nil
}

func (p *AnacondaInstallerISOTree) tarPayloadStages() ([]*osbuild.Stage, error) {
	stages := make([]*osbuild.Stage, 0)

	// Create the payload tarball
	stages = append(stages, osbuild.NewTarStage(&osbuild.TarStageOptions{Filename: p.PayloadPath}, p.OSPipeline.name))

	// If the KSPath is set, we need to add the kickstart stage to this (bootiso-tree) pipeline.
	// If it's not specified here, it should have been added to the InteractiveDefaults in the anaconda-tree.
	if p.Kickstart != nil && p.Kickstart.Path != "" {
		kickstartOptions, err := osbuild.NewKickstartStageOptionsWithLiveIMG(
			p.Kickstart.Path,
			p.Kickstart.Users,
			p.Kickstart.Groups,
			makeISORootPath(p.PayloadPath))

		if err != nil {
			return nil, fmt.Errorf("failed to create kickstart stage options: %w", err)
		}

		kickstartStages, err := p.makeKickstartStages(kickstartOptions)
		if err != nil {
			return nil, fmt.Errorf("cannot create kickstart stages: %w", err)
		}
		stages = append(stages, kickstartStages...)
	}
	return stages, nil
}

// Create the base kickstart stage with any options required for unattended
// installation if set and with any extra file insertion stage required for
// extra kickstart content.
func (p *AnacondaInstallerISOTree) makeKickstartStages(stageOptions *osbuild.KickstartStageOptions) ([]*osbuild.Stage, error) {
	kickstartOptions := p.Kickstart
	if kickstartOptions == nil {
		kickstartOptions = new(kickstart.Options)
	}

	stages := make([]*osbuild.Stage, 0)

	// kickstart.New() already validates the options but they may have been
	// modified since then, so validate them before we create the stages
	if err := p.Kickstart.Validate(); err != nil {
		return nil, err
	}

	if kickstartOptions.UserFile != nil {
		stages = append(stages, osbuild.NewKickstartStage(stageOptions))
		if kickstartOptions.UserFile != nil {
			kickstartFile, err := stageOptions.IncludeRaw(kickstartOptions.UserFile.Contents)
			if err != nil {
				return nil, err
			}

			p.Files = append(p.Files, kickstartFile)
		}
	}

	if kickstartOptions.Unattended {
		// set the default options for Unattended kickstart
		stageOptions.DisplayMode = "text"

		// override options that can be configured by the image type or the user
		stageOptions.Lang = "en_US.UTF-8"
		if kickstartOptions.Language != nil {
			stageOptions.Lang = *kickstartOptions.Language
		}

		stageOptions.Keyboard = "us"
		if kickstartOptions.Keyboard != nil {
			stageOptions.Keyboard = *kickstartOptions.Keyboard
		}

		stageOptions.Timezone = "UTC"
		if kickstartOptions.Timezone != nil {
			stageOptions.Timezone = *kickstartOptions.Timezone
		}

		stageOptions.Reboot = &osbuild.RebootOptions{Eject: true}
		stageOptions.RootPassword = &osbuild.RootPasswordOptions{Lock: true}

		stageOptions.ZeroMBR = true
		stageOptions.ClearPart = &osbuild.ClearPartOptions{All: true, InitLabel: true}
		stageOptions.AutoPart = &osbuild.AutoPartOptions{Type: "plain", FSType: "xfs", NoHome: true}

		stageOptions.Network = []osbuild.NetworkOptions{
			{BootProto: "dhcp", Device: "link", Activate: common.ToPtr(true), OnBoot: "on"},
		}
	}

	if sudoersPost := makeKickstartSudoersPost(kickstartOptions.SudoNopasswd); sudoersPost != nil {
		stageOptions.Post = append(stageOptions.Post, *sudoersPost)
	}
	stages = append(stages, osbuild.NewKickstartStage(stageOptions))

	if p.SubscriptionPipeline != nil {
		subscriptionPath := "/subscription"
		stages = append(stages, osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{Paths: []osbuild.MkdirStagePath{{Path: subscriptionPath, Parents: true, ExistOk: true}}}))
		inputName := "subscription-tree"
		copyInputs := osbuild.NewPipelineTreeInputs(inputName, p.SubscriptionPipeline.Name())
		copyOptions := &osbuild.CopyStageOptions{}
		copyOptions.Paths = append(copyOptions.Paths,
			osbuild.CopyStagePath{
				From: fmt.Sprintf("input://%s/", inputName),
				To:   fmt.Sprintf("tree://%s/", subscriptionPath),
			},
		)
		stages = append(stages, osbuild.NewCopyStageSimple(copyOptions, copyInputs))
		systemPath := "/mnt/sysimage"
		if p.ostreeCommitSpec != nil || p.containerSpec != nil {
			// ostree based system: use /mnt/sysroot instead
			systemPath = "/mnt/sysroot"

		}
		stageOptions.Post = append(stageOptions.Post, makeKickstartSubscriptionPost(subscriptionPath, systemPath)...)

		// include a readme file on the ISO in the subscription path to explain what it's for
		subscriptionReadme, err := fsnode.NewFile(
			filepath.Join(subscriptionPath, "README"),
			nil, nil, nil,
			[]byte(`Subscription services and credentials

This directory contains files necessary for registering the system on first boot after installation. These files are copied to the installed system and services are enabled to activate the subscription on boot.`),
		)
		if err != nil {
			return nil, err
		}
		p.Files = append(p.Files, subscriptionReadme)
	}

	stages = append(stages, osbuild.GenFileNodesStages(p.Files)...)

	return stages, nil
}

// makeISORootPath return a path that can be used to address files and folders
// in the root of the iso
func makeISORootPath(p string) string {
	fullpath := path.Join("/run/install/repo", p)
	return fmt.Sprintf("file://%s", fullpath)
}

func makeKickstartSudoersPost(names []string) *osbuild.PostOptions {
	if len(names) == 0 {
		return nil
	}
	echoLineFmt := `echo -e "%[1]s\tALL=(ALL)\tNOPASSWD: ALL" > "/etc/sudoers.d/%[1]s"`
	chmodLineFmt := `chmod 0440 /etc/sudoers.d/%[1]s`

	filenames := make(map[string]bool)
	sort.Strings(names)
	post := &osbuild.PostOptions{}
	for _, name := range names {
		if filenames[name] {
			continue
		}
		post.Commands = append(post.Commands,
			fmt.Sprintf(echoLineFmt, name),
			fmt.Sprintf(chmodLineFmt, name),
		)
		filenames[name] = true
	}

	post.Commands = append(post.Commands, "restorecon -rvF /etc/sudoers.d")
	return post
}

func makeKickstartSubscriptionPost(source, dest string) []osbuild.PostOptions {
	fullSourcePath := filepath.Join("/run/install/repo", source, "etc/*")
	return []osbuild.PostOptions{
		{
			// we need to use --nochroot so the command can access files on the ISO
			NoChroot: true,
			Commands: []string{
				fmt.Sprintf("cp -r %s %s", fullSourcePath, dest),
			},
		},
		{
			Commands: []string{"systemctl enable osbuild-subscription-register.service"},
		},
	}
}
