package generic

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/bootc"
	"github.com/osbuild/images/pkg/customizations/fdo"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/ignition"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rpmmd"
)

func osCustomizations(t *imageType, osPackageSet rpmmd.PackageSet, containers []container.SourceSpec, c *blueprint.Customizations) (manifest.OSCustomizations, error) {
	imageConfig := t.getDefaultImageConfig()

	osc := manifest.OSCustomizations{}

	if t.ImageTypeYAML.Bootable || t.ImageTypeYAML.RPMOSTree {
		osc.KernelName = c.GetKernel().Name

		var kernelOptions []string
		// XXX: keep in sync with the identical copy in rhel/images.go
		if t.defaultImageConfig != nil && len(t.defaultImageConfig.KernelOptions) > 0 {
			kernelOptions = append(kernelOptions, t.defaultImageConfig.KernelOptions...)
		}
		if bpKernel := c.GetKernel(); bpKernel.Append != "" {
			kernelOptions = append(kernelOptions, bpKernel.Append)
		}
		osc.KernelOptionsAppend = kernelOptions
	}

	osc.FIPS = c.GetFIPS()

	osc.BasePackages = osPackageSet.Include
	osc.ExcludeBasePackages = osPackageSet.Exclude
	osc.ExtraBaseRepos = osPackageSet.Repositories

	osc.Containers = containers

	osc.GPGKeyFiles = imageConfig.GPGKeyFiles
	if rpm := c.GetRPM(); rpm != nil && rpm.ImportKeys != nil {
		osc.GPGKeyFiles = append(osc.GPGKeyFiles, rpm.ImportKeys.Files...)
	}

	if imageConfig.ExcludeDocs != nil {
		osc.ExcludeDocs = *imageConfig.ExcludeDocs
	}

	if !t.ImageTypeYAML.BootISO {
		// don't put users and groups in the payload of an installer
		// add them via kickstart instead
		osc.Groups = users.GroupsFromBP(c.GetGroups())

		osc.Users = users.UsersFromBP(c.GetUsers())
		osc.Users = append(osc.Users, imageConfig.Users...)
	}

	osc.EnabledServices = imageConfig.EnabledServices
	osc.DisabledServices = imageConfig.DisabledServices
	osc.MaskedServices = imageConfig.MaskedServices
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
	} else if imageConfig.Hostname != nil {
		osc.Hostname = *imageConfig.Hostname
	}

	if imageConfig.InstallWeakDeps != nil {
		osc.InstallWeakDeps = *imageConfig.InstallWeakDeps
	}

	timezone, ntpServers := c.GetTimezoneSettings()
	if timezone != nil {
		osc.Timezone = *timezone
	} else if imageConfig.Timezone != nil {
		osc.Timezone = *imageConfig.Timezone
	}

	if len(ntpServers) > 0 {
		chronyServers := make([]osbuild.ChronyConfigServer, 0, len(ntpServers))
		for _, server := range ntpServers {
			chronyServers = append(chronyServers, osbuild.ChronyConfigServer{Hostname: server})
		}
		osc.ChronyConfig = &osbuild.ChronyStageOptions{
			Servers: chronyServers,
		}
	} else if imageConfig.TimeSynchronization != nil {
		osc.ChronyConfig = imageConfig.TimeSynchronization
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
	if t.ImageTypeYAML.RPMOSTree && len(containers) > 0 {
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
		if t.ImageTypeYAML.RPMOSTree {
			panic("unexpected oscap options for ostree image type")
		}

		oscapDataNode, err := fsnode.NewDirectory(oscap.DataDir, nil, nil, nil, true)
		if err != nil {
			panic(fmt.Sprintf("unexpected error creating required OpenSCAP directory: %s", oscap.DataDir))
		}
		osc.Directories = append(osc.Directories, oscapDataNode)

		remediationConfig, err := oscap.NewConfigs(*oscapConfig, imageConfig.DefaultOSCAPDatastream)
		if err != nil {
			panic(fmt.Errorf("error creating OpenSCAP configs: %w", err))
		}

		osc.OpenSCAPRemediationConfig = remediationConfig
	}

	osc.ShellInit = imageConfig.ShellInit

	osc.Grub2Config = imageConfig.Grub2Config
	osc.Sysconfig = imageConfig.SysconfigStageOptions()
	osc.SystemdLogind = imageConfig.SystemdLogind
	osc.CloudInit = imageConfig.CloudInit
	osc.Modprobe = imageConfig.Modprobe
	osc.DracutConf = imageConfig.DracutConf
	osc.SystemdDropin = imageConfig.SystemdDropin
	osc.SystemdUnit = imageConfig.SystemdUnit
	osc.Authselect = imageConfig.Authselect
	osc.SELinuxConfig = imageConfig.SELinuxConfig
	osc.Tuned = imageConfig.Tuned
	osc.Tmpfilesd = imageConfig.Tmpfilesd
	osc.PamLimitsConf = imageConfig.PamLimitsConf
	osc.Sysctld = imageConfig.Sysctld
	osc.DNFConfig = imageConfig.DNFConfigOptions(t.arch.distro.OsVersion())
	osc.SshdConfig = imageConfig.SshdConfig
	osc.AuthConfig = imageConfig.Authconfig
	osc.PwQuality = imageConfig.PwQuality
	osc.NetworkManager = imageConfig.NetworkManager

	if imageConfig.WSL != nil {
		osc.WSLConfig = osbuild.NewWSLConfStageOptions(imageConfig.WSL.Config)
		osc.WSLDistributionConfig = osbuild.NewWSLDistributionConfStageOptions(imageConfig.WSL.DistributionConfig)
	}

	osc.Files = append(osc.Files, imageConfig.Files...)
	osc.Directories = append(osc.Directories, imageConfig.Directories...)

	ca, err := c.GetCACerts()
	if err != nil {
		panic(fmt.Sprintf("unexpected error checking CA certs: %v", err))
	}
	if ca != nil {
		osc.CACerts = ca.PEMCerts
	}

	if imageConfig.MachineIdUninitialized != nil {
		osc.MachineIdUninitialized = *imageConfig.MachineIdUninitialized
	}

	if imageConfig.MountUnits != nil {
		osc.MountUnits = *imageConfig.MountUnits
	}

	return osc, nil
}

