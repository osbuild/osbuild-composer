package manifest

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/environment"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

type OSTree struct {
	Parent *OSTreeParent
}

type OSTreeParent struct {
	Checksum string
	URL      string
}

// OSCustomizations encapsulates all configuration applied to the base
// operating independently of where and how it is integrated and what
// workload it is running.
// TODO: move out kernel/bootloader/cloud-init/... to other
//       abstractions, this should ideally only contain things that
//       can always be applied.
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
	// KernelName indicates that a kernel is installed, and names the kernel
	// package.
	KernelName string
	// KernelOptionsAppend are appended to the kernel commandline
	KernelOptionsAppend []string
	// UEFIVendor indicates whether or not the image should support UEFI and
	// if set namespaces the UEFI binaries with this string.
	GPGKeyFiles      []string
	Language         string
	Keyboard         *string
	Hostname         string
	Timezone         string
	NTPServers       []string
	EnabledServices  []string
	DisabledServices []string
	DefaultTarget    string

	// SELinux policy, when set it enables the labeling of the tree with the
	// selected profile
	SElinux string

	// Do not install documentation
	ExcludeDocs bool

	// TODO: drop blueprint types from the API
	Groups   []blueprint.GroupCustomization
	Users    []blueprint.UserCustomization
	Firewall *blueprint.FirewallCustomization
	// TODO: drop osbuild types from the API
	Grub2Config    *osbuild.GRUB2Config
	Sysconfig      []*osbuild.SysconfigStageOptions
	SystemdLogind  []*osbuild.SystemdLogindStageOptions
	CloudInit      []*osbuild.CloudInitStageOptions
	Modprobe       []*osbuild.ModprobeStageOptions
	DracutConf     []*osbuild.DracutConfStageOptions
	SystemdUnit    []*osbuild.SystemdUnitStageOptions
	Authselect     *osbuild.AuthselectStageOptions
	SELinuxConfig  *osbuild.SELinuxConfigStageOptions
	Tuned          *osbuild.TunedStageOptions
	Tmpfilesd      []*osbuild.TmpfilesdStageOptions
	PamLimitsConf  []*osbuild.PamLimitsConfStageOptions
	Sysctld        []*osbuild.SysctldStageOptions
	DNFConfig      []*osbuild.DNFConfigStageOptions
	SshdConfig     *osbuild.SshdConfigStageOptions
	AuthConfig     *osbuild.AuthconfigStageOptions
	PwQuality      *osbuild.PwqualityConfStageOptions
	OpenSCAPConfig *osbuild.OscapRemediationStageOptions
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
	// OSTree configuration, if nil the tree cannot be in an OSTree commit
	OSTree *OSTree
	// Partition table, if nil the tree cannot be put on a partioned disk
	PartitionTable *disk.PartitionTable

	repos        []rpmmd.RepoConfig
	packageSpecs []rpmmd.PackageSpec
	platform     platform.Platform
	kernelVer    string
}

