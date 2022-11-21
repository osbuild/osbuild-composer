package rhel9

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/users"
)

func edgeImagePipelines(t *imageType, customizations *blueprint.Customizations, filename string, options distro.ImageOptions, rng *rand.Rand) ([]osbuild.Pipeline, string, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	ostreeRepoPath := "/ostree/repo"
	imgName := "image.raw"

	partitionTable, err := t.getPartitionTable(nil, options, rng)
	if err != nil {
		return nil, "", err
	}

	// prepare ostree deployment tree
	treePipeline := ostreeDeployPipeline(t, partitionTable, ostreeRepoPath, rng, customizations, options)
	pipelines = append(pipelines, *treePipeline)

	// make raw image from tree
	imagePipeline := liveImagePipeline(treePipeline.Name, imgName, partitionTable, t.arch, "")
	pipelines = append(pipelines, *imagePipeline)

	// compress image
	xzPipeline := xzArchivePipeline(imagePipeline.Name, imgName, filename)
	pipelines = append(pipelines, *xzPipeline)

	return pipelines, xzPipeline.Name, nil
}

func buildPipeline(repos []rpmmd.RepoConfig, buildPackageSpecs []rpmmd.PackageSpec, runner string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "build"
	p.Runner = runner
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(buildPackageSpecs)))
	p.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(true)))
	return p
}

func edgeSimplifiedInstallerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, containers []container.Spec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey], t.arch.distro.runner.String()))
	installerPackages := packageSetSpecs[installerPkgsKey]
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(installerPackages, "kernel")
	imgName := "disk.img.xz"
	installDevice := customizations.GetInstallationDevice()

	// create the raw image
	imagePipelines, imgPipelineName, err := edgeImagePipelines(t, customizations, imgName, options, rng)
	if err != nil {
		return nil, err
	}

	pipelines = append(pipelines, imagePipelines...)

	// create boot ISO with raw image
	d := t.arch.distro
	archName := t.Arch().Name()
	installerTreePipeline := simplifiedInstallerTreePipeline(repos, installerPackages, kernelVer, archName, d.product, d.osVersion, "edge", customizations.GetFDO())
	isolabel := fmt.Sprintf(d.isolabelTmpl, archName)
	efibootTreePipeline := simplifiedInstallerEFIBootTreePipeline(installDevice, kernelVer, archName, d.vendor, d.product, d.osVersion, isolabel, customizations.GetFDO())
	bootISOTreePipeline := simplifiedInstallerBootISOTreePipeline(imgPipelineName, kernelVer, rng)

	pipelines = append(pipelines, *installerTreePipeline, *efibootTreePipeline, *bootISOTreePipeline)
	pipelines = append(pipelines, *bootISOPipeline(t.Filename(), d.isolabelTmpl, t.Arch().Name(), false))

	return pipelines, nil
}

func simplifiedInstallerBootISOTreePipeline(archivePipelineName, kver string, rng *rand.Rand) *osbuild.Pipeline {
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
					UUID:       disk.NewVolIDFromRand(rng),
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
	copyInputs := osbuild.NewPipelineTreeInputs(inputName, "efiboot-tree")
	copyOptions, copyDevices, copyMounts := osbuild.GenCopyFSTreeOptions(inputName, "efiboot-tree", filename, &pt)
	p.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	inputName = "coi"
	copyInputs = osbuild.NewPipelineTreeInputs(inputName, "coi-tree")
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
	copyInputs = osbuild.NewPipelineTreeInputs(inputName, "efiboot-tree")
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

func simplifiedInstallerEFIBootTreePipeline(installDevice, kernelVer, arch, vendor, product, osVersion, isolabel string, fdo *blueprint.FDOCustomization) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "efiboot-tree"
	p.Build = "name:build"
	p.AddStage(osbuild.NewGrubISOStage(grubISOStageOptions(installDevice, kernelVer, arch, vendor, product, osVersion, isolabel, fdo)))
	return p
}

func simplifiedInstallerTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, kernelVer, arch, product, osVersion, variant string, fdo *blueprint.FDOCustomization) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "coi-tree"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(packages)))
	p.AddStage(osbuild.NewBuildstampStage(buildStampStageOptions(arch, product, osVersion, variant)))
	p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "C.UTF-8"}))
	dracutStageOptions := dracutStageOptions(kernelVer, arch, []string{
		"coreos-installer",
		"fdo",
	})
	if fdo.HasFDO() && fdo.DiunPubKeyRootCerts != "" {
		p.AddStage(osbuild.NewFDOStageForRootCerts(fdo.DiunPubKeyRootCerts))
		dracutStageOptions.Install = []string{"/fdo_diun_pub_key_root_certs.pem"}
	}
	p.AddStage(osbuild.NewDracutStage(dracutStageOptions))
	return p
}

