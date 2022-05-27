package rhel90beta

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func qcow2Pipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options)
	if err != nil {
		return nil, err
	}

	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	treePipeline = prependKernelCmdlineStage(treePipeline, t, partitionTable)

	if options.Subscription == nil {
		// RHSM DNF plugins should be by default disabled on RHEL Guest KVM images
		treePipeline.AddStage(osbuild.NewRHSMStage(&osbuild.RHSMStageOptions{
			DnfPlugins: &osbuild.RHSMStageOptionsDnfPlugins{
				ProductID: &osbuild.RHSMStageOptionsDnfPlugin{
					Enabled: false,
				},
				SubscriptionManager: &osbuild.RHSMStageOptionsDnfPlugin{
					Enabled: false,
				},
			},
		}))
	}
	treePipeline.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(partitionTable)))
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	treePipeline.AddStage(bootloaderConfigStage(t, partitionTable, customizations.GetKernel(), kernelVer))
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, osbuild.QEMUFormatQCOW2, osbuild.QCOW2Options{Compat: "1.1"})
	pipelines = append(pipelines, *qemuPipeline)

	return pipelines, nil
}

func prependKernelCmdlineStage(pipeline *osbuild.Pipeline, t *imageType, pt *disk.PartitionTable) *osbuild.Pipeline {
	rootFs := pt.FindMountable("/")
	if rootFs == nil {
		panic("root filesystem must be defined for kernel-cmdline stage, this is a programming error")
	}
	rootFsUUID := rootFs.GetFSSpec().UUID
	kernelStage := osbuild.NewKernelCmdlineStage(osbuild.NewKernelCmdlineStageOptions(rootFsUUID, t.kernelOptions))
	pipeline.Stages = append([]*osbuild.Stage{kernelStage}, pipeline.Stages...)
	return pipeline
}

func vhdPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))
	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options)
	if err != nil {
		return nil, err
	}

	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	treePipeline = prependKernelCmdlineStage(treePipeline, t, partitionTable)
	treePipeline.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(partitionTable)))
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	treePipeline.AddStage(bootloaderConfigStage(t, partitionTable, customizations.GetKernel(), kernelVer))
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, osbuild.QEMUFormatVPC, nil)
	pipelines = append(pipelines, *qemuPipeline)
	return pipelines, nil
}

func vmdkPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))
	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options)
	if err != nil {
		return nil, err
	}

	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	treePipeline = prependKernelCmdlineStage(treePipeline, t, partitionTable)
	treePipeline.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(partitionTable)))
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	treePipeline.AddStage(bootloaderConfigStage(t, partitionTable, customizations.GetKernel(), kernelVer))
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, osbuild.QEMUFormatVMDK, osbuild.VMDKOptions{Subformat: osbuild.VMDKSubformatStreamOptimized})
	pipelines = append(pipelines, *qemuPipeline)
	return pipelines, nil
}

func openstackPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))
	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options)
	if err != nil {
		return nil, err
	}

	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}
	treePipeline = prependKernelCmdlineStage(treePipeline, t, partitionTable)
	treePipeline.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(partitionTable)))
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	treePipeline.AddStage(bootloaderConfigStage(t, partitionTable, customizations.GetKernel(), kernelVer))
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, osbuild.QEMUFormatQCOW2, nil)
	pipelines = append(pipelines, *qemuPipeline)
	return pipelines, nil
}

