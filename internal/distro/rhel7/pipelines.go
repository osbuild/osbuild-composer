package rhel7

import (
	"fmt"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func buildPipeline(repos []rpmmd.RepoConfig, buildPackageSpecs []rpmmd.PackageSpec, runner string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "build"
	p.Runner = runner
	p.AddStage(osbuild.NewRPMStage(rpmStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(buildPackageSpecs)))
	p.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(true)))
	return p
}

func prependKernelCmdlineStage(pipeline *osbuild.Pipeline, t *imageType, pt *disk.PartitionTable) *osbuild.Pipeline {
	if t.arch.name == distro.S390xArchName {
		rootFs := pt.FindMountable("/")
		if rootFs == nil {
			panic("s390x image must have a root filesystem, this is a programming error")
		}
		kernelStage := osbuild.NewKernelCmdlineStage(osbuild.NewKernelCmdlineStageOptions(rootFs.GetFSSpec().UUID, t.kernelOptions))
		pipeline.Stages = append([]*osbuild.Stage{kernelStage}, pipeline.Stages...)
	}
	return pipeline
}

func osPipeline(t *imageType,
	repos []rpmmd.RepoConfig,
	packages []rpmmd.PackageSpec,
	c *blueprint.Customizations,
	options distro.ImageOptions,
	pt *disk.PartitionTable) (*osbuild.Pipeline, error) {

	imageConfig := t.getDefaultImageConfig()
	p := new(osbuild.Pipeline)

	p.Name = "os"
	p.Build = "name:build"

	rpmOptions := osbuild.NewRPMStageOptions(repos)
	rpmOptions.GPGKeysFromTree = imageConfig.GPGKeyFiles
	p.AddStage(osbuild.NewRPMStage(rpmOptions, osbuild.NewRpmStageSourceFilesInputs(packages)))

	// Difference to RHEL8, 9 pipelines: no BLS stage

	language, keyboard := c.GetPrimaryLocale()
	if language != nil {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: *language}))
	} else {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: imageConfig.Locale}))
	}
	if keyboard != nil {
		p.AddStage(osbuild.NewKeymapStage(&osbuild.KeymapStageOptions{Keymap: *keyboard}))
	} else if imageConfig.Keyboard != nil {
		p.AddStage(osbuild.NewKeymapStage(imageConfig.Keyboard))
	}

	if hostname := c.GetHostname(); hostname != nil {
		p.AddStage(osbuild.NewHostnameStage(&osbuild.HostnameStageOptions{Hostname: *hostname}))
	}

	timezone, ntpServers := c.GetTimezoneSettings()
	if timezone != nil {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: *timezone}))
	} else {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: imageConfig.Timezone}))
	}

	if len(ntpServers) > 0 {
		p.AddStage(osbuild.NewChronyStage(&osbuild.ChronyStageOptions{Timeservers: ntpServers}))
	} else if imageConfig.TimeSynchronization != nil {
		p.AddStage(osbuild.NewChronyStage(imageConfig.TimeSynchronization))
	}

	if groups := c.GetGroups(); len(groups) > 0 {
		p.AddStage(osbuild.NewGroupsStage(osbuild.NewGroupsStageOptions(groups)))
	}

	if userOptions, err := osbuild.NewUsersStageOptions(c.GetUsers(), false); err != nil {
		return nil, err
	} else if userOptions != nil {
		p.AddStage(osbuild.NewUsersStage(userOptions))
	}

	if services := c.GetServices(); services != nil || imageConfig.EnabledServices != nil ||
		imageConfig.DisabledServices != nil || imageConfig.DefaultTarget != "" {
		p.AddStage(osbuild.NewSystemdStage(systemdStageOptions(
			imageConfig.EnabledServices,
			imageConfig.DisabledServices,
			services,
			imageConfig.DefaultTarget,
		)))
	}

	var fwStageOptions *osbuild.FirewallStageOptions
	if firewallCustomization := c.GetFirewall(); firewallCustomization != nil {
		fwStageOptions = firewallStageOptions(firewallCustomization)
	}
	if firewallConfig := imageConfig.Firewall; firewallConfig != nil {
		// merge the user-provided firewall config with the default one
		if fwStageOptions != nil {
			fwStageOptions = &osbuild.FirewallStageOptions{
				// Prefer the firewall ports and services settings provided
				// via BP customization.
				Ports:            fwStageOptions.Ports,
				EnabledServices:  fwStageOptions.EnabledServices,
				DisabledServices: fwStageOptions.DisabledServices,
				// Default zone can not be set using BP customizations, therefore
				// default to the one provided in the default image configuration.
				DefaultZone: firewallConfig.DefaultZone,
			}
		} else {
			fwStageOptions = firewallConfig
		}
	}
	if fwStageOptions != nil {
		p.AddStage(osbuild.NewFirewallStage(fwStageOptions))
	}

	for _, sysconfigConfig := range imageConfig.Sysconfig {
		p.AddStage(osbuild.NewSysconfigStage(sysconfigConfig))
	}

	if t.arch.distro.isRHEL() {
		if options.Subscription != nil {
			commands := []string{
				fmt.Sprintf("/usr/sbin/subscription-manager register --org=%s --activationkey=%s --serverurl %s --baseurl %s", options.Subscription.Organization, options.Subscription.ActivationKey, options.Subscription.ServerUrl, options.Subscription.BaseUrl),
			}
			if options.Subscription.Insights {
				commands = append(commands, "/usr/bin/insights-client --register")
			}
			p.AddStage(osbuild.NewFirstBootStage(&osbuild.FirstBootStageOptions{
				Commands:       commands,
				WaitForNetwork: true,
			}))

			if rhsmConfig, exists := imageConfig.RHSMConfig[distro.RHSMConfigWithSubscription]; exists {
				p.AddStage(osbuild.NewRHSMStage(rhsmConfig))
			}
		} else {
			if rhsmConfig, exists := imageConfig.RHSMConfig[distro.RHSMConfigNoSubscription]; exists {
				p.AddStage(osbuild.NewRHSMStage(rhsmConfig))
			}
		}
	}

	for _, systemdLogindConfig := range imageConfig.SystemdLogind {
		p.AddStage(osbuild.NewSystemdLogindStage(systemdLogindConfig))
	}

	for _, cloudInitConfig := range imageConfig.CloudInit {
		p.AddStage(osbuild.NewCloudInitStage(cloudInitConfig))
	}

	for _, modprobeConfig := range imageConfig.Modprobe {
		p.AddStage(osbuild.NewModprobeStage(modprobeConfig))
	}

	for _, dracutConfConfig := range imageConfig.DracutConf {
		p.AddStage(osbuild.NewDracutConfStage(dracutConfConfig))
	}

	for _, systemdUnitConfig := range imageConfig.SystemdUnit {
		p.AddStage(osbuild.NewSystemdUnitStage(systemdUnitConfig))
	}

	if authselectConfig := imageConfig.Authselect; authselectConfig != nil {
		p.AddStage(osbuild.NewAuthselectStage(authselectConfig))
	}

	if seLinuxConfig := imageConfig.SELinuxConfig; seLinuxConfig != nil {
		p.AddStage(osbuild.NewSELinuxConfigStage(seLinuxConfig))
	}

	if tunedConfig := imageConfig.Tuned; tunedConfig != nil {
		p.AddStage(osbuild.NewTunedStage(tunedConfig))
	}

	for _, tmpfilesdConfig := range imageConfig.Tmpfilesd {
		p.AddStage(osbuild.NewTmpfilesdStage(tmpfilesdConfig))
	}

	for _, pamLimitsConfConfig := range imageConfig.PamLimitsConf {
		p.AddStage(osbuild.NewPamLimitsConfStage(pamLimitsConfConfig))
	}

	for _, sysctldConfig := range imageConfig.Sysctld {
		p.AddStage(osbuild.NewSysctldStage(sysctldConfig))
	}

	for _, dnfConfig := range imageConfig.DNFConfig {
		p.AddStage(osbuild.NewDNFConfigStage(dnfConfig))
	}

	if sshdConfig := imageConfig.SshdConfig; sshdConfig != nil {
		p.AddStage((osbuild.NewSshdConfigStage(sshdConfig)))
	}

	if authConfig := imageConfig.Authconfig; authConfig != nil {
		p.AddStage(osbuild.NewAuthconfigStage(authConfig))
	}

	if pwQuality := imageConfig.PwQuality; pwQuality != nil {
		p.AddStage(osbuild.NewPwqualityConfStage(pwQuality))
	}

	if waConfig := imageConfig.WAAgentConfig; waConfig != nil {
		p.AddStage(osbuild.NewWAAgentConfStage(waConfig))
	}

	if yumConfig := imageConfig.YumConfig; yumConfig != nil {
		p.AddStage(osbuild.NewYumConfigStage(yumConfig))
	}

	for _, yumRepo := range imageConfig.YUMRepos {
		p.AddStage(osbuild.NewYumReposStage(yumRepo))
	}

	if udevRules := imageConfig.UdevRules; udevRules != nil {
		p.AddStage(osbuild.NewUdevRulesStage(udevRules))
	}

	if pt != nil {

		kernelOptions := osbuild.GenImageKernelOptions(pt)
		if t.kernelOptions != "" {
			kernelOptions = append(kernelOptions, t.kernelOptions)
		}
		if bpKernel := c.GetKernel(); bpKernel.Append != "" {
			kernelOptions = append(kernelOptions, bpKernel.Append)
		}

		p.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(pt)))
		kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packages, c.GetKernel().Name)

		var bootloader *osbuild.Stage
		if t.arch.name == distro.S390xArchName {
			p = prependKernelCmdlineStage(p, t, pt)
			bootloader = osbuild.NewZiplStage(new(osbuild.ZiplStageOptions))
		} else {
			product := osbuild.GRUB2Product{
				Name:    t.arch.distro.product,
				Version: t.arch.distro.osVersion,
				Nick:    t.arch.distro.nick,
			}

			ver, _ := rpmmd.GetVerStrFromPackageSpecList(packages, "dracut-config-rescue")
			haveRescue := ver != ""

			id := "76a22bf4-f153-4541-b6c7-0332c0dfaeac"
			entries := osbuild.MakeGrub2MenuEntries(id, kernelVer, product, haveRescue)

			var vendor string // whether to boot via uefi, and if so the vendor
			var legacy string // whether to boot via legacy, and if so the platform

			bt := t.getBootType()

			if bt == distro.HybridBootType || bt == distro.LegacyBootType {
				legacy = t.arch.legacy
			}

			if bt == distro.HybridBootType || bt == distro.UEFIBootType {
				vendor = t.arch.distro.vendor
			}

			// we rely on stage option validation to detect invalid boot configurations
			options := osbuild.NewGrub2LegacyStageOptions(
				imageConfig.Grub2Config,
				pt,
				kernelOptions,
				legacy,
				vendor,
				entries,
			)
			bootloader = osbuild.NewGrub2LegacyStage(options)
		}

		p.AddStage(bootloader)
	}

	p.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))

	return p, nil
}

