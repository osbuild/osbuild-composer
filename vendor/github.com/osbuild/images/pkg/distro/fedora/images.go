package fedora

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/fdo"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/ignition"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

// HELPERS

func osCustomizations(
	t *imageType,
	osPackageSet rpmmd.PackageSet,
	containers []container.SourceSpec,
	c *blueprint.Customizations) manifest.OSCustomizations {

	imageConfig := t.getDefaultImageConfig()

	osc := manifest.OSCustomizations{}

	if t.bootable || t.rpmOstree {
		osc.KernelName = c.GetKernel().Name

		var kernelOptions []string
		if t.kernelOptions != "" {
			kernelOptions = append(kernelOptions, t.kernelOptions)
		}
		if bpKernel := c.GetKernel(); bpKernel.Append != "" {
			kernelOptions = append(kernelOptions, bpKernel.Append)
		}
		osc.KernelOptionsAppend = kernelOptions
	}

	osc.FIPS = c.GetFIPS()

	osc.ExtraBasePackages = osPackageSet.Include
	osc.ExcludeBasePackages = osPackageSet.Exclude
	osc.ExtraBaseRepos = osPackageSet.Repositories

	osc.Containers = containers

	osc.GPGKeyFiles = imageConfig.GPGKeyFiles
	if imageConfig.ExcludeDocs != nil {
		osc.ExcludeDocs = *imageConfig.ExcludeDocs
	}

	if !t.bootISO {
		// don't put users and groups in the payload of an installer
		// add them via kickstart instead
		osc.Groups = users.GroupsFromBP(c.GetGroups())
		osc.Users = users.UsersFromBP(c.GetUsers())
	}

	osc.EnabledServices = imageConfig.EnabledServices
	osc.DisabledServices = imageConfig.DisabledServices
	if imageConfig.DefaultTarget != nil {
		osc.DefaultTarget = *imageConfig.DefaultTarget
	}

	if fw := c.GetFirewall(); fw != nil {
		options := osbuild.FirewallStageOptions{
			Ports: fw.Ports,
		}

		if fw.Services != nil {
			options.EnabledServices = fw.Services.Enabled
			options.DisabledServices = fw.Services.Disabled
		}
		osc.Firewall = &options
	}

	language, keyboard := c.GetPrimaryLocale()
	if language != nil {
		osc.Language = *language
	} else if imageConfig.Locale != nil {
		osc.Language = *imageConfig.Locale
	}
	if keyboard != nil {
		osc.Keyboard = keyboard
	} else if imageConfig.Keyboard != nil {
		osc.Keyboard = &imageConfig.Keyboard.Keymap
	}

	if hostname := c.GetHostname(); hostname != nil {
		osc.Hostname = *hostname
	} else {
		osc.Hostname = "localhost.localdomain"
	}

	timezone, ntpServers := c.GetTimezoneSettings()
	if timezone != nil {
		osc.Timezone = *timezone
	} else if imageConfig.Timezone != nil {
		osc.Timezone = *imageConfig.Timezone
	}

	if len(ntpServers) > 0 {
		for _, server := range ntpServers {
			osc.NTPServers = append(osc.NTPServers, osbuild.ChronyConfigServer{Hostname: server})
		}
	} else if imageConfig.TimeSynchronization != nil {
		osc.NTPServers = imageConfig.TimeSynchronization.Servers
	}

	// Relabel the tree, unless the `NoSElinux` flag is explicitly set to `true`
	if imageConfig.NoSElinux == nil || imageConfig.NoSElinux != nil && !*imageConfig.NoSElinux {
		osc.SElinux = "targeted"
	}

	var err error
	osc.Directories, err = blueprint.DirectoryCustomizationsToFsNodeDirectories(c.GetDirectories())
	if err != nil {
		// In theory this should never happen, because the blueprint directory customizations
		// should have been validated before this point.
		panic(fmt.Sprintf("failed to convert directory customizations to fs node directories: %v", err))
	}

	osc.Files, err = blueprint.FileCustomizationsToFsNodeFiles(c.GetFiles())
	if err != nil {
		// In theory this should never happen, because the blueprint file customizations
		// should have been validated before this point.
		panic(fmt.Sprintf("failed to convert file customizations to fs node files: %v", err))
	}

	// OSTree commits do not include data in `/var` since that is tied to the
	// deployment, rather than the commit. Therefore the containers need to be
	// stored in a different location, like `/usr/share`, and the container
	// storage engine configured accordingly.
	if t.rpmOstree && len(containers) > 0 {
		storagePath := "/usr/share/containers/storage"
		osc.ContainersStorage = &storagePath
	}

	if containerStorage := c.GetContainerStorage(); containerStorage != nil {
		osc.ContainersStorage = containerStorage.StoragePath
	}

	customRepos, err := c.GetRepositories()
	if err != nil {
		// This shouldn't happen and since the repos
		// should have already been validated
		panic(fmt.Sprintf("failed to get custom repos: %v", err))
	}

	// This function returns a map of filename and corresponding yum repos
	// and a list of fs node files for the inline gpg keys so we can save
	// them to disk. This step also swaps the inline gpg key with the path
	// to the file in the os file tree
	yumRepos, gpgKeyFiles, err := blueprint.RepoCustomizationsToRepoConfigAndGPGKeyFiles(customRepos)
	if err != nil {
		panic(fmt.Sprintf("failed to convert inline gpgkeys to fs node files: %v", err))
	}

	// add the gpg key files to the list of files to be added to the tree
	if len(gpgKeyFiles) > 0 {
		osc.Files = append(osc.Files, gpgKeyFiles...)
	}

	for filename, repos := range yumRepos {
		osc.YUMRepos = append(osc.YUMRepos, osbuild.NewYumReposStageOptions(filename, repos))
	}

	if oscapConfig := c.GetOpenSCAP(); oscapConfig != nil {
		if t.rpmOstree {
			panic("unexpected oscap options for ostree image type")
		}

		// although the osbuild stage will create this directory,
		// it's probably better to ensure that it is created here
		dataDirNode, err := fsnode.NewDirectory(oscapDataDir, nil, nil, nil, true)
		if err != nil {
			panic("unexpected error creating OpenSCAP data directory")
		}

		osc.Directories = append(osc.Directories, dataDirNode)

		var datastream = oscapConfig.DataStream
		if datastream == "" {
			datastream = oscap.DefaultFedoraDatastream()
		}

		oscapStageOptions := osbuild.OscapConfig{
			Datastream:  datastream,
			ProfileID:   oscapConfig.ProfileID,
			Compression: true,
		}

		if oscapConfig.Tailoring != nil {
			newProfile, tailoringFilepath, tailoringDir, err := oscap.GetTailoringFile(oscapConfig.ProfileID)
			if err != nil {
				panic(fmt.Sprintf("unexpected error creating tailoring file options: %v", err))
			}

			tailoringOptions := osbuild.OscapAutotailorConfig{
				NewProfile: newProfile,
				Datastream: datastream,
				ProfileID:  oscapConfig.ProfileID,
				Selected:   oscapConfig.Tailoring.Selected,
				Unselected: oscapConfig.Tailoring.Unselected,
			}

			osc.OpenSCAPTailorConfig = osbuild.NewOscapAutotailorStageOptions(
				tailoringFilepath,
				tailoringOptions,
			)

			// overwrite the profile id with the new tailoring id
			oscapStageOptions.ProfileID = newProfile
			oscapStageOptions.Tailoring = tailoringFilepath

			// add the parent directory for the tailoring file
			osc.Directories = append(osc.Directories, tailoringDir)
		}

		osc.OpenSCAPConfig = osbuild.NewOscapRemediationStageOptions(oscapDataDir, oscapStageOptions)
	}

	osc.ShellInit = imageConfig.ShellInit

	osc.Grub2Config = imageConfig.Grub2Config
	osc.Sysconfig = imageConfig.Sysconfig
	osc.SystemdLogind = imageConfig.SystemdLogind
	osc.CloudInit = imageConfig.CloudInit
	osc.Modprobe = imageConfig.Modprobe
	osc.DracutConf = imageConfig.DracutConf
	osc.SystemdUnit = imageConfig.SystemdUnit
	osc.Authselect = imageConfig.Authselect
	osc.SELinuxConfig = imageConfig.SELinuxConfig
	osc.Tuned = imageConfig.Tuned
	osc.Tmpfilesd = imageConfig.Tmpfilesd
	osc.PamLimitsConf = imageConfig.PamLimitsConf
	osc.Sysctld = imageConfig.Sysctld
	osc.DNFConfig = imageConfig.DNFConfig
	osc.SshdConfig = imageConfig.SshdConfig
	osc.AuthConfig = imageConfig.Authconfig
	osc.PwQuality = imageConfig.PwQuality
	osc.WSLConfig = imageConfig.WSLConfig

	osc.Files = append(osc.Files, imageConfig.Files...)
	osc.Directories = append(osc.Directories, imageConfig.Directories...)

	return osc
}

