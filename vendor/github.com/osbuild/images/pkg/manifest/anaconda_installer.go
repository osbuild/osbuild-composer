package manifest

import (
	"errors"
	"fmt"
	"os"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/depsolvednf"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

type AnacondaInstallerType int

const (
	AnacondaInstallerTypeLive AnacondaInstallerType = iota + 1
	AnacondaInstallerTypePayload
	AnacondaInstallerTypeNetinst
)

// An Anaconda represents the installer tree as found on an ISO this can be either
// a payload installer or a live installer depending on `Type`.
type AnacondaInstaller struct {
	Base

	// The type of the Anaconda installer tree to prepare, this can be either
	// a 'live' or a 'payload' and it controls which stages are added to the
	// manifest.
	Type AnacondaInstallerType

	// InstallerCustomizations to apply to the installer pipeline(s)
	InstallerCustomizations InstallerCustomizations

	// ISOCustomizations to apply to the ISO pipeline(s)
	ISOCustomizations ISOCustomizations

	// Packages to install and/or exclude in addition to the ones required by the
	// pipeline.
	ExtraPackages   []string
	ExcludePackages []string

	// Extra repositories to install packages from
	ExtraRepos []rpmmd.RepoConfig

	// Biosdevname indicates whether or not biosdevname should be used to
	// name network devices when booting the installer. This may affect
	// the naming of network devices on the target system.
	Biosdevname bool

	platform platform.Platform
	// depsolveRepos holds the repository configuration used by
	// getPackageSetChain() for depsolving. After depsolving, use
	// depsolveResult.Repos which contains only repos that provided packages.
	depsolveRepos  []rpmmd.RepoConfig
	depsolveResult *depsolvednf.DepsolveResult
	kernelName     string
	kernelVer      string

	// some images (like bootc installers) know their path in
	// advance
	KernelPath    string
	InitramfsPath string
	// bootc installer cannot use /root as installer home
	InstallerHome string

	// BootcLivefsContainer is the container to use to
	// as the base live filesystem. This is mutally exclusive
	// with using RPMs.
	BootcLivefsContainer      *container.SourceSpec
	bootcLivefsContainerSpecs []container.Spec

	// Interactive defaults is a kickstart stage that can be provided, it
	// will be written to /usr/share/anaconda/interactive-defaults
	InteractiveDefaults *AnacondaInteractiveDefaults

	// Kickstart options that will be written to the interactive defaults
	// kickstart file. Currently only supports Users and Groups. Other
	// properties are ignored.
	InteractiveDefaultsKickstart *kickstart.Options

	Files []*fsnode.File

	// SELinux policy, when set it enables the labeling of the installer
	// tree with the selected profile and selects the required package
	// for depsolving
	SELinux string

	// Locale for the installer. This should be set to the same locale as the
	// ISO OS payload, if known.
	Locale string
}

func NewAnacondaInstaller(installerType AnacondaInstallerType,
	buildPipeline Build,
	platform platform.Platform,
	repos []rpmmd.RepoConfig,
	kernelName string,
	instCust InstallerCustomizations,
	isoCust ISOCustomizations,
) *AnacondaInstaller {
	name := "anaconda-tree"
	p := &AnacondaInstaller{
		Base:                    NewBase(name, buildPipeline),
		Type:                    installerType,
		platform:                platform,
		depsolveRepos:           filterRepos(repos, name),
		kernelName:              kernelName,
		InstallerCustomizations: instCust,
		ISOCustomizations:       isoCust,
	}
	buildPipeline.addDependent(p)
	return p
}

// TODO: refactor - what is required to boot and what to build, and
// do they all belong in this pipeline?
func (p *AnacondaInstaller) anacondaBootPackageSet() ([]string, error) {
	packages := []string{
		"grub2-tools",
		"grub2-tools-extra",
		"grub2-tools-minimal",
		"efibootmgr",
	}

	switch p.platform.GetArch() {
	case arch.ARCH_X86_64:
		packages = append(packages,
			"grub2-efi-x64",
			"grub2-efi-x64-cdboot",
			"grub2-pc",
			"grub2-pc-modules",
			"shim-x64",
		)

		if p.ISOCustomizations.BootType == SyslinuxISOBoot {
			packages = append(packages,
				"syslinux",
				"syslinux-nonlinux",
			)
		}
	case arch.ARCH_AARCH64:
		packages = append(packages,
			"grub2-efi-aa64-cdboot",
			"grub2-efi-aa64",
			"shim-aa64",
		)
	default:
		return nil, fmt.Errorf("unsupported arch: %s", p.platform.GetArch())
	}

	return packages, nil
}

func (p *AnacondaInstaller) getContainerSources() []container.SourceSpec {
	if p.BootcLivefsContainer == nil {
		return nil
	}
	return []container.SourceSpec{*p.BootcLivefsContainer}
}

func (p *AnacondaInstaller) getBuildPackages(Distro) ([]string, error) {
	// when using a bootc container for the livefs there is no
	// need to get packages
	if p.BootcLivefsContainer != nil {
		return nil, nil
	}

	packages, err := p.anacondaBootPackageSet()
	if err != nil {
		return nil, fmt.Errorf("cannot get anaconda boot packages: %w", err)
	}
	packages = append(packages,
		"rpm",
		"shadow-utils", // The pipeline always creates a root and installer user
	)

	if len(p.InstallerCustomizations.LoraxTemplatePackage) > 0 {
		packages = append(packages,
			p.InstallerCustomizations.LoraxTemplatePackage,
		)
	}

	if p.SELinux != "" {
		packages = append(packages, "policycoreutils", fmt.Sprintf("selinux-policy-%s", p.SELinux))
	}

	return packages, nil
}

func (p *AnacondaInstaller) SetKernelVer(kVer string) {
	p.kernelVer = kVer
}

// getPackageSetChain returns the packages to install
// It will also include weak deps for the Live installer type
func (p *AnacondaInstaller) getPackageSetChain(Distro) ([]rpmmd.PackageSet, error) {
	// when using a bootc container for the livefs there is no
	// need to get packages
	if p.BootcLivefsContainer != nil {
		return nil, nil
	}

	packages, err := p.anacondaBootPackageSet()
	if err != nil {
		return nil, fmt.Errorf("cannot get anaconda boot packages: %w", err)
	}

	// Install firmware packages and other platform specific packages
	packages = append(packages, p.platform.GetPackages()...)

	if p.Biosdevname {
		packages = append(packages, "biosdevname")
	}

	if p.SELinux != "" {
		packages = append(packages, fmt.Sprintf("selinux-policy-%s", p.SELinux))
	}

	return []rpmmd.PackageSet{
		{
			Include:         append(packages, p.ExtraPackages...),
			Exclude:         p.ExcludePackages,
			Repositories:    append(p.depsolveRepos, p.ExtraRepos...),
			InstallWeakDeps: p.InstallerCustomizations.InstallWeakDeps,
		},
	}, nil
}

func (p *AnacondaInstaller) getPackageSpecs() rpmmd.PackageList {
	if p.depsolveResult == nil {
		return nil
	}
	return p.depsolveResult.Transactions.AllPackages()
}

func (p *AnacondaInstaller) serializeStart(inputs Inputs) error {
	if p.depsolveResult != nil && len(p.bootcLivefsContainerSpecs) > 0 {
		return errors.New("AnacondaInstaller: double call to serializeStart()")
	}
	p.depsolveResult = &inputs.Depsolved
	// bootc-installers will get the kernelVer via introspection
	if len(p.depsolveResult.Transactions) > 0 && p.kernelName != "" {
		kernelPkg, err := p.depsolveResult.Transactions.FindPackage(p.kernelName)
		if err != nil {
			return fmt.Errorf("AnacondaInstaller: %w", err)
		}
		p.kernelVer = kernelPkg.EVRA()
	}
	p.bootcLivefsContainerSpecs = inputs.Containers
	return nil
}

func (p *AnacondaInstaller) serializeEnd() {
	if p.depsolveResult == nil && len(p.bootcLivefsContainerSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.kernelVer = ""
	p.depsolveResult = nil
	p.bootcLivefsContainerSpecs = nil
}

func installerRootUser() osbuild.UsersStageOptionsUser {
	return osbuild.UsersStageOptionsUser{
		Password: common.ToPtr(""),
	}
}

func (p *AnacondaInstaller) serialize() (osbuild.Pipeline, error) {
	if p.depsolveResult == nil && len(p.bootcLivefsContainerSpecs) == 0 {
		return osbuild.Pipeline{}, fmt.Errorf("AnacondaInstaller: serialization not started")
	}
	if len(p.depsolveResult.Transactions) > 0 && len(p.bootcLivefsContainerSpecs) > 0 {
		return osbuild.Pipeline{}, fmt.Errorf("AnacondaInstaller: using packages and containers at the same time is not allowed")
	}

	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	if len(p.depsolveResult.Transactions) > 0 {
		baseOptions := osbuild.RPMStageOptions{}
		// Documentation is only installed on live installer images
		if p.Type != AnacondaInstallerTypeLive {
			baseOptions.Exclude = &osbuild.Exclude{Docs: true}
		}
		rpmStages, err := osbuild.GenRPMStagesFromTransactions(p.depsolveResult.Transactions, &baseOptions)
		if err != nil {
			return osbuild.Pipeline{}, err
		}
		pipeline.AddStages(rpmStages...)
	} else {
		image := osbuild.NewContainersInputForSingleSource(p.bootcLivefsContainerSpecs[0])
		stage, err := osbuild.NewContainerDeployStage(image, &osbuild.ContainerDeployOptions{RemoveSignatures: true})
		if err != nil {
			return pipeline, err
		}
		pipeline.AddStage(stage)
	}

	pipeline.AddStage(osbuild.NewBuildstampStage(&osbuild.BuildstampStageOptions{
		Arch:    p.platform.GetArch().String(),
		Product: p.InstallerCustomizations.Product,
		Variant: p.InstallerCustomizations.Variant,
		Version: p.InstallerCustomizations.OSVersion,
		Final:   !p.InstallerCustomizations.Preview,
	}))

	locale := p.Locale
	if locale == "" {
		// default to C.UTF-8 if unset
		locale = "C.UTF-8"
	}
	pipeline.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: locale}))

	// Let's do a bunch of sanity checks that are dependent on the installer type
	// being serialized
	switch p.Type {
	case AnacondaInstallerTypeLive:
		if p.InteractiveDefaultsKickstart != nil && (len(p.InteractiveDefaultsKickstart.Users) != 0 || len(p.InteractiveDefaultsKickstart.Groups) != 0) {
			return osbuild.Pipeline{}, fmt.Errorf("anaconda installer type live does not support users and groups customization")
		}
		if p.InteractiveDefaults != nil {
			return osbuild.Pipeline{}, fmt.Errorf("anaconda installer type live does not support interactive defaults")
		}
		liveStages, err := p.liveStages()
		if err != nil {
			return osbuild.Pipeline{}, fmt.Errorf("live stages generation failed: %w", err)
		}
		pipeline.AddStages(liveStages...)
	case AnacondaInstallerTypePayload, AnacondaInstallerTypeNetinst:
		payloadStages, err := p.payloadStages()
		if err != nil {
			return osbuild.Pipeline{}, fmt.Errorf("payload stages generation failed: %w", err)
		}
		pipeline.AddStages(payloadStages...)
	default:
		return osbuild.Pipeline{}, fmt.Errorf("invalid anaconda installer type")
	}

	return pipeline, nil
}

