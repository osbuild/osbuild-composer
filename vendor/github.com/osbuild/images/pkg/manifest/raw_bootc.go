package manifest

import (
	"fmt"
	"os"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/artifact"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
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

	// SELinux policy, when set it enables the labeling of the tree with the
	// selected profile
	SELinux string
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

		containers: containers,
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

func (p *RawBootcImage) serializeStart(_ []rpmmd.PackageSpec, containerSpecs []container.Spec, _ []ostree.CommitSpec, _ []rpmmd.RepoConfig) {
	if len(p.containerSpecs) > 0 {
		panic("double call to serializeStart()")
	}
	p.containerSpecs = containerSpecs
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

func (p *RawBootcImage) serialize() osbuild.Pipeline {
	pipeline := p.Base.serialize()

	pt := p.PartitionTable
	if pt == nil {
		panic(fmt.Errorf("no partition table in live image"))
	}

	for _, stage := range osbuild.GenImagePrepareStages(pt, p.filename, osbuild.PTSfdisk) {
		pipeline.AddStage(stage)
	}

	if len(p.containerSpecs) != 1 {
		panic(fmt.Errorf("expected a single container input got %v", p.containerSpecs))
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
		panic(err)
	}
	st, err := osbuild.NewBootcInstallToFilesystemStage(opts, inputs, devices, mounts, p.platform)
	if err != nil {
		panic(err)
	}
	pipeline.AddStage(st)

	for _, stage := range osbuild.GenImageFinishStages(pt, p.filename) {
		pipeline.AddStage(stage)
	}

	// all our customizations work directly on the mounted deployment
	// root from the image so generate the devices/mounts for all
	devices, mounts, err = osbuild.GenBootupdDevicesMounts(p.filename, p.PartitionTable, p.platform)
	if err != nil {
		panic(fmt.Sprintf("gen devices stage failed %v", err))
	}
	mounts = append(mounts, *osbuild.NewOSTreeDeploymentMountDefault("ostree.deployment", osbuild.OSTreeMountSourceMount))
	mounts = append(mounts, *osbuild.NewBindMount("bind-ostree-deployment-to-tree", "mount://", "tree://"))

	// we always include the fstab stage
	fstabStage := osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(pt))
	fstabStage.Mounts = mounts
	fstabStage.Devices = devices
	pipeline.AddStage(fstabStage)

	// customize the image
	if len(p.Groups) > 0 {
		groupsStage := osbuild.GenGroupsStage(p.Groups)
		groupsStage.Mounts = mounts
		groupsStage.Devices = devices
		pipeline.AddStage(groupsStage)
	}

	if len(p.Users) > 0 {
		// ensure home root dir (currently /var/home, /var/roothome) is
		// available
		mkdirStage := osbuild.NewMkdirStage(&osbuild.MkdirStageOptions{
			Paths: buildHomedirPaths(p.Users),
		})
		mkdirStage.Mounts = mounts
		mkdirStage.Devices = devices
		pipeline.AddStage(mkdirStage)

		// add the users
		usersStage, err := osbuild.GenUsersStage(p.Users, false)
		if err != nil {
			panic(fmt.Sprintf("user stage failed %v", err))
		}
		usersStage.Mounts = mounts
		usersStage.Devices = devices
		pipeline.AddStage(usersStage)

		// add selinux
		if p.SELinux != "" {
			opts := &osbuild.SELinuxStageOptions{
				FileContexts: fmt.Sprintf("etc/selinux/%s/contexts/files/file_contexts", p.SELinux),
				ExcludePaths: []string{"/sysroot"},
			}
			selinuxStage := osbuild.NewSELinuxStage(opts)
			selinuxStage.Mounts = mounts
			selinuxStage.Devices = devices
			pipeline.AddStage(selinuxStage)
		}
	}

	return pipeline
}

// XXX: copied from raw.go
func (p *RawBootcImage) Export() *artifact.Artifact {
	p.Base.export = true
	return artifact.New(p.Name(), p.Filename(), nil)
}
