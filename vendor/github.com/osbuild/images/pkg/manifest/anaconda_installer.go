package manifest

import (
	"fmt"
	"os"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

type AnacondaInstallerType int

const (
	AnacondaInstallerTypeLive AnacondaInstallerType = iota + 1
	AnacondaInstallerTypePayload
)

// An Anaconda represents the installer tree as found on an ISO this can be either
// a payload installer or a live installer depending on `Type`.
type AnacondaInstaller struct {
	Base

	// The type of the Anaconda installer tree to prepare, this can be either
	// a 'live' or a 'payload' and it controls which stages are added to the
	// manifest.
	Type AnacondaInstallerType

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

	// Variant is the variant of the product being installed, if applicable.
	Variant string

	platform     platform.Platform
	repos        []rpmmd.RepoConfig
	packageSpecs []rpmmd.PackageSpec
	kernelName   string
	kernelVer    string
	product      string
	version      string
	preview      bool

	// Interactive defaults is a kickstart stage that can be provided, it
	// will be written to /usr/share/anaconda/interactive-defaults
	InteractiveDefaults *AnacondaInteractiveDefaults

	// Kickstart options that will be written to the interactive defaults
	// kickstart file. Currently only supports Users and Groups. Other
	// properties are ignored.
	InteractiveDefaultsKickstart *kickstart.Options

	// Additional anaconda modules to enable
	AdditionalAnacondaModules []string
	// Anaconda modules to explicitly disable
	DisabledAnacondaModules []string

	// Additional dracut modules and drivers to enable
	AdditionalDracutModules []string
	AdditionalDrivers       []string

	Files []*fsnode.File

	// Temporary
	UseRHELLoraxTemplates bool

	// Uses the old, deprecated, Anaconda config option "kickstart-modules".
	// Only for RHEL 8.
	UseLegacyAnacondaConfig bool

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
	kernelName,
	product,
	version string,
	preview bool) *AnacondaInstaller {
	name := "anaconda-tree"
	p := &AnacondaInstaller{
		Base:       NewBase(name, buildPipeline),
		Type:       installerType,
		platform:   platform,
		repos:      filterRepos(repos, name),
		kernelName: kernelName,
		product:    product,
		version:    version,
		preview:    preview,
	}
	buildPipeline.addDependent(p)
	return p
}

// TODO: refactor - what is required to boot and what to build, and
// do they all belong in this pipeline?
func (p *AnacondaInstaller) anacondaBootPackageSet() []string {
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
			"syslinux",
			"syslinux-nonlinux",
		)
	case arch.ARCH_AARCH64:
		packages = append(packages,
			"grub2-efi-aa64-cdboot",
			"grub2-efi-aa64",
			"shim-aa64",
		)
	default:
		panic(fmt.Sprintf("unsupported arch: %s", p.platform.GetArch()))
	}

	return packages
}

func (p *AnacondaInstaller) getBuildPackages(Distro) []string {
	packages := p.anacondaBootPackageSet()
	packages = append(packages,
		"rpm",
		"shadow-utils", // The pipeline always creates a root and installer user
	)

	if p.UseRHELLoraxTemplates {
		packages = append(packages,
			"lorax-templates-rhel",
		)
	} else {
		packages = append(packages,
			"lorax-templates-generic",
		)
	}

	if p.SELinux != "" {
		packages = append(packages, "policycoreutils", fmt.Sprintf("selinux-policy-%s", p.SELinux))
	}

	return packages
}

// getPackageSetChain returns the packages to install
// It will also include weak deps for the Live installer type
func (p *AnacondaInstaller) getPackageSetChain(Distro) []rpmmd.PackageSet {
	packages := p.anacondaBootPackageSet()

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
			Repositories:    append(p.repos, p.ExtraRepos...),
			InstallWeakDeps: true,
		},
	}
}

func (p *AnacondaInstaller) getPackageSpecs() []rpmmd.PackageSpec {
	return p.packageSpecs
}

func (p *AnacondaInstaller) serializeStart(inputs Inputs) {
	if len(p.packageSpecs) > 0 {
		panic("double call to serializeStart()")
	}
	p.packageSpecs = inputs.Depsolved.Packages
	if p.kernelName != "" {
		p.kernelVer = rpmmd.GetVerStrFromPackageSpecListPanic(p.packageSpecs, p.kernelName)
	}
	p.repos = append(p.repos, inputs.Depsolved.Repos...)
}

