package manifest

import (
	"errors"
	"fmt"
	"os"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
)

// A RawBootcImage represents a raw bootc image file which can be booted in a
// hypervisor.
type RawBootcImage struct {
	Base

	filename string
	platform platform.Platform

	containers     []container.SourceSpec
	containerSpecs []container.Spec

	// customizations go here because there is no intermediate
	// tree, with `bootc install to-filesystem` we can only work
	// with the image itself
	PartitionTable *disk.PartitionTable

	KernelOptionsAppend []string

	// The users to put into the image, note that /etc/paswd (and friends)
	// will become unmanaged state by bootc when used
	Users  []users.User
	Groups []users.Group

	// Custom directories and files to create in the image
	Directories []*fsnode.Directory
	Files       []*fsnode.File

	// SELinux policy, when set it enables the labeling of the tree with the
	// selected profile
	SELinux string

	// What type of mount configuration should we create, systemd units, fstab
	// or none
	MountConfiguration osbuild.MountConfiguration

	// Source pipeline for files written to raw partitions
	SourcePipeline string

	// This is used to copy the container kernel and initramfs into the PXE tar tree
	KernelVersion string

	// This adds directories to the root filesystem so that dmsquash-live will boot it
	LiveBoot bool
}

func (p RawBootcImage) Filename() string {
	return p.filename
}

func (p *RawBootcImage) SetFilename(filename string) {
	p.filename = filename
}

func NewRawBootcImage(buildPipeline Build, containers []container.SourceSpec, platform platform.Platform) *RawBootcImage {
	p := &RawBootcImage{
		Base:     NewBase("image", buildPipeline),
		filename: "disk.img",
		platform: platform,

		containers:     containers,
		SourcePipeline: buildPipeline.Name(),
	}
	buildPipeline.addDependent(p)
	return p
}

func (p *RawBootcImage) getContainerSources() []container.SourceSpec {
	return p.containers
}

func (p *RawBootcImage) getContainerSpecs() []container.Spec {
	return p.containerSpecs
}

func (p *RawBootcImage) serializeStart(inputs Inputs) error {
	if len(p.containerSpecs) > 0 {
		return errors.New("RawBootcImage: double call to serializeStart()")
	}
	p.containerSpecs = inputs.Containers
	return nil
}

func (p *RawBootcImage) serializeEnd() {
	if len(p.containerSpecs) == 0 {
		panic("serializeEnd() call when serialization not in progress")
	}
	p.containerSpecs = nil
}

func buildHomedirPaths(users []users.User) []osbuild.MkdirStagePath {
	var containsRootUser, containsNormalUser bool

	for _, user := range users {
		if user.Name == "root" {
			containsRootUser = true
		} else {
			containsNormalUser = true
		}
	}

	rootHomePath := osbuild.MkdirStagePath{
		Path:    "/var/roothome",
		Mode:    common.ToPtr(os.FileMode(0700)),
		ExistOk: true,
	}
	userHomePath := osbuild.MkdirStagePath{
		Path:    "/var/home",
		Mode:    common.ToPtr(os.FileMode(0755)),
		ExistOk: true,
	}
	switch {
	case containsRootUser && containsNormalUser:
		return []osbuild.MkdirStagePath{rootHomePath, userHomePath}
	case containsRootUser:
		return []osbuild.MkdirStagePath{rootHomePath}
	case containsNormalUser:
		return []osbuild.MkdirStagePath{userHomePath}
	default:
		return nil
	}
}