// ec2BaseTreePipeline returns the base OS pipeline common for all EC2 image types.
//
// The expectation is that specific EC2 image types can extend the returned pipeline
// by appending additional stages.
//
// The argument `withRHUI` should be set to `true` only if the image package set includes RHUI client packages.
//
// Note: the caller of this function has to append the `osbuild.NewSELinuxStage(selinuxStageOptions(false))` stage
// as the last one to the returned pipeline. The stage is not appended on purpose, to allow caller to append
// any additional stages to the pipeline, but before the SELinuxStage, which must be always the last one.
func ec2BaseTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec,
	c *blueprint.Customizations, options distro.ImageOptions, enabledServices, disabledServices []string,
	defaultTarget string, withRHUI bool, pt *disk.PartitionTable) (*osbuild.Pipeline, error) {
	p := new(osbuild.Pipeline)
	p.Name = "os"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(packages)))

	// If the /boot is on a separate partition, the prefix for the BLS stage must be ""
	if pt.FindMountable("/boot") == nil {
		p.AddStage(osbuild.NewFixBLSStage(&osbuild.FixBLSStageOptions{}))
	} else {
		p.AddStage(osbuild.NewFixBLSStage(&osbuild.FixBLSStageOptions{Prefix: common.StringToPtr("")}))
	}

	language, keyboard := c.GetPrimaryLocale()
	if language != nil {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: *language}))
	} else {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US.UTF-8"}))
	}
	if keyboard != nil {
		p.AddStage(osbuild.NewKeymapStage(&osbuild.KeymapStageOptions{Keymap: *keyboard}))
	} else {
		p.AddStage(osbuild.NewKeymapStage(&osbuild.KeymapStageOptions{
			Keymap: "us",
			X11Keymap: &osbuild.X11KeymapOptions{
				Layouts: []string{"us"},
			},
		}))
	}

	if hostname := c.GetHostname(); hostname != nil {
		p.AddStage(osbuild.NewHostnameStage(&osbuild.HostnameStageOptions{Hostname: *hostname}))
	}

	timezone, ntpServers := c.GetTimezoneSettings()
	if timezone != nil {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: *timezone}))
	} else {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: "UTC"}))
	}

	if len(ntpServers) > 0 {
		p.AddStage(osbuild.NewChronyStage(&osbuild.ChronyStageOptions{Timeservers: ntpServers}))
	} else {
		p.AddStage(osbuild.NewChronyStage(&osbuild.ChronyStageOptions{
			Servers: []osbuild.ChronyConfigServer{
				{
					Hostname: "169.254.169.123",
					Prefer:   common.BoolToPtr(true),
					Iburst:   common.BoolToPtr(true),
					Minpoll:  common.IntToPtr(4),
					Maxpoll:  common.IntToPtr(4),
				},
			},
			// empty string will remove any occurrences of the option from the configuration
			LeapsecTz: common.StringToPtr(""),
		}))
	}

	if groups := c.GetGroups(); len(groups) > 0 {
		p.AddStage(osbuild.NewGroupsStage(osbuild.NewGroupsStageOptions(groups)))
	}

	if userOptions, err := osbuild.NewUsersStageOptions(c.GetUsers(), false); err != nil {
		return nil, err
	} else if userOptions != nil {
		p.AddStage(osbuild.NewUsersStage(userOptions))
	}

	if services := c.GetServices(); services != nil || enabledServices != nil || disabledServices != nil || defaultTarget != "" {
		p.AddStage(osbuild.NewSystemdStage(systemdStageOptions(enabledServices, disabledServices, services, defaultTarget)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(firewallStageOptions(firewall)))
	}

	p.AddStage(osbuild.NewSystemdLogindStage(&osbuild.SystemdLogindStageOptions{
		Filename: "00-getty-fixes.conf",
		Config: osbuild.SystemdLogindConfigDropin{

			Login: osbuild.SystemdLogindConfigLoginSection{
				NAutoVTs: common.IntToPtr(0),
			},
		},
	}))

	p.AddStage(osbuild.NewSysconfigStage(&osbuild.SysconfigStageOptions{
		Kernel: &osbuild.SysconfigKernelOptions{
			UpdateDefault: true,
			DefaultKernel: "kernel",
		},
		Network: &osbuild.SysconfigNetworkOptions{
			Networking: true,
			NoZeroConf: true,
		},
		NetworkScripts: &osbuild.NetworkScriptsOptions{
			IfcfgFiles: map[string]osbuild.IfcfgFile{
				"eth0": {
					Device:    "eth0",
					Bootproto: osbuild.IfcfgBootprotoDHCP,
					OnBoot:    common.BoolToPtr(true),
					Type:      osbuild.IfcfgTypeEthernet,
					UserCtl:   common.BoolToPtr(true),
					PeerDNS:   common.BoolToPtr(true),
					IPv6Init:  common.BoolToPtr(false),
				},
			},
		},
	}))

	p.AddStage(osbuild.NewCloudInitStage(&osbuild.CloudInitStageOptions{
		Filename: "00-rhel-default-user.cfg",
		Config: osbuild.CloudInitConfigFile{
			SystemInfo: &osbuild.CloudInitConfigSystemInfo{
				DefaultUser: &osbuild.CloudInitConfigDefaultUser{
					Name: "ec2-user",
				},
			},
		},
	}))

	p.AddStage(osbuild.NewModprobeStage(&osbuild.ModprobeStageOptions{
		Filename: "blacklist-nouveau.conf",
		Commands: osbuild.ModprobeConfigCmdList{
			osbuild.NewModprobeConfigCmdBlacklist("nouveau"),
		},
	}))

	p.AddStage(osbuild.NewDracutConfStage(&osbuild.DracutConfStageOptions{
		Filename: "sgdisk.conf",
		Config: osbuild.DracutConfigFile{
			Install: []string{"sgdisk"},
		},
	}))

	// RHBZ#1822853
	p.AddStage(osbuild.NewSystemdUnitStage(&osbuild.SystemdUnitStageOptions{
		Unit:   "nm-cloud-setup.service",
		Dropin: "10-rh-enable-for-ec2.conf",
		Config: osbuild.SystemdServiceUnitDropin{
			Service: &osbuild.SystemdUnitServiceSection{
				Environment: "NM_CLOUD_SETUP_EC2=yes",
			},
		},
	}))

	p.AddStage(osbuild.NewAuthselectStage(&osbuild.AuthselectStageOptions{
		Profile: "sssd",
	}))

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
	} else {
		// The EC2 images should keep the RHSM DNF plugins enabled (RHBZ#1996670)
		rhsmStageOptions := &osbuild.RHSMStageOptions{
			// RHBZ#1932802
			SubMan: &osbuild.RHSMStageOptionsSubMan{
				Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
					AutoRegistration: common.BoolToPtr(true),
				},
			},
		}

		// Disable RHSM redhat.repo management only if the image uses RHUI
		// for content. Otherwise subscribing the system manually after booting
		// it would result in empty redhat.repo. Without RHUI, such system
		// would have no way to get Red Hat content, but enable the repo
		// management manually, which would be very confusing.
		// RHBZ#1932802
		if withRHUI {
			rhsmStageOptions.SubMan.Rhsm = &osbuild.SubManConfigRHSMSection{
				ManageRepos: common.BoolToPtr(false),
			}
		}

		p.AddStage(osbuild.NewRHSMStage(rhsmStageOptions))
	}

	return p, nil
}