func ostreeDeploymentCustomizations(
	t *imageType,
	c *blueprint.Customizations) (manifest.OSTreeDeploymentCustomizations, error) {

	if !t.ImageTypeYAML.RPMOSTree || !t.ImageTypeYAML.Bootable {
		return manifest.OSTreeDeploymentCustomizations{}, fmt.Errorf("ostree deployment customizations are only supported for bootable rpm-ostree images")
	}

	imageConfig := t.getDefaultImageConfig()
	deploymentConf := manifest.OSTreeDeploymentCustomizations{}

	var kernelOptions []string
	if len(t.defaultImageConfig.KernelOptions) > 0 {
		kernelOptions = append(kernelOptions, t.defaultImageConfig.KernelOptions...)
	}
	if bpKernel := c.GetKernel(); bpKernel != nil && bpKernel.Append != "" {
		kernelOptions = append(kernelOptions, bpKernel.Append)
	}

	if imageConfig.IgnitionPlatform != nil {
		deploymentConf.IgnitionPlatform = *imageConfig.IgnitionPlatform
	}

	switch deploymentConf.IgnitionPlatform {
	case "metal":
		if bpIgnition := c.GetIgnition(); bpIgnition != nil && bpIgnition.FirstBoot != nil && bpIgnition.FirstBoot.ProvisioningURL != "" {
			kernelOptions = append(kernelOptions, "ignition.config.url="+bpIgnition.FirstBoot.ProvisioningURL)
		}
	}
	deploymentConf.KernelOptionsAppend = kernelOptions

	deploymentConf.FIPS = c.GetFIPS()

	deploymentConf.Users = users.UsersFromBP(c.GetUsers())
	deploymentConf.Groups = users.GroupsFromBP(c.GetGroups())

	var err error
	deploymentConf.Directories, err = blueprint.DirectoryCustomizationsToFsNodeDirectories(c.GetDirectories())
	if err != nil {
		return manifest.OSTreeDeploymentCustomizations{}, err
	}
	deploymentConf.Files, err = blueprint.FileCustomizationsToFsNodeFiles(c.GetFiles())
	if err != nil {
		return manifest.OSTreeDeploymentCustomizations{}, err
	}

	language, keyboard := c.GetPrimaryLocale()
	if language != nil {
		deploymentConf.Locale = *language
	} else if imageConfig.Locale != nil {
		deploymentConf.Locale = *imageConfig.Locale
	}
	if keyboard != nil {
		deploymentConf.Keyboard = *keyboard
	} else if imageConfig.Keyboard != nil {
		deploymentConf.Keyboard = imageConfig.Keyboard.Keymap
	}

	if imageConfig.OSTreeConfSysrootReadOnly != nil {
		deploymentConf.SysrootReadOnly = *imageConfig.OSTreeConfSysrootReadOnly
	}

	if imageConfig.LockRootUser != nil {
		deploymentConf.LockRoot = *imageConfig.LockRootUser
	}

	for _, fs := range c.GetFilesystems() {
		deploymentConf.CustomFileSystems = append(deploymentConf.CustomFileSystems, fs.Mountpoint)
	}

	return deploymentConf, nil
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

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], containers, bp.Customizations)
	if err != nil {
		return nil, err
	}

	img.Environment = &t.ImageTypeYAML.Environment
	img.Workload = workload
	img.Compression = t.ImageTypeYAML.Compression
	if bp.Minimal {
		// Disable weak dependencies if the 'minimal' option is enabled
		img.OSCustomizations.InstallWeakDeps = false
	}
	// TODO: move generation into LiveImage
	pt, err := t.getPartitionTable(bp.Customizations, options, rng)
	if err != nil {
		return nil, err
	}
	img.PartitionTable = pt

	img.Filename = t.Filename()

	return img, nil
}