func liveImagePipeline(inputPipelineName string, outputFilename string, pt *disk.PartitionTable, arch *architecture, kernelVer string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "image"
	p.Build = "name:build"

	for _, stage := range osbuild.GenImagePrepareStages(pt, outputFilename, osbuild.PTSgdisk) {
		p.AddStage(stage)
	}

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := osbuild.GenCopyFSTreeOptions(inputName, inputPipelineName, outputFilename, pt)
	copyInputs := osbuild.NewCopyStagePipelineTreeInputs(inputName, inputPipelineName)
	p.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	for _, stage := range osbuild.GenImageFinishStages(pt, outputFilename) {
		p.AddStage(stage)
	}

	loopback := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: outputFilename})
	p.AddStage(bootloaderInstStage(outputFilename, pt, arch, kernelVer, copyDevices, copyMounts, loopback))
	return p
}

func qemuPipeline(inputPipelineName, inputFilename, outputFilename string, format osbuild.QEMUFormat, formatOptions osbuild.QEMUFormatOptions) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = string(format)
	p.Build = "name:build"

	qemuStage := osbuild.NewQEMUStage(
		osbuild.NewQEMUStageOptions(outputFilename, format, formatOptions),
		osbuild.NewQemuStagePipelineFilesInputs(inputPipelineName, inputFilename),
	)
	p.AddStage(qemuStage)
	return p
}

func xzArchivePipeline(inputPipelineName, inputFilename, outputFilename string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "archive"
	p.Build = "name:build"

	p.AddStage(osbuild.NewXzStage(
		osbuild.NewXzStageOptions(outputFilename),
		osbuild.NewFilesInputs(osbuild.NewFilesInputReferencesPipeline(inputPipelineName, inputFilename)),
	))

	return p
}