// IMAGES

func diskImage(workload workload.Workload,
	t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewDiskImage()
	img.Platform = t.platform
	img.OSCustomizations = osCustomizations(t, packageSets[osPkgsKey], containers, bp.Customizations)
	img.Environment = t.environment
	img.Workload = workload
	img.Compression = t.compression
	if bp.Minimal {
		// Disable weak dependencies if the 'minimal' option is enabled
		img.InstallWeakDeps = common.ToPtr(false)
	}
	// TODO: move generation into LiveImage
	pt, err := t.getPartitionTable(bp.Customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}
	img.PartitionTable = pt

	img.Filename = t.Filename()

	return img, nil
}

func containerImage(workload workload.Workload,
	t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {
	img := image.NewBaseContainer()

	img.Platform = t.platform
	img.OSCustomizations = osCustomizations(t, packageSets[osPkgsKey], containers, bp.Customizations)
	img.Environment = t.environment
	img.Workload = workload

	img.Filename = t.Filename()

	return img, nil
}

func liveInstallerImage(workload workload.Workload,
	t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewAnacondaLiveInstaller()

	img.Platform = t.platform
	img.Workload = workload
	img.ExtraBasePackages = packageSets[installerPkgsKey]

	d := t.arch.distro

	img.ISOLabelTempl = d.isolabelTmpl
	img.Product = d.product
	img.OSName = "fedora"
	img.OSVersion = d.osVersion
	img.Release = fmt.Sprintf("%s %s", d.product, d.osVersion)

	img.Filename = t.Filename()

	return img, nil
}

func imageInstallerImage(workload workload.Workload,
	t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewAnacondaTarInstaller()

	// Enable anaconda-webui for Fedora > 38
	distro := t.Arch().Distro()
	if !common.VersionLessThan(distro.Releasever(), "38") {
		img.AdditionalAnacondaModules = []string{
			"org.fedoraproject.Anaconda.Modules.Security",
			"org.fedoraproject.Anaconda.Modules.Timezone",
			"org.fedoraproject.Anaconda.Modules.Localization",
		}
		img.AdditionalKernelOpts = []string{"inst.webui", "inst.webui.remote"}
	}
	img.AdditionalAnacondaModules = append(img.AdditionalAnacondaModules, "org.fedoraproject.Anaconda.Modules.Users")

	customizations := bp.Customizations
	img.Platform = t.platform
	img.Workload = workload
	img.OSCustomizations = osCustomizations(t, packageSets[osPkgsKey], containers, customizations)
	img.ExtraBasePackages = packageSets[installerPkgsKey]
	img.Users = users.UsersFromBP(customizations.GetUsers())
	img.Groups = users.GroupsFromBP(customizations.GetGroups())

	img.SquashfsCompression = "lz4"

	d := t.arch.distro

	img.ISOLabelTempl = d.isolabelTmpl
	img.Product = d.product
	img.OSName = "fedora"
	img.OSVersion = d.osVersion
	img.Release = fmt.Sprintf("%s %s", d.product, d.osVersion)

	img.Filename = t.Filename()

	return img, nil
}

func iotCommitImage(workload workload.Workload,
	t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	parentCommit, commitRef := makeOSTreeParentCommit(options.OSTree, t.OSTreeRef())
	img := image.NewOSTreeArchive(commitRef)

	d := t.arch.distro

	img.Platform = t.platform
	img.OSCustomizations = osCustomizations(t, packageSets[osPkgsKey], containers, bp.Customizations)
	if !common.VersionLessThan(d.Releasever(), "38") {
		// see https://github.com/ostreedev/ostree/issues/2840
		img.OSCustomizations.Presets = []osbuild.Preset{
			{
				Name:  "ignition-firstboot-complete.service",
				State: osbuild.StateEnable,
			},
			{
				Name:  "coreos-ignition-write-issues.service",
				State: osbuild.StateEnable,
			},
			{
				Name:  "fdo-client-linuxapp.service",
				State: osbuild.StateEnable,
			},
		}
	}
	img.Environment = t.environment
	img.Workload = workload
	img.OSTreeParent = parentCommit
	img.OSVersion = d.osVersion
	img.Filename = t.Filename()
	img.InstallWeakDeps = false

	return img, nil
}

func iotContainerImage(workload workload.Workload,
	t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	parentCommit, commitRef := makeOSTreeParentCommit(options.OSTree, t.OSTreeRef())
	img := image.NewOSTreeContainer(commitRef)
	d := t.arch.distro
	img.Platform = t.platform
	img.OSCustomizations = osCustomizations(t, packageSets[osPkgsKey], containers, bp.Customizations)
	if !common.VersionLessThan(d.Releasever(), "38") {
		// see https://github.com/ostreedev/ostree/issues/2840
		img.OSCustomizations.Presets = []osbuild.Preset{
			{
				Name:  "ignition-firstboot-complete.service",
				State: osbuild.StateEnable,
			},
			{
				Name:  "coreos-ignition-write-issues.service",
				State: osbuild.StateEnable,
			},
			{
				Name:  "fdo-client-linuxapp.service",
				State: osbuild.StateEnable,
			},
		}
	}
	img.ContainerLanguage = img.OSCustomizations.Language
	img.Environment = t.environment
	img.Workload = workload
	img.OSTreeParent = parentCommit
	img.OSVersion = d.osVersion
	img.ExtraContainerPackages = packageSets[containerPkgsKey]
	img.Filename = t.Filename()

	return img, nil
}

func iotInstallerImage(workload workload.Workload,
	t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	d := t.arch.distro

	commit, err := makeOSTreePayloadCommit(options.OSTree, t.OSTreeRef())
	if err != nil {
		return nil, fmt.Errorf("%s: %s", t.Name(), err.Error())
	}

	img := image.NewAnacondaOSTreeInstaller(commit)

	customizations := bp.Customizations
	img.FIPS = customizations.GetFIPS()
	img.Platform = t.platform
	img.ExtraBasePackages = packageSets[installerPkgsKey]
	img.Users = users.UsersFromBP(customizations.GetUsers())
	img.Groups = users.GroupsFromBP(customizations.GetGroups())
	img.AdditionalAnacondaModules = []string{
		"org.fedoraproject.Anaconda.Modules.Timezone",
		"org.fedoraproject.Anaconda.Modules.Localization",
		"org.fedoraproject.Anaconda.Modules.Users",
	}

	img.SquashfsCompression = "lz4"

	img.ISOLabelTempl = d.isolabelTmpl
	img.Product = d.product
	img.Variant = "IoT"
	img.OSName = "fedora-iot"
	img.Remote = "fedora-iot"
	img.OSVersion = d.osVersion
	img.Release = fmt.Sprintf("%s %s", d.product, d.osVersion)

	img.Filename = t.Filename()

	return img, nil
}

func iotImage(workload workload.Workload,
	t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	commit, err := makeOSTreePayloadCommit(options.OSTree, t.OSTreeRef())
	if err != nil {
		return nil, fmt.Errorf("%s: %s", t.Name(), err.Error())
	}
	img := image.NewOSTreeDiskImageFromCommit(commit)

	distro := t.Arch().Distro()

	customizations := bp.Customizations
	img.FIPS = customizations.GetFIPS()
	img.Users = users.UsersFromBP(customizations.GetUsers())
	img.Groups = users.GroupsFromBP(customizations.GetGroups())

	img.Directories, err = blueprint.DirectoryCustomizationsToFsNodeDirectories(customizations.GetDirectories())
	if err != nil {
		return nil, err
	}
	img.Files, err = blueprint.FileCustomizationsToFsNodeFiles(customizations.GetFiles())
	if err != nil {
		return nil, err
	}

	img.KernelOptionsAppend = []string{"modprobe.blacklist=vc4"}
	img.Keyboard = "us"
	img.Locale = "C.UTF-8"

	img.SysrootReadOnly = true
	img.KernelOptionsAppend = append(img.KernelOptionsAppend, "rw")

	img.Platform = t.platform
	img.Workload = workload

	img.Remote = ostree.Remote{
		Name: "fedora-iot",
	}
	img.OSName = "fedora-iot"
	img.LockRoot = true

	if !common.VersionLessThan(distro.Releasever(), "38") {
		img.KernelOptionsAppend = append(img.KernelOptionsAppend, "coreos.no_persist_ip")
		switch img.Platform.GetImageFormat() {
		case platform.FORMAT_RAW:
			img.IgnitionPlatform = "metal"
			if bpIgnition := customizations.GetIgnition(); bpIgnition != nil && bpIgnition.FirstBoot != nil && bpIgnition.FirstBoot.ProvisioningURL != "" {
				img.KernelOptionsAppend = append(img.KernelOptionsAppend, "ignition.config.url="+bpIgnition.FirstBoot.ProvisioningURL)
			}
		case platform.FORMAT_QCOW2:
			img.IgnitionPlatform = "qemu"
		}
	}

	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		img.KernelOptionsAppend = append(img.KernelOptionsAppend, kopts.Append)
	}

	// TODO: move generation into LiveImage
	pt, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}
	img.PartitionTable = pt

	img.Filename = t.Filename()
	img.Compression = t.compression

	return img, nil
}