func ostreeDeployPipeline(
	t *imageType,
	pt *disk.PartitionTable,
	repoPath string,
	rng *rand.Rand,
	c *blueprint.Customizations,
	options distro.ImageOptions,
) *osbuild.Pipeline {

	p := new(osbuild.Pipeline)
	p.Name = "image-tree"
	p.Build = "name:build"
	osname := "redhat"
	remote := "rhel-edge"

	p.AddStage(osbuild.OSTreeInitFsStage())
	p.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: repoPath, Remote: remote},
		osbuild.NewOstreePullStageInputs("org.osbuild.source", options.OSTree.FetchChecksum, options.OSTree.ImageRef),
	))
	p.AddStage(osbuild.NewOSTreeOsInitStage(
		&osbuild.OSTreeOsInitStageOptions{
			OSName: osname,
		},
	))
	p.AddStage(osbuild.NewOSTreeConfigStage(ostreeConfigStageOptions(repoPath, false)))
	p.AddStage(osbuild.NewMkdirStage(efiMkdirStageOptions()))
	kernelOpts := osbuild.GenImageKernelOptions(pt)
	p.AddStage(osbuild.NewOSTreeDeployStage(
		&osbuild.OSTreeDeployStageOptions{
			OsName: osname,
			Ref:    options.OSTree.ImageRef,
			Remote: remote,
			Mounts: []string{"/boot", "/boot/efi"},
			Rootfs: osbuild.Rootfs{
				Label: "root",
			},
			KernelOpts: kernelOpts,
		},
	))

	if options.OSTree.URL != "" {
		p.AddStage(osbuild.NewOSTreeRemotesStage(
			&osbuild.OSTreeRemotesStageOptions{
				Repo: "/ostree/repo",
				Remotes: []osbuild.OSTreeRemote{
					{
						Name: remote,
						URL:  options.OSTree.URL,
					},
				},
			},
		))
	}

	p.AddStage(osbuild.NewOSTreeFillvarStage(
		&osbuild.OSTreeFillvarStageOptions{
			Deployment: osbuild.OSTreeDeployment{
				OSName: osname,
				Ref:    options.OSTree.ImageRef,
			},
		},
	))

	fstabOptions := osbuild.NewFSTabStageOptions(pt)
	fstabOptions.OSTree = &osbuild.OSTreeFstab{
		Deployment: osbuild.OSTreeDeployment{
			OSName: osname,
			Ref:    options.OSTree.ImageRef,
		},
	}
	p.AddStage(osbuild.NewFSTabStage(fstabOptions))

	if bpUsers := c.GetUsers(); len(bpUsers) > 0 {
		usersStage, err := osbuild.GenUsersStage(users.UsersFromBP(bpUsers), false)
		if err != nil {
			panic(err)
		}
		usersStage.MountOSTree(osname, options.OSTree.ImageRef, 0)
		p.AddStage(usersStage)
	}
	if bpGroups := c.GetGroups(); len(bpGroups) > 0 {
		groupsStage := osbuild.GenGroupsStage(users.GroupsFromBP(bpGroups))
		groupsStage.MountOSTree(osname, options.OSTree.ImageRef, 0)
		p.AddStage(groupsStage)
	}

	p.AddStage(bootloaderConfigStage(t, *pt, "", true, true))

	p.AddStage(osbuild.NewOSTreeSelinuxStage(
		&osbuild.OSTreeSelinuxStageOptions{
			Deployment: osbuild.OSTreeDeployment{
				OSName: osname,
				Ref:    options.OSTree.ImageRef,
			},
		},
	))
	return p
}

func bootISOPipeline(filename, isolabel, arch string, isolinux bool) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "bootiso"
	p.Build = "name:build"

	p.AddStage(osbuild.NewXorrisofsStage(xorrisofsStageOptions(filename, isolabel, arch, isolinux), "bootiso-tree"))
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
	copyInputs := osbuild.NewPipelineTreeInputs(inputName, inputPipelineName)
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

func bootloaderConfigStage(t *imageType, partitionTable disk.PartitionTable, kernelVer string, install, greenboot bool) *osbuild.Stage {
	if t.Arch().Name() == distro.S390xArchName {
		return osbuild.NewZiplStage(new(osbuild.ZiplStageOptions))
	}

	uefi := t.supportsUEFI()
	legacy := t.arch.legacy

	options := osbuild.NewGrub2StageOptionsUnified(&partitionTable, kernelVer, uefi, legacy, t.arch.distro.vendor, install)
	options.Greenboot = greenboot

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