func ec2X86_64BaseTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec,
	c *blueprint.Customizations, options distro.ImageOptions, enabledServices, disabledServices []string,
	defaultTarget string, withRHUI bool, pt *disk.PartitionTable) (*osbuild.Pipeline, error) {

	treePipeline, err := ec2BaseTreePipeline(repos, packages, c, options, enabledServices, disabledServices, defaultTarget, withRHUI, pt)
	if err != nil {
		return nil, err
	}

	// EC2 x86_64-specific stages
	// Add 'nvme' driver to handle the case when initramfs is getting forcefully
	// rebuild on a Xen instance (and not able to boot on Nitro after that).
	treePipeline.AddStage(osbuild.NewDracutConfStage(&osbuild.DracutConfStageOptions{
		Filename: "ec2.conf",
		Config: osbuild.DracutConfigFile{
			AddDrivers: []string{
				"nvme",
				"xen-blkfront",
			},
		},
	}))

	return treePipeline, nil
}

func ec2CommonPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions,
	repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec,
	rng *rand.Rand, withRHUI bool, diskfile string) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	var treePipeline *osbuild.Pipeline
	switch arch := t.arch.Name(); arch {
	// rhel-ec2-x86_64, rhel-ha-ec2
	case distro.X86_64ArchName:
		treePipeline, err = ec2X86_64BaseTreePipeline(repos, packageSetSpecs[osPkgsKey], customizations, options, t.enabledServices, t.disabledServices, t.defaultTarget, withRHUI, partitionTable)
	// rhel-ec2-aarch64
	case distro.Aarch64ArchName:
		treePipeline, err = ec2BaseTreePipeline(repos, packageSetSpecs[osPkgsKey], customizations, options, t.enabledServices, t.disabledServices, t.defaultTarget, withRHUI, partitionTable)
	default:
		return nil, fmt.Errorf("ec2CommonPipelines: unsupported image architecture: %q", arch)
	}
	if err != nil {
		return nil, err
	}

	treePipeline = prependKernelCmdlineStage(treePipeline, t, partitionTable)
	treePipeline.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(partitionTable)))
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	treePipeline.AddStage(bootloaderConfigStage(t, partitionTable, customizations.GetKernel(), kernelVer))
	// The last stage must be the SELinux stage
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)
	return pipelines, nil
}