func iotSimplifiedInstallerImage(workload workload.Workload,
	t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	commit, err := makeOSTreePayloadCommit(options.OSTree, t.OSTreeRef())
	if err != nil {
		return nil, fmt.Errorf("%s: %s", t.Name(), err.Error())
	}
	rawImg := image.NewOSTreeDiskImageFromCommit(commit)

	customizations := bp.Customizations
	rawImg.FIPS = customizations.GetFIPS()
	rawImg.Users = users.UsersFromBP(customizations.GetUsers())
	rawImg.Groups = users.GroupsFromBP(customizations.GetGroups())

	rawImg.KernelOptionsAppend = []string{"modprobe.blacklist=vc4"}
	rawImg.Keyboard = "us"
	rawImg.Locale = "C.UTF-8"
	if !common.VersionLessThan(t.arch.distro.osVersion, "38") {
		rawImg.SysrootReadOnly = true
		rawImg.KernelOptionsAppend = append(rawImg.KernelOptionsAppend, "rw")
	}

	rawImg.Platform = t.platform
	rawImg.Workload = workload
	rawImg.Remote = ostree.Remote{
		Name: "fedora-iot",
	}
	rawImg.OSName = "fedora"
	rawImg.LockRoot = true

	if !common.VersionLessThan(t.arch.distro.osVersion, "38") {
		rawImg.IgnitionPlatform = "metal"
		rawImg.KernelOptionsAppend = append(rawImg.KernelOptionsAppend, "coreos.no_persist_ip")
		if bpIgnition := customizations.GetIgnition(); bpIgnition != nil && bpIgnition.FirstBoot != nil && bpIgnition.FirstBoot.ProvisioningURL != "" {
			rawImg.KernelOptionsAppend = append(rawImg.KernelOptionsAppend, "ignition.config.url="+bpIgnition.FirstBoot.ProvisioningURL)
		}
	}

	// TODO: move generation into LiveImage
	pt, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}
	rawImg.PartitionTable = pt

	rawImg.Filename = t.Filename()

	if kopts := customizations.GetKernel(); kopts != nil && kopts.Append != "" {
		rawImg.KernelOptionsAppend = append(rawImg.KernelOptionsAppend, kopts.Append)
	}

	img := image.NewOSTreeSimplifiedInstaller(rawImg, customizations.InstallationDevice)
	img.ExtraBasePackages = packageSets[installerPkgsKey]
	// img.Workload = workload
	img.Platform = t.platform
	img.Filename = t.Filename()
	if bpFDO := customizations.GetFDO(); bpFDO != nil {
		img.FDO = fdo.FromBP(*bpFDO)
	}
	// ignition configs from blueprint
	if bpIgnition := customizations.GetIgnition(); bpIgnition != nil {
		if bpIgnition.Embedded != nil {
			var err error
			img.IgnitionEmbedded, err = ignition.EmbeddedOptionsFromBP(*bpIgnition.Embedded)
			if err != nil {
				return nil, err
			}
		}
	}

	d := t.arch.distro
	img.ISOLabelTempl = d.isolabelTmpl
	img.Product = d.product
	img.Variant = "iot"
	img.OSName = "fedora"
	img.OSVersion = d.osVersion

	return img, nil
}