func (p *RawBootcImage) serialize() (osbuild.Pipeline, error) {
	pipeline, err := p.Base.serialize()
	if err != nil {
		return osbuild.Pipeline{}, err
	}

	pt := p.PartitionTable
	if pt == nil {
		return osbuild.Pipeline{}, fmt.Errorf("no partition table in live image")
	}

	for _, stage := range osbuild.GenImagePrepareStages(pt, p.filename, osbuild.PTSfdisk, p.SourcePipeline) {
		pipeline.AddStage(stage)
	}

	if len(p.containerSpecs) != 1 {
		return osbuild.Pipeline{}, fmt.Errorf("expected a single container input got %v", p.containerSpecs)
	}
	opts := &osbuild.BootcInstallToFilesystemOptions{
		Kargs: p.KernelOptionsAppend,
	}
	if len(p.containers) > 0 {
		opts.TargetImgref = p.containers[0].Name
	}
	inputs := osbuild.ContainerDeployInputs{
		Images: osbuild.NewContainersInputForSingleSource(p.containerSpecs[0]),
	}
	devices, mounts, err := osbuild.GenBootupdDevicesMounts(p.filename, p.PartitionTable, p.platform)
	if err != nil {
		return osbuild.Pipeline{}, err
	}
	st, err := osbuild.NewBootcInstallToFilesystemStage(opts, inputs, devices, mounts, p.platform)
	if err != nil {
		return osbuild.Pipeline{}, err
	}
	pipeline.AddStage(st)

	for _, stage := range osbuild.GenImageFinishStages(pt, p.filename) {
		pipeline.AddStage(stage)
	}

	// all our customizations work directly on the mounted deployment
	// root from the image so generate the devices/mounts for all
	devices, mounts, err = osbuild.GenBootupdDevicesMounts(p.filename, p.PartitionTable, p.platform)
	if err != nil {
		return osbuild.Pipeline{}, fmt.Errorf("gen devices stage failed %w", err)
	}
	mounts = append(mounts, *osbuild.NewOSTreeDeploymentMountDefault("ostree.deployment", osbuild.OSTreeMountSourceMount))
	mounts = append(mounts, *osbuild.NewBindMount("bind-ostree-deployment-to-tree", "mount://", "tree://"))

	postStages := []*osbuild.Stage{}

	fsCfgStages, err := filesystemConfigStages(pt, p.MountConfiguration)
	if err != nil {
		return osbuild.Pipeline{}, err
	}
	for _, stage := range fsCfgStages {
		stage.Mounts = mounts
		stage.Devices = devices
		postStages = append(postStages, stage)
	}

	// customize the image
	if len(p.Groups) > 0 {
		groupsStage := osbuild.GenGroupsStage(p.Groups)
		groupsStage.Mounts = mounts
		groupsStage.Devices = devices
		postStages = append(postStages, groupsStage)
	}

	if len(p.Users) > 0 {
		// ensure home root dir (currently /var/home, /var/roothome) is
		// available
		mkdirStage := osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
			Paths: buildHomedirPaths(p.Users),
		})
		mkdirStage.Mounts = mounts
		mkdirStage.Devices = devices
		postStages = append(postStages, mkdirStage)

		// add the users
		usersStage, err := osbuild.GenUsersStage(p.Users, false)
		if err != nil {
			return osbuild.Pipeline{}, fmt.Errorf("user stage failed %w", err)
		}
		usersStage.Mounts = mounts
		usersStage.Devices = devices
		postStages = append(postStages, usersStage)
	}

	if p.LiveBoot {
		// The dracut dmsquash-live module has a check for the root filesystem
		// This is a kludge to work around this: On Fedora and RHEL10 it expects /usr in the root
		// of the filesystem, and on RHEL9 it expects /proc
		// dracut version 110 and later also checks for /ostree but we cannot depend on that
		// version being available everywhere.
		var dirNodes []*fsnode.Directory
		for _, path := range []string{"/usr", "/proc"} {
			d, err := fsnode.NewDirectory(path, common.ToPtr(os.FileMode(0755)), nil, nil, false)
			if err != nil {
				return osbuild.Pipeline{}, fmt.Errorf("directory failed %w", err)
			}
			dirNodes = append(dirNodes, d)
		}
		stages := osbuild.GenDirectoryNodesStages(dirNodes)

		// NOTE this filters out the deployment mount, the /usr and /proc directories
		// need to be in the root of the ostree filesystem, not in the deployment
		var noDeploymentMounts []osbuild.Mount
		for _, m := range mounts {
			if m.Type == "org.osbuild.ostree.deployment" {
				continue
			}
			noDeploymentMounts = append(noDeploymentMounts, m)
		}
		for _, stage := range stages {
			stage.Mounts = noDeploymentMounts
			stage.Devices = devices
		}
		postStages = append(postStages, stages...)
	}

	// First create custom directories, because some of the custom files may depend on them
	if len(p.Directories) > 0 {

		stages := osbuild.GenDirectoryNodesStages(p.Directories)
		for _, stage := range stages {
			stage.Mounts = mounts
			stage.Devices = devices
		}
		postStages = append(postStages, stages...)
	}

	if len(p.Files) > 0 {
		stages := osbuild.GenFileNodesStages(p.Files)
		for _, stage := range stages {
			stage.Mounts = mounts
			stage.Devices = devices
		}
		postStages = append(postStages, stages...)
	}

	pipeline.AddStages(postStages...)

	// In case we created any files in the deploy directory we need to relabel
	// then per the selinux policy
	if p.SELinux != "" {
		if len(postStages) > 0 {
			for _, changedFile := range []string{"/etc", "/var"} {
				opts := &osbuild.SELinuxStageOptions{
					Target:       "tree://" + changedFile,
					FileContexts: fmt.Sprintf("etc/selinux/%s/contexts/files/file_contexts", p.SELinux),
					ExcludePaths: []string{"/sysroot"},
				}
				selinuxStage := osbuild.NewSELinuxStage(opts)
				selinuxStage.Mounts = mounts
				selinuxStage.Devices = devices
				pipeline.AddStage(selinuxStage)
			}
		}
	}

	return pipeline, nil
}

// XXX: duplicated from os.go
func (p *RawBootcImage) getInline() []string {
	inlineData := []string{}

	// inline data for custom files
	for _, file := range p.Files {
		inlineData = append(inlineData, string(file.Data()))
	}

	return inlineData
}

// XXX: copied from raw.go
func (p *RawBootcImage) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename(), nil)
}
