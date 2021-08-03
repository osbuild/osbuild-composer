package rhel85

import (
	"fmt"
	"math/rand"
	"os"
	"strings"

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
	treePipeline, err := osPipeline(repos, packageSetSpecs[osPkgsKey], packageSetSpecs[blueprintPkgsKey], customizations, options, t.enabledServices, t.disabledServices, t.defaultTarget)
	if err != nil {
		return nil, err
	}

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
	partitionTable := defaultPartitionTable(options, t.arch, rng)
	treePipeline.AddStage(osbuild.NewFSTabStage(partitionTable.FSTabStageOptionsV2()))
	treePipeline.AddStage(osbuild.NewGRUB2Stage(grub2StageOptions(&partitionTable, t.kernelOptions, customizations.GetKernel(), packageSetSpecs[blueprintPkgsKey], t.supportsUEFI(), t.arch.legacy)))
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, &partitionTable, t.arch.legacy)
	pipelines = append(pipelines, *imagePipeline)

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, "qcow2", "0.10")
	pipelines = append(pipelines, *qemuPipeline)

	return pipelines, nil
}

func vhdPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))
	treePipeline, err := osPipeline(repos, packageSetSpecs[osPkgsKey], packageSetSpecs[blueprintPkgsKey], customizations, options, t.enabledServices, t.disabledServices, t.defaultTarget)
	if err != nil {
		return nil, err
	}

	partitionTable := defaultPartitionTable(options, t.arch, rng)
	treePipeline.AddStage(osbuild.NewFSTabStage(partitionTable.FSTabStageOptionsV2()))
	treePipeline.AddStage(osbuild.NewGRUB2Stage(grub2StageOptions(&partitionTable, t.kernelOptions, customizations.GetKernel(), packageSetSpecs[blueprintPkgsKey], t.supportsUEFI(), t.arch.legacy)))
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, &partitionTable, t.arch.legacy)
	pipelines = append(pipelines, *imagePipeline)
	if err != nil {
		return nil, err
	}

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, "vpc", "")
	pipelines = append(pipelines, *qemuPipeline)
	return pipelines, nil
}

func vmdkPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))
	treePipeline, err := osPipeline(repos, packageSetSpecs[osPkgsKey], packageSetSpecs[blueprintPkgsKey], customizations, options, t.enabledServices, t.disabledServices, t.defaultTarget)
	if err != nil {
		return nil, err
	}

	partitionTable := defaultPartitionTable(options, t.arch, rng)
	treePipeline.AddStage(osbuild.NewFSTabStage(partitionTable.FSTabStageOptionsV2()))
	treePipeline.AddStage(osbuild.NewGRUB2Stage(grub2StageOptions(&partitionTable, t.kernelOptions, customizations.GetKernel(), packageSetSpecs[blueprintPkgsKey], t.supportsUEFI(), t.arch.legacy)))
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, &partitionTable, t.arch.legacy)
	pipelines = append(pipelines, *imagePipeline)
	if err != nil {
		return nil, err
	}

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, "vmdk", "")
	pipelines = append(pipelines, *qemuPipeline)
	return pipelines, nil
}

func openstackPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))
	treePipeline, err := osPipeline(repos, packageSetSpecs[osPkgsKey], packageSetSpecs[blueprintPkgsKey], customizations, options, t.enabledServices, t.disabledServices, t.defaultTarget)
	if err != nil {
		return nil, err
	}

	partitionTable := defaultPartitionTable(options, t.arch, rng)
	treePipeline.AddStage(osbuild.NewFSTabStage(partitionTable.FSTabStageOptionsV2()))
	treePipeline.AddStage(osbuild.NewGRUB2Stage(grub2StageOptions(&partitionTable, t.kernelOptions, customizations.GetKernel(), packageSetSpecs[blueprintPkgsKey], t.supportsUEFI(), t.arch.legacy)))
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, &partitionTable, t.arch.legacy)
	pipelines = append(pipelines, *imagePipeline)
	if err != nil {
		return nil, err
	}

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, "qcow2", "")
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
func ec2BaseTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, bpPackages []rpmmd.PackageSpec,
	c *blueprint.Customizations, options distro.ImageOptions, enabledServices, disabledServices []string,
	defaultTarget string, withRHUI bool) (*osbuild.Pipeline, error) {
	p := new(osbuild.Pipeline)
	p.Name = "os"
	p.Build = "name:build"
	packages = append(packages, bpPackages...)
	p.AddStage(osbuild.NewRPMStage(rpmStageOptions(repos), rpmStageInputs(packages)))
	p.AddStage(osbuild.NewFixBLSStage())

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
		p.AddStage(osbuild.NewGroupsStage(groupStageOptions(groups)))
	}

	if users := c.GetUsers(); len(users) > 0 {
		userOptions, err := userStageOptions(users)
		if err != nil {
			return nil, err
		}
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
		Kernel: osbuild.SysconfigKernelOptions{
			UpdateDefault: true,
			DefaultKernel: "kernel",
		},
		Network: osbuild.SysconfigNetworkOptions{
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
			fmt.Sprintf("/usr/sbin/subscription-manager register --org=%d --activationkey=%s --serverurl %s --baseurl %s", options.Subscription.Organization, options.Subscription.ActivationKey, options.Subscription.ServerUrl, options.Subscription.BaseUrl),
		}
		if options.Subscription.Insights {
			commands = append(commands, "/usr/bin/insights-client --register")
		}

		p.AddStage(osbuild.NewFirstBootStage(&osbuild.FirstBootStageOptions{
			Commands:       commands,
			WaitForNetwork: true,
		}))
	} else {
		rhsmStageOptions := &osbuild.RHSMStageOptions{
			DnfPlugins: &osbuild.RHSMStageOptionsDnfPlugins{
				ProductID: &osbuild.RHSMStageOptionsDnfPlugin{
					Enabled: false,
				},
				SubscriptionManager: &osbuild.RHSMStageOptionsDnfPlugin{
					Enabled: false,
				},
			},
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

func ec2X86_64BaseTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, bpPackages []rpmmd.PackageSpec,
	c *blueprint.Customizations, options distro.ImageOptions, enabledServices, disabledServices []string,
	defaultTarget string, withRHUI bool) (*osbuild.Pipeline, error) {

	treePipeline, err := ec2BaseTreePipeline(repos, packages, bpPackages, c, options, enabledServices, disabledServices, defaultTarget, withRHUI)
	if err != nil {
		return nil, err
	}

	// EC2 x86_64-specific stages
	treePipeline.AddStage(osbuild.NewDracutConfStage(&osbuild.DracutConfStageOptions{
		Filename: "xen.conf",
		Config: osbuild.DracutConfigFile{
			AddDrivers: []string{
				"xen-netfront",
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

	var treePipeline *osbuild.Pipeline
	var err error
	switch arch := t.arch.Name(); arch {
	// rhel-ec2-x86_64, rhel-ha-ec2
	case x86_64ArchName:
		treePipeline, err = ec2X86_64BaseTreePipeline(repos, packageSetSpecs[osPkgsKey], packageSetSpecs[blueprintPkgsKey], customizations, options, t.enabledServices, t.disabledServices, t.defaultTarget, withRHUI)
	// rhel-ec2-aarch64
	case aarch64ArchName:
		treePipeline, err = ec2BaseTreePipeline(repos, packageSetSpecs[osPkgsKey], packageSetSpecs[blueprintPkgsKey], customizations, options, t.enabledServices, t.disabledServices, t.defaultTarget, withRHUI)
	default:
		return nil, fmt.Errorf("ec2CommonPipelines: unsupported image architecture: %q", arch)
	}
	if err != nil {
		return nil, err
	}
	partitionTable := ec2PartitionTable(options, t.arch, rng)
	treePipeline.AddStage(osbuild.NewFSTabStage(partitionTable.FSTabStageOptionsV2()))
	treePipeline.AddStage(osbuild.NewGRUB2Stage(grub2StageOptions(&partitionTable, t.kernelOptions, customizations.GetKernel(), packageSetSpecs[blueprintPkgsKey], t.supportsUEFI(), t.arch.legacy)))
	// The last stage must be the SELinux stage
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, &partitionTable, t.arch.legacy)
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

func tarPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	treePipeline, err := osPipeline(repos, packageSetSpecs[osPkgsKey], packageSetSpecs[blueprintPkgsKey], customizations, options, t.enabledServices, t.disabledServices, t.defaultTarget)
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
	kernelPkg := new(rpmmd.PackageSpec)
	installerPackages := packageSetSpecs[installerPkgsKey]
	for _, pkg := range installerPackages {
		if pkg.Name == "kernel" {
			kernelPkg = &pkg
			break
		}
	}
	if kernelPkg == nil {
		return nil, fmt.Errorf("kernel package not found in installer package set")
	}
	kernelVer := fmt.Sprintf("%s-%s.%s", kernelPkg.Version, kernelPkg.Release, kernelPkg.Arch)
	ostreeRepoPath := "/ostree/repo"
	pipelines = append(pipelines, *anacondaTreePipeline(repos, installerPackages, kernelVer, t.Arch().Name(), ostreePayloadStages(options, ostreeRepoPath)))
	pipelines = append(pipelines, *bootISOTreePipeline(kernelVer, t.Arch().Name(), ostreeKickstartStageOptions(fmt.Sprintf("file://%s", ostreeRepoPath), options.OSTree.Ref)))
	pipelines = append(pipelines, *bootISOPipeline(t.Filename(), t.Arch().Name()))
	return pipelines, nil
}

func tarInstallerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	treePipeline, err := osPipeline(repos, packageSetSpecs[osPkgsKey], packageSetSpecs[blueprintPkgsKey], customizations, options, t.enabledServices, t.disabledServices, t.defaultTarget)
	if err != nil {
		return nil, err
	}
	treePipeline.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))
	pipelines = append(pipelines, *treePipeline)

	kernelPkg := new(rpmmd.PackageSpec)
	installerPackages := packageSetSpecs[installerPkgsKey]
	for _, pkg := range installerPackages {
		if pkg.Name == "kernel" {
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
	pipelines = append(pipelines, *anacondaTreePipeline(repos, installerPackages, kernelVer, t.Arch().Name(), tarPayloadStages))
	pipelines = append(pipelines, *bootISOTreePipeline(kernelVer, t.Arch().Name(), tarKickstartStageOptions(fmt.Sprintf("file://%s", tarPath))))
	pipelines = append(pipelines, *bootISOPipeline(t.Filename(), t.Arch().Name()))
	return pipelines, nil
}

func edgeCorePipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))

	treePipeline, err := ostreeTreePipeline(repos, packageSetSpecs[osPkgsKey], packageSetSpecs[blueprintPkgsKey], customizations, options, t.enabledServices, t.disabledServices, t.defaultTarget)
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
	p.Runner = "org.osbuild.rhel85"
	p.AddStage(osbuild.NewRPMStage(rpmStageOptions(repos), rpmStageInputs(buildPackageSpecs)))
	p.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(true)))
	return p
}

func osPipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, bpPackages []rpmmd.PackageSpec, c *blueprint.Customizations, options distro.ImageOptions, enabledServices, disabledServices []string, defaultTarget string) (*osbuild.Pipeline, error) {
	p := new(osbuild.Pipeline)
	p.Name = "os"
	p.Build = "name:build"
	packages = append(packages, bpPackages...)
	p.AddStage(osbuild.NewRPMStage(rpmStageOptions(repos), rpmStageInputs(packages)))
	p.AddStage(osbuild.NewFixBLSStage())
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
		p.AddStage(osbuild.NewGroupsStage(groupStageOptions(groups)))
	}

	if users := c.GetUsers(); len(users) > 0 {
		userOptions, err := userStageOptions(users)
		if err != nil {
			return nil, err
		}
		p.AddStage(osbuild.NewUsersStage(userOptions))
	}

	if services := c.GetServices(); services != nil || enabledServices != nil || disabledServices != nil || defaultTarget != "" {
		p.AddStage(osbuild.NewSystemdStage(systemdStageOptions(enabledServices, disabledServices, services, defaultTarget)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(firewallStageOptions(firewall)))
	}

	// These are the current defaults for the sysconfig stage. This can be changed to be image type exclusive if different configs are needed.
	p.AddStage(osbuild.NewSysconfigStage(&osbuild.SysconfigStageOptions{
		Kernel: osbuild.SysconfigKernelOptions{
			UpdateDefault: true,
			DefaultKernel: "kernel",
		},
		Network: osbuild.SysconfigNetworkOptions{
			Networking: true,
			NoZeroConf: true,
		},
	}))

	if options.Subscription != nil {
		commands := []string{
			fmt.Sprintf("/usr/sbin/subscription-manager register --org=%d --activationkey=%s --serverurl %s --baseurl %s", options.Subscription.Organization, options.Subscription.ActivationKey, options.Subscription.ServerUrl, options.Subscription.BaseUrl),
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

func ostreeTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, bpPackages []rpmmd.PackageSpec, c *blueprint.Customizations, options distro.ImageOptions, enabledServices, disabledServices []string, defaultTarget string) (*osbuild.Pipeline, error) {
	p := new(osbuild.Pipeline)
	p.Name = "ostree-tree"
	p.Build = "name:build"

	packages = append(packages, bpPackages...)
	p.AddStage(osbuild.NewRPMStage(rpmStageOptions(repos), rpmStageInputs(packages)))
	p.AddStage(osbuild.NewFixBLSStage())
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
		p.AddStage(osbuild.NewGroupsStage(groupStageOptions(groups)))
	}

	if users := c.GetUsers(); len(users) > 0 {
		userOptions, err := userStageOptions(users)
		if err != nil {
			return nil, err
		}
		p.AddStage(osbuild.NewUsersStage(userOptions))
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
		Kernel: osbuild.SysconfigKernelOptions{
			UpdateDefault: true,
			DefaultKernel: "kernel",
		},
		Network: osbuild.SysconfigNetworkOptions{
			Networking: true,
			NoZeroConf: true,
		},
	}))

	if options.Subscription != nil {
		commands := []string{
			fmt.Sprintf("/usr/sbin/subscription-manager register --org=%d --activationkey=%s --serverurl %s --baseurl %s", options.Subscription.Organization, options.Subscription.ActivationKey, options.Subscription.ServerUrl, options.Subscription.BaseUrl),
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
	p.AddStage(osbuild.NewRPMStage(rpmStageOptions(repos), rpmStageInputs(packages)))
	language, _ := c.GetPrimaryLocale()
	if language != nil {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: *language}))
	} else {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US"}))
	}
	p.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: "/var/www/html/repo"}))

	p.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: "/var/www/html/repo"},
		ostreePullStageInputs("org.osbuild.pipeline", "name:ostree-commit", options.OSTree.Ref),
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
		ostreePullStageInputs("org.osbuild.source", options.OSTree.Parent, options.OSTree.Ref),
	))

	return stages
}

func edgeSimplifiedInstallerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey]))
	kernelPkg := new(rpmmd.PackageSpec)
	installerPackages := packageSetSpecs[installerPkgsKey]
	for _, pkg := range installerPackages {
		if pkg.Name == "kernel" {
			kernelPkg = &pkg
			break
		}
	}
	if kernelPkg == nil {
		return nil, fmt.Errorf("kernel package not found in installer package set")
	}
	imgName := "disk.img"
	imgNameCompressed := "disk.img.xz"
	ostreeRepoPath := "/ostree/repo"

	if options.Size == 0 {
		options.Size = 10737418240
	}
	partitionTable := defaultPartitionTableEdge(options, t.arch, rng)
	kernelVer := fmt.Sprintf("%s-%s.%s", kernelPkg.Version, kernelPkg.Release, kernelPkg.Arch)

	imageTreePipeline := *simplifiedInstallerImageTreePipeline(&partitionTable, kernelVer, customizations.GetKernel(), t.arch, t.supportsUEFI(), t.kernelOptions, rng, options, ostreePayloadStages(options, ostreeRepoPath))
	pipelines = append(pipelines, imageTreePipeline)
	imagePipeline := *simplifiedInstallerImagePipeline(imgName, imageTreePipeline.Name, &partitionTable, t.arch)
	pipelines = append(pipelines, imagePipeline)
	xzArchivePipeline := *xzArchivePipeline(imagePipeline.Name, imgName, imgNameCompressed)
	pipelines = append(pipelines, xzArchivePipeline)
	installerTreePipeline := *simplifiedInstallerTreePipeline(repos, installerPackages, kernelVer, t.Arch().Name())
	pipelines = append(pipelines, installerTreePipeline)
	//	efibootTreePipeline := *simplifiedInstallerEFIBootTreePipeline(options.InstallationDevice, kernelVer, t.arch.name)
	efibootTreePipeline := *simplifiedInstallerEFIBootTreePipeline("/dev/vda", kernelVer, t.Arch().Name())
	pipelines = append(pipelines, efibootTreePipeline)
	bootISOTreePipeline := simplifiedInstallerBootISOTreePipeline(xzArchivePipeline.Name, kernelVer, t.Arch().Name())
	pipelines = append(pipelines, *bootISOTreePipeline)
	pipelines = append(pipelines, *bootISOPipeline(t.Filename(), t.Arch().Name()))

	return pipelines, nil
}

