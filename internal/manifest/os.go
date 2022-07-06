package manifest

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/environment"
	"github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

type OSPipelineOSTree struct {
	Parent *OSPipelineOSTreeParent
}

type OSPipelineOSTreeParent struct {
	Checksum string
	URL      string
}

// OSPipeline represents the filesystem tree of the target image. This roughly
// correpsonds to the root filesystem once an instance of the image is running.
type OSPipeline struct {
	BasePipeline
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
	// Environment the system will run in
	Environment environment.Environment
	// Workload to install on top of the base system
	Workload workload.Workload
	// OSTree configuration, if nil the tree cannot be in an OSTree commit
	OSTree *OSPipelineOSTree
	// Partition table, if nil the tree cannot be put on a partioned disk
	PartitionTable *disk.PartitionTable
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
	// TODO: drop osbuild2 types from the API
	Grub2Config   *osbuild2.GRUB2Config
	Sysconfig     []*osbuild2.SysconfigStageOptions
	SystemdLogind []*osbuild2.SystemdLogindStageOptions
	CloudInit     []*osbuild2.CloudInitStageOptions
	Modprobe      []*osbuild2.ModprobeStageOptions
	DracutConf    []*osbuild2.DracutConfStageOptions
	SystemdUnit   []*osbuild2.SystemdUnitStageOptions
	Authselect    *osbuild2.AuthselectStageOptions
	SELinuxConfig *osbuild2.SELinuxConfigStageOptions
	Tuned         *osbuild2.TunedStageOptions
	Tmpfilesd     []*osbuild2.TmpfilesdStageOptions
	PamLimitsConf []*osbuild2.PamLimitsConfStageOptions
	Sysctld       []*osbuild2.SysctldStageOptions
	DNFConfig     []*osbuild2.DNFConfigStageOptions
	SshdConfig    *osbuild2.SshdConfigStageOptions
	AuthConfig    *osbuild2.AuthconfigStageOptions
	PwQuality     *osbuild2.PwqualityConfStageOptions

	repos        []rpmmd.RepoConfig
	packageSpecs []rpmmd.PackageSpec
	platform     platform.Platform
	kernelVer    string
}

