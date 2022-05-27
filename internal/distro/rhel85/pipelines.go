package rhel85

import (
	"fmt"
	"math/rand"
	"path"
	"path/filepath"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func qcow2Pipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options, partitionTable)
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
	treePipeline.AddStage(bootloaderConfigStage(t, partitionTable, customizations.GetKernel(), kernelVer, false, false))
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, osbuild.QEMUFormatQCOW2, osbuild.QCOW2Options{Compat: "0.10"})
	pipelines = append(pipelines, *qemuPipeline)

	return pipelines, nil
}

func prependKernelCmdlineStage(pipeline *osbuild.Pipeline, t *imageType, pt *disk.PartitionTable) *osbuild.Pipeline {
	if t.Arch().Name() == distro.S390xArchName {
		rootFs := pt.FindMountable("/")
		if rootFs == nil {
			panic("s390x image must have a root filesystem, this is a programming error")
		}
		kernelStage := osbuild.NewKernelCmdlineStage(osbuild.NewKernelCmdlineStageOptions(rootFs.GetFSSpec().UUID, t.kernelOptions))
		pipeline.Stages = append([]*osbuild.Stage{kernelStage}, pipeline.Stages...)
	}
	return pipeline
}

func vhdPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))
	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}
	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options, partitionTable)
	if err != nil {
		return nil, err
	}

	treePipeline.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(partitionTable)))
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	treePipeline.AddStage(bootloaderConfigStage(t, partitionTable, customizations.GetKernel(), kernelVer, false, false))
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)
	if err != nil {
		return nil, err
	}

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, osbuild.QEMUFormatVPC, nil)
	pipelines = append(pipelines, *qemuPipeline)
	return pipelines, nil
}

func vmdkPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))
	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options, partitionTable)
	if err != nil {
		return nil, err
	}

	treePipeline.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(partitionTable)))
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	treePipeline.AddStage(bootloaderConfigStage(t, partitionTable, customizations.GetKernel(), kernelVer, false, false))
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)
	if err != nil {
		return nil, err
	}

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, osbuild.QEMUFormatVMDK, osbuild.VMDKOptions{Subformat: osbuild.VMDKSubformatStreamOptimized})
	pipelines = append(pipelines, *qemuPipeline)
	return pipelines, nil
}

func openstackPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))
	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options, partitionTable)
	if err != nil {
		return nil, err
	}

	treePipeline.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(partitionTable)))
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	treePipeline.AddStage(bootloaderConfigStage(t, partitionTable, customizations.GetKernel(), kernelVer, false, false))
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)
	if err != nil {
		return nil, err
	}

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

	p.AddStage((osbuild.NewSshdConfigStage(&osbuild.SshdConfigStageOptions{
		Config: osbuild.SshdConfigConfig{
			PasswordAuthentication: common.BoolToPtr(false),
		},
	})))

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

	treePipeline.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(partitionTable)))
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	treePipeline.AddStage(bootloaderConfigStage(t, partitionTable, customizations.GetKernel(), kernelVer, false, false))
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