func simplifiedInstallerBootISOTreePipeline(archivePipelineName, kver, arch string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "bootiso-tree"
	p.Build = "name:build"

	p.AddStage(osbuild.NewCopyStageFiles(
		&osbuild.CopyStageOptions{
			Paths: []osbuild.CopyStagePath{
				{
					From: "input://file/disk.img.xz",
					To:   "tree:///disk.img.xz",
				},
			},
		},
		osbuild.NewFilesInputs(osbuild.NewFilesInputReferencesPipeline("archive", "disk.img.xz")),
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
	loopback := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: "images/efiboot.img"})
	p.AddStage(osbuild.NewTruncateStage(&osbuild.TruncateStageOptions{Filename: "images/efiboot.img", Size: "20MB"}))

	pt := disk.PartitionTable{
		Size: 2500000,
		Partitions: []disk.Partition{
			{
				Start: 0,
				Size:  2500000,
				Filesystem: &disk.Filesystem{
					Type:       "vfat",
					Mountpoint: "/",
				},
			},
		},
	}

	devOptions, ok := loopback.Options.(*osbuild.LoopbackDeviceOptions)
	if !ok {
		panic("mkfsStages: failed to convert device options to loopback options")
	}
	stageDevice := osbuild.NewLoopbackDevice(
		&osbuild.LoopbackDeviceOptions{
			Filename: devOptions.Filename,
			Start:    0,
			Size:     25000000,
		},
	)
	options := &osbuild.MkfsFATStageOptions{
		VolID: "7B7795E7",
		Label: "COI",
	}
	devices := &osbuild.MkfsFATStageDevices{Device: *stageDevice}
	stage := osbuild.NewMkfsFATStage(options, devices)
	p.AddStage(stage)

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := copyFSTreeOptions(inputName, "efiboot-tree", &pt, loopback)
	copyInputs := copyPipelineTreeInputs(inputName, "efiboot-tree")
	p.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	copyInputs = copyPipelineTreeInputs(inputName, "coi-tree")
	p.AddStage(osbuild.NewCopyStage(
		&osbuild.CopyStageOptions{
			Paths: []osbuild.CopyStagePath{
				{
					From: fmt.Sprintf("input://root-tree/boot/vmlinuz-%s.el8.%s", kver, arch),
					To:   "tree:///images/pxeboot/vmlinuz",
				},
				{
					From: fmt.Sprintf("input://root-tree/boot/initramfs-%s.el8.%s.img", kver, arch),
					To:   "tree:///images/pxeboot/initrd.img",
				},
			},
		},
		copyInputs,
		nil,
		nil,
	))

	copyInputs = copyPipelineTreeInputs(inputName, "efiboot-tree")
	p.AddStage(osbuild.NewCopyStage(
		&osbuild.CopyStageOptions{
			Paths: []osbuild.CopyStagePath{
				{
					From: "input://root-tree/EFI",
					To:   "tree:///",
				},
			},
		},
		copyInputs,
		nil,
		nil,
	))

	return p
}

