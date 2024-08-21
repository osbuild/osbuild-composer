package manifest

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/bootc"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/customizations/shell"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rhsm/facts"
	"github.com/osbuild/images/pkg/rpmmd"
)

// OSCustomizations encapsulates all configuration applied to the base
// operating system independently of where and how it is integrated and what
// workload it is running.
// TODO: move out kernel/bootloader/cloud-init/... to other
//
//	abstractions, this should ideally only contain things that
//	can always be applied.
type OSCustomizations struct {

	// Packages to install in addition to the ones required by the
	// pipeline.
	ExtraBasePackages []string

	// Packages to exclude from the base package set. This is useful in
	// case of weak dependencies, comps groups, or where multiple packages
	// can satisfy a dependency. Must not conflict with the included base
	// package set.
	ExcludeBasePackages []string

	// Additional repos to install the base packages from.
	ExtraBaseRepos []rpmmd.RepoConfig

	// Containers to embed in the image (source specification)
	// TODO: move to workload
	Containers []container.SourceSpec

	// KernelName indicates that a kernel is installed, and names the kernel
	// package.
	KernelName string

	// KernelOptionsAppend are appended to the kernel commandline
	KernelOptionsAppend []string

	// KernelOptionsBootloader controls whether kernel command line options
	// should be specified in the bootloader grubenv configuration. Otherwise
	// they are specified in /etc/kernel/cmdline (default).
	//
	// NB: The kernel options need to be still specified in /etc/default/grub
	// under the GRUB_CMDLINE_LINUX variable. The reason is that it is used by
	// the 10_linux script executed by grub2-mkconfig to override the kernel
	// options in /etc/kernel/cmdline if the file has older timestamp than
	// /etc/default/grub.
	//
	// This should only be used for RHEL 8 and CentOS 8 images that use grub
	// (non s390x).  Newer releases (9+) should keep this disabled.
	KernelOptionsBootloader bool

	GPGKeyFiles      []string
	Language         string
	Keyboard         *string
	X11KeymapLayouts []string
	Hostname         string
	Timezone         string
	EnabledServices  []string
	DisabledServices []string
	MaskedServices   []string
	DefaultTarget    string

	// SELinux policy, when set it enables the labeling of the tree with the
	// selected profile
	SElinux string

	SELinuxForceRelabel *bool

	// Do not install documentation
	ExcludeDocs bool

	Groups []users.Group
	Users  []users.User

	ShellInit []shell.InitFile

	// TODO: drop osbuild types from the API
	Firewall            *osbuild.FirewallStageOptions
	Grub2Config         *osbuild.GRUB2Config
	Sysconfig           []*osbuild.SysconfigStageOptions
	SystemdLogind       []*osbuild.SystemdLogindStageOptions
	CloudInit           []*osbuild.CloudInitStageOptions
	Modprobe            []*osbuild.ModprobeStageOptions
	DracutConf          []*osbuild.DracutConfStageOptions
	SystemdUnit         []*osbuild.SystemdUnitStageOptions
	Authselect          *osbuild.AuthselectStageOptions
	SELinuxConfig       *osbuild.SELinuxConfigStageOptions
	Tuned               *osbuild.TunedStageOptions
	Tmpfilesd           []*osbuild.TmpfilesdStageOptions
	PamLimitsConf       []*osbuild.PamLimitsConfStageOptions
	Sysctld             []*osbuild.SysctldStageOptions
	DNFConfig           []*osbuild.DNFConfigStageOptions
	DNFAutomaticConfig  *osbuild.DNFAutomaticConfigStageOptions
	YUMConfig           *osbuild.YumConfigStageOptions
	YUMRepos            []*osbuild.YumReposStageOptions
	SshdConfig          *osbuild.SshdConfigStageOptions
	GCPGuestAgentConfig *osbuild.GcpGuestAgentConfigOptions
	AuthConfig          *osbuild.AuthconfigStageOptions
	PwQuality           *osbuild.PwqualityConfStageOptions
	NTPServers          []osbuild.ChronyConfigServer
	WAAgentConfig       *osbuild.WAAgentConfStageOptions
	UdevRules           *osbuild.UdevRulesStageOptions
	WSLConfig           *osbuild.WSLConfStageOptions
	LeapSecTZ           *string
	FactAPIType         *facts.APIType
	Presets             []osbuild.Preset
	ContainersStorage   *string

	// OpenSCAP config
	OpenSCAPRemediationConfig *oscap.RemediationConfig

	Subscription *subscription.ImageOptions
	// The final RHSM config to be applied to the image
	RHSMConfig *subscription.RHSMConfig

	// Custom directories and files to create in the image
	Directories []*fsnode.Directory
	Files       []*fsnode.File

	FIPS bool

	// NoBLS configures the image bootloader with traditional menu entries
	// instead of BLS. Required for legacy systems like RHEL 7.
	NoBLS bool
}