// osPipelineRhel86 is a backport of the osPipeline from RHEL-86
//
// This pipeline generator takes distro.ImageConfig instance, which
// defines the image default configuration
func osPipelineRhel86(t *imageType,
	imageConfig *distro.ImageConfig,
	repos []rpmmd.RepoConfig,
	packages []rpmmd.PackageSpec,
	c *blueprint.Customizations,
	options distro.ImageOptions,
	pt *disk.PartitionTable) (*osbuild.Pipeline, error) {
	p := new(osbuild.Pipeline)
	if t.rpmOstree {
		p.Name = "ostree-tree"
	} else {
		p.Name = "os"
	}
	p.Build = "name:build"

	if t.rpmOstree && options.OSTree.Parent != "" && options.OSTree.URL != "" {
		p.AddStage(osbuild.NewOSTreePasswdStage("org.osbuild.source", options.OSTree.Parent))
	}

	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(packages)))

	// If the /boot is on a separate partition, the prefix for the BLS stage must be ""
	if pt == nil || pt.FindMountable("/boot") == nil {
		p.AddStage(osbuild.NewFixBLSStage(&osbuild.FixBLSStageOptions{}))
	} else {
		p.AddStage(osbuild.NewFixBLSStage(&osbuild.FixBLSStageOptions{Prefix: common.StringToPtr("")}))
	}

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

	if !t.bootISO {
		// don't put users and groups in the payload of an installer
		// add them via kickstart instead
		if groups := c.GetGroups(); len(groups) > 0 {
			p.AddStage(osbuild.NewGroupsStage(osbuild.NewGroupsStageOptions(groups)))
		}

		if userOptions, err := osbuild.NewUsersStageOptions(c.GetUsers(), false); err != nil {
			return nil, err
		} else if userOptions != nil {
			if t.rpmOstree {
				// for ostree, writing the key during user creation is
				// redundant and can cause issues so create users without keys
				// and write them on first boot
				userOptionsSansKeys, err := osbuild.NewUsersStageOptions(c.GetUsers(), true)
				if err != nil {
					return nil, err
				}
				p.AddStage(osbuild.NewUsersStage(userOptionsSansKeys))
				p.AddStage(osbuild.NewFirstBootStage(usersFirstBootOptions(userOptions)))
			} else {
				p.AddStage(osbuild.NewUsersStage(userOptions))
			}
		}
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

	if dnfAutomaticConfig := imageConfig.DNFAutomaticConfig; dnfAutomaticConfig != nil {
		p.AddStage(osbuild.NewDNFAutomaticConfigStage(dnfAutomaticConfig))
	}

	for _, yumRepo := range imageConfig.YUMRepos {
		p.AddStage(osbuild.NewYumReposStage(yumRepo))
	}

	if udevRules := imageConfig.UdevRules; udevRules != nil {
		p.AddStage(osbuild.NewUdevRulesStage(udevRules))
	}

	if pt != nil {
		p = prependKernelCmdlineStage(p, t, pt)
		p.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(pt)))
		kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packages, c.GetKernel().Name)
		p.AddStage(bootloaderConfigStage(t, pt, c.GetKernel(), kernelVer, false, false))
	}

	p.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))

	if t.rpmOstree {
		p.AddStage(osbuild.NewOSTreePrepTreeStage(&osbuild.OSTreePrepTreeStageOptions{
			EtcGroupMembers: []string{
				// NOTE: We may want to make this configurable.
				"wheel", "docker",
			},
		}))
	}

	return p, nil
}

// gcePipelinesRhel86 is a slightly modified RHEL-86 version of gcePipelines() function
func gcePipelinesRhel86(t *imageType, imageConfig *distro.ImageConfig, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	treePipeline, err := osPipelineRhel86(t, imageConfig, repos, packageSetSpecs[osPkgsKey], customizations, options, partitionTable)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.raw"
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)

	archivePipeline := tarArchivePipeline("archive", imagePipeline.Name, &osbuild.TarStageOptions{
		Filename: t.Filename(),
		Format:   osbuild.TarArchiveFormatOldgnu,
		RootNode: osbuild.TarRootNodeOmit,
		// import of the image to GCP fails in case the options below are enabled, which is the default
		ACLs:    common.BoolToPtr(false),
		SELinux: common.BoolToPtr(false),
		Xattrs:  common.BoolToPtr(false),
	})
	pipelines = append(pipelines, *archivePipeline)

	return pipelines, nil
}