func simplifiedInstallerEFIBootTreePipeline(installDevice, kernelVer, arch string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "efiboot-tree"
	p.Build = "name:build"

	isolabel := fmt.Sprintf("RHEL-8-5-0-BaseOS-%s", arch)
	p.AddStage(osbuild.NewGrubISOStage(
		&osbuild.GrubISOStageOptions{
			Product: osbuild.Product{
				Name:    "Red Hat Enterprise Linux",
				Version: osVersion,
			},
			ISOLabel:   isolabel,
			Kernel:     kernelVer,
			KernelOpts: fmt.Sprintf("rd.neednet=1 console=tty0 console=ttyS0 edge.liveiso=%s coreos.inst.install_dev=%s coreos.inst.image_file=/run/media/iso/disk.img.xz coreos.inst.insecure", isolabel, installDevice),
			EFI: osbuild.EFI{
				Architectures: []string{
					"IA32",
					"X64",
				},
				Vendor: "redhat",
			},
		},
	))

	return p
}

func simplifiedInstallerTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, kernelVer string, arch string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "coi-tree"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(rpmStageOptions(repos), rpmStageInputs(packages)))
	p.AddStage(osbuild.NewBuildstampStage(buildStampStageOptions(arch)))
	p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US.UTF-8"}))
	p.AddStage(osbuild.NewSystemdStage(systemdStageOptions([]string{"coreos-installer"}, nil, nil, "")))
	p.AddStage(osbuild.NewDracutStage(dracutStageOptions(kernelVer, []string{
		"qemu",
		"qemu-net",
		"fs-lib",
		"network",
		"rdcore",
		"url-lib",
		"crypt-gpg",
		"pollcdrom",
		"dmsquash-live",
	})))

	return p
}

func simplifiedInstallerImagePipeline(outputFilename, inputPipelineName string, pt *disk.PartitionTable, arch *architecture) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "image"
	p.Build = "name:build"

	loopback := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: outputFilename})
	p.AddStage(osbuild.NewTruncateStage(&osbuild.TruncateStageOptions{Filename: outputFilename, Size: fmt.Sprintf("%d", pt.Size)}))
	sfOptions, sfDevices := sfdiskStageOptions(pt, loopback)
	p.AddStage(osbuild.NewSfdiskStage(sfOptions, sfDevices))

	for _, stage := range mkfsStages(pt, loopback) {
		p.AddStage(stage)
	}

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := copyFSTreeOptions(inputName, inputPipelineName, pt, loopback)
	copyInputs := copyPipelineTreeInputs(inputName, inputPipelineName)
	p.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	if arch.legacy != "" {
		p.AddStage(osbuild.NewGrub2InstStage(grub2InstStageOptions(outputFilename, pt, arch.legacy)))
	}

	return p
}