// payloadStages creates the stages needed to boot Anaconda
// - root and install users
// - lorax postinstall templates to setup the boot environment
// - Anaconda spoke configuration
// - Generic initrd with support for the boot iso
// - SELinux in permissive mode
// - Default Anaconda kickstart (optional)
func (p *AnacondaInstaller) payloadStages() ([]*osbuild.Stage, error) {
	stages := make([]*osbuild.Stage, 0)

	installUID := 0
	installGID := 0
	// bootc systems needs to be able to override this to /var/roothome
	installHome := p.InstallerHome
	if installHome == "" {
		installHome = "/root"
	}
	installShell := "/usr/libexec/anaconda/run-anaconda"
	installPassword := ""
	installUser := osbuild.UsersStageOptionsUser{
		UID:      &installUID,
		GID:      &installGID,
		Home:     &installHome,
		Shell:    &installShell,
		Password: &installPassword,
	}

	usersStageOptions := &osbuild.UsersStageOptions{
		Users: map[string]osbuild.UsersStageOptionsUser{
			"root":    installerRootUser(),
			"install": installUser,
		},
	}
	stages = append(stages, osbuild.NewUsersStage(usersStageOptions))

	// Limit the Anaconda spokes on non-netinst iso types
	if p.Type != AnacondaInstallerTypeNetinst {
		var anacondaStageOptions *osbuild.AnacondaStageOptions
		if p.InstallerCustomizations.UseLegacyAnacondaConfig {
			anacondaStageOptions = osbuild.NewAnacondaStageOptionsLegacy(p.InstallerCustomizations.EnabledAnacondaModules, p.InstallerCustomizations.DisabledAnacondaModules)
		} else {
			anacondaStageOptions = osbuild.NewAnacondaStageOptions(p.InstallerCustomizations.EnabledAnacondaModules, p.InstallerCustomizations.DisabledAnacondaModules)
		}
		stages = append(stages, osbuild.NewAnacondaStage(anacondaStageOptions))
	}

	// Some lorax templates have to run before initramfs re-generation and some of
	// them afterwards
	deferredTemplates := []InstallerLoraxTemplate{}

	// Run lorax scripts to setup booting the iso and removing unneeded files
	for _, tmpl := range p.InstallerCustomizations.LoraxTemplates {
		// If a template is marked to run after dracut generation we put it in our
		// deferreds and continue
		if tmpl.AfterDracut {
			deferredTemplates = append(deferredTemplates, tmpl)
			continue
		}

		stages = append(stages, osbuild.NewLoraxScriptStage(&osbuild.LoraxScriptStageOptions{
			Path: tmpl.Path,
			Branding: osbuild.Branding{
				Release: p.InstallerCustomizations.LoraxReleasePackage,
				Logos:   p.InstallerCustomizations.LoraxLogosPackage,
			},
			BaseArch: p.platform.GetArch().String(),
		}))
	}

	// Create a generic initrd suitable for booting Anaconda and activating supported hardware
	dracutOptions, err := p.dracutStageOptions()
	if err != nil {
		return nil, fmt.Errorf("cannot get dracut stage options: %w", err)
	}
	stages = append(stages, osbuild.NewDracutStage(dracutOptions))

	// Run lorax scripts that were deferred to after initramfs
	for _, tmpl := range deferredTemplates {
		stages = append(stages, osbuild.NewLoraxScriptStage(&osbuild.LoraxScriptStageOptions{
			Path: tmpl.Path,
			Branding: osbuild.Branding{
				Release: p.InstallerCustomizations.LoraxReleasePackage,
				Logos:   p.InstallerCustomizations.LoraxLogosPackage,
			},
			BaseArch: p.platform.GetArch().String(),
		}))
	}

	stages = append(stages, osbuild.NewSELinuxConfigStage(&osbuild.SELinuxConfigStageOptions{State: osbuild.SELinuxStatePermissive}))

	// SELinux is not supported on the non-live-installers (see the previous
	// stage setting SELinux to permissive. It's an error to set it to anything
	// that isn't an empty string
	if p.SELinux != "" {
		return nil, fmt.Errorf("payload installers do not support SELinux policies (got policy %q)", p.SELinux)
	}

	if p.InteractiveDefaults != nil {
		var ksUsers []users.User
		var ksGroups []users.Group
		if p.InteractiveDefaultsKickstart != nil {
			ksUsers = p.InteractiveDefaultsKickstart.Users
			ksGroups = p.InteractiveDefaultsKickstart.Groups
		}
		kickstartOptions, err := osbuild.NewKickstartStageOptionsWithLiveIMG(
			osbuild.KickstartPathInteractiveDefaults,
			ksUsers,
			ksGroups,
			p.InteractiveDefaults.TarPath,
		)

		if err != nil {
			return nil, fmt.Errorf("failed to create kickstart stage options for interactive defaults: %w", err)
		}

		stages = append(stages, osbuild.NewKickstartStage(kickstartOptions))
	}

	return stages, nil
}