func tarImage(workload workload.Workload,
	t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {
	img := image.NewArchive()

	img.Platform = t.platform

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], containers, bp.Customizations)
	if err != nil {
		return nil, err
	}

	d := t.arch.distro

	img.Environment = &t.ImageTypeYAML.Environment
	img.Workload = workload
	img.Compression = t.ImageTypeYAML.Compression
	img.OSVersion = d.OsVersion()

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

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], containers, bp.Customizations)
	if err != nil {
		return nil, err
	}

	img.Environment = &t.ImageTypeYAML.Environment
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

	img.Product = d.Product()
	img.Variant = "Workstation"
	img.OSVersion = d.OsVersion()
	img.Release = fmt.Sprintf("%s %s", d.Product(), d.OsVersion())
	img.Preview = d.DistroYAML.Preview

	var err error
	img.ISOLabel, err = t.ISOLabel()
	if err != nil {
		return nil, err
	}

	img.Filename = t.Filename()

	// Enable grub2 BIOS iso on x86_64 only
	if img.Platform.GetArch() == arch.ARCH_X86_64 {
		img.ISOBoot = manifest.Grub2ISOBoot
	}

	if locale := t.getDefaultImageConfig().Locale; locale != nil {
		img.Locale = *locale
	}

	installerConfig, err := t.getDefaultInstallerConfig()
	if err != nil {
		return nil, err
	}
	if installerConfig != nil {
		img.AdditionalDracutModules = append(img.AdditionalDracutModules, installerConfig.AdditionalDracutModules...)
		img.AdditionalDrivers = append(img.AdditionalDrivers, installerConfig.AdditionalDrivers...)
		if installerConfig.SquashfsRootfs != nil && *installerConfig.SquashfsRootfs {
			img.RootfsType = manifest.SquashfsRootfs
		}
	}

	imgConfig := t.getDefaultImageConfig()
	if imgConfig != nil && imgConfig.IsoRootfsType != nil {
		img.RootfsType = *imgConfig.IsoRootfsType
	}

	return img, nil
}