func simplifiedInstallerImageTreePipeline(pt *disk.PartitionTable, kernelVer string, kernel *blueprint.KernelCustomization, arch *architecture, uefi bool, kernelOptions string, rng *rand.Rand, options distro.ImageOptions, payloadStages []*osbuild.Stage) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "image-tree"
	p.Build = "name:build"
	repo := "/ostree/repo"
	osname := "redhat"

	for _, stage := range payloadStages {
		p.AddStage(stage)
	}

	p.AddStage(osbuild.OSTreeInitFsStage())
	p.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: repo},
		ostreePullStageInputs("org.osbuild.source", options.OSTree.Parent, options.OSTree.Ref),
	))
	p.AddStage(osbuild.NewOSTreeOsInitStage(
		&osbuild.OSTreeOsInitStageOptions{
			OsName: osname,
		},
	))
	p.AddStage(osbuild.NewOSTreeConfigStage(
		&osbuild.OSTreeConfigStageOptions{
			Repo: repo,
			Config: osbuild.OstreeConfigOptions{
				Sysroot: osbuild.SysrootOptions{
					ReadOnly: true,
				},
			},
		},
	))
	p.AddStage(osbuild.NewMkdirStage(
		&osbuild.MkdirStageOptions{
			Paths: []osbuild.Path{
				{
					Path: "/boot/efi",
					Mode: os.FileMode(448),
				},
			},
		},
	))
	p.AddStage(osbuild.NewOSTreeDeployStage(
		&osbuild.OSTreeDeployStageOptions{
			OsName: osname,
			Ref:    options.OSTree.Ref,
			Mounts: []string{"/boot", "/boot/efi"},
			Rootfs: osbuild.Rootfs{
				Label: "root",
			},
			KernelOpts: []string{
				"console=tty0",
				"console=ttyS0",
				"systemd.log_target=console",
				"systemd.journald.forward_to_console=1",
			},
		},
	))
	p.AddStage(osbuild.NewOSTreeFillvarStage(
		&osbuild.OSTreeFillvarStageOptions{
			Deployment: osbuild.Deployment{
				OsName: osname,
				Ref:    options.OSTree.Ref,
			},
		},
	))
	
	fstabOptions := pt.FSTabStageOptionsV2()
	fstabOptions.Deployment = &osbuild.Deployment{
				OsName: osname,
				Ref:    options.OSTree.Ref,
			}
	p.AddStage(osbuild2.NewFSTabStage(fstabOptions))

	p.AddStage(osbuild.NewOSTreeSelinuxStage(
		&osbuild.OSTreeSelinuxStageOptions{
			Deployment: osbuild.Deployment{
				OsName: osname,
				Ref:    options.OSTree.Ref,
			},
		},
	))
	grub2Options := grub2StageOptions(
		pt,
		kernelOptions,
		nil,
		nil,
		true,
		arch.legacy,
	)
	grub2Options.UEFI.Install = common.BoolToPtr(true)
	p.AddStage(osbuild.NewGRUB2Stage(grub2Options))
	return p
}

func anacondaTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, kernelVer string, arch string, payloadStages []*osbuild.Stage) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "anaconda-tree"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(rpmStageOptions(repos), rpmStageInputs(packages)))
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
	p.AddStage(osbuild.NewAnacondaStage(anacondaStageOptions()))
	p.AddStage(osbuild.NewLoraxScriptStage(loraxScriptStageOptions(arch)))
	p.AddStage(osbuild.NewDracutStage(dracutStageOptions(kernelVer, nil)))

	return p
}

func bootISOTreePipeline(kernelVer string, arch string, ksOptions *osbuild.KickstartStageOptions) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "bootiso-tree"
	p.Build = "name:build"

	p.AddStage(osbuild.NewBootISOMonoStage(bootISOMonoStageOptions(kernelVer, arch), bootISOMonoStageInputs()))
	p.AddStage(osbuild.NewKickstartStage(ksOptions))
	p.AddStage(osbuild.NewDiscinfoStage(discinfoStageOptions(arch)))

	return p
}
func bootISOPipeline(filename string, arch string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "bootiso"
	p.Build = "name:build"

	p.AddStage(osbuild.NewXorrisofsStage(xorrisofsStageOptions(filename, arch), xorrisofsStageInputs()))
	p.AddStage(osbuild.NewImplantisomd5Stage(&osbuild.Implantisomd5StageOptions{Filename: filename}))

	return p
}