func ec2SapPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions,
	repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec,
	rng *rand.Rand, withRHUI bool, diskfile string) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	var treePipeline *osbuild.Pipeline
	switch arch := t.arch.Name(); arch {
	// rhel-sap-ec2
	case distro.X86_64ArchName:
		treePipeline, err = ec2X86_64BaseTreePipeline(repos, packageSetSpecs[osPkgsKey], customizations, options, t.enabledServices, t.disabledServices, t.defaultTarget, withRHUI, partitionTable)
	default:
		return nil, fmt.Errorf("ec2SapPipelines: unsupported image architecture: %q", arch)
	}
	if err != nil {
		return nil, err
	}

	// SAP-specific configuration
	treePipeline.AddStage(osbuild.NewSELinuxConfigStage(&osbuild.SELinuxConfigStageOptions{
		State: osbuild.SELinuxStatePermissive,
	}))

	// RHBZ#1960617
	treePipeline.AddStage(osbuild.NewTunedStage(osbuild.NewTunedStageOptions("sap-hana")))

	// RHBZ#1959979
	treePipeline.AddStage(osbuild.NewTmpfilesdStage(osbuild.NewTmpfilesdStageOptions("sap.conf",
		[]osbuild.TmpfilesdConfigLine{
			{
				Type: "x",
				Path: "/tmp/.sap*",
			},
			{
				Type: "x",
				Path: "/tmp/.hdb*lock",
			},
			{
				Type: "x",
				Path: "/tmp/.trex*lock",
			},
		},
	)))

	// RHBZ#1959963
	treePipeline.AddStage(osbuild.NewPamLimitsConfStage(osbuild.NewPamLimitsConfStageOptions("99-sap.conf",
		[]osbuild.PamLimitsConfigLine{
			{
				Domain: "@sapsys",
				Type:   osbuild.PamLimitsTypeHard,
				Item:   osbuild.PamLimitsItemNofile,
				Value:  osbuild.PamLimitsValueInt(65536),
			},
			{
				Domain: "@sapsys",
				Type:   osbuild.PamLimitsTypeSoft,
				Item:   osbuild.PamLimitsItemNofile,
				Value:  osbuild.PamLimitsValueInt(65536),
			},
			{
				Domain: "@dba",
				Type:   osbuild.PamLimitsTypeHard,
				Item:   osbuild.PamLimitsItemNofile,
				Value:  osbuild.PamLimitsValueInt(65536),
			},
			{
				Domain: "@dba",
				Type:   osbuild.PamLimitsTypeSoft,
				Item:   osbuild.PamLimitsItemNofile,
				Value:  osbuild.PamLimitsValueInt(65536),
			},
			{
				Domain: "@sapsys",
				Type:   osbuild.PamLimitsTypeHard,
				Item:   osbuild.PamLimitsItemNproc,
				Value:  osbuild.PamLimitsValueUnlimited,
			},
			{
				Domain: "@sapsys",
				Type:   osbuild.PamLimitsTypeSoft,
				Item:   osbuild.PamLimitsItemNproc,
				Value:  osbuild.PamLimitsValueUnlimited,
			},
			{
				Domain: "@dba",
				Type:   osbuild.PamLimitsTypeHard,
				Item:   osbuild.PamLimitsItemNproc,
				Value:  osbuild.PamLimitsValueUnlimited,
			},
			{
				Domain: "@dba",
				Type:   osbuild.PamLimitsTypeSoft,
				Item:   osbuild.PamLimitsItemNproc,
				Value:  osbuild.PamLimitsValueUnlimited,
			},
		},
	)))

	// RHBZ#1959962
	treePipeline.AddStage(osbuild.NewSysctldStage(osbuild.NewSysctldStageOptions("sap.conf",
		[]osbuild.SysctldConfigLine{
			{
				Key:   "kernel.pid_max",
				Value: "4194304",
			},
			{
				Key:   "vm.max_map_count",
				Value: "2147483647",
			},
		},
	)))

	// E4S/EUS
	treePipeline.AddStage(osbuild.NewDNFConfigStage(osbuild.NewDNFConfigStageOptions(
		[]osbuild.DNFVariable{
			{
				Name:  "releasever",
				Value: "9.0",
			},
		},
		nil,
	)))

	treePipeline = prependKernelCmdlineStage(treePipeline, t, partitionTable)
	treePipeline.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(partitionTable)))
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	treePipeline.AddStage(bootloaderConfigStage(t, partitionTable, customizations.GetKernel(), kernelVer))
	// The last stage must be the SELinux stage
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)
	return pipelines, nil
}

// ec2Pipelines returns pipelines which produce uncompressed EC2 images which are expected to use RHSM for content
func ec2Pipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	return ec2CommonPipelines(t, customizations, options, repos, packageSetSpecs, rng, false, t.Filename())
}