func getDefaultGceByosImageConfig() *distro.ImageConfig {
	return &distro.ImageConfig{
		Timezone: "UTC",
		TimeSynchronization: &osbuild.ChronyStageOptions{
			Timeservers: []string{"metadata.google.internal"},
		},
		Firewall: &osbuild.FirewallStageOptions{
			DefaultZone: "trusted",
		},
		EnabledServices: []string{
			"sshd",
			"rngd",
			"dnf-automatic.timer",
		},
		DisabledServices: []string{
			"sshd-keygen@",
			"reboot.target",
		},
		DefaultTarget: "multi-user.target",
		Locale:        "en_US.UTF-8",
		Keyboard: &osbuild.KeymapStageOptions{
			Keymap: "us",
		},
		DNFConfig: []*osbuild.DNFConfigStageOptions{
			{
				Config: &osbuild.DNFConfig{
					Main: &osbuild.DNFConfigMain{
						IPResolve: "4",
					},
				},
			},
		},
		DNFAutomaticConfig: &osbuild.DNFAutomaticConfigStageOptions{
			Config: &osbuild.DNFAutomaticConfig{
				Commands: &osbuild.DNFAutomaticConfigCommands{
					ApplyUpdates: common.BoolToPtr(true),
					UpgradeType:  osbuild.DNFAutomaticUpgradeTypeSecurity,
				},
			},
		},
		YUMRepos: []*osbuild.YumReposStageOptions{
			{
				Filename: "google-cloud.repo",
				Repos: []osbuild.YumRepository{
					{
						Id:           "google-compute-engine",
						Name:         "Google Compute Engine",
						BaseURL:      []string{"https://packages.cloud.google.com/yum/repos/google-compute-engine-el8-x86_64-stable"},
						Enabled:      common.BoolToPtr(true),
						GPGCheck:     common.BoolToPtr(true),
						RepoGPGCheck: common.BoolToPtr(false),
						GPGKey: []string{
							"https://packages.cloud.google.com/yum/doc/yum-key.gpg",
							"https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg",
						},
					},
					{
						Id:           "google-cloud-sdk",
						Name:         "Google Cloud SDK",
						BaseURL:      []string{"https://packages.cloud.google.com/yum/repos/cloud-sdk-el8-x86_64"},
						Enabled:      common.BoolToPtr(true),
						GPGCheck:     common.BoolToPtr(true),
						RepoGPGCheck: common.BoolToPtr(false),
						GPGKey: []string{
							"https://packages.cloud.google.com/yum/doc/yum-key.gpg",
							"https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg",
						},
					},
				},
			},
		},
		RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
			distro.RHSMConfigNoSubscription: {
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					// Don't disable RHSM redhat.repo management on the GCE
					// image, which is BYOS and does not use RHUI for content.
					// Otherwise subscribing the system manually after booting
					// it would result in empty redhat.repo. Without RHUI, such
					// system would have no way to get Red Hat content, but
					// enable the repo management manually, which would be very
					// confusing.
				},
			},
			distro.RHSMConfigWithSubscription: {
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					// do not disable the redhat.repo management if the user
					// explicitly request the system to be subscribed
				},
			},
		},
		SshdConfig: &osbuild.SshdConfigStageOptions{
			Config: osbuild.SshdConfigConfig{
				PasswordAuthentication: common.BoolToPtr(false),
				ClientAliveInterval:    common.IntToPtr(420),
				PermitRootLogin:        osbuild.PermitRootLoginValueNo,
			},
		},
		Sysconfig: []*osbuild.SysconfigStageOptions{
			{
				Kernel: &osbuild.SysconfigKernelOptions{
					DefaultKernel: "kernel-core",
					UpdateDefault: true,
				},
			},
		},
		Modprobe: []*osbuild.ModprobeStageOptions{
			{
				Filename: "blacklist-floppy.conf",
				Commands: osbuild.ModprobeConfigCmdList{
					osbuild.NewModprobeConfigCmdBlacklist("floppy"),
				},
			},
		},
	}
}

// GCE BYOS image
func gceByosPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	return gcePipelinesRhel86(t, getDefaultGceByosImageConfig(), customizations, options, repos, packageSetSpecs, rng)
}