func liveImagePipeline(inputPipelineName string, outputFilename string, pt *disk.PartitionTable, platform string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "image"
	p.Build = "name:build"

	loopback := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: outputFilename})
	p.AddStage(osbuild.NewTruncateStage(&osbuild.TruncateStageOptions{Filename: outputFilename, Size: fmt.Sprintf("%d", pt.Size)}))
	sfOptions, sfDevices := sfdiskStageOptions(pt, loopback)
	p.AddStage(osbuild.NewSfdiskStage(sfOptions, sfDevices))

	for _, stage := range mkfsStages(pt, loopback) {
		p.AddStage(stage)
	}

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := copyFSTreeOptions(inputName, inputPipelineName, pt, loopback)
	copyInputs := copyPipelineTreeInputs(inputName, inputPipelineName)
	p.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	if platform != "" {
		p.AddStage(osbuild.NewGrub2InstStage(grub2InstStageOptions(outputFilename, pt, platform)))
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

// mkfsStages generates a list of org.osbuild.mkfs.* stages based on a
// partition table description for a single device node
func mkfsStages(pt *disk.PartitionTable, device *osbuild.Device) []*osbuild2.Stage {
	stages := make([]*osbuild2.Stage, 0, len(pt.Partitions))

	// assume loopback device for simplicity since it's the only one currently supported
	// panic if the conversion fails
	devOptions, ok := device.Options.(*osbuild.LoopbackDeviceOptions)
	if !ok {
		panic("mkfsStages: failed to convert device options to loopback options")
	}

	for _, p := range pt.Partitions {
		if p.Filesystem == nil {
			// no filesystem for partition (e.g., BIOS boot)
			continue
		}
		var stage *osbuild.Stage
		stageDevice := osbuild.NewLoopbackDevice(
			&osbuild.LoopbackDeviceOptions{
				Filename: devOptions.Filename,
				Start:    p.Start,
				Size:     p.Size,
			},
		)
		switch p.Filesystem.Type {
		case "xfs":
			options := &osbuild.MkfsXfsStageOptions{
				UUID:  p.Filesystem.UUID,
				Label: p.Filesystem.Label,
			}
			devices := &osbuild.MkfsXfsStageDevices{Device: *stageDevice}
			stage = osbuild.NewMkfsXfsStage(options, devices)
		case "vfat":
			options := &osbuild.MkfsFATStageOptions{
				VolID: strings.Replace(p.Filesystem.UUID, "-", "", -1),
			}
			devices := &osbuild.MkfsFATStageDevices{Device: *stageDevice}
			stage = osbuild.NewMkfsFATStage(options, devices)
		case "btrfs":
			options := &osbuild.MkfsBtrfsStageOptions{
				UUID:  p.Filesystem.UUID,
				Label: p.Filesystem.Label,
			}
			devices := &osbuild.MkfsBtrfsStageDevices{Device: *stageDevice}
			stage = osbuild.NewMkfsBtrfsStage(options, devices)
		case "ext4":
			options := &osbuild.MkfsExt4StageOptions{
				UUID:  p.Filesystem.UUID,
				Label: p.Filesystem.Label,
			}
			devices := &osbuild.MkfsExt4StageDevices{Device: *stageDevice}
			stage = osbuild.NewMkfsExt4Stage(options, devices)
		default:
			panic("unknown fs type " + p.Type)
		}
		stages = append(stages, stage)
	}
	return stages
}

func qemuPipeline(inputPipelineName, inputFilename, outputFilename, format, qcow2Compat string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = format
	p.Build = "name:build"

	qemuStage := osbuild.NewQEMUStage(qemuStageOptions(outputFilename, format, qcow2Compat), qemuStageInputs(inputPipelineName, inputFilename))
	p.AddStage(qemuStage)
	return p
}
