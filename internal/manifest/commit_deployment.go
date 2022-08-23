package manifest

import (
	"os"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/platform"
)

// OSTreeDeployment represents the filesystem tree of a target image based
// on a deployed ostree commit.
type OSTreeDeployment struct {
	Base

	Remote ostree.Remote

	OSVersion string

	osTreeCommit string
	osTreeURL    string
	osTreeRef    string

	osName string

	KernelOptionsAppend []string
	Keyboard            string
	Locale              string

	platform platform.Platform

	PartitionTable *disk.PartitionTable
}

// NewOSTreeDeployment creates a pipeline for an ostree deployment from a
// commit.
func NewOSTreeDeployment(m *Manifest,
	buildPipeline *Build,
	ref string,
	commit string,
	url string,
	osName string,
	platform platform.Platform) *OSTreeDeployment {

	p := &OSTreeDeployment{
		Base:         NewBase(m, "image-tree", buildPipeline),
		osTreeCommit: commit,
		osTreeURL:    url,
		osTreeRef:    ref,
		osName:       osName,
		platform:     platform,
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

func (p *OSTreeDeployment) getOSTreeCommits() []osTreeCommit {
	return []osTreeCommit{
		{
			checksum: p.osTreeCommit,
			url:      p.osTreeURL,
		},
	}
}

func (p *OSTreeDeployment) serialize() osbuild.Pipeline {
	const repoPath = "/ostree/repo"

	pipeline := p.Base.serialize()

	pipeline.AddStage(osbuild.OSTreeInitFsStage())
	pipeline.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: repoPath, Remote: p.Remote.Name},
		osbuild.NewOstreePullStageInputs("org.osbuild.source", p.osTreeCommit, p.osTreeRef),
	))
	pipeline.AddStage(osbuild.NewOSTreeOsInitStage(
		&osbuild.OSTreeOsInitStageOptions{
			OSName: p.osName,
		},
	))
	pipeline.AddStage(osbuild.NewOSTreeConfigStage(
		&osbuild.OSTreeConfigStageOptions{
			Repo: repoPath,
			Config: &osbuild.OSTreeConfig{
				Sysroot: &osbuild.SysrootOptions{
					ReadOnly:   common.BoolToPtr(true),
					Bootloader: "none",
				},
			},
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

	pipeline.AddStage(osbuild.NewOSTreeDeployStage(
		&osbuild.OSTreeDeployStageOptions{
			OsName: p.osName,
			Ref:    p.osTreeRef,
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
		remoteURL = p.osTreeURL
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
				Ref:    p.osTreeRef,
			},
		},
	))

	fstabOptions := osbuild.NewFSTabStageOptions(p.PartitionTable)
	fstabStage := osbuild.NewFSTabStage(fstabOptions)
	fstabStage.MountOSTree(p.osName, p.osTreeRef, 0)
	pipeline.AddStage(fstabStage)

	userOptions := &osbuild.UsersStageOptions{
		Users: map[string]osbuild.UsersStageOptionsUser{
			"root": {
				Password: common.StringToPtr("!locked"), // this is treated as crypted and locks/disables the password
			},
		},
	}
	userStage := osbuild.NewUsersStage(userOptions)
	userStage.MountOSTree(p.osName, p.osTreeRef, 0)
	pipeline.AddStage(userStage)

	if p.Keyboard != "" {
		options := &osbuild.KeymapStageOptions{
			Keymap: p.Keyboard,
		}
		keymapStage := osbuild.NewKeymapStage(options)
		keymapStage.MountOSTree(p.osName, p.osTreeRef, 0)
		pipeline.AddStage(keymapStage)
	}

	if p.Locale != "" {
		options := &osbuild.LocaleStageOptions{
			Language: p.Locale,
		}
		localeStage := osbuild.NewLocaleStage(options)
		localeStage.MountOSTree(p.osName, p.osTreeRef, 0)
		pipeline.AddStage(localeStage)
	}

	// TODO: Add users?
	// NOTE: Users can be embedded in a commit, but we should also support adding them at deploy time.

	options := osbuild.NewGrub2StageOptionsUnified(p.PartitionTable,
		"",
		p.platform.GetUEFIVendor() != "",
		p.platform.GetBIOSPlatform(),
		p.platform.GetUEFIVendor(), true)
	options.Greenboot = true
	bootloader := osbuild.NewGRUB2Stage(options)
	pipeline.AddStage(bootloader)

	pipeline.AddStage(osbuild.NewOSTreeSelinuxStage(
		&osbuild.OSTreeSelinuxStageOptions{
			Deployment: osbuild.OSTreeDeployment{
				OSName: p.osName,
				Ref:    p.osTreeRef,
			},
		},
	))

	return pipeline
}