func getDefaultGceRhuiImageConfig() *distro.ImageConfig {
	defaultGceRhuiImageConfig := &distro.ImageConfig{
		RHSMConfig: map[distro.RHSMSubscriptionStatus]*osbuild.RHSMStageOptions{
			distro.RHSMConfigNoSubscription: {
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					Rhsm: &osbuild.SubManConfigRHSMSection{
						ManageRepos: common.BoolToPtr(false),
					},
				},
			},
			distro.RHSMConfigWithSubscription: {
				SubMan: &osbuild.RHSMStageOptionsSubMan{
					Rhsmcertd: &osbuild.SubManConfigRHSMCERTDSection{
						AutoRegistration: common.BoolToPtr(true),
					},
					// do not disable the redhat.repo management if the user
					// explicitly request the system to be subscribed
				},
			},
		},
	}
	defaultGceRhuiImageConfig = defaultGceRhuiImageConfig.InheritFrom(getDefaultGceByosImageConfig())
	return defaultGceRhuiImageConfig
}

// GCE RHUI image
func gceRhuiPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	return gcePipelinesRhel86(t, getDefaultGceRhuiImageConfig(), customizations, options, repos, packageSetSpecs, rng)
}

func tarArchivePipeline(name, inputPipelineName string, tarOptions *osbuild.TarStageOptions) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = name
	p.Build = "name:build"
	p.AddStage(osbuild.NewTarStage(tarOptions, osbuild.NewTarStagePipelineTreeInputs(inputPipelineName)))
	return p
}

func tarPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options, nil)
	if err != nil {
		return nil, err
	}
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)
	tarPipeline := tarArchivePipeline("root-tar", treePipeline.Name, &osbuild.TarStageOptions{Filename: "root.tar.xz"})
	pipelines = append(pipelines, *tarPipeline)
	return pipelines, nil
}

//makeISORootPath return a path that can be used to address files and folders in
//the root of the iso
func makeISORootPath(p string) string {
	fullpath := path.Join("/run/install/repo", p)
	return fmt.Sprintf("file://%s", fullpath)
}

func edgeInstallerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))
	installerPackages := packageSetSpecs[installerPkgsKey]
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(installerPackages, "kernel")
	ostreeRepoPath := "/ostree/repo"
	payloadStages := ostreePayloadStages(options, ostreeRepoPath)
	kickstartOptions, err := osbuild.NewKickstartStageOptions(kspath, "", customizations.GetUsers(), customizations.GetGroups(), makeISORootPath(ostreeRepoPath), options.OSTree.Ref, "rhel")
	if err != nil {
		return nil, err
	}
	ksUsers := len(customizations.GetUsers())+len(customizations.GetGroups()) > 0
	pipelines = append(pipelines, *anacondaTreePipeline(repos, installerPackages, kernelVer, t.Arch().Name(), ksUsers))
	pipelines = append(pipelines, *bootISOTreePipeline(kernelVer, t.Arch().Name(), kickstartOptions, payloadStages))
	pipelines = append(pipelines, *bootISOPipeline(t.Filename(), t.Arch().Name(), false))
	return pipelines, nil
}

func imageInstallerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options, nil)
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
	tarPayloadStages := []*osbuild.Stage{osbuild.NewTarStage(&osbuild.TarStageOptions{Filename: tarPath}, osbuild.NewTarStagePipelineTreeInputs(treePipeline.Name))}
	kickstartOptions, err := osbuild.NewKickstartStageOptions(kspath, makeISORootPath(tarPath), customizations.GetUsers(), customizations.GetGroups(), "", "", "rhel")
	if err != nil {
		return nil, err
	}

	pipelines = append(pipelines, *anacondaTreePipeline(repos, installerPackages, kernelVer, t.Arch().Name(), true))
	pipelines = append(pipelines, *bootISOTreePipeline(kernelVer, t.Arch().Name(), kickstartOptions, tarPayloadStages))
	pipelines = append(pipelines, *bootISOPipeline(t.Filename(), t.Arch().Name(), true))
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
	tarPipeline := tarArchivePipeline("commit-archive", "ostree-commit", &osbuild.TarStageOptions{Filename: t.Filename()})
	pipelines = append(pipelines, *tarPipeline)
	return pipelines, nil
}

func edgeContainerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines, err := edgeCorePipelines(t, customizations, options, repos, packageSetSpecs)
	if err != nil {
		return nil, err
	}

	nginxConfigPath := "/etc/nginx.conf"
	httpPort := "8080"
	pipelines = append(pipelines, *containerTreePipeline(repos, packageSetSpecs[containerPkgsKey], options, customizations, nginxConfigPath, httpPort))
	pipelines = append(pipelines, *containerPipeline(t, nginxConfigPath, httpPort))
	return pipelines, nil
}

func edgeImagePipelines(t *imageType, filename string, options distro.ImageOptions, rng *rand.Rand) ([]osbuild.Pipeline, string, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	ostreeRepoPath := "/ostree/repo"
	imgName := "image.raw"

	partitionTable, err := t.getPartitionTable(nil, options, rng)
	if err != nil {
		return nil, "", err
	}

	// prepare ostree deployment tree
	treePipeline := ostreeDeployPipeline(t, partitionTable, ostreeRepoPath, nil, "", rng, options)
	pipelines = append(pipelines, *treePipeline)

	// make raw image from tree
	imagePipeline := liveImagePipeline(treePipeline.Name, imgName, partitionTable, t.arch, "")
	pipelines = append(pipelines, *imagePipeline)

	// compress image
	xzPipeline := xzArchivePipeline(imagePipeline.Name, imgName, filename)
	pipelines = append(pipelines, *xzPipeline)

	return pipelines, xzPipeline.Name, nil
}

func edgeRawImagePipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	imgName := t.filename

	// create the raw image
	imagePipelines, _, err := edgeImagePipelines(t, imgName, options, rng)
	if err != nil {
		return nil, err
	}

	pipelines = append(pipelines, imagePipelines...)

	return pipelines, nil
}

func buildPipeline(repos []rpmmd.RepoConfig, buildPackageSpecs []rpmmd.PackageSpec) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "build"
	p.Runner = "org.osbuild.rhel85"
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(buildPackageSpecs)))
	p.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(true)))
	return p
}

func osPipeline(t *imageType,
	repos []rpmmd.RepoConfig,
	packages []rpmmd.PackageSpec,
	c *blueprint.Customizations,
	options distro.ImageOptions,
	pt *disk.PartitionTable) (*osbuild.Pipeline, error) {
	p := new(osbuild.Pipeline)
	p.Name = "os"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(packages)))
	// If the /boot is on a separate partition, the prefix for the BLS stage must be ""
	if pt == nil || pt.FindMountable("/boot") == nil {
		// NOTE(akoutsou): this is technically not needed when pt == nil, but
		// it was done unconditionally before, so we should keep the image
		// definitions the same
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

	if options.OSTree.Parent != "" && options.OSTree.URL != "" {
		p.AddStage(osbuild2.NewOSTreePasswdStage("org.osbuild.source", options.OSTree.Parent))
	}

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

func containerTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, options distro.ImageOptions, c *blueprint.Customizations, nginxConfigPath, listenPort string) *osbuild.Pipeline {
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

	htmlRoot := "/usr/share/nginx/html"
	repoPath := filepath.Join(htmlRoot, "repo")
	p.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: repoPath}))

	p.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: repoPath},
		osbuild.NewOstreePullStageInputs("org.osbuild.pipeline", "name:ostree-commit", options.OSTree.Ref),
	))

	// make nginx log directory world writeable, otherwise nginx can't start in
	// an unprivileged container
	p.AddStage(osbuild.NewChmodStage(chmodStageOptions("/var/log/nginx", "o+w", true)))

	p.AddStage(osbuild.NewNginxConfigStage(nginxConfigStageOptions(nginxConfigPath, htmlRoot, listenPort)))
	return p
}