// liveStages creates the stages needed to boot a live image with Anaconda installed
// - root user
// - livesys service to setup the live environment
// - Configure GNOME livesys session
// - Generic initrd with support for the boot iso
// - SELinux in permissive mode
// - Default Anaconda kickstart (optional)
func (p *AnacondaInstaller) liveStages() ([]*osbuild.Stage, error) {
	stages := make([]*osbuild.Stage, 0)

	usersStageOptions := &osbuild.UsersStageOptions{
		Users: map[string]osbuild.UsersStageOptionsUser{
			"root": installerRootUser(),
		},
	}
	stages = append(stages, osbuild.NewUsersStage(usersStageOptions))

	systemdStageOptions := &osbuild.SystemdStageOptions{
		EnabledServices: []string{
			"livesys.service",
			"livesys-late.service",
		},
	}

	stages = append(stages, osbuild.NewSystemdStage(systemdStageOptions))

	livesysMode := os.FileMode(int(0644))
	livesysFile, err := fsnode.NewFile("/etc/sysconfig/livesys", &livesysMode, "root", "root", []byte("livesys_session=\"gnome\""))

	if err != nil {
		return nil, err
	}

	p.Files = []*fsnode.File{livesysFile}

	stages = append(stages, osbuild.GenFileNodesStages(p.Files)...)

	// Create a generic initrd suitable for booting the live iso and activating supported hardware
	dracutOptions, err := p.dracutStageOptions()
	if err != nil {
		return nil, fmt.Errorf("cannot get dracut stage options: %w", err)
	}
	stages = append(stages, osbuild.NewDracutStage(dracutOptions))

	if p.SELinux != "" {
		stages = append(stages, osbuild.NewSELinuxStage(&osbuild.SELinuxStageOptions{
			FileContexts: fmt.Sprintf("etc/selinux/%s/contexts/files/file_contexts", p.SELinux),
		}))
	}

	return stages, nil
}

