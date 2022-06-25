package pipeline

import (
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type AnacondaPipeline struct {
	Pipeline
	Repos        []rpmmd.RepoConfig
	PackageSpecs []rpmmd.PackageSpec
	Users        bool
	KernelVer    string
	Arch         string
	Product      string
	OSVersion    string
	Variant      string
	Biosdevname  bool
}

func NewAnacondaPipeline(buildPipeline *BuildPipeline) AnacondaPipeline {
	return AnacondaPipeline{
		Pipeline: New("anaconda-tree", &buildPipeline.Pipeline),
	}
}

func (p AnacondaPipeline) Serialize() osbuild2.Pipeline {
	pipeline := p.Pipeline.Serialize()

	pipeline.AddStage(osbuild2.NewRPMStage(osbuild2.NewRPMStageOptions(p.Repos), osbuild2.NewRpmStageSourceFilesInputs(p.PackageSpecs)))
	pipeline.AddStage(osbuild2.NewBuildstampStage(&osbuild2.BuildstampStageOptions{
		Arch:    p.Arch,
		Product: p.Product,
		Version: p.OSVersion,
		Variant: p.Variant,
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
		BaseArch: p.Arch,
	}))
	pipeline.AddStage(osbuild2.NewDracutStage(dracutStageOptions(p.KernelVer, p.Biosdevname, []string{
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