func (p *AnacondaInstaller) serializeEnd() {
	if len(p.packageSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.kernelVer = ""
	p.packageSpecs = nil
}

func installerRootUser() osbuild.UsersStageOptionsUser {
	return osbuild.UsersStageOptionsUser{
		Password: common.ToPtr(""),
	}
}

func (p *AnacondaInstaller) serialize() osbuild.Pipeline {
	if len(p.packageSpecs) == 0 {
		panic("serialization not started")
	}

	pipeline := p.Base.serialize()
	options := osbuild.NewRPMStageOptions(p.repos)
	// Documentation is only installed on live installer images
	if p.Type != AnacondaInstallerTypeLive {
		options.Exclude = &osbuild.Exclude{Docs: true}
	}

	pipeline.AddStage(osbuild.NewRPMStage(options, osbuild.NewRpmStageSourceFilesInputs(p.packageSpecs)))
	pipeline.AddStage(osbuild.NewBuildstampStage(&osbuild.BuildstampStageOptions{
		Arch:    p.platform.GetArch().String(),
		Product: p.product,
		Variant: p.Variant,
		Version: p.version,
		Final:   !p.preview,
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
			panic("anaconda installer type live does not support users and groups customization")
		}
		if p.InteractiveDefaults != nil {
			panic("anaconda installer type live does not support interactive defaults")
		}
		pipeline.AddStages(p.liveStages()...)
	case AnacondaInstallerTypePayload:
		pipeline.AddStages(p.payloadStages()...)
	default:
		panic("invalid anaconda installer type")
	}

	return pipeline
}

// payloadStages creates the stages needed to boot Anaconda
// - root and install users
// - lorax postinstall templates to setup the boot environment
// - Anaconda spoke configuration
// - Generic initrd with support for the boot iso
// - SELinux in permissive mode
// - Default Anaconda kickstart (optional)
func (p *AnacondaInstaller) payloadStages() []*osbuild.Stage {
	stages := make([]*osbuild.Stage, 0)

	installUID := 0
	installGID := 0
	installHome := "/root"
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

	var anacondaStageOptions *osbuild.AnacondaStageOptions
	if p.UseLegacyAnacondaConfig {
		anacondaStageOptions = osbuild.NewAnacondaStageOptionsLegacy(p.AdditionalAnacondaModules, p.DisabledAnacondaModules)
	} else {
		anacondaStageOptions = osbuild.NewAnacondaStageOptions(p.AdditionalAnacondaModules, p.DisabledAnacondaModules)
	}
	stages = append(stages, osbuild.NewAnacondaStage(anacondaStageOptions))

	LoraxPath := "99-generic/runtime-postinstall.tmpl"
	if p.UseRHELLoraxTemplates {
		LoraxPath = "80-rhel/runtime-postinstall.tmpl"
	}
	stages = append(stages, osbuild.NewLoraxScriptStage(&osbuild.LoraxScriptStageOptions{
		Path:     LoraxPath,
		BaseArch: p.platform.GetArch().String(),
	}))

	// Create a generic initrd suitable for booting Anaconda and activating supported hardware
	dracutOptions := p.dracutStageOptions()
	stages = append(stages, osbuild.NewDracutStage(dracutOptions))

	stages = append(stages, osbuild.NewSELinuxConfigStage(&osbuild.SELinuxConfigStageOptions{State: osbuild.SELinuxStatePermissive}))

	// SELinux is not supported on the non-live-installers (see the previous
	// stage setting SELinux to permissive. It's an error to set it to anything
	// that isn't an empty string
	if p.SELinux != "" {
		panic("payload installers do not support SELinux policies")
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
			panic(fmt.Sprintf("failed to create kickstart stage options for interactive defaults: %v", err))
		}

		stages = append(stages, osbuild.NewKickstartStage(kickstartOptions))
	}

	return stages
}

// liveStages creates the stages needed to boot a live image with Anaconda installed
// - root user
// - livesys service to setup the live environment
// - Configure GNOME livesys session
// - Generic initrd with support for the boot iso
// - SELinux in permissive mode
// - Default Anaconda kickstart (optional)
func (p *AnacondaInstaller) liveStages() []*osbuild.Stage {
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
		panic(err)
	}

	p.Files = []*fsnode.File{livesysFile}

	stages = append(stages, osbuild.GenFileNodesStages(p.Files)...)

	// Create a generic initrd suitable for booting the live iso and activating supported hardware
	dracutOptions := p.dracutStageOptions()
	stages = append(stages, osbuild.NewDracutStage(dracutOptions))

	if p.SELinux != "" {
		stages = append(stages, osbuild.NewSELinuxStage(&osbuild.SELinuxStageOptions{
			FileContexts: fmt.Sprintf("etc/selinux/%s/contexts/files/file_contexts", p.SELinux),
		}))
	}

	return stages
}

// dracutStageOptions returns the basic dracut setup with anaconda support
// This is based on the dracut generic config (also called no-hostonly) with
// additional modules needed to support booting the iso and running Anaconda.
//
// NOTE: The goal is to let dracut maintain support for most of the modules and
// only add what is needed to support the boot iso and anaconda. When new
// hardware support is needed in the inird it just needs to be added to dracut,
// not everything that uses dracut (eg. anaconda, lorax, osbuild).
func (p *AnacondaInstaller) dracutStageOptions() *osbuild.DracutStageOptions {
	// Common settings
	options := osbuild.DracutStageOptions{
		Kernel:         []string{p.kernelVer},
		EarlyMicrocode: false,
		AddModules:     []string{"pollcdrom", "qemu", "qemu-net"},
		Extra:          []string{"--xz"},
		AddDrivers:     p.AdditionalDrivers,
	}
	options.AddModules = append(options.AddModules, p.AdditionalDracutModules...)

	if p.Biosdevname {
		options.AddModules = append(options.AddModules, "biosdevname")
	}

	switch p.Type {
	case AnacondaInstallerTypePayload:
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
	}

	return &options
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
