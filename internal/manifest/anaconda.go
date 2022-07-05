package manifest

import (
	"fmt"

	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

// An AnacondaPipeline represents the installer tree as found on an ISO.
type AnacondaPipeline struct {
	BasePipeline
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

	repos        []rpmmd.RepoConfig
	packageSpecs []rpmmd.PackageSpec
	kernelName   string
	kernelVer    string
	arch         string
	product      string
	version      string
}

// NewAnacondaPipeline creates an anaconda pipeline object. repos and packages
// indicate the content to build the installer from, which is distinct from the
// packages the installer will install on the target system. kernelName is the
// name of the kernel package the intsaller will use. arch is the supported
// architecture. Product and version refers to the product the installer is the
// installer for.
func NewAnacondaPipeline(m *Manifest,
	buildPipeline *BuildPipeline,
	repos []rpmmd.RepoConfig,
	kernelName,
	arch,
	product,
	version string) *AnacondaPipeline {
	p := &AnacondaPipeline{
		BasePipeline: NewBasePipeline(m, "anaconda-tree", buildPipeline, nil),
		repos:        repos,
		kernelName:   kernelName,
		arch:         arch,
		product:      product,
		version:      version,
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

// TODO: refactor - what is required to boot and what to build, and
// do they all belong in this pipeline?
func (p *AnacondaPipeline) anacondaBootPackageSet() []string {
	packages := []string{
		"grub2-tools",
		"grub2-tools-extra",
		"grub2-tools-minimal",
		"efibootmgr",
	}

	// TODO: use constants for architectures
	switch p.arch {
	case "x86_64":
		packages = append(packages,
			"grub2-efi-x64",
			"grub2-efi-x64-cdboot",
			"grub2-pc",
			"grub2-pc-modules",
			"shim-x64",
			"syslinux",
			"syslinux-nonlinux",
		)
	case "aarch64":
		packages = append(packages,
			"grub2-efi-aa64-cdboot",
			"grub2-efi-aa64",
			"shim-aa64",
		)
	default:
		panic(fmt.Sprintf("unsupported arch: %s", p.arch))
	}

	return packages
}

func (p *AnacondaPipeline) getBuildPackages() []string {
	packages := p.anacondaBootPackageSet()
	packages = append(packages,
		"lorax-templates-generic",
	)
	return packages
}

func (p *AnacondaPipeline) getPackageSetChain() []rpmmd.PackageSet {
	packages := p.anacondaBootPackageSet()
	return []rpmmd.PackageSet{
		{
			Include:      append(packages, p.ExtraPackages...),
			Repositories: append(p.repos, p.ExtraRepos...),
		},
	}
}

func (p *AnacondaPipeline) getPackageSpecs() []rpmmd.PackageSpec {
	return p.packageSpecs
}

func (p *AnacondaPipeline) serializeStart(packages []rpmmd.PackageSpec) {
	if len(p.packageSpecs) > 0 {
		panic("double call to serializeStart()")
	}
	p.packageSpecs = packages
	if p.kernelName != "" {
		p.kernelVer = rpmmd.GetVerStrFromPackageSpecListPanic(p.packageSpecs, p.kernelName)
	}
}

func (p *AnacondaPipeline) serializeEnd() {
	if len(p.packageSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.kernelVer = ""
	p.packageSpecs = nil
}

func (p *AnacondaPipeline) serialize() osbuild2.Pipeline {
	if len(p.packageSpecs) == 0 {
		panic("serialization not started")
	}
	pipeline := p.BasePipeline.serialize()

	pipeline.AddStage(osbuild2.NewRPMStage(osbuild2.NewRPMStageOptions(p.repos), osbuild2.NewRpmStageSourceFilesInputs(p.packageSpecs)))
	pipeline.AddStage(osbuild2.NewBuildstampStage(&osbuild2.BuildstampStageOptions{
		Arch:    p.arch,
		Product: p.product,
		Variant: p.Variant,
		Version: p.version,
		Final:   true,
	}))
	pipeline.AddStage(osbuild2.NewLocaleStage(&osbuild2.LocaleStageOptions{Language: "en_US.UTF-8"}))

	rootPassword := ""
	rootUser := osbuild2.UsersStageOptionsUser{
		Password: &rootPassword,
	}

	installUID := 0
	installGID := 0
	installHome := "/root"
	installShell := "/usr/libexec/anaconda/run-anaconda"
	installPassword := ""
	installUser := osbuild2.UsersStageOptionsUser{
		UID:      &installUID,
		GID:      &installGID,
		Home:     &installHome,
		Shell:    &installShell,
		Password: &installPassword,
	}
	usersStageOptions := &osbuild2.UsersStageOptions{
		Users: map[string]osbuild2.UsersStageOptionsUser{
			"root":    rootUser,
			"install": installUser,
		},
	}

	pipeline.AddStage(osbuild2.NewUsersStage(usersStageOptions))
	pipeline.AddStage(osbuild2.NewAnacondaStage(osbuild2.NewAnacondaStageOptions(p.Users)))
	pipeline.AddStage(osbuild2.NewLoraxScriptStage(&osbuild2.LoraxScriptStageOptions{
		Path:     "99-generic/runtime-postinstall.tmpl",
		BaseArch: p.arch,
	}))
	pipeline.AddStage(osbuild2.NewDracutStage(dracutStageOptions(p.kernelVer, p.Biosdevname, []string{
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
	pipeline.AddStage(osbuild2.NewSELinuxConfigStage(&osbuild2.SELinuxConfigStageOptions{State: osbuild2.SELinuxStatePermissive}))

	return pipeline
}

func dracutStageOptions(kernelVer string, biosdevname bool, additionalModules []string) *osbuild2.DracutStageOptions {
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
	return &osbuild2.DracutStageOptions{
		Kernel:  kernel,
		Modules: modules,
		Install: []string{"/.buildstamp"},
	}
}