func containerPipeline(t *imageType, nginxConfigPath, listenPort string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "container"
	p.Build = "name:build"
	options := &osbuild.OCIArchiveStageOptions{
		Architecture: t.arch.Name(),
		Filename:     t.Filename(),
		Config: &osbuild.OCIArchiveConfig{
			Cmd:          []string{"nginx", "-c", nginxConfigPath},
			ExposedPorts: []string{listenPort},
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

func edgeSimplifiedInstallerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))
	installerPackages := packageSetSpecs[installerPkgsKey]
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(installerPackages, "kernel")
	imgName := "disk.img.xz"
	installDevice := customizations.GetInstallationDevice()

	// create the raw image
	imagePipelines, imgPipelineName, err := edgeImagePipelines(t, imgName, options, rng)
	if err != nil {
		return nil, err
	}

	pipelines = append(pipelines, imagePipelines...)

	// create boot ISO with raw image
	installerTreePipeline := simplifiedInstallerTreePipeline(repos, installerPackages, kernelVer, t.Arch().Name())
	efibootTreePipeline := simplifiedInstallerEFIBootTreePipeline(installDevice, kernelVer, t.Arch().Name())
	bootISOTreePipeline := simplifiedInstallerBootISOTreePipeline(imgPipelineName, kernelVer)

	pipelines = append(pipelines, *installerTreePipeline, *efibootTreePipeline, *bootISOTreePipeline)
	pipelines = append(pipelines, *bootISOPipeline(t.Filename(), t.Arch().Name(), false))

	return pipelines, nil
}