// NewOS creates a new OS pipeline. osTree indicates whether or not the
// system is ostree based. osTreeParent indicates (for ostree systems) what the
// parent commit is. repos are the reposotories to install RPMs from. packages
// are the depsolved pacakges to be installed into the tree. partitionTable
// represents the disk layout of the target system. kernelName is the name of the
// kernel package that will be used on the target system.
func NewOS(m *Manifest,
	buildPipeline *Build,
	platform platform.Platform,
	repos []rpmmd.RepoConfig) *OS {
	p := &OS{
		Base:     NewBase(m, "os", buildPipeline),
		repos:    repos,
		platform: platform,
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *OS) getPackageSetChain() []rpmmd.PackageSet {
	packages := p.platform.GetPackages()

	if p.KernelName != "" {
		packages = append(packages, p.KernelName)
	}

	// If we have a logical volume we need to include the lvm2 package
	if p.PartitionTable != nil && p.OSTree == nil {
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

	chain := []rpmmd.PackageSet{
		{
			Include:      append(packages, p.ExtraBasePackages...),
			Exclude:      p.ExcludeBasePackages,
			Repositories: append(p.repos, p.ExtraBaseRepos...),
		},
	}

	if p.Workload != nil {
		workloadPackages := p.Workload.GetPackages()
		if len(workloadPackages) > 0 {
			chain = append(chain, rpmmd.PackageSet{
				Include:      workloadPackages,
				Repositories: append(p.repos, p.Workload.GetRepos()...),
			})
		}
	}

	return chain
}

func (p *OS) getBuildPackages() []string {
	packages := p.platform.GetBuildPackages()
	packages = append(packages, "rpm")
	if p.OSTree != nil {
		packages = append(packages, "rpm-ostree")
	}
	if p.SElinux != "" {
		packages = append(packages, "policycoreutils")
		packages = append(packages, fmt.Sprintf("selinux-policy-%s", p.SElinux))
	}
	if p.OpenSCAPConfig != nil {
		packages = append(packages, "openscap-scanner", "scap-security-guide")
	}
	return packages
}

func (p *OS) getOSTreeCommits() []osTreeCommit {
	commits := []osTreeCommit{}
	if p.OSTree != nil && p.OSTree.Parent != nil {
		commits = append(commits, osTreeCommit{
			checksum: p.OSTree.Parent.Checksum,
			url:      p.OSTree.Parent.URL,
		})
	}
	return commits
}

func (p *OS) getPackageSpecs() []rpmmd.PackageSpec {
	return p.packageSpecs
}

func (p *OS) serializeStart(packages []rpmmd.PackageSpec) {
	if len(p.packageSpecs) > 0 {
		panic("double call to serializeStart()")
	}
	p.packageSpecs = packages
	if p.KernelName != "" {
		p.kernelVer = rpmmd.GetVerStrFromPackageSpecListPanic(p.packageSpecs, p.KernelName)
	}
}

func (p *OS) serializeEnd() {
	if len(p.packageSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.kernelVer = ""
	p.packageSpecs = nil
}

func (p *OS) serialize() osbuild.Pipeline {
	if len(p.packageSpecs) == 0 {
		panic("serialization not started")
	}

	pipeline := p.Base.serialize()

	if p.OSTree != nil && p.OSTree.Parent != nil {
		pipeline.AddStage(osbuild.NewOSTreePasswdStage("org.osbuild.source", p.OSTree.Parent.Checksum))
	}

	rpmOptions := osbuild.NewRPMStageOptions(p.repos)
	if p.ExcludeDocs {
		if rpmOptions.Exclude == nil {
			rpmOptions.Exclude = &osbuild.Exclude{}
		}
		rpmOptions.Exclude.Docs = true
	}
	rpmOptions.GPGKeysFromTree = p.GPGKeyFiles
	pipeline.AddStage(osbuild.NewRPMStage(rpmOptions, osbuild.NewRpmStageSourceFilesInputs(p.packageSpecs)))

	// If the /boot is on a separate partition, the prefix for the BLS stage must be ""
	if p.PartitionTable == nil || p.PartitionTable.FindMountable("/boot") == nil {
		pipeline.AddStage(osbuild.NewFixBLSStage(&osbuild.FixBLSStageOptions{}))
	} else {
		pipeline.AddStage(osbuild.NewFixBLSStage(&osbuild.FixBLSStageOptions{Prefix: common.StringToPtr("")}))
	}

	pipeline.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: p.Language}))

	if p.Keyboard != nil {
		pipeline.AddStage(osbuild.NewKeymapStage(&osbuild.KeymapStageOptions{Keymap: *p.Keyboard}))
	}

	pipeline.AddStage(osbuild.NewHostnameStage(&osbuild.HostnameStageOptions{Hostname: p.Hostname}))
	pipeline.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: p.Timezone}))

	if len(p.NTPServers) > 0 {
		pipeline.AddStage(osbuild.NewChronyStage(&osbuild.ChronyStageOptions{Timeservers: p.NTPServers}))
	}

	if len(p.Groups) > 0 {
		pipeline.AddStage(osbuild.NewGroupsStage(osbuild.NewGroupsStageOptions(p.Groups)))
	}

	if len(p.Users) > 0 {
		userOptions, err := osbuild.NewUsersStageOptions(p.Users, false)
		if err != nil {
			// TODO: move encryption into weldr
			panic("password encryption failed")
		}
		if p.OSTree != nil {
			// for ostree, writing the key during user creation is
			// redundant and can cause issues so create users without keys
			// and write them on first boot
			userOptionsSansKeys, err := osbuild.NewUsersStageOptions(p.Users, true)
			if err != nil {
				// TODO: move encryption into weldr
				panic("password encryption failed")
			}
			pipeline.AddStage(osbuild.NewUsersStage(userOptionsSansKeys))
			pipeline.AddStage(osbuild.NewFirstBootStage(usersFirstBootOptions(userOptions)))
		} else {
			pipeline.AddStage(osbuild.NewUsersStage(userOptions))
		}
	}

	enabledServices := []string{}
	disabledServices := []string{}
	enabledServices = append(enabledServices, p.EnabledServices...)
	disabledServices = append(disabledServices, p.DisabledServices...)
	if p.Environment != nil {
		enabledServices = append(enabledServices, p.Environment.GetServices()...)
	}
	if p.Workload != nil {
		enabledServices = append(enabledServices, p.Workload.GetServices()...)
		disabledServices = append(disabledServices, p.Workload.GetDisabledServices()...)
	}
	if len(enabledServices) != 0 ||
		len(disabledServices) != 0 || p.DefaultTarget != "" {
		pipeline.AddStage(osbuild.NewSystemdStage(&osbuild.SystemdStageOptions{
			EnabledServices:  enabledServices,
			DisabledServices: disabledServices,
			DefaultTarget:    p.DefaultTarget,
		}))
	}

	if p.Firewall != nil {
		options := osbuild.FirewallStageOptions{
			Ports: p.Firewall.Ports,
		}

		if p.Firewall.Services != nil {
			options.EnabledServices = p.Firewall.Services.Enabled
			options.DisabledServices = p.Firewall.Services.Disabled
		}

		pipeline.AddStage(osbuild.NewFirewallStage(&options))
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

	if p.SshdConfig != nil {
		pipeline.AddStage((osbuild.NewSshdConfigStage(p.SshdConfig)))
	}

	if p.AuthConfig != nil {
		pipeline.AddStage(osbuild.NewAuthconfigStage(p.AuthConfig))
	}

	if p.PwQuality != nil {
		pipeline.AddStage(osbuild.NewPwqualityConfStage(p.PwQuality))
	}

	if pt := p.PartitionTable; pt != nil {
		kernelOptions := osbuild.GenImageKernelOptions(p.PartitionTable)
		kernelOptions = append(kernelOptions, p.KernelOptionsAppend...)
		pipeline = prependKernelCmdlineStage(pipeline, strings.Join(kernelOptions, " "), pt)

		pipeline.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(pt)))

		var bootloader *osbuild.Stage
		switch p.platform.GetArch() {
		case platform.ARCH_S390X:
			bootloader = osbuild.NewZiplStage(new(osbuild.ZiplStageOptions))
		default:
			options := osbuild.NewGrub2StageOptionsUnified(pt,
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
			bootloader = osbuild.NewGRUB2Stage(options)
		}

		pipeline.AddStage(bootloader)
	}

	if p.OpenSCAPConfig != nil {
		pipeline.AddStage(osbuild.NewOscapRemediationStage(p.OpenSCAPConfig))
	}

	if p.SElinux != "" {
		pipeline.AddStage(osbuild.NewSELinuxStage(&osbuild.SELinuxStageOptions{
			FileContexts: fmt.Sprintf("etc/selinux/%s/contexts/files/file_contexts", p.SElinux),
		}))
	}

	if p.OSTree != nil {
		pipeline.AddStage(osbuild.NewOSTreePrepTreeStage(&osbuild.OSTreePrepTreeStageOptions{
			EtcGroupMembers: []string{
				// NOTE: We may want to make this configurable.
				"wheel", "docker",
			},
		}))
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

func usersFirstBootOptions(usersStageOptions *osbuild.UsersStageOptions) *osbuild.FirstBootStageOptions {
	cmds := make([]string, 0, 3*len(usersStageOptions.Users)+2)
	// workaround for creating authorized_keys file for user
	// need to special case the root user, which has its home in a different place
	varhome := filepath.Join("/var", "home")
	roothome := filepath.Join("/var", "roothome")

	for name, user := range usersStageOptions.Users {
		if user.Key != nil {
			var home string

			if name == "root" {
				home = roothome
			} else {
				home = filepath.Join(varhome, name)
			}

			sshdir := filepath.Join(home, ".ssh")

			cmds = append(cmds, fmt.Sprintf("mkdir -p %s", sshdir))
			cmds = append(cmds, fmt.Sprintf("sh -c 'echo %q >> %q'", *user.Key, filepath.Join(sshdir, "authorized_keys")))
			cmds = append(cmds, fmt.Sprintf("chown %s:%s -Rc %s", name, name, sshdir))
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

func (p *OS) GetPlatform() platform.Platform {
	return p.platform
}