// dracutStageOptions returns the basic dracut setup with anaconda support
// This is based on the dracut generic config (also called no-hostonly) with
// additional modules needed to support booting the iso and running Anaconda.
//
// NOTE: The goal is to let dracut maintain support for most of the modules and
// only add what is needed to support the boot iso and anaconda. When new
// hardware support is needed in the inird it just needs to be added to dracut,
// not everything that uses dracut (eg. anaconda, lorax, osbuild).
func (p *AnacondaInstaller) dracutStageOptions() (*osbuild.DracutStageOptions, error) {
	// Common settings
	options := osbuild.DracutStageOptions{
		Kernel:         []string{p.kernelVer},
		EarlyMicrocode: false,
		AddModules:     []string{"pollcdrom", "qemu", "qemu-net"},
		Extra:          []string{"--xz"},
		AddDrivers:     p.InstallerCustomizations.AdditionalDrivers,
	}
	if p.InitramfsPath != "" {
		// dracut will by default write to /boot/initrmfs-$ver
		// so we need to override if we have explicit paths
		options.Extra = append(options.Extra, p.InitramfsPath)
	}

	options.AddModules = append(options.AddModules, p.InstallerCustomizations.AdditionalDracutModules...)

	if p.Biosdevname {
		options.AddModules = append(options.AddModules, "biosdevname")
	}

	switch p.Type {
	case AnacondaInstallerTypePayload, AnacondaInstallerTypeNetinst:
		// Lorax calls the boot.iso dracut with:
		// --nomdadmconf --nolvmconf --xz --install '/.buildstamp' --no-early-microcode
		// --add 'fips anaconda pollcdrom qemu qemu-net prefixdevname-tools'
		options.Install = []string{"./buildstamp"}
		options.AddModules = append(options.AddModules, []string{
			"fips",
			"anaconda",
			"prefixdevname-tools",
		}...)
		options.Extra = append(options.Extra, []string{
			"--nomdadmconf",
			"--nolvmconf",
		}...)
	case AnacondaInstallerTypeLive:
		// livemedia-creator calls the live iso dracut with:
		// --xz --no-hostonly -no-early-microcode --debug
		// --add 'livenet dmsquash-live dmsquash-live-ntfs convertfs pollcdrom qemu qemu-net'
		options.AddModules = append(options.AddModules, []string{
			"livenet",
			"dmsquash-live",
			"convertfs",
		}...)
	default:
		return nil, fmt.Errorf("unknown AnacondaInstallerType %v in dracutStageOptions", p.Type)
	}

	return &options, nil
}

func (p *AnacondaInstaller) Platform() platform.Platform {
	return p.platform
}

type AnacondaInteractiveDefaults struct {
	TarPath string
}

func NewAnacondaInteractiveDefaults(tarPath string) *AnacondaInteractiveDefaults {
	i := &AnacondaInteractiveDefaults{
		TarPath: tarPath,
	}

	return i
}

func (p *AnacondaInstaller) getInline() []string {
	inlineData := []string{}

	// inline data for custom files
	for _, file := range p.Files {
		inlineData = append(inlineData, string(file.Data()))
	}

	return inlineData
}
