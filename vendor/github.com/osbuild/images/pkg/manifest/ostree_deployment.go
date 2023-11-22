package manifest

import (
	"os"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/fsnode"
	"github.com/osbuild/images/internal/users"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

// OSTreeDeployment represents the filesystem tree of a target image based
// on a deployed ostree commit.
type OSTreeDeployment struct {
	Base

	Remote ostree.Remote

	OSVersion string

	commitSource ostree.SourceSpec
	ostreeSpecs  []ostree.CommitSpec

	SysrootReadOnly bool

	osName string

	KernelOptionsAppend []string
	Keyboard            string
	Locale              string

	Users  []users.User
	Groups []users.Group

	platform platform.Platform

	PartitionTable *disk.PartitionTable

	// Whether ignition is in use or not
	ignition bool

	// Specifies the ignition platform to use
	ignitionPlatform string

	Directories []*fsnode.Directory
	Files       []*fsnode.File

	EnabledServices  []string
	DisabledServices []string

	FIPS bool
}

// NewOSTreeDeployment creates a pipeline for an ostree deployment from a
// commit.
func NewOSTreeDeployment(buildPipeline *Build,
	m *Manifest,
	commit ostree.SourceSpec,
	osName string,
	ignition bool,
	ignitionPlatform string,
	platform platform.Platform) *OSTreeDeployment {

	p := &OSTreeDeployment{
		Base:             NewBase(m, "ostree-deployment", buildPipeline),
		commitSource:     commit,
		osName:           osName,
		platform:         platform,
		ignition:         ignition,
		ignitionPlatform: ignitionPlatform,
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *OSTreeDeployment) getBuildPackages(Distro) []string {
	packages := []string{
		"rpm-ostree",
	}
	return packages
}

func (p *OSTreeDeployment) getOSTreeCommits() []ostree.CommitSpec {
	return p.ostreeSpecs
}

func (p *OSTreeDeployment) getOSTreeCommitSources() []ostree.SourceSpec {
	return []ostree.SourceSpec{
		p.commitSource,
	}
}

func (p *OSTreeDeployment) serializeStart(packages []rpmmd.PackageSpec, containers []container.Spec, commits []ostree.CommitSpec) {
	if len(p.ostreeSpecs) > 0 {
		panic("double call to serializeStart()")
	}

	if len(commits) != 1 {
		panic("pipeline requires exactly one ostree commit")
	}

	p.ostreeSpecs = commits
}

func (p *OSTreeDeployment) serializeEnd() {
	if len(p.ostreeSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}

	p.ostreeSpecs = nil
}

func (p *OSTreeDeployment) serialize() osbuild.Pipeline {
	if len(p.ostreeSpecs) == 0 {
		panic("serialization not started")
	}
	if len(p.ostreeSpecs) > 1 {
		panic("multiple ostree commit specs found; this is a programming error")
	}
	commit := p.ostreeSpecs[0]

	const repoPath = "/ostree/repo"

	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.OSTreeInitFsStage())
	pipeline.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: repoPath, Remote: p.Remote.Name},
		osbuild.NewOstreePullStageInputs("org.osbuild.source", commit.Checksum, commit.Ref),
	))
	pipeline.AddStage(osbuild.NewOSTreeOsInitStage(
		&osbuild.OSTreeOsInitStageOptions{
			OSName: p.osName,
		},
	))
	pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.MkdirStagePath{
			{
				Path: "/boot/efi",
				Mode: common.ToPtr(os.FileMode(0700)),
			},
		},
	}))
	kernelOpts := osbuild.GenImageKernelOptions(p.PartitionTable)
	kernelOpts = append(kernelOpts, p.KernelOptionsAppend...)

	if p.ignition {
		if p.ignitionPlatform == "" {
			panic("ignition is enabled but ignition platform ID is not set")
		}
		kernelOpts = append(kernelOpts,
			"coreos.no_persist_ip", // users cannot add connections as we don't have a live iso, this prevents connections to bleed into the system from the ign initrd
			"ignition.platform.id="+p.ignitionPlatform,
			"$ignition_firstboot",
		)
	}

	if p.FIPS {
		kernelOpts = append(kernelOpts, osbuild.GenFIPSKernelOptions(p.PartitionTable)...)
		p.Files = append(p.Files, osbuild.GenFIPSFiles()...)
	}

	pipeline.AddStage(osbuild.NewOSTreeDeployStage(
		&osbuild.OSTreeDeployStageOptions{
			OsName: p.osName,
			Ref:    commit.Ref,
			Remote: p.Remote.Name,
			Mounts: []string{"/boot", "/boot/efi"},
			Rootfs: osbuild.Rootfs{
				Label: "root",
			},
			KernelOpts: kernelOpts,
		},
	))

	if p.Remote.URL != "" {
		pipeline.AddStage(osbuild.NewOSTreeRemotesStage(
			&osbuild.OSTreeRemotesStageOptions{
				Repo: "/ostree/repo",
				Remotes: []osbuild.OSTreeRemote{
					{
						Name:        p.Remote.Name,
						URL:         p.Remote.URL,
						ContentURL:  p.Remote.ContentURL,
						GPGKeyPaths: p.Remote.GPGKeyPaths,
					},
				},
			},
		))
	}

	pipeline.AddStage(osbuild.NewOSTreeFillvarStage(
		&osbuild.OSTreeFillvarStageOptions{
			Deployment: osbuild.OSTreeDeployment{
				OSName: p.osName,
				Ref:    commit.Ref,
			},
		},
	))

	configStage := osbuild.NewOSTreeConfigStage(
		&osbuild.OSTreeConfigStageOptions{
			Repo: repoPath,
			Config: &osbuild.OSTreeConfig{
				Sysroot: &osbuild.SysrootOptions{
					ReadOnly:   &p.SysrootReadOnly,
					Bootloader: "none",
				},
			},
		},
	)
	configStage.MountOSTree(p.osName, commit.Ref, 0)
	pipeline.AddStage(configStage)

	fstabOptions := osbuild.NewFSTabStageOptions(p.PartitionTable)
	fstabStage := osbuild.NewFSTabStage(fstabOptions)
	fstabStage.MountOSTree(p.osName, commit.Ref, 0)
	pipeline.AddStage(fstabStage)

	if len(p.Users) > 0 {
		usersStage, err := osbuild.GenUsersStage(p.Users, false)
		if err != nil {
			panic("password encryption failed")
		}
		usersStage.MountOSTree(p.osName, commit.Ref, 0)
		pipeline.AddStage(usersStage)
	}

	if len(p.Groups) > 0 {
		grpStage := osbuild.GenGroupsStage(p.Groups)
		grpStage.MountOSTree(p.osName, commit.Ref, 0)
		pipeline.AddStage(grpStage)
	}

	if p.ignition {
		pipeline.AddStage(osbuild.NewIgnitionStage(&osbuild.IgnitionStageOptions{
			// This is a workaround to make the systemd believe it's firstboot when ignition runs on real firstboot.
			// Right now, since we ship /etc/machine-id, systemd thinks it's not firstboot and ignition depends on it
			// to run on the real firstboot to enable services from presets.
			// Since this only applies to artifacts with ignition and changing machineid-compat at commit creation time may
			// have undesiderable effect, we're doing it here as a stopgap. We may revisit this in the future.
			Network: []string{
				"systemd.firstboot=off",
				"systemd.condition-first-boot=true",
			},
		}))

		// We enable / disable services below using the systemd stage, but its effect
		// may be overridden by systemd which may reset enabled / disabled services on
		// firstboot (which happend on F37+). This behavior, if available, is triggered
		// only when Ignition is used. To prevent this and to not have a special cases
		// in the code based on distro version, we enable / disable services also by
		// creating a preset file.
		if len(p.EnabledServices) != 0 || len(p.DisabledServices) != 0 {
			presetsStage := osbuild.GenServicesPresetStage(p.EnabledServices, p.DisabledServices)
			presetsStage.MountOSTree(p.osName, commit.Ref, 0)
			pipeline.AddStage(presetsStage)
		}
	}

	// if no root password is set, lock the root account
	hasRoot := false
	for _, user := range p.Users {
		if user.Name == "root" {
			hasRoot = true
			break
		}
	}

	if !hasRoot {
		userOptions := &osbuild.UsersStageOptions{
			Users: map[string]osbuild.UsersStageOptionsUser{
				"root": {
					Password: common.ToPtr("!locked"), // this is treated as crypted and locks/disables the password
				},
			},
		}
		rootLockStage := osbuild.NewUsersStage(userOptions)
		rootLockStage.MountOSTree(p.osName, commit.Ref, 0)
		pipeline.AddStage(rootLockStage)
	}

	if p.Keyboard != "" {
		options := &osbuild.KeymapStageOptions{
			Keymap: p.Keyboard,
		}
		keymapStage := osbuild.NewKeymapStage(options)
		keymapStage.MountOSTree(p.osName, commit.Ref, 0)
		pipeline.AddStage(keymapStage)
	}

	if p.Locale != "" {
		options := &osbuild.LocaleStageOptions{
			Language: p.Locale,
		}
		localeStage := osbuild.NewLocaleStage(options)
		localeStage.MountOSTree(p.osName, commit.Ref, 0)
		pipeline.AddStage(localeStage)
	}

	if p.FIPS {
		for _, stage := range osbuild.GenFIPSStages() {
			stage.MountOSTree(p.osName, commit.Ref, 0)
			pipeline.AddStage(stage)
		}
	}

	grubOptions := osbuild.NewGrub2StageOptionsUnified(p.PartitionTable,
		strings.Join(kernelOpts, " "),
		"",
		p.platform.GetUEFIVendor() != "",
		p.platform.GetBIOSPlatform(),
		p.platform.GetUEFIVendor(), true)
	grubOptions.Greenboot = true
	grubOptions.Ignition = p.ignition
	grubOptions.Config = &osbuild.GRUB2Config{
		Default:        "saved",
		Timeout:        1,
		TerminalOutput: []string{"console"},
	}
	bootloader := osbuild.NewGRUB2Stage(grubOptions)
	bootloader.MountOSTree(p.osName, commit.Ref, 0)
	pipeline.AddStage(bootloader)

	// First create custom directories, because some of the files may depend on them
	if len(p.Directories) > 0 {
		dirStages := osbuild.GenDirectoryNodesStages(p.Directories)
		for _, stage := range dirStages {
			stage.MountOSTree(p.osName, commit.Ref, 0)
		}
		pipeline.AddStages(dirStages...)
	}

	if len(p.Files) > 0 {
		fileStages := osbuild.GenFileNodesStages(p.Files)
		for _, stage := range fileStages {
			stage.MountOSTree(p.osName, commit.Ref, 0)
		}
		pipeline.AddStages(fileStages...)
	}

	if len(p.EnabledServices) != 0 || len(p.DisabledServices) != 0 {
		systemdStage := osbuild.NewSystemdStage(&osbuild.SystemdStageOptions{
			EnabledServices:  p.EnabledServices,
			DisabledServices: p.DisabledServices,
		})
		systemdStage.MountOSTree(p.osName, commit.Ref, 0)
		pipeline.AddStage(systemdStage)
	}

	pipeline.AddStage(osbuild.NewOSTreeSelinuxStage(
		&osbuild.OSTreeSelinuxStageOptions{
			Deployment: osbuild.OSTreeDeployment{
				OSName: p.osName,
				Ref:    commit.Ref,
			},
		},
	))

	return pipeline
}

func (p *OSTreeDeployment) getInline() []string {
	inlineData := []string{}

	// inline data for custom files
	for _, file := range p.Files {
		inlineData = append(inlineData, string(file.Data()))
	}

	return inlineData
}