func imageInstallerImage(workload workload.Workload,
	t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	customizations := bp.Customizations

	img := image.NewAnacondaTarInstaller()

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], containers, bp.Customizations)
	if err != nil {
		return nil, err
	}

	img.Kickstart, err = kickstart.New(customizations)
	if err != nil {
		return nil, err
	}
	img.Kickstart.Language = &img.OSCustomizations.Language
	img.Kickstart.Keyboard = img.OSCustomizations.Keyboard
	img.Kickstart.Timezone = &img.OSCustomizations.Timezone

	if img.Kickstart.Unattended {
		// NOTE: this is not supported right now because the
		// image-installer on Fedora isn't working when unattended.
		// These options are probably necessary but could change.
		// Unattended/non-interactive installations are better set to text
		// time since they might be running headless and a UI is
		// unnecessary.
		img.AdditionalKernelOpts = []string{"inst.text", "inst.noninteractive"}
	}

	instCust, err := customizations.GetInstaller()
	if err != nil {
		return nil, err
	}
	if instCust != nil && instCust.Modules != nil {
		img.AdditionalAnacondaModules = append(img.AdditionalAnacondaModules, instCust.Modules.Enable...)
		img.DisabledAnacondaModules = append(img.DisabledAnacondaModules, instCust.Modules.Disable...)
	}
	img.AdditionalAnacondaModules = append(img.AdditionalAnacondaModules, anaconda.ModuleUsers)

	img.Platform = t.platform
	img.Workload = workload

	img.ExtraBasePackages = packageSets[installerPkgsKey]

	installerConfig, err := t.getDefaultInstallerConfig()
	if err != nil {
		return nil, err
	}

	if installerConfig != nil {
		img.AdditionalDracutModules = append(img.AdditionalDracutModules, installerConfig.AdditionalDracutModules...)
		img.AdditionalDrivers = append(img.AdditionalDrivers, installerConfig.AdditionalDrivers...)
		if installerConfig.SquashfsRootfs != nil && *installerConfig.SquashfsRootfs {
			img.RootfsType = manifest.SquashfsRootfs
		}
	}

	d := t.arch.distro

	img.Product = d.Product()

	img.OSVersion = d.OsVersion()
	img.Release = fmt.Sprintf("%s %s", d.Product(), d.OsVersion())
	img.Variant = t.Variant
	img.Preview = d.DistroYAML.Preview

	img.ISOLabel, err = t.ISOLabel()
	if err != nil {
		return nil, err
	}

	img.Filename = t.Filename()

	img.RootfsCompression = "xz" // This also triggers using the bcj filter
	imgConfig := t.getDefaultImageConfig()
	if imgConfig != nil && imgConfig.IsoRootfsType != nil {
		img.RootfsType = *imgConfig.IsoRootfsType
	}

	// Enable grub2 BIOS iso on x86_64 only
	if img.Platform.GetArch() == arch.ARCH_X86_64 {
		img.ISOBoot = manifest.Grub2ISOBoot
	}

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

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], containers, bp.Customizations)
	if err != nil {
		return nil, err
	}
	imgConfig := t.getDefaultImageConfig()
	if imgConfig != nil {
		img.OSCustomizations.Presets = imgConfig.Presets
	}

	img.Environment = &t.ImageTypeYAML.Environment
	img.Workload = workload
	img.OSTreeParent = parentCommit
	img.OSVersion = d.OsVersion()
	img.Filename = t.Filename()
	img.InstallWeakDeps = false

	return img, nil
}