// rhelEc2Pipelines returns pipelines which produce XZ-compressed EC2 images which are expected to use RHUI for content
func rhelEc2Pipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	rawImageFilename := "image.raw"

	pipelines, err := ec2CommonPipelines(t, customizations, options, repos, packageSetSpecs, rng, true, rawImageFilename)
	if err != nil {
		return nil, err
	}

	lastPipeline := pipelines[len(pipelines)-1]
	pipelines = append(pipelines, *xzArchivePipeline(lastPipeline.Name, rawImageFilename, t.Filename()))

	return pipelines, nil
}

// rhelEc2SapPipelines returns pipelines which produce XZ-compressed EC2 SAP images which are expected to use RHUI for content
func rhelEc2SapPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	rawImageFilename := "image.raw"

	pipelines, err := ec2SapPipelines(t, customizations, options, repos, packageSetSpecs, rng, true, rawImageFilename)
	if err != nil {
		return nil, err
	}

	lastPipeline := pipelines[len(pipelines)-1]
	pipelines = append(pipelines, *xzArchivePipeline(lastPipeline.Name, rawImageFilename, t.Filename()))

	return pipelines, nil
}

func tarPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options)
	if err != nil {
		return nil, err
	}
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)
	tarPipeline := osbuild.Pipeline{
		Name:  "root-tar",
		Build: "name:build",
	}
	tarPipeline.AddStage(tarStage("os", "root.tar.xz"))
	pipelines = append(pipelines, tarPipeline)
	return pipelines, nil
}

func edgeInstallerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))
	installerPackages := packageSetSpecs[installerPkgsKey]
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(installerPackages, "kernel")
	ostreeRepoPath := "/ostree/repo"

	ksUsers := len(customizations.GetUsers())+len(customizations.GetGroups()) > 0
	pipelines = append(pipelines, *anacondaTreePipeline(repos, installerPackages, kernelVer, t.Arch().Name(), ostreePayloadStages(options, ostreeRepoPath), ksUsers))
	kickstartOptions, err := osbuild.NewKickstartStageOptions(kspath, "", customizations.GetUsers(), customizations.GetGroups(), fmt.Sprintf("file://%s", ostreeRepoPath), options.OSTree.Ref, "rhel")
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, *bootISOTreePipeline(kernelVer, t.Arch().Name(), kickstartOptions))
	pipelines = append(pipelines, *bootISOPipeline(t.Filename(), t.Arch().Name()))
	return pipelines, nil
}

func imageInstallerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options)
	if err != nil {
		return nil, err
	}
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	kernelPkg := new(rpmmd.PackageSpec)
	installerPackages := packageSetSpecs[installerPkgsKey]
	for _, pkg := range installerPackages {
		if pkg.Name == "kernel" {
			// Implicit memory alasing doesn't couse any bug in this case
			/* #nosec G601 */
			kernelPkg = &pkg
			break
		}
	}
	if kernelPkg == nil {
		return nil, fmt.Errorf("kernel package not found in installer package set")
	}
	kernelVer := fmt.Sprintf("%s-%s.%s", kernelPkg.Version, kernelPkg.Release, kernelPkg.Arch)

	tarPath := "/liveimg.tar"
	tarPayloadStages := []*osbuild.Stage{tarStage("os", tarPath)}
	pipelines = append(pipelines, *anacondaTreePipeline(repos, installerPackages, kernelVer, t.Arch().Name(), tarPayloadStages, true))
	kickstartOptions, err := osbuild.NewKickstartStageOptions(kspath, fmt.Sprintf("file://%s", tarPath), customizations.GetUsers(), customizations.GetGroups(), "", "", "rhel")
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, *bootISOTreePipeline(kernelVer, t.Arch().Name(), kickstartOptions))
	pipelines = append(pipelines, *bootISOPipeline(t.Filename(), t.Arch().Name()))
	return pipelines, nil
}

func edgeCorePipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	treePipeline, err := ostreeTreePipeline(repos, packageSetSpecs[osPkgsKey], customizations, options, t.enabledServices, t.disabledServices, t.defaultTarget)
	if err != nil {
		return nil, err
	}

	pipelines = append(pipelines, *treePipeline)
	pipelines = append(pipelines, *ostreeCommitPipeline(options))

	return pipelines, nil
}

func edgeCommitPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines, err := edgeCorePipelines(t, customizations, options, repos, packageSetSpecs)
	if err != nil {
		return nil, err
	}
	tarPipeline := osbuild.Pipeline{
		Name:  "commit-archive",
		Build: "name:build",
	}
	tarPipeline.AddStage(tarStage("ostree-commit", t.Filename()))
	pipelines = append(pipelines, tarPipeline)
	return pipelines, nil
}

func edgeContainerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines, err := edgeCorePipelines(t, customizations, options, repos, packageSetSpecs)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, *containerTreePipeline(repos, packageSetSpecs[containerPkgsKey], options, customizations))
	pipelines = append(pipelines, *containerPipeline(t))
	return pipelines, nil
}

func buildPipeline(repos []rpmmd.RepoConfig, buildPackageSpecs []rpmmd.PackageSpec) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "build"
	p.Runner = "org.osbuild.rhel90"
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(buildPackageSpecs)))
	p.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(true)))
	return p
}

func osPipeline(t *imageType,
	repos []rpmmd.RepoConfig,
	packages []rpmmd.PackageSpec,
	c *blueprint.Customizations,
	options distro.ImageOptions) (*osbuild.Pipeline, error) {
	p := new(osbuild.Pipeline)
	p.Name = "os"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(packages)))
	p.AddStage(osbuild.NewFixBLSStage(&osbuild.FixBLSStageOptions{}))
	language, keyboard := c.GetPrimaryLocale()
	if language != nil {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: *language}))
	} else {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US.UTF-8"}))
	}
	if keyboard != nil {
		p.AddStage(osbuild.NewKeymapStage(&osbuild.KeymapStageOptions{Keymap: *keyboard}))
	}
	if hostname := c.GetHostname(); hostname != nil {
		p.AddStage(osbuild.NewHostnameStage(&osbuild.HostnameStageOptions{Hostname: *hostname}))
	}

	timezone, ntpServers := c.GetTimezoneSettings()
	if timezone != nil {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: *timezone}))
	} else {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: "America/New_York"}))
	}

	if len(ntpServers) > 0 {
		p.AddStage(osbuild.NewChronyStage(&osbuild.ChronyStageOptions{Timeservers: ntpServers}))
	}

	if !t.bootISO {
		if groups := c.GetGroups(); len(groups) > 0 {
			p.AddStage(osbuild.NewGroupsStage(osbuild.NewGroupsStageOptions(groups)))
		}

		if userOptions, err := osbuild.NewUsersStageOptions(c.GetUsers(), false); err != nil {
			return nil, err
		} else if userOptions != nil {
			p.AddStage(osbuild.NewUsersStage(userOptions))
		}
	}

	if services := c.GetServices(); services != nil || t.enabledServices != nil || t.disabledServices != nil || t.defaultTarget != "" {
		p.AddStage(osbuild.NewSystemdStage(systemdStageOptions(t.enabledServices, t.disabledServices, services, t.defaultTarget)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(firewallStageOptions(firewall)))
	}

	// These are the current defaults for the sysconfig stage. This can be changed to be image type exclusive if different configs are needed.
	p.AddStage(osbuild.NewSysconfigStage(&osbuild.SysconfigStageOptions{
		Kernel: &osbuild.SysconfigKernelOptions{
			UpdateDefault: true,
			DefaultKernel: "kernel",
		},
		Network: &osbuild.SysconfigNetworkOptions{
			Networking: true,
			NoZeroConf: true,
		},
	}))

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
		},
		))
	}
	return p, nil
}

func ostreeTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, c *blueprint.Customizations, options distro.ImageOptions, enabledServices, disabledServices []string, defaultTarget string) (*osbuild.Pipeline, error) {
	p := new(osbuild.Pipeline)
	p.Name = "ostree-tree"
	p.Build = "name:build"

	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(packages)))
	p.AddStage(osbuild.NewFixBLSStage(&osbuild.FixBLSStageOptions{}))
	language, keyboard := c.GetPrimaryLocale()
	if language != nil {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: *language}))
	} else {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US.UTF-8"}))
	}
	if keyboard != nil {
		p.AddStage(osbuild.NewKeymapStage(&osbuild.KeymapStageOptions{Keymap: *keyboard}))
	}
	if hostname := c.GetHostname(); hostname != nil {
		p.AddStage(osbuild.NewHostnameStage(&osbuild.HostnameStageOptions{Hostname: *hostname}))
	}

	timezone, ntpServers := c.GetTimezoneSettings()
	if timezone != nil {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: *timezone}))
	} else {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: "America/New_York"}))
	}

	if len(ntpServers) > 0 {
		p.AddStage(osbuild.NewChronyStage(&osbuild.ChronyStageOptions{Timeservers: ntpServers}))
	}

	if groups := c.GetGroups(); len(groups) > 0 {
		p.AddStage(osbuild.NewGroupsStage(osbuild.NewGroupsStageOptions(groups)))
	}

	if userOptions, err := osbuild.NewUsersStageOptions(c.GetUsers(), false); err != nil {
		return nil, err
	} else if userOptions != nil {
		// for ostree, writing the key during user creation is redundant and
		// can cause issues so create users without keys and write them on
		// first boot
		userOptionsSansKeys, err := osbuild.NewUsersStageOptions(c.GetUsers(), true)
		if err != nil {
			return nil, err
		}
		p.AddStage(osbuild.NewUsersStage(userOptionsSansKeys))
		p.AddStage(osbuild.NewFirstBootStage(usersFirstBootOptions(userOptions)))
	}

	if services := c.GetServices(); services != nil || enabledServices != nil || disabledServices != nil || defaultTarget != "" {
		p.AddStage(osbuild.NewSystemdStage(systemdStageOptions(enabledServices, disabledServices, services, defaultTarget)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(firewallStageOptions(firewall)))
	}

	// These are the current defaults for the sysconfig stage. This can be changed to be image type exclusive if different configs are needed.
	p.AddStage(osbuild.NewSysconfigStage(&osbuild.SysconfigStageOptions{
		Kernel: &osbuild.SysconfigKernelOptions{
			UpdateDefault: true,
			DefaultKernel: "kernel",
		},
		Network: &osbuild.SysconfigNetworkOptions{
			Networking: true,
			NoZeroConf: true,
		},
	}))

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
		},
		))
	}

	p.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	p.AddStage(osbuild.NewOSTreePrepTreeStage(&osbuild.OSTreePrepTreeStageOptions{
		EtcGroupMembers: []string{
			// NOTE: We may want to make this configurable.
			"wheel", "docker",
		},
	}))
	return p, nil
}
func ostreeCommitPipeline(options distro.ImageOptions) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "ostree-commit"
	p.Build = "name:build"
	p.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: "/repo"}))

	commitStageInput := new(osbuild.OSTreeCommitStageInput)
	commitStageInput.Type = "org.osbuild.tree"
	commitStageInput.Origin = "org.osbuild.pipeline"
	commitStageInput.References = osbuild.OSTreeCommitStageReferences{"name:ostree-tree"}

	p.AddStage(osbuild.NewOSTreeCommitStage(
		&osbuild.OSTreeCommitStageOptions{
			Ref:       options.OSTree.Ref,
			OSVersion: osVersion,
			Parent:    options.OSTree.Parent,
		},
		&osbuild.OSTreeCommitStageInputs{Tree: commitStageInput}),
	)
	return p
}

func tarStage(source, filename string) *osbuild.Stage {
	tree := new(osbuild.TarStageInput)
	tree.Type = "org.osbuild.tree"
	tree.Origin = "org.osbuild.pipeline"
	tree.References = []string{"name:" + source}
	return osbuild.NewTarStage(&osbuild.TarStageOptions{Filename: filename}, &osbuild.TarStageInputs{Tree: tree})
}

func containerTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, options distro.ImageOptions, c *blueprint.Customizations) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "container-tree"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(packages)))
	language, _ := c.GetPrimaryLocale()
	if language != nil {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: *language}))
	} else {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US"}))
	}
	p.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: "/var/www/html/repo"}))

	p.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: "/var/www/html/repo"},
		osbuild.NewOstreePullStageInputs("org.osbuild.pipeline", "name:ostree-commit", options.OSTree.Ref),
	))
	return p
}

func containerPipeline(t *imageType) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "container"
	p.Build = "name:build"
	options := &osbuild.OCIArchiveStageOptions{
		Architecture: t.arch.Name(),
		Filename:     t.Filename(),
		Config: &osbuild.OCIArchiveConfig{
			Cmd:          []string{"httpd", "-D", "FOREGROUND"},
			ExposedPorts: []string{"80"},
		},
	}
	baseInput := new(osbuild.OCIArchiveStageInput)
	baseInput.Type = "org.osbuild.tree"
	baseInput.Origin = "org.osbuild.pipeline"
	baseInput.References = []string{"name:container-tree"}
	inputs := &osbuild.OCIArchiveStageInputs{Base: baseInput}
	p.AddStage(osbuild.NewOCIArchiveStage(options, inputs))
	return p
}