// OS represents the filesystem tree of the target image. This roughly
// corresponds to the root filesystem once an instance of the image is running.
type OS struct {
	Base
	// Customizations to apply to the base OS
	OSCustomizations
	// Environment the system will run in
	Environment environment.Environment
	// Workload to install on top of the base system
	Workload workload.Workload
	// Ref of ostree commit (optional). If empty the tree cannot be in an ostree commit
	OSTreeRef string
	// OSTreeParent source spec (optional). If nil the new commit (if
	// applicable) will have no parent
	OSTreeParent *ostree.SourceSpec

	// Enabling Bootupd runs bootupctl generate-update-metadata in the tree to
	// transform /usr/lib/ostree-boot into a bootupd-compatible update
	// payload. Only works with ostree-based images.
	Bootupd bool

	// Add a bootc config file to the image (for bootable containers)
	BootcConfig *bootc.Config

	// Partition table, if nil the tree cannot be put on a partitioned disk
	PartitionTable *disk.PartitionTable

	// content-related fields
	repos            []rpmmd.RepoConfig
	packageSpecs     []rpmmd.PackageSpec
	containerSpecs   []container.Spec
	ostreeParentSpec *ostree.CommitSpec

	platform  platform.Platform
	kernelVer string

	OSProduct string
	OSVersion string
	OSNick    string

	// InstallWeakDeps enables installation of weak dependencies for packages
	// that are statically defined for the pipeline.
	// Defaults to True.
	InstallWeakDeps bool
}