func bootableContainerImage(workload workload.Workload,
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

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], containers, bp.Customizations)
	if err != nil {
		return nil, err
	}

	img.Environment = &t.ImageTypeYAML.Environment
	img.Workload = workload
	img.OSTreeParent = parentCommit
	img.OSVersion = d.OsVersion()
	img.Filename = t.Filename()
	img.InstallWeakDeps = false
	img.BootContainer = true
	id, err := distro.ParseID(d.Name())
	if err != nil {
		return nil, err
	}
	img.BootcConfig = &bootc.Config{
		Filename:           fmt.Sprintf("20-%s.toml", id.Name),
		RootFilesystemType: "ext4",
	}

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

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], containers, bp.Customizations)
	if err != nil {
		return nil, err
	}

	imgConfig := t.getDefaultImageConfig()
	if imgConfig != nil {
		img.OSCustomizations.Presets = imgConfig.Presets
	}

	img.ContainerLanguage = img.OSCustomizations.Language
	img.Environment = &t.ImageTypeYAML.Environment
	img.Workload = workload
	img.OSTreeParent = parentCommit
	img.OSVersion = d.OsVersion()
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

	img.Kickstart, err = kickstart.New(customizations)
	if err != nil {
		return nil, err
	}
	img.Kickstart.OSTree = &kickstart.OSTree{
		OSName: t.OSTree.Name,
		Remote: t.OSTree.Remote,
	}
	img.Kickstart.Path = osbuild.KickstartPathOSBuild
	img.Kickstart.Language, img.Kickstart.Keyboard = customizations.GetPrimaryLocale()
	// ignore ntp servers - we don't currently support setting these in the
	// kickstart though kickstart does support setting them
	img.Kickstart.Timezone, _ = customizations.GetTimezoneSettings()

	instCust, err := customizations.GetInstaller()
	if err != nil {
		return nil, err
	}
	if instCust != nil && instCust.Modules != nil {
		img.AdditionalAnacondaModules = append(img.AdditionalAnacondaModules, instCust.Modules.Enable...)
		img.DisabledAnacondaModules = append(img.DisabledAnacondaModules, instCust.Modules.Disable...)
	}

	installerConfig, err := t.getDefaultInstallerConfig()
	if err != nil {
		return nil, err
	}

	if installerConfig != nil {
		img.AdditionalDracutModules = append(img.AdditionalDracutModules, installerConfig.AdditionalDracutModules...)
		img.AdditionalDrivers = append(img.AdditionalDrivers, installerConfig.AdditionalDrivers...)
		img.AdditionalAnacondaModules = append(img.AdditionalAnacondaModules, installerConfig.AdditionalAnacondaModules...)
		if installerConfig.SquashfsRootfs != nil && *installerConfig.SquashfsRootfs {
			img.RootfsType = manifest.SquashfsRootfs
		}
	}

	// On Fedora anaconda needs dbus-broker, but isn't added when dracut runs.
	img.AdditionalDracutModules = append(img.AdditionalDracutModules, "dbus-broker")

	img.Product = d.Product()
	img.Variant = "IoT"
	img.OSVersion = d.OsVersion()
	img.Release = fmt.Sprintf("%s %s", d.Product(), d.OsVersion())
	img.Preview = d.DistroYAML.Preview

	img.ISOLabel, err = t.ISOLabel()
	if err != nil {
		return nil, err
	}

	img.Filename = t.Filename()

	img.RootfsCompression = "xz" // This also triggers using the bcj filter
	imgConfig := t.getDefaultImageConfig()
	if imgConfig != nil && imgConfig.IsoRootfsType != nil {
		img.RootfsType = *imgConfig.IsoRootfsType
	}

	// Enable grub2 BIOS iso on x86_64 only
	if img.Platform.GetArch() == arch.ARCH_X86_64 {
		img.ISOBoot = manifest.Grub2ISOBoot
	}

	if locale := t.getDefaultImageConfig().Locale; locale != nil {
		img.Locale = *locale
	}

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

	customizations := bp.Customizations
	deploymentConfig, err := ostreeDeploymentCustomizations(t, customizations)
	if err != nil {
		return nil, err
	}
	img.OSTreeDeploymentCustomizations = deploymentConfig

	img.Platform = t.platform
	img.Workload = workload

	img.Remote = ostree.Remote{
		Name: t.OSTree.Remote,
	}
	img.OSName = t.OSTree.Remote

	// TODO: move generation into LiveImage
	pt, err := t.getPartitionTable(customizations, options, rng)
	if err != nil {
		return nil, err
	}
	img.PartitionTable = pt

	img.Filename = t.Filename()
	img.Compression = t.ImageTypeYAML.Compression

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
	deploymentConfig, err := ostreeDeploymentCustomizations(t, customizations)
	if err != nil {
		return nil, err
	}
	rawImg.OSTreeDeploymentCustomizations = deploymentConfig

	rawImg.Platform = t.platform
	rawImg.Workload = workload
	rawImg.Remote = ostree.Remote{
		Name: t.OSTree.Remote,
	}
	rawImg.OSName = t.OSTree.Name

	// TODO: move generation into LiveImage
	pt, err := t.getPartitionTable(customizations, options, rng)
	if err != nil {
		return nil, err
	}
	rawImg.PartitionTable = pt

	rawImg.Filename = t.Filename()

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

	installerConfig, err := t.getDefaultInstallerConfig()
	if err != nil {
		return nil, err
	}

	if installerConfig != nil {
		img.AdditionalDracutModules = append(img.AdditionalDracutModules, installerConfig.AdditionalDracutModules...)
		img.AdditionalDrivers = append(img.AdditionalDrivers, installerConfig.AdditionalDrivers...)
	}

	img.AdditionalDracutModules = append(img.AdditionalDracutModules, "dbus-broker")

	d := t.arch.distro
	img.Product = d.Product()
	img.Variant = "IoT"
	img.OSName = t.OSTree.Name
	img.OSVersion = d.OsVersion()

	img.ISOLabel, err = t.ISOLabel()
	if err != nil {
		return nil, err
	}

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