func ostreePayloadStages(options distro.ImageOptions, ostreeRepoPath string) []*osbuild.Stage {
	stages := make([]*osbuild.Stage, 0)

	// ostree commit payload
	stages = append(stages, osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: ostreeRepoPath}))
	stages = append(stages, osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: ostreeRepoPath},
		osbuild.NewOstreePullStageInputs("org.osbuild.source", options.OSTree.Parent, options.OSTree.Ref),
	))

	return stages
}

func anacondaTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, kernelVer string, arch string, payloadStages []*osbuild.Stage, users bool) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "anaconda-tree"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(packages)))
	for _, stage := range payloadStages {
		p.AddStage(stage)
	}
	p.AddStage(osbuild.NewBuildstampStage(buildStampStageOptions(arch)))
	p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US.UTF-8"}))

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

	p.AddStage(osbuild.NewUsersStage(usersStageOptions))
	p.AddStage(osbuild.NewAnacondaStage(osbuild.NewAnacondaStageOptions(users)))
	p.AddStage(osbuild.NewLoraxScriptStage(loraxScriptStageOptions(arch)))
	p.AddStage(osbuild.NewDracutStage(dracutStageOptions(kernelVer)))

	return p
}

func bootISOTreePipeline(kernelVer string, arch string, ksOptions *osbuild.KickstartStageOptions) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "bootiso-tree"
	p.Build = "name:build"

	p.AddStage(osbuild.NewBootISOMonoStage(bootISOMonoStageOptions(kernelVer, arch), osbuild.NewBootISOMonoStagePipelineTreeInputs("anaconda-tree")))
	p.AddStage(osbuild.NewKickstartStage(ksOptions))
	p.AddStage(osbuild.NewDiscinfoStage(discinfoStageOptions(arch)))

	return p
}
func bootISOPipeline(filename string, arch string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "bootiso"
	p.Build = "name:build"

	p.AddStage(osbuild.NewXorrisofsStage(xorrisofsStageOptions(filename, arch), osbuild.NewXorrisofsStagePipelineTreeInputs("bootiso-tree")))
	p.AddStage(osbuild.NewImplantisomd5Stage(&osbuild.Implantisomd5StageOptions{Filename: filename}))

	return p
}

func liveImagePipeline(inputPipelineName string, outputFilename string, pt *disk.PartitionTable, arch *architecture, kernelVer string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "image"
	p.Build = "name:build"

	for _, stage := range osbuild.GenImagePrepareStages(pt, outputFilename, osbuild.PTSfdisk) {
		p.AddStage(stage)
	}

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := osbuild.GenCopyFSTreeOptions(inputName, inputPipelineName, outputFilename, pt)
	copyInputs := osbuild.NewCopyStagePipelineTreeInputs(inputName, inputPipelineName)
	p.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	loopback := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: outputFilename})
	p.AddStage(bootloaderInstStage(outputFilename, pt, arch, kernelVer, copyDevices, copyMounts, loopback))

	for _, stage := range osbuild.GenImageFinishStages(pt, outputFilename) {
		p.AddStage(stage)
	}

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

func bootloaderConfigStage(t *imageType, partitionTable *disk.PartitionTable, kernel *blueprint.KernelCustomization, kernelVer string) *osbuild.Stage {
	if t.Arch().Name() == distro.S390xArchName {
		return osbuild.NewZiplStage(new(osbuild.ZiplStageOptions))
	}

	kernelOptions := t.kernelOptions
	uefi := t.supportsUEFI()
	legacy := t.arch.legacy
	options := osbuild.NewGrub2StageOptions(partitionTable, kernelOptions, kernel, kernelVer, uefi, legacy, "redhat", false)

	// `NewGrub2StageOptions` now might set this to "saved"; but when 9.0 beta was released
	// it was not available. To keep manifests from changing we need to reset it here.
	if options.Config != nil {
		options.Config.Default = ""
	}

	return osbuild.NewGRUB2Stage(options)
}

func bootloaderInstStage(filename string, pt *disk.PartitionTable, arch *architecture, kernelVer string, devices *osbuild.Devices, mounts *osbuild.Mounts, disk *osbuild.Device) *osbuild.Stage {
	platform := arch.legacy
	if platform != "" {
		return osbuild.NewGrub2InstStage(osbuild.NewGrub2InstStageOption(filename, pt, platform))
	}

	if arch.name == distro.S390xArchName {
		return osbuild.NewZiplInstStage(osbuild.NewZiplInstStageOptions(kernelVer, pt), disk, devices, mounts)
	}

	return nil
}