// NewOS creates a new OS pipeline. build is the build pipeline to use for
// building the OS pipeline. platform is the target platform for the final
// image. repos are the repositories to install RPMs from.
func NewOS(buildPipeline Build, platform platform.Platform, repos []rpmmd.RepoConfig) *OS {
	name := "os"
	p := &OS{
		Base:            NewBase(name, buildPipeline),
		repos:           filterRepos(repos, name),
		platform:        platform,
		InstallWeakDeps: true,
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *OS) getPackageSetChain(Distro) []rpmmd.PackageSet {
	packages := p.platform.GetPackages()

	if p.KernelName != "" {
		packages = append(packages, p.KernelName)
	}

	// If we have a logical volume we need to include the lvm2 package.
	// OSTree-based images (commit and container) aren't bootable images and
	// don't have partition tables.
	if p.PartitionTable != nil && p.OSTreeRef == "" {
		packages = append(packages, p.PartitionTable.GetBuildPackages()...)
	}

	if p.Environment != nil {
		packages = append(packages, p.Environment.GetPackages()...)
	}

	if len(p.NTPServers) > 0 {
		packages = append(packages, "chrony")
	}

	if p.SElinux != "" {
		packages = append(packages, fmt.Sprintf("selinux-policy-%s", p.SElinux))
	}

	if p.OpenSCAPRemediationConfig != nil {
		packages = append(packages, "openscap-scanner", "scap-security-guide", "xz")
	}

	// Make sure the right packages are included for subscriptions
	// rhc always uses insights, and depends on subscription-manager
	// non-rhc uses subscription-manager and optionally includes Insights
	if p.Subscription != nil {
		packages = append(packages, "subscription-manager")
		if p.Subscription.Rhc {
			packages = append(packages, "rhc", "insights-client", "rhc-worker-playbook")
		} else if p.Subscription.Insights {
			packages = append(packages, "insights-client")
		}
	}

	if len(p.Users) > 0 {
		// org.osbuild.users runs useradd, usermod, passwd, and
		// mkhomedir_helper in the os tree using chroot. Most image types
		// should already have the required packages, but some minimal image
		// types, like 'tar' don't, so let's add them for the stage to run and
		// to enable user management in the image.
		packages = append(packages, "shadow-utils", "pam", "passwd")

	}

	osRepos := append(p.repos, p.ExtraBaseRepos...)

	chain := []rpmmd.PackageSet{
		{
			Include:         append(packages, p.ExtraBasePackages...),
			Exclude:         p.ExcludeBasePackages,
			Repositories:    osRepos,
			InstallWeakDeps: p.InstallWeakDeps,
		},
	}

	if p.Workload != nil {
		workloadPackages := p.Workload.GetPackages()
		if len(workloadPackages) > 0 {
			chain = append(chain, rpmmd.PackageSet{
				Include:      workloadPackages,
				Repositories: append(osRepos, p.Workload.GetRepos()...),
			})
		}
	}

	return chain
}

func (p *OS) getContainerSources() []container.SourceSpec {
	return p.OSCustomizations.Containers
}

func (p *OS) getBuildPackages(distro Distro) []string {
	packages := p.platform.GetBuildPackages()
	if p.PartitionTable != nil {
		packages = append(packages, p.PartitionTable.GetBuildPackages()...)
	}
	packages = append(packages, "rpm")
	if p.OSTreeRef != "" {
		packages = append(packages, "rpm-ostree")
	}
	if p.SElinux != "" {
		packages = append(packages, "policycoreutils", fmt.Sprintf("selinux-policy-%s", p.SElinux))
	}
	if len(p.CloudInit) > 0 {
		switch distro {
		case DISTRO_EL7:
			packages = append(packages, "python3-PyYAML")
		default:
			packages = append(packages, "python3-pyyaml")
		}
	}
	if len(p.DNFConfig) > 0 || p.RHSMConfig != nil || p.WSLConfig != nil {
		packages = append(packages, "python3-iniparse")
	}

	if len(p.OSCustomizations.Containers) > 0 {
		if p.OSCustomizations.ContainersStorage != nil {
			switch distro {
			case DISTRO_EL8:
				packages = append(packages, "python3-pytoml")
			case DISTRO_EL10:
			default:
				packages = append(packages, "python3-toml")
			}
		}
		packages = append(packages, "skopeo")
	}

	if p.OpenSCAPRemediationConfig != nil && p.OpenSCAPRemediationConfig.TailoringConfig != nil {
		packages = append(packages, "openscap-utils")
	}

	if p.BootcConfig != nil {
		switch distro {
		case DISTRO_EL8:
			packages = append(packages, "python3-pytoml")
		case DISTRO_EL10:
		default:
			packages = append(packages, "python3-toml")
		}
	}

	return packages
}

func (p *OS) getOSTreeCommitSources() []ostree.SourceSpec {
	if p.OSTreeParent == nil {
		return nil
	}

	return []ostree.SourceSpec{
		*p.OSTreeParent,
	}
}

func (p *OS) getOSTreeCommits() []ostree.CommitSpec {
	if p.ostreeParentSpec == nil {
		return nil
	}
	return []ostree.CommitSpec{*p.ostreeParentSpec}
}

func (p *OS) getPackageSpecs() []rpmmd.PackageSpec {
	return p.packageSpecs
}

func (p *OS) getContainerSpecs() []container.Spec {
	return p.containerSpecs
}

func (p *OS) serializeStart(packages []rpmmd.PackageSpec, containers []container.Spec, commits []ostree.CommitSpec, rpmRepos []rpmmd.RepoConfig) {
	if len(p.packageSpecs) > 0 {
		panic("double call to serializeStart()")
	}

	p.packageSpecs = packages
	p.containerSpecs = containers
	if len(commits) > 0 {
		if len(commits) > 1 {
			panic("pipeline supports at most one ostree commit")
		}
		p.ostreeParentSpec = &commits[0]
	}

	if p.KernelName != "" {
		p.kernelVer = rpmmd.GetVerStrFromPackageSpecListPanic(p.packageSpecs, p.KernelName)
	}

	p.repos = append(p.repos, rpmRepos...)
}

func (p *OS) serializeEnd() {
	if len(p.packageSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.kernelVer = ""
	p.packageSpecs = nil
	p.containerSpecs = nil
	p.ostreeParentSpec = nil
}

func (p *OS) serialize() osbuild.Pipeline {
	if len(p.packageSpecs) == 0 {
		panic("serialization not started")
	}

	pipeline := p.Base.serialize()

	if p.ostreeParentSpec != nil {
		pipeline.AddStage(osbuild.NewOSTreePasswdStage("org.osbuild.source", p.ostreeParentSpec.Checksum))
	}

	// collect all repos for this pipeline to create the repository options
	allRepos := append(p.repos, p.ExtraBaseRepos...)
	if p.Workload != nil {
		allRepos = append(allRepos, p.Workload.GetRepos()...)
	}
	rpmOptions := osbuild.NewRPMStageOptions(allRepos)
	if p.ExcludeDocs {
		if rpmOptions.Exclude == nil {
			rpmOptions.Exclude = &osbuild.Exclude{}
		}
		rpmOptions.Exclude.Docs = true
	}
	rpmOptions.GPGKeysFromTree = p.GPGKeyFiles
	if p.OSTreeRef != "" {
		rpmOptions.OSTreeBooted = common.ToPtr(true)
		rpmOptions.DBPath = "/usr/share/rpm"
	}
	pipeline.AddStage(osbuild.NewRPMStage(rpmOptions, osbuild.NewRpmStageSourceFilesInputs(p.packageSpecs)))

	if !p.NoBLS {
		// If the /boot is on a separate partition, the prefix for the BLS stage must be ""
		if p.PartitionTable == nil || p.PartitionTable.FindMountable("/boot") == nil {
			pipeline.AddStage(osbuild.NewFixBLSStage(&osbuild.FixBLSStageOptions{}))
		} else {
			pipeline.AddStage(osbuild.NewFixBLSStage(&osbuild.FixBLSStageOptions{Prefix: common.ToPtr("")}))
		}
	}

	if len(p.containerSpecs) > 0 {
		var storagePath string
		if containerStore := p.OSCustomizations.ContainersStorage; containerStore != nil {
			storagePath = *containerStore
		}

		for _, stage := range osbuild.GenContainerStorageStages(storagePath, p.containerSpecs) {
			pipeline.AddStage(stage)
		}
	}

	pipeline.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: p.Language}))

	if p.Keyboard != nil {
		keymapOptions := &osbuild.KeymapStageOptions{Keymap: *p.Keyboard}
		if len(p.X11KeymapLayouts) > 0 {
			keymapOptions.X11Keymap = &osbuild.X11KeymapOptions{Layouts: p.X11KeymapLayouts}
		}
		pipeline.AddStage(osbuild.NewKeymapStage(keymapOptions))
	}

	if p.Hostname != "" {
		pipeline.AddStage(osbuild.NewHostnameStage(&osbuild.HostnameStageOptions{Hostname: p.Hostname}))
	}
	pipeline.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: p.Timezone}))

	if len(p.NTPServers) > 0 {
		chronyOptions := &osbuild.ChronyStageOptions{Servers: p.NTPServers}
		if p.LeapSecTZ != nil {
			chronyOptions.LeapsecTz = p.LeapSecTZ
		}
		pipeline.AddStage(osbuild.NewChronyStage(chronyOptions))
	}

	if len(p.Groups) > 0 {
		pipeline.AddStage(osbuild.GenGroupsStage(p.Groups))
	}

	if len(p.Users) > 0 {
		if p.OSTreeRef != "" {
			// for ostree, writing the key during user creation is
			// redundant and can cause issues so create users without keys
			// and write them on first boot
			usersStageSansKeys, err := osbuild.GenUsersStage(p.Users, true)
			if err != nil {
				// TODO: move encryption into weldr
				panic("password encryption failed")
			}
			pipeline.AddStage(usersStageSansKeys)
			pipeline.AddStage(osbuild.NewFirstBootStage(usersFirstBootOptions(p.Users)))
		} else {
			usersStage, err := osbuild.GenUsersStage(p.Users, false)
			if err != nil {
				// TODO: move encryption into weldr
				panic("password encryption failed")
			}
			pipeline.AddStage(usersStage)
		}
	}

	if p.Firewall != nil {
		pipeline.AddStage(osbuild.NewFirewallStage(p.Firewall))
	}

	for _, sysconfigConfig := range p.Sysconfig {
		pipeline.AddStage(osbuild.NewSysconfigStage(sysconfigConfig))
	}

	for _, systemdLogindConfig := range p.SystemdLogind {
		pipeline.AddStage(osbuild.NewSystemdLogindStage(systemdLogindConfig))
	}

	for _, cloudInitConfig := range p.CloudInit {
		pipeline.AddStage(osbuild.NewCloudInitStage(cloudInitConfig))
	}

	for _, modprobeConfig := range p.Modprobe {
		pipeline.AddStage(osbuild.NewModprobeStage(modprobeConfig))
	}

	for _, dracutConfConfig := range p.DracutConf {
		pipeline.AddStage(osbuild.NewDracutConfStage(dracutConfConfig))
	}

	for _, systemdUnitConfig := range p.SystemdUnit {
		pipeline.AddStage(osbuild.NewSystemdUnitStage(systemdUnitConfig))
	}

	if p.Authselect != nil {
		pipeline.AddStage(osbuild.NewAuthselectStage(p.Authselect))
	}

	if p.SELinuxConfig != nil {
		pipeline.AddStage(osbuild.NewSELinuxConfigStage(p.SELinuxConfig))
	}

	if p.Tuned != nil {
		pipeline.AddStage(osbuild.NewTunedStage(p.Tuned))
	}

	for _, tmpfilesdConfig := range p.Tmpfilesd {
		pipeline.AddStage(osbuild.NewTmpfilesdStage(tmpfilesdConfig))
	}

	for _, pamLimitsConfConfig := range p.PamLimitsConf {
		pipeline.AddStage(osbuild.NewPamLimitsConfStage(pamLimitsConfConfig))
	}

	for _, sysctldConfig := range p.Sysctld {
		pipeline.AddStage(osbuild.NewSysctldStage(sysctldConfig))
	}

	for _, dnfConfig := range p.DNFConfig {
		pipeline.AddStage(osbuild.NewDNFConfigStage(dnfConfig))
	}

	if p.DNFAutomaticConfig != nil {
		pipeline.AddStage(osbuild.NewDNFAutomaticConfigStage(p.DNFAutomaticConfig))
	}

	for _, yumRepo := range p.YUMRepos {
		pipeline.AddStage(osbuild.NewYumReposStage(yumRepo))
	}

	if p.YUMConfig != nil {
		pipeline.AddStage(osbuild.NewYumConfigStage(p.YUMConfig))
	}

	if p.GCPGuestAgentConfig != nil {
		pipeline.AddStage(osbuild.NewGcpGuestAgentConfigStage(p.GCPGuestAgentConfig))
	}

	if p.SshdConfig != nil {
		pipeline.AddStage((osbuild.NewSshdConfigStage(p.SshdConfig)))
	}

	if p.AuthConfig != nil {
		pipeline.AddStage(osbuild.NewAuthconfigStage(p.AuthConfig))
	}

	if p.PwQuality != nil {
		pipeline.AddStage(osbuild.NewPwqualityConfStage(p.PwQuality))
	}

	// If subscription settings are included there are 3 possible setups:
	// - Register the system with rhc and enable Insights
	// - Register with subscription-manager, no Insights or rhc
	// - Register with subscription-manager and enable Insights, no rhc
	if p.Subscription != nil {
		// Write a key file that will contain the org ID and activation key to be sourced in the systemd service.
		// The file will also act as the ConditionFirstBoot file.
		subkeyFilepath := "/etc/osbuild-subscription-register.env"
		subkeyContent := fmt.Sprintf("ORG_ID=%s\nACTIVATION_KEY=%s", p.Subscription.Organization, p.Subscription.ActivationKey)
		if subkeyFile, err := fsnode.NewFile(subkeyFilepath, nil, "root", "root", []byte(subkeyContent)); err == nil {
			p.Files = append(p.Files, subkeyFile)
		} else {
			panic(err)
		}

		var commands []string
		if p.Subscription.Rhc {
			// TODO: replace org ID and activation key with env vars
			// Use rhc for registration instead of subscription manager
			commands = []string{fmt.Sprintf("/usr/bin/rhc connect --organization=${ORG_ID} --activation-key=${ACTIVATION_KEY} --server %s", p.Subscription.ServerUrl)}
			// insights-client creates the .gnupg directory during boot process, and is labeled incorrectly
			commands = append(commands, "restorecon -R /root/.gnupg")
			// execute the rhc post install script as the selinuxenabled check doesn't work in the buildroot container
			commands = append(commands, "/usr/sbin/semanage permissive --add rhcd_t")
			if p.OSTreeRef != "" {
				p.runInsightsClientOnBoot()
			}
		} else {
			commands = []string{fmt.Sprintf("/usr/sbin/subscription-manager register --org=${ORG_ID} --activationkey=${ACTIVATION_KEY} --serverurl %s --baseurl %s", p.Subscription.ServerUrl, p.Subscription.BaseUrl)}

			// Insights is optional when using subscription-manager
			if p.Subscription.Insights {
				commands = append(commands, "/usr/bin/insights-client --register")
				// insights-client creates the .gnupg directory during boot process, and is labeled incorrectly
				commands = append(commands, "restorecon -R /root/.gnupg")
				if p.OSTreeRef != "" {
					p.runInsightsClientOnBoot()
				}
			}
		}

		commands = append(commands, fmt.Sprintf("/usr/bin/rm %s", subkeyFilepath))

		subscribeServiceFile := "osbuild-subscription-register.service"
		regServiceStageOptions := &osbuild.SystemdUnitCreateStageOptions{
			Filename: subscribeServiceFile,
			UnitType: "system",
			UnitPath: osbuild.Usr,
			Config: osbuild.SystemdServiceUnit{
				Unit: &osbuild.Unit{
					Description:         "First-boot service for registering with Red Hat subscription manager and/or insights",
					ConditionPathExists: []string{subkeyFilepath},
					Wants:               []string{"network-online.target"},
					After:               []string{"network-online.target"},
				},
				Service: &osbuild.Service{
					Type:            osbuild.Oneshot,
					RemainAfterExit: false,
					ExecStart:       commands,
					EnvironmentFile: []string{subkeyFilepath},
				},
				Install: &osbuild.Install{
					WantedBy: []string{"default.target"},
				},
			},
		}
		pipeline.AddStage(osbuild.NewSystemdUnitCreateStage(regServiceStageOptions))
		p.EnabledServices = append(p.EnabledServices, subscribeServiceFile)
	}

	if p.RHSMConfig != nil {
		pipeline.AddStage(osbuild.NewRHSMStage(osbuild.NewRHSMStageOptions(p.RHSMConfig)))
	}

	if p.WAAgentConfig != nil {
		pipeline.AddStage(osbuild.NewWAAgentConfStage(p.WAAgentConfig))
	}

	if p.UdevRules != nil {
		pipeline.AddStage(osbuild.NewUdevRulesStage(p.UdevRules))
	}

	if pt := p.PartitionTable; pt != nil {
		kernelOptions := osbuild.GenImageKernelOptions(p.PartitionTable)
		kernelOptions = append(kernelOptions, p.KernelOptionsAppend...)

		if p.FIPS {
			kernelOptions = append(kernelOptions, osbuild.GenFIPSKernelOptions(p.PartitionTable)...)
			pipeline.AddStage(osbuild.NewDracutStage(&osbuild.DracutStageOptions{
				Kernel:     []string{p.kernelVer},
				AddModules: []string{"fips"},
			}))
		}

		if !p.KernelOptionsBootloader || p.platform.GetArch() == arch.ARCH_S390X {
			pipeline = prependKernelCmdlineStage(pipeline, strings.Join(kernelOptions, " "), pt)
		}

		opts, err := osbuild.NewFSTabStageOptions(pt)
		if err != nil {
			panic(err)
		}
		pipeline.AddStage(osbuild.NewFSTabStage(opts))

		var bootloader *osbuild.Stage
		switch p.platform.GetArch() {
		case arch.ARCH_S390X:
			bootloader = osbuild.NewZiplStage(new(osbuild.ZiplStageOptions))
		default:
			if p.NoBLS {
				// BLS entries not supported: use grub2.legacy
				id := "76a22bf4-f153-4541-b6c7-0332c0dfaeac"
				product := osbuild.GRUB2Product{
					Name:    p.OSProduct,
					Version: p.OSVersion,
					Nick:    p.OSNick,
				}

				_, err := rpmmd.GetVerStrFromPackageSpecList(p.packageSpecs, "dracut-config-rescue")
				hasRescue := err == nil
				bootloader = osbuild.NewGrub2LegacyStage(
					osbuild.NewGrub2LegacyStageOptions(
						p.Grub2Config,
						p.PartitionTable,
						kernelOptions,
						p.platform.GetBIOSPlatform(),
						p.platform.GetUEFIVendor(),
						osbuild.MakeGrub2MenuEntries(id, p.kernelVer, product, hasRescue),
					),
				)
			} else {
				options := osbuild.NewGrub2StageOptions(pt,
					strings.Join(kernelOptions, " "),
					p.kernelVer,
					p.platform.GetUEFIVendor() != "",
					p.platform.GetBIOSPlatform(),
					p.platform.GetUEFIVendor(), false)
				if cfg := p.Grub2Config; cfg != nil {
					// TODO: don't store Grub2Config in OSPipeline, making the overrides unnecessary
					// grub2.Config.Default is owned and set by `NewGrub2StageOptionsUnified`
					// and thus we need to preserve it
					if options.Config != nil {
						cfg.Default = options.Config.Default
					}

					options.Config = cfg
				}
				if p.KernelOptionsBootloader {
					options.WriteCmdLine = nil
					if options.UEFI != nil {
						options.UEFI.Unified = false
					}
				}
				bootloader = osbuild.NewGRUB2Stage(options)
			}
		}

		pipeline.AddStage(bootloader)
	}

	if p.FactAPIType != nil {
		pipeline.AddStage(osbuild.NewRHSMFactsStage(&osbuild.RHSMFactsStageOptions{
			Facts: osbuild.RHSMFacts{
				ApiType: p.FactAPIType.String(),
			},
		}))
	}

	if p.OSTreeRef != "" {
		pipeline.AddStage(osbuild.NewSystemdJournaldStage(
			&osbuild.SystemdJournaldStageOptions{
				Filename: "10-persistent.conf",
				Config: osbuild.SystemdJournaldConfigDropin{
					Journal: osbuild.SystemdJournaldConfigJournalSection{
						Storage: osbuild.StoragePresistent,
					},
				},
			}))
	}

	// First create custom directories, because some of the custom files may depend on them
	if len(p.Directories) > 0 {
		pipeline.AddStages(osbuild.GenDirectoryNodesStages(p.Directories)...)
	}

	if len(p.Files) > 0 {
		pipeline.AddStages(osbuild.GenFileNodesStages(p.Files)...)
	}

	enabledServices := []string{}
	disabledServices := []string{}
	maskedServices := []string{}
	enabledServices = append(enabledServices, p.EnabledServices...)
	disabledServices = append(disabledServices, p.DisabledServices...)
	maskedServices = append(maskedServices, p.MaskedServices...)
	if p.Environment != nil {
		enabledServices = append(enabledServices, p.Environment.GetServices()...)
	}
	if p.Workload != nil {
		enabledServices = append(enabledServices, p.Workload.GetServices()...)
		disabledServices = append(disabledServices, p.Workload.GetDisabledServices()...)
	}
	if len(enabledServices) != 0 ||
		len(disabledServices) != 0 ||
		len(maskedServices) != 0 || p.DefaultTarget != "" {
		pipeline.AddStage(osbuild.NewSystemdStage(&osbuild.SystemdStageOptions{
			EnabledServices:  enabledServices,
			DisabledServices: disabledServices,
			MaskedServices:   maskedServices,
			DefaultTarget:    p.DefaultTarget,
		}))
	}
	if len(p.ShellInit) > 0 {
		pipeline.AddStage(osbuild.GenShellInitStage(p.ShellInit))
	}

	if p.WSLConfig != nil {
		pipeline.AddStage(osbuild.NewWSLConfStage(p.WSLConfig))
	}

	if p.FIPS {
		p.Files = append(p.Files, osbuild.GenFIPSFiles()...)
		for _, stage := range osbuild.GenFIPSStages() {
			pipeline.AddStage(stage)
		}
	}

	// NOTE: We need to run the OpenSCAP stages as the last stage before SELinux
	// since the remediation may change file permissions and other aspects of the
	// hardened image
	if remediationConfig := p.OpenSCAPRemediationConfig; remediationConfig != nil {
		if remediationConfig.TailoringConfig != nil {
			tailoringStageOpts := osbuild.NewOscapAutotailorStageOptions(remediationConfig)
			pipeline.AddStage(osbuild.NewOscapAutotailorStage(tailoringStageOpts))
		}
		remediationStageOpts := osbuild.NewOscapRemediationStageOptions(oscap.DataDir, remediationConfig)
		pipeline.AddStage(osbuild.NewOscapRemediationStage(remediationStageOpts))
	}

	if len(p.Presets) != 0 {
		pipeline.AddStage(osbuild.NewSystemdPresetStage(&osbuild.SystemdPresetStageOptions{
			Presets: p.Presets,
		}))
	}

	if p.SElinux != "" {
		pipeline.AddStage(osbuild.NewSELinuxStage(&osbuild.SELinuxStageOptions{
			FileContexts:     fmt.Sprintf("etc/selinux/%s/contexts/files/file_contexts", p.SElinux),
			ForceAutorelabel: p.SELinuxForceRelabel,
		}))
	}

	if p.OSTreeRef != "" {
		pipeline.AddStage(osbuild.NewOSTreePrepTreeStage(&osbuild.OSTreePrepTreeStageOptions{
			EtcGroupMembers: []string{
				// NOTE: We may want to make this configurable.
				"wheel", "docker",
			},
		}))
		if p.Bootupd {
			pipeline.AddStage(osbuild.NewBootupdGenMetadataStage())
		}
		if cfg := p.BootcConfig; cfg != nil {
			pipeline.AddStage(osbuild.NewBootcInstallConfigStage(
				osbuild.GenBootcInstallOptions(cfg.Filename, cfg.RootFilesystemType),
			))
		}
	} else {
		if p.Bootupd {
			panic("bootupd is only compatible with ostree-based images, this is a programming error")
		}
		if p.BootcConfig != nil {
			panic("bootc config is only compatible with ostree-based images, this is a programming error")
		}
	}

	return pipeline
}

