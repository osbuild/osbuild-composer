package manifest

import (
	"fmt"

	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

// An Anaconda represents the installer tree as found on an ISO.
type Anaconda struct {
	Base
	// Packages to install in addition to the ones required by the
	// pipeline.
	ExtraPackages []string
	// Extra repositories to install packages from
	ExtraRepos []rpmmd.RepoConfig
	// Users indicate whether or not the user spoke should be enabled in
	// anaconda. If it is, users specified in a kickstart will be configured,
	// and in case no users are provided in a kickstart the user will be
	// prompted to configure them at install time. If this is set to false
	// any kickstart provided users are ignored and the user is never
	// prompted to configure users during installation.
	Users bool
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
}

// NewAnaconda creates an anaconda pipeline object. repos and packages
// indicate the content to build the installer from, which is distinct from the
// packages the installer will install on the target system. kernelName is the
// name of the kernel package the intsaller will use. arch is the supported
// architecture. Product and version refers to the product the installer is the
// installer for.
func NewAnaconda(m *Manifest,
	buildPipeline *Build,
	platform platform.Platform,
	repos []rpmmd.RepoConfig,
	kernelName,
	product,
	version string) *Anaconda {
	p := &Anaconda{
		Base:       NewBase(m, "anaconda-tree", buildPipeline),
		platform:   platform,
		repos:      repos,
		kernelName: kernelName,
		product:    product,
		version:    version,
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

// TODO: refactor - what is required to boot and what to build, and
// do they all belong in this pipeline?
func (p *Anaconda) anacondaBootPackageSet() []string {
	packages := []string{
		"grub2-tools",
		"grub2-tools-extra",
		"grub2-tools-minimal",
		"efibootmgr",
	}

	switch p.platform.GetArch() {
	case platform.ARCH_X86_64:
		packages = append(packages,
			"grub2-efi-x64",
			"grub2-efi-x64-cdboot",
			"grub2-pc",
			"grub2-pc-modules",
			"shim-x64",
			"syslinux",
			"syslinux-nonlinux",
		)
	case platform.ARCH_AARCH64:
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

func (p *Anaconda) getBuildPackages() []string {
	packages := p.anacondaBootPackageSet()
	packages = append(packages,
		"rpm",
		"lorax-templates-generic",
	)
	return packages
}

func (p *Anaconda) getPackageSetChain() []rpmmd.PackageSet {
	packages := p.anacondaBootPackageSet()
	if p.Biosdevname {
		packages = append(packages, "biosdevname")
	}
	return []rpmmd.PackageSet{
		{
			Include:      append(packages, p.ExtraPackages...),
			Repositories: append(p.repos, p.ExtraRepos...),
		},
	}
}

func (p *Anaconda) getPackageSpecs() []rpmmd.PackageSpec {
	return p.packageSpecs
}

func (p *Anaconda) serializeStart(packages []rpmmd.PackageSpec) {
	if len(p.packageSpecs) > 0 {
		panic("double call to serializeStart()")
	}
	p.packageSpecs = packages
	if p.kernelName != "" {
		p.kernelVer = rpmmd.GetVerStrFromPackageSpecListPanic(p.packageSpecs, p.kernelName)
	}
}

func (p *Anaconda) serializeEnd() {
	if len(p.packageSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.kernelVer = ""
	p.packageSpecs = nil
}

func (p *Anaconda) serialize() osbuild.Pipeline {
	if len(p.packageSpecs) == 0 {
		panic("serialization not started")
	}
	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(p.repos), osbuild.NewRpmStageSourceFilesInputs(p.packageSpecs)))
	pipeline.AddStage(osbuild.NewBuildstampStage(&osbuild.BuildstampStageOptions{
		Arch:    p.platform.GetArch().String(),
		Product: p.product,
		Variant: p.Variant,
		Version: p.version,
		Final:   true,
	}))
	pipeline.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US.UTF-8"}))

	rootPassword := ""
	rootUser := osbuild.UsersStageOptionsUser{
		Password: &rootPassword,
	}

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
			"root":    rootUser,
			"install": installUser,
		},
	}

	pipeline.AddStage(osbuild.NewUsersStage(usersStageOptions))
	pipeline.AddStage(osbuild.NewAnacondaStage(osbuild.NewAnacondaStageOptions(p.Users)))
	pipeline.AddStage(osbuild.NewLoraxScriptStage(&osbuild.LoraxScriptStageOptions{
		Path:     "99-generic/runtime-postinstall.tmpl",
		BaseArch: p.platform.GetArch().String(),
	}))
	pipeline.AddStage(osbuild.NewDracutStage(dracutStageOptions(p.kernelVer, p.Biosdevname, []string{
		"anaconda",
		"rdma",
		"rngd",
		"multipath",
		"fcoe",
		"fcoe-uefi",
		"iscsi",
		"lunmask",
		"nfs",
	})))
	pipeline.AddStage(osbuild.NewSELinuxConfigStage(&osbuild.SELinuxConfigStageOptions{State: osbuild.SELinuxStatePermissive}))

	return pipeline
}

func dracutStageOptions(kernelVer string, biosdevname bool, additionalModules []string) *osbuild.DracutStageOptions {
	kernel := []string{kernelVer}
	modules := []string{
		"bash",
		"systemd",
		"fips",
		"systemd-initrd",
		"modsign",
		"nss-softokn",
		"i18n",
		"convertfs",
		"network-manager",
		"network",
		"ifcfg",
		"url-lib",
		"drm",
		"plymouth",
		"crypt",
		"dm",
		"dmsquash-live",
		"kernel-modules",
		"kernel-modules-extra",
		"kernel-network-modules",
		"livenet",
		"lvm",
		"mdraid",
		"qemu",
		"qemu-net",
		"resume",
		"rootfs-block",
		"terminfo",
		"udev-rules",
		"dracut-systemd",
		"pollcdrom",
		"usrmount",
		"base",
		"fs-lib",
		"img-lib",
		"shutdown",
		"uefi-lib",
	}

	if biosdevname {
		modules = append(modules, "biosdevname")
	}

	modules = append(modules, additionalModules...)
	return &osbuild.DracutStageOptions{
		Kernel:  kernel,
		Modules: modules,
		Install: []string{"/.buildstamp"},
	}
}

func (p *Anaconda) GetPlatform() platform.Platform {
	return p.platform
}