func simplifiedInstallerBootISOTreePipeline(archivePipelineName, kver string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "bootiso-tree"
	p.Build = "name:build"

	p.AddStage(osbuild.NewCopyStageSimple(
		&osbuild.CopyStageOptions{
			Paths: []osbuild.CopyStagePath{
				{
					From: "input://file/disk.img.xz",
					To:   "tree:///disk.img.xz",
				},
			},
		},
		osbuild.NewFilesInputs(osbuild.NewFilesInputReferencesPipeline(archivePipelineName, "disk.img.xz")),
	))

	p.AddStage(osbuild.NewMkdirStage(
		&osbuild.MkdirStageOptions{
			Paths: []osbuild.Path{
				{
					Path: "images",
				},
				{
					Path: "images/pxeboot",
				},
			},
		},
	))

	pt := disk.PartitionTable{
		Size: 20971520,
		Partitions: []disk.Partition{
			{
				Start: 0,
				Size:  20971520,
				Payload: &disk.Filesystem{
					Type:       "vfat",
					Mountpoint: "/",
				},
			},
		},
	}

	filename := "images/efiboot.img"
	loopback := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: filename})
	p.AddStage(osbuild.NewTruncateStage(&osbuild.TruncateStageOptions{Filename: filename, Size: fmt.Sprintf("%d", pt.Size)}))

	for _, stage := range osbuild.GenMkfsStages(&pt, loopback) {
		p.AddStage(stage)
	}

	inputName := "root-tree"
	copyInputs := osbuild.NewCopyStagePipelineTreeInputs(inputName, "efiboot-tree")
	copyOptions, copyDevices, copyMounts := osbuild.GenCopyFSTreeOptions(inputName, "efiboot-tree", filename, &pt)
	p.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	inputName = "coi"
	copyInputs = osbuild.NewCopyStagePipelineTreeInputs(inputName, "coi-tree")
	p.AddStage(osbuild.NewCopyStageSimple(
		&osbuild.CopyStageOptions{
			Paths: []osbuild.CopyStagePath{
				{
					From: fmt.Sprintf("input://%s/boot/vmlinuz-%s", inputName, kver),
					To:   "tree:///images/pxeboot/vmlinuz",
				},
				{
					From: fmt.Sprintf("input://%s/boot/initramfs-%s.img", inputName, kver),
					To:   "tree:///images/pxeboot/initrd.img",
				},
			},
		},
		copyInputs,
	))

	inputName = "efi-tree"
	copyInputs = osbuild.NewCopyStagePipelineTreeInputs(inputName, "efiboot-tree")
	p.AddStage(osbuild.NewCopyStageSimple(
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

	return p
}

func simplifiedInstallerEFIBootTreePipeline(installDevice, kernelVer, arch string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "efiboot-tree"
	p.Build = "name:build"

	isolabel := fmt.Sprintf("RHEL-8-5-0-BaseOS-%s", arch)

	var architectures []string

	if arch == distro.X86_64ArchName {
		architectures = []string{"IA32", "X64"}
	} else if arch == distro.Aarch64ArchName {
		architectures = []string{"AA64"}
	} else {
		panic("unsupported architecture")
	}

	p.AddStage(osbuild.NewGrubISOStage(
		&osbuild.GrubISOStageOptions{
			Product: osbuild.Product{
				Name:    "Red Hat Enterprise Linux",
				Version: osVersion,
			},
			ISOLabel: isolabel,
			Kernel: osbuild.ISOKernel{
				Dir: "/images/pxeboot",
				Opts: []string{"rd.neednet=1",
					"console=tty0",
					"console=ttyS0",
					"systemd.log_target=console",
					"systemd.journald.forward_to_console=1",
					"edge.liveiso=" + isolabel,
					"coreos.inst.install_dev=" + installDevice,
					"coreos.inst.image_file=/run/media/iso/disk.img.xz",
					"coreos.inst.insecure"},
			},
			Architectures: architectures,
			Vendor:        "redhat",
		},
	))

	return p
}

func simplifiedInstallerTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, kernelVer string, arch string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "coi-tree"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(packages)))
	p.AddStage(osbuild.NewBuildstampStage(buildStampStageOptions(arch)))
	p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US.UTF-8"}))
	p.AddStage(osbuild.NewSystemdStage(systemdStageOptions([]string{"coreos-installer"}, nil, nil, "")))
	p.AddStage(osbuild.NewDracutStage(dracutStageOptions(kernelVer, arch, []string{
		"rdcore",
	})))

	return p
}

func ostreeDeployPipeline(
	t *imageType,
	pt *disk.PartitionTable,
	repoPath string,
	kernel *blueprint.KernelCustomization,
	kernelVer string,
	rng *rand.Rand,
	options distro.ImageOptions,
) *osbuild.Pipeline {

	p := new(osbuild.Pipeline)
	p.Name = "image-tree"
	p.Build = "name:build"
	osname := "redhat"

	p.AddStage(osbuild.OSTreeInitFsStage())
	p.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: repoPath},
		osbuild.NewOstreePullStageInputs("org.osbuild.source", options.OSTree.Parent, options.OSTree.Ref),
	))
	p.AddStage(osbuild.NewOSTreeOsInitStage(
		&osbuild.OSTreeOsInitStageOptions{
			OSName: osname,
		},
	))
	p.AddStage(osbuild.NewOSTreeConfigStage(ostreeConfigStageOptions(repoPath, true)))
	p.AddStage(osbuild.NewMkdirStage(efiMkdirStageOptions()))
	kernelOpts := []string{
		"console=tty0",
		"console=ttyS0",
	}
	kernelOpts = append(kernelOpts, osbuild.GenImageKernelOptions(pt)...)
	p.AddStage(osbuild.NewOSTreeDeployStage(
		&osbuild.OSTreeDeployStageOptions{
			OsName: osname,
			Ref:    options.OSTree.Ref,
			Mounts: []string{"/boot", "/boot/efi"},
			Rootfs: osbuild.Rootfs{
				Label: "root",
			},
			KernelOpts: kernelOpts,
		},
	))
	p.AddStage(osbuild.NewOSTreeFillvarStage(
		&osbuild.OSTreeFillvarStageOptions{
			Deployment: osbuild.OSTreeDeployment{
				OSName: osname,
				Ref:    options.OSTree.Ref,
			},
		},
	))

	fstabOptions := osbuild.NewFSTabStageOptions(pt)
	fstabOptions.OSTree = &osbuild.OSTreeFstab{
		Deployment: osbuild.OSTreeDeployment{
			OSName: osname,
			Ref:    options.OSTree.Ref,
		},
	}
	p.AddStage(osbuild2.NewFSTabStage(fstabOptions))

	// TODO: Add users?

	p.AddStage(bootloaderConfigStage(t, pt, kernel, kernelVer, true, true))

	p.AddStage(osbuild.NewOSTreeSelinuxStage(
		&osbuild.OSTreeSelinuxStageOptions{
			Deployment: osbuild.OSTreeDeployment{
				OSName: osname,
				Ref:    options.OSTree.Ref,
			},
		},
	))
	return p
}

func anacondaTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, kernelVer string, arch string, users bool) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "anaconda-tree"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(packages)))
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
	p.AddStage(osbuild.NewDracutStage(dracutStageOptions(kernelVer, arch, []string{
		"anaconda",
	})))

	return p
}

func bootISOTreePipeline(kernelVer string, arch string, ksOptions *osbuild.KickstartStageOptions, payloadStages []*osbuild.Stage) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "bootiso-tree"
	p.Build = "name:build"

	p.AddStage(osbuild.NewBootISOMonoStage(bootISOMonoStageOptions(kernelVer, arch), osbuild.NewBootISOMonoStagePipelineTreeInputs("anaconda-tree")))
	p.AddStage(osbuild.NewKickstartStage(ksOptions))
	p.AddStage(osbuild.NewDiscinfoStage(discinfoStageOptions(arch)))

	for _, stage := range payloadStages {
		p.AddStage(stage)
	}

	return p
}
func bootISOPipeline(filename string, arch string, isolinux bool) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "bootiso"
	p.Build = "name:build"

	p.AddStage(osbuild.NewXorrisofsStage(xorrisofsStageOptions(filename, arch, isolinux), osbuild.NewXorrisofsStagePipelineTreeInputs("bootiso-tree")))
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

	for _, stage := range osbuild.GenImageFinishStages(pt, outputFilename) {
		p.AddStage(stage)
	}

	loopback := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: outputFilename})
	p.AddStage(bootloaderInstStage(outputFilename, pt, arch, kernelVer, copyDevices, copyMounts, loopback))
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

func bootloaderConfigStage(t *imageType, partitionTable *disk.PartitionTable, kernel *blueprint.KernelCustomization, kernelVer string, install, greenboot bool) *osbuild.Stage {
	if t.Arch().Name() == distro.S390xArchName {
		return osbuild.NewZiplStage(new(osbuild.ZiplStageOptions))
	}

	kernelOptions := t.kernelOptions
	uefi := t.supportsUEFI()
	legacy := t.arch.legacy

	options := osbuild.NewGrub2StageOptions(partitionTable, kernelOptions, kernel, kernelVer, uefi, legacy, "redhat", install)
	options.Greenboot = greenboot

	// `NewGrub2StageOptions` now might set this to "saved"; but when 8.5 was released
	// it was not available. To keep manifests from changing we need to reset it here.
	if options.Config != nil {
		options.Config.Default = ""
	}

	// before unifying the org.osbuild.grub2 stage option generator, we didn't
	// set the following for RHEL 8.5, so we need to revert here to maintain
	// the old behaviour
	if uefi {
		options.UEFI.Unified = false
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