func prependKernelCmdlineStage(pipeline osbuild.Pipeline, kernelOptions string, pt *disk.PartitionTable) osbuild.Pipeline {
	rootFs := pt.FindMountable("/")
	if rootFs == nil {
		panic("root filesystem must be defined for kernel-cmdline stage, this is a programming error")
	}
	rootFsUUID := rootFs.GetFSSpec().UUID
	kernelStage := osbuild.NewKernelCmdlineStage(osbuild.NewKernelCmdlineStageOptions(rootFsUUID, kernelOptions))
	pipeline.Stages = append([]*osbuild.Stage{kernelStage}, pipeline.Stages...)
	return pipeline
}

func usersFirstBootOptions(users []users.User) *osbuild.FirstBootStageOptions {
	cmds := make([]string, 0, 3*len(users)+2)
	// workaround for creating authorized_keys file for user
	// need to special case the root user, which has its home in a different place
	varhome := filepath.Join("/var", "home")
	roothome := filepath.Join("/var", "roothome")

	for _, user := range users {
		if user.Key != nil {
			var home string

			if user.Name == "root" {
				home = roothome
			} else {
				home = filepath.Join(varhome, user.Name)
			}

			sshdir := filepath.Join(home, ".ssh")

			cmds = append(cmds, fmt.Sprintf("mkdir -p %s", sshdir))
			cmds = append(cmds, fmt.Sprintf("sh -c 'echo %q >> %q'", *user.Key, filepath.Join(sshdir, "authorized_keys")))
			cmds = append(cmds, fmt.Sprintf("chown %s:%s -Rc %s", user.Name, user.Name, sshdir))
		}
	}
	cmds = append(cmds, fmt.Sprintf("restorecon -rvF %s", varhome))
	cmds = append(cmds, fmt.Sprintf("restorecon -rvF %s", roothome))

	options := &osbuild.FirstBootStageOptions{
		Commands:       cmds,
		WaitForNetwork: false,
	}

	return options
}