// NewOSPipeline creates a new OS pipeline. osTree indicates whether or not the
// system is ostree based. osTreeParent indicates (for ostree systems) what the
// parent commit is. repos are the reposotories to install RPMs from. packages
// are the depsolved pacakges to be installed into the tree. partitionTable
// represents the disk layout of the target system. kernelName is the name of the
// kernel package that will be used on the target system.
func NewOSPipeline(m *Manifest,
	buildPipeline *BuildPipeline,
	platform platform.Platform,
	repos []rpmmd.RepoConfig) *OSPipeline {
	p := &OSPipeline{
		BasePipeline: NewBasePipeline(m, "os", buildPipeline, nil),
		repos:        repos,
		platform:     platform,
		Language:     "C.UTF-8",
		Hostname:     "localhost.localdomain",
		Timezone:     "UTC",
		SElinux:      "targeted",
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *OSPipeline) getPackageSetChain() []rpmmd.PackageSet {
	packages := p.platform.GetPackages()

	if p.KernelName != "" {
		packages = append(packages, p.KernelName)
	}

	// If we have a logical volume we need to include the lvm2 package
	if p.PartitionTable != nil && p.OSTree == nil {
		hasLVM := false
		hasBtrfs := false
		hasXFS := false
		hasFAT := false
		hasEXT4 := false

		introspectPT := func(e disk.Entity, path []disk.Entity) error {
			switch ent := e.(type) {
			case *disk.LVMLogicalVolume:
				hasLVM = true
			case *disk.Btrfs:
				hasBtrfs = true
			case *disk.Filesystem:
				switch ent.GetFSType() {
				case "vfat":
					hasFAT = true
				case "btrfs":
					hasBtrfs = true
				case "xfs":
					hasXFS = true
				case "ext4":
					hasEXT4 = true
				}
			}
			return nil
		}
		_ = p.PartitionTable.ForEachEntity(introspectPT)

		// TODO: LUKS
		if hasLVM {
			packages = append(packages, "lvm2")
		}
		if hasBtrfs {
			packages = append(packages, "btrfs-progs")
		}
		if hasXFS {
			packages = append(packages, "xfsprogs")
		}
		if hasFAT {
			packages = append(packages, "dosfstools")
		}
		if hasEXT4 {
			packages = append(packages, "e2fsprogs")
		}
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
		chain = append(chain, rpmmd.PackageSet{
			Include:      p.Workload.GetPackages(),
			Repositories: append(p.repos, p.Workload.GetRepos()...),
		})
	}

	return chain
}

func (p *OSPipeline) getBuildPackages() []string {
	packages := p.platform.GetBuildPackages()
	if p.OSTree != nil {
		packages = append(packages, "rpm-ostree")
	}
	return packages
}

func (p *OSPipeline) getOSTreeCommits() []osTreeCommit {
	commits := []osTreeCommit{}
	if p.OSTree != nil && p.OSTree.Parent != nil {
		commits = append(commits, osTreeCommit{
			checksum: p.OSTree.Parent.Checksum,
			url:      p.OSTree.Parent.URL,
		})
	}
	return commits
}

func (p *OSPipeline) getPackageSpecs() []rpmmd.PackageSpec {
	return p.packageSpecs
}

func (p *OSPipeline) serializeStart(packages []rpmmd.PackageSpec) {
	if len(p.packageSpecs) > 0 {
		panic("double call to serializeStart()")
	}
	p.packageSpecs = packages
	if p.KernelName != "" {
		p.kernelVer = rpmmd.GetVerStrFromPackageSpecListPanic(p.packageSpecs, p.KernelName)
	}
}

func (p *OSPipeline) serializeEnd() {
	if len(p.packageSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.kernelVer = ""
	p.packageSpecs = nil
}

func (p *OSPipeline) serialize() osbuild2.Pipeline {
	if len(p.packageSpecs) == 0 {
		panic("serialization not started")
	}

	pipeline := p.BasePipeline.serialize()

	if p.OSTree != nil && p.OSTree.Parent != nil {
		pipeline.AddStage(osbuild2.NewOSTreePasswdStage("org.osbuild.source", p.OSTree.Parent.Checksum))
	}

	rpmOptions := osbuild2.NewRPMStageOptions(p.repos)
	if p.ExcludeDocs {
		if rpmOptions.Exclude == nil {
			rpmOptions.Exclude = &osbuild2.Exclude{}
		}
		rpmOptions.Exclude.Docs = true
	}
	rpmOptions.GPGKeysFromTree = p.GPGKeyFiles
	pipeline.AddStage(osbuild2.NewRPMStage(rpmOptions, osbuild2.NewRpmStageSourceFilesInputs(p.packageSpecs)))

	// If the /boot is on a separate partition, the prefix for the BLS stage must be ""
	if p.PartitionTable == nil || p.PartitionTable.FindMountable("/boot") == nil {
		pipeline.AddStage(osbuild2.NewFixBLSStage(&osbuild2.FixBLSStageOptions{}))
	} else {
		pipeline.AddStage(osbuild2.NewFixBLSStage(&osbuild2.FixBLSStageOptions{Prefix: common.StringToPtr("")}))
	}

	pipeline.AddStage(osbuild2.NewLocaleStage(&osbuild2.LocaleStageOptions{Language: p.Language}))

	if p.Keyboard != nil {
		pipeline.AddStage(osbuild2.NewKeymapStage(&osbuild2.KeymapStageOptions{Keymap: *p.Keyboard}))
	}

	pipeline.AddStage(osbuild2.NewHostnameStage(&osbuild2.HostnameStageOptions{Hostname: p.Hostname}))
	pipeline.AddStage(osbuild2.NewTimezoneStage(&osbuild2.TimezoneStageOptions{Zone: p.Timezone}))

	if len(p.NTPServers) > 0 {
		pipeline.AddStage(osbuild2.NewChronyStage(&osbuild2.ChronyStageOptions{Timeservers: p.NTPServers}))
	}

	if len(p.Groups) > 0 {
		pipeline.AddStage(osbuild2.NewGroupsStage(osbuild2.NewGroupsStageOptions(p.Groups)))
	}

	if len(p.Users) > 0 {
		userOptions, err := osbuild2.NewUsersStageOptions(p.Users, false)
		if err != nil {
			// TODO: move encryption into weldr
			panic("password encryption failed")
		}
		if p.OSTree != nil {
			// for ostree, writing the key during user creation is
			// redundant and can cause issues so create users without keys
			// and write them on first boot
			userOptionsSansKeys, err := osbuild2.NewUsersStageOptions(p.Users, true)
			if err != nil {
				// TODO: move encryption into weldr
				panic("password encryption failed")
			}
			pipeline.AddStage(osbuild2.NewUsersStage(userOptionsSansKeys))
			pipeline.AddStage(osbuild2.NewFirstBootStage(usersFirstBootOptions(userOptions)))
		} else {
			pipeline.AddStage(osbuild2.NewUsersStage(userOptions))
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
		pipeline.AddStage(osbuild2.NewSystemdStage(&osbuild2.SystemdStageOptions{
			EnabledServices:  enabledServices,
			DisabledServices: disabledServices,
			DefaultTarget:    p.DefaultTarget,
		}))
	}

	if p.Firewall != nil {
		options := osbuild2.FirewallStageOptions{
			Ports: p.Firewall.Ports,
		}

		if p.Firewall.Services != nil {
			options.EnabledServices = p.Firewall.Services.Enabled
			options.DisabledServices = p.Firewall.Services.Disabled
		}

		pipeline.AddStage(osbuild2.NewFirewallStage(&options))
	}

	for _, sysconfigConfig := range p.Sysconfig {
		pipeline.AddStage(osbuild2.NewSysconfigStage(sysconfigConfig))
	}

	for _, systemdLogindConfig := range p.SystemdLogind {
		pipeline.AddStage(osbuild2.NewSystemdLogindStage(systemdLogindConfig))
	}

	for _, cloudInitConfig := range p.CloudInit {
		pipeline.AddStage(osbuild2.NewCloudInitStage(cloudInitConfig))
	}

	for _, modprobeConfig := range p.Modprobe {
		pipeline.AddStage(osbuild2.NewModprobeStage(modprobeConfig))
	}

	for _, dracutConfConfig := range p.DracutConf {
		pipeline.AddStage(osbuild2.NewDracutConfStage(dracutConfConfig))
	}

	for _, systemdUnitConfig := range p.SystemdUnit {
		pipeline.AddStage(osbuild2.NewSystemdUnitStage(systemdUnitConfig))
	}

	if p.Authselect != nil {
		pipeline.AddStage(osbuild2.NewAuthselectStage(p.Authselect))
	}

	if p.SELinuxConfig != nil {
		pipeline.AddStage(osbuild2.NewSELinuxConfigStage(p.SELinuxConfig))
	}

	if p.Tuned != nil {
		pipeline.AddStage(osbuild2.NewTunedStage(p.Tuned))
	}

	for _, tmpfilesdConfig := range p.Tmpfilesd {
		pipeline.AddStage(osbuild2.NewTmpfilesdStage(tmpfilesdConfig))
	}

	for _, pamLimitsConfConfig := range p.PamLimitsConf {
		pipeline.AddStage(osbuild2.NewPamLimitsConfStage(pamLimitsConfConfig))
	}

	for _, sysctldConfig := range p.Sysctld {
		pipeline.AddStage(osbuild2.NewSysctldStage(sysctldConfig))
	}

	for _, dnfConfig := range p.DNFConfig {
		pipeline.AddStage(osbuild2.NewDNFConfigStage(dnfConfig))
	}

	if p.SshdConfig != nil {
		pipeline.AddStage((osbuild2.NewSshdConfigStage(p.SshdConfig)))
	}

	if p.AuthConfig != nil {
		pipeline.AddStage(osbuild2.NewAuthconfigStage(p.AuthConfig))
	}

	if p.PwQuality != nil {
		pipeline.AddStage(osbuild2.NewPwqualityConfStage(p.PwQuality))
	}

	if pt := p.PartitionTable; pt != nil {
		kernelOptions := osbuild2.GenImageKernelOptions(p.PartitionTable)
		kernelOptions = append(kernelOptions, p.KernelOptionsAppend...)
		pipeline = prependKernelCmdlineStage(pipeline, strings.Join(kernelOptions, " "), pt)

		pipeline.AddStage(osbuild2.NewFSTabStage(osbuild2.NewFSTabStageOptions(pt)))

		var bootloader *osbuild2.Stage
		switch p.platform.GetArch() {
		case platform.ARCH_S390X:
			bootloader = osbuild2.NewZiplStage(new(osbuild2.ZiplStageOptions))
		default:
			options := osbuild2.NewGrub2StageOptionsUnified(pt,
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
			bootloader = osbuild2.NewGRUB2Stage(options)
		}

		pipeline.AddStage(bootloader)
	}

	if p.SElinux != "" {
		pipeline.AddStage(osbuild2.NewSELinuxStage(&osbuild2.SELinuxStageOptions{
			FileContexts: fmt.Sprintf("etc/selinux/%s/contexts/files/file_contexts", p.SElinux),
		}))
	}

	if p.OSTree != nil {
		pipeline.AddStage(osbuild2.NewOSTreePrepTreeStage(&osbuild2.OSTreePrepTreeStageOptions{
			EtcGroupMembers: []string{
				// NOTE: We may want to make this configurable.
				"wheel", "docker",
			},
		}))
	}

	return pipeline
}

func prependKernelCmdlineStage(pipeline osbuild2.Pipeline, kernelOptions string, pt *disk.PartitionTable) osbuild2.Pipeline {
	rootFs := pt.FindMountable("/")
	if rootFs == nil {
		panic("root filesystem must be defined for kernel-cmdline stage, this is a programming error")
	}
	rootFsUUID := rootFs.GetFSSpec().UUID
	kernelStage := osbuild2.NewKernelCmdlineStage(osbuild2.NewKernelCmdlineStageOptions(rootFsUUID, kernelOptions))
	pipeline.Stages = append([]*osbuild2.Stage{kernelStage}, pipeline.Stages...)
	return pipeline
}

func usersFirstBootOptions(usersStageOptions *osbuild2.UsersStageOptions) *osbuild2.FirstBootStageOptions {
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

	options := &osbuild2.FirstBootStageOptions{
		Commands:       cmds,
		WaitForNetwork: false,
	}

	return options
}