// Create an ostree SourceSpec to define an ostree parent commit using the user
// options and the default ref for the image type.  Additionally returns the
// ref to be used for the new commit to be created.
func makeOSTreeParentCommit(options *ostree.ImageOptions, defaultRef string) (*ostree.SourceSpec, string) {
	commitRef := defaultRef
	if options == nil {
		// nothing to do
		return nil, commitRef
	}
	if options.ImageRef != "" {
		// user option overrides default commit ref
		commitRef = options.ImageRef
	}

	var parentCommit *ostree.SourceSpec
	if options.URL == "" {
		// no parent
		return nil, commitRef
	}

	// ostree URL specified: set source spec for parent commit
	parentRef := options.ParentRef
	if parentRef == "" {
		// parent ref not set: use image ref
		parentRef = commitRef

	}
	parentCommit = &ostree.SourceSpec{
		URL:  options.URL,
		Ref:  parentRef,
		RHSM: options.RHSM,
	}
	return parentCommit, commitRef
}

// Create an ostree SourceSpec to define an ostree payload using the user options and the default ref for the image type.
func makeOSTreePayloadCommit(options *ostree.ImageOptions, defaultRef string) (ostree.SourceSpec, error) {
	if options == nil || options.URL == "" {
		// this should be caught by checkOptions() in distro, but it's good
		// to guard against it here as well
		return ostree.SourceSpec{}, fmt.Errorf("ostree commit URL required")
	}

	commitRef := defaultRef
	if options.ImageRef != "" {
		// user option overrides default commit ref
		commitRef = options.ImageRef
	}

	return ostree.SourceSpec{
		URL:  options.URL,
		Ref:  commitRef,
		RHSM: options.RHSM,
	}, nil
}

// initialSetupKickstart returns the File configuration for a kickstart file
// that's required to enable initial-setup to run on first boot.
func initialSetupKickstart() *fsnode.File {
	file, err := fsnode.NewFile("/root/anaconda-ks.cfg", nil, "root", "root", []byte("# Run initial-setup on first boot\n# Created by osbuild\nfirstboot --reconfig\n"))
	if err != nil {
		panic(err)
	}
	return file
}