func (p *OS) Platform() platform.Platform {
	return p.platform
}

func (p *OS) getInline() []string {
	inlineData := []string{}

	// inline data for custom files
	for _, file := range p.Files {
		inlineData = append(inlineData, string(file.Data()))
	}

	return inlineData
}

// For ostree-based systems, creates a drop-in file for the insights-client
// service to run on boot and enables the service. This is only meant for
// ostree-based systems.
func (p *OS) runInsightsClientOnBoot() {
	// Insights-client collection must occur at boot time  so
	// that the current ostree commit hash can be reflected
	// after upgrade. Otherwise, the upgrade shows as failed in
	// the console UI.
	// Add a drop-in file that enables insights-client.service to
	// run on successful boot.
	// See https://issues.redhat.com/browse/HMS-4031
	//
	// NOTE(akoutsou): drop-in files can normally be created with the
	// org.osbuild.systemd.unit stage but the stage doesn't support
	// all the options we need. This is a temporary workaround
	// until we get the stage updated to support everything we need.
	icDropinFilepath, icDropinContents := insightsClientDropin()
	if icDropinDirectory, err := fsnode.NewDirectory(filepath.Dir(icDropinFilepath), nil, "root", "root", true); err == nil {
		p.Directories = append(p.Directories, icDropinDirectory)
	}
	if icDropinFile, err := fsnode.NewFile(icDropinFilepath, nil, "root", "root", []byte(icDropinContents)); err == nil {
		p.Files = append(p.Files, icDropinFile)
	} else {
		panic(err)
	}
	// Enable the service now that it's "enable-able"
	p.EnabledServices = append(p.EnabledServices, "insights-client.service")
}

// Filename and contents for the insights-client service drop-in.
// This is a temporary workaround until the org.osbuild.systemd.unit stage
// gains support for all the options we need.
func insightsClientDropin() (string, string) {
	return "/etc/systemd/system/insights-client.service.d/override.conf", `[Unit]
Requisite=greenboot-healthcheck.service
After=network-online.target greenboot-healthcheck.service osbuild-first-boot.service
[Install]
WantedBy=multi-user.target`
}
