package manifest

import (
	"os"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/platform"
	"github.com/osbuild/osbuild-composer/internal/users"
)

// OSTreeDeployment represents the filesystem tree of a target image based
// on a deployed ostree commit.
type OSTreeDeployment struct {
	Base

	Remote ostree.Remote

	OSVersion string

	commit ostree.CommitSpec

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
}

// NewOSTreeDeployment creates a pipeline for an ostree deployment from a
// commit.
func NewOSTreeDeployment(m *Manifest,
	buildPipeline *Build,
	commit ostree.CommitSpec,
	osName string,
	ignition bool,
	platform platform.Platform) *OSTreeDeployment {

	p := &OSTreeDeployment{
		Base:     NewBase(m, "image-tree", buildPipeline),
		commit:   commit,
		osName:   osName,
		platform: platform,
		ignition: ignition,
	}
	buildPipeline.addDependent(p)
	m.addPipeline(p)
	return p
}

func (p *OSTreeDeployment) getBuildPackages() []string {
	packages := []string{
		"rpm-ostree",
	}
	return packages
}

func (p *OSTreeDeployment) getOSTreeCommits() []ostree.CommitSpec {
	return []ostree.CommitSpec{p.commit}
}

func (p *OSTreeDeployment) serialize() osbuild.Pipeline {
	const repoPath = "/ostree/repo"

	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.OSTreeInitFsStage())
	pipeline.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: repoPath, Remote: p.Remote.Name},
		osbuild.NewOstreePullStageInputs("org.osbuild.source", p.commit.Checksum, p.commit.Ref),
	))
	pipeline.AddStage(osbuild.NewOSTreeOsInitStage(
		&osbuild.OSTreeOsInitStageOptions{
			OSName: p.osName,
		},
	))
	pipeline.AddStage(osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
		Paths: []osbuild.Path{
			{
				Path: "/boot/efi",
				Mode: os.FileMode(0700),
			},
		},
	}))
	kernelOpts := osbuild.GenImageKernelOptions(p.PartitionTable)
	kernelOpts = append(kernelOpts, p.KernelOptionsAppend...)

	if p.ignition {
		kernelOpts = append(kernelOpts,
			"coreos.no_persist_ip", // users cannot add connections as we don't have a live iso, this prevents connections to bleed into the system from the ign initrd
			"ignition.platform.id=metal",
			"$ignition_firstboot",
		)
	}

	pipeline.AddStage(osbuild.NewOSTreeDeployStage(
		&osbuild.OSTreeDeployStageOptions{
			OsName: p.osName,
			Ref:    p.commit.Ref,
			Remote: p.Remote.Name,
			Mounts: []string{"/boot", "/boot/efi"},
			Rootfs: osbuild.Rootfs{
				Label: "root",
			},
			KernelOpts: kernelOpts,
		},
	))

	remoteURL := p.Remote.URL
	if remoteURL == "" {
		// if the remote URL for the image is not specified, use the source commit URL
		remoteURL = p.commit.URL
	}
	pipeline.AddStage(osbuild.NewOSTreeRemotesStage(
		&osbuild.OSTreeRemotesStageOptions{
			Repo: "/ostree/repo",
			Remotes: []osbuild.OSTreeRemote{
				{
					Name:        p.Remote.Name,
					URL:         remoteURL,
					ContentURL:  p.Remote.ContentURL,
					GPGKeyPaths: p.Remote.GPGKeyPaths,
				},
			},
		},
	))

	pipeline.AddStage(osbuild.NewOSTreeFillvarStage(
		&osbuild.OSTreeFillvarStageOptions{
			Deployment: osbuild.OSTreeDeployment{
				OSName: p.osName,
				Ref:    p.commit.Ref,
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
	configStage.MountOSTree(p.osName, p.commit.Ref, 0)
	pipeline.AddStage(configStage)

	fstabOptions := osbuild.NewFSTabStageOptions(p.PartitionTable)
	fstabStage := osbuild.NewFSTabStage(fstabOptions)
	fstabStage.MountOSTree(p.osName, p.commit.Ref, 0)
	pipeline.AddStage(fstabStage)

	if len(p.Users) > 0 {
		usersStage, err := osbuild.GenUsersStage(p.Users, false)
		if err != nil {
			panic("password encryption failed")
		}
		usersStage.MountOSTree(p.osName, p.commit.Ref, 0)
		pipeline.AddStage(usersStage)
	}

	if len(p.Groups) > 0 {
		grpStage := osbuild.GenGroupsStage(p.Groups)
		grpStage.MountOSTree(p.osName, p.commit.Ref, 0)
		pipeline.AddStage(grpStage)
	}

	if p.ignition {
		pipeline.AddStage(osbuild.NewIgnitionStage(&osbuild.IgnitionStageOptions{}))
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
		rootLockStage.MountOSTree(p.osName, p.commit.Ref, 0)
		pipeline.AddStage(rootLockStage)
	}

	if p.Keyboard != "" {
		options := &osbuild.KeymapStageOptions{
			Keymap: p.Keyboard,
		}
		keymapStage := osbuild.NewKeymapStage(options)
		keymapStage.MountOSTree(p.osName, p.commit.Ref, 0)
		pipeline.AddStage(keymapStage)
	}

	if p.Locale != "" {
		options := &osbuild.LocaleStageOptions{
			Language: p.Locale,
		}
		localeStage := osbuild.NewLocaleStage(options)
		localeStage.MountOSTree(p.osName, p.commit.Ref, 0)
		pipeline.AddStage(localeStage)
	}

	grubOptions := osbuild.NewGrub2StageOptionsUnified(p.PartitionTable,
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
	grubOptions.KernelOptions = strings.Join(kernelOpts, ",")
	bootloader := osbuild.NewGRUB2Stage(grubOptions)
	bootloader.MountOSTree(p.osName, p.commit.Ref, 0)
	pipeline.AddStage(bootloader)

	pipeline.AddStage(osbuild.NewOSTreeSelinuxStage(
		&osbuild.OSTreeSelinuxStageOptions{
			Deployment: osbuild.OSTreeDeployment{
				OSName: p.osName,
				Ref:    p.commit.Ref,
			},
		},
	))

	return pipeline
}
