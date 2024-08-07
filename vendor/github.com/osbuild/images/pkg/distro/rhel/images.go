package rhel

import (
	"fmt"
	"math/rand"

	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/fdo"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/ignition"
	"github.com/osbuild/images/pkg/customizations/kickstart"
	"github.com/osbuild/images/pkg/customizations/oscap"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rpmmd"
)

func osCustomizations(
	t *ImageType,
	osPackageSet rpmmd.PackageSet,
	options distro.ImageOptions,
	containers []container.SourceSpec,
	c *blueprint.Customizations,
) (manifest.OSCustomizations, error) {

	imageConfig := t.getDefaultImageConfig()

	osc := manifest.OSCustomizations{}

	if t.Bootable || t.RPMOSTree {
		osc.KernelName = c.GetKernel().Name

		var kernelOptions []string
		if t.KernelOptions != "" {
			kernelOptions = append(kernelOptions, t.KernelOptions)
		}
		if bpKernel := c.GetKernel(); bpKernel.Append != "" {
			kernelOptions = append(kernelOptions, bpKernel.Append)
		}
		osc.KernelOptionsAppend = kernelOptions
		if imageConfig.KernelOptionsBootloader != nil {
			osc.KernelOptionsBootloader = *imageConfig.KernelOptionsBootloader
		}
	}

	osc.FIPS = c.GetFIPS()

	osc.ExtraBasePackages = osPackageSet.Include
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

	if !t.BootISO {
		// don't put users and groups in the payload of an installer
		// add them via kickstart instead
		osc.Groups = users.GroupsFromBP(c.GetGroups())
		osc.Users = users.UsersFromBP(c.GetUsers())
	}

	osc.EnabledServices = imageConfig.EnabledServices
	osc.DisabledServices = imageConfig.DisabledServices
	osc.MaskedServices = imageConfig.MaskedServices
	if imageConfig.DefaultTarget != nil {
		osc.DefaultTarget = *imageConfig.DefaultTarget
	}

	osc.Firewall = imageConfig.Firewall
	if fw := c.GetFirewall(); fw != nil {
		options := osbuild.FirewallStageOptions{
			Ports: fw.Ports,
		}

		if fw.Services != nil {
			options.EnabledServices = fw.Services.Enabled
			options.DisabledServices = fw.Services.Disabled
		}
		if fw.Zones != nil {
			for _, z := range fw.Zones {
				options.Zones = append(options.Zones, osbuild.FirewallZone{
					Name:    *z.Name,
					Sources: z.Sources,
				})
			}
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
		if imageConfig.Keyboard.X11Keymap != nil {
			osc.X11KeymapLayouts = imageConfig.Keyboard.X11Keymap.Layouts
		}
	}

	if hostname := c.GetHostname(); hostname != nil {
		osc.Hostname = *hostname
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
		osc.LeapSecTZ = imageConfig.TimeSynchronization.LeapsecTz
	}

	// Relabel the tree, unless the `NoSElinux` flag is explicitly set to `true`
	if imageConfig.NoSElinux == nil || imageConfig.NoSElinux != nil && !*imageConfig.NoSElinux {
		osc.SElinux = "targeted"
		osc.SELinuxForceRelabel = imageConfig.SELinuxForceRelabel
	}

	if t.IsRHEL() && options.Facts != nil {
		osc.FactAPIType = &options.Facts.APIType
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
	if t.RPMOSTree && len(containers) > 0 {
		storagePath := "/usr/share/containers/storage"
		osc.ContainersStorage = &storagePath
	}

	if containerStorage := c.GetContainerStorage(); containerStorage != nil {
		osc.ContainersStorage = containerStorage.StoragePath
	}

	// set yum repos first, so it doesn't get overridden by
	// imageConfig.YUMRepos
	osc.YUMRepos = imageConfig.YUMRepos

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
		if t.RPMOSTree {
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

	var subscriptionStatus subscription.RHSMStatus
	if options.Subscription != nil {
		subscriptionStatus = subscription.RHSMConfigWithSubscription
	} else {
		subscriptionStatus = subscription.RHSMConfigNoSubscription
	}
	if rhsmConfig, exists := imageConfig.RHSMConfig[subscriptionStatus]; exists {
		osc.RHSMConfig = rhsmConfig
	}

	if bpRhsmConfig := subscription.RHSMConfigFromBP(c.GetRHSM()); bpRhsmConfig != nil {
		osc.RHSMConfig = osc.RHSMConfig.Update(bpRhsmConfig)
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
	osc.DNFAutomaticConfig = imageConfig.DNFAutomaticConfig
	osc.YUMConfig = imageConfig.YumConfig
	osc.SshdConfig = imageConfig.SshdConfig
	osc.AuthConfig = imageConfig.Authconfig
	osc.PwQuality = imageConfig.PwQuality
	osc.Subscription = options.Subscription
	osc.WAAgentConfig = imageConfig.WAAgentConfig
	osc.UdevRules = imageConfig.UdevRules
	osc.GCPGuestAgentConfig = imageConfig.GCPGuestAgentConfig
	osc.WSLConfig = imageConfig.WSLConfig

	osc.Files = append(osc.Files, imageConfig.Files...)
	osc.Directories = append(osc.Directories, imageConfig.Directories...)

	if imageConfig.NoBLS != nil {
		osc.NoBLS = *imageConfig.NoBLS
	}

	return osc, nil
}

func ostreeDeploymentCustomizations(
	t *ImageType,
	c *blueprint.Customizations) (manifest.OSTreeDeploymentCustomizations, error) {

	if !t.RPMOSTree || !t.Bootable {
		return manifest.OSTreeDeploymentCustomizations{}, fmt.Errorf("ostree deployment customizations are only supported for bootable rpm-ostree images")
	}

	imageConfig := t.getDefaultImageConfig()
	deploymentConf := manifest.OSTreeDeploymentCustomizations{}

	var kernelOptions []string
	if t.KernelOptions != "" {
		kernelOptions = append(kernelOptions, t.KernelOptions)
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

func DiskImage(workload workload.Workload,
	t *ImageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewDiskImage()
	img.Platform = t.platform

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[OSPkgsKey], options, containers, customizations)
	if err != nil {
		return nil, err
	}

	img.Environment = t.Environment
	img.Workload = workload
	img.Compression = t.Compression
	// TODO: move generation into LiveImage
	pt, err := t.GetPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}
	img.PartitionTable = pt

	img.Filename = t.Filename()

	img.VPCForceSize = t.DiskImageVPCForceSize

	if img.OSCustomizations.NoBLS {
		img.OSProduct = t.Arch().Distro().Product()
		img.OSVersion = t.Arch().Distro().OsVersion()
		img.OSNick = t.Arch().Distro().Codename()
	}

	if t.DiskImagePartTool != nil {
		img.PartTool = *t.DiskImagePartTool
	}

	return img, nil
}

func EdgeCommitImage(workload workload.Workload,
	t *ImageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	parentCommit, commitRef := makeOSTreeParentCommit(options.OSTree, t.OSTreeRef())
	img := image.NewOSTreeArchive(commitRef)

	img.Platform = t.platform

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[OSPkgsKey], options, containers, customizations)
	if err != nil {
		return nil, err
	}

	img.Environment = t.Environment
	img.Workload = workload
	img.OSTreeParent = parentCommit
	img.OSVersion = t.Arch().Distro().OsVersion()
	img.Filename = t.Filename()

	return img, nil
}

func EdgeContainerImage(workload workload.Workload,
	t *ImageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	parentCommit, commitRef := makeOSTreeParentCommit(options.OSTree, t.OSTreeRef())
	img := image.NewOSTreeContainer(commitRef)

	img.Platform = t.platform

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[OSPkgsKey], options, containers, customizations)
	if err != nil {
		return nil, err
	}

	img.ContainerLanguage = img.OSCustomizations.Language
	img.Environment = t.Environment
	img.Workload = workload
	img.OSTreeParent = parentCommit
	img.OSVersion = t.Arch().Distro().OsVersion()
	img.ExtraContainerPackages = packageSets[ContainerPkgsKey]
	img.Filename = t.Filename()

	return img, nil
}

func EdgeInstallerImage(workload workload.Workload,
	t *ImageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	commit, err := makeOSTreePayloadCommit(options.OSTree, t.OSTreeRef())
	if err != nil {
		return nil, fmt.Errorf("%s: %s", t.Name(), err.Error())
	}

	img := image.NewAnacondaOSTreeInstaller(commit)

	img.Platform = t.platform
	img.ExtraBasePackages = packageSets[InstallerPkgsKey]

	if t.Arch().Distro().Releasever() == "8" {
		// NOTE: RHEL 8 only supports the older Anaconda configs
		img.UseLegacyAnacondaConfig = true
	}

	img.Kickstart, err = kickstart.New(customizations)
	if err != nil {
		return nil, err
	}
	img.Kickstart.OSTree = &kickstart.OSTree{
		OSName: "rhel-edge",
	}
	img.Kickstart.Path = osbuild.KickstartPathOSBuild
	img.Kickstart.Language, img.Kickstart.Keyboard = customizations.GetPrimaryLocale()
	// ignore ntp servers - we don't currently support setting these in the
	// kickstart though kickstart does support setting them
	img.Kickstart.Timezone, _ = customizations.GetTimezoneSettings()

	img.SquashfsCompression = "xz"

	installerConfig, err := t.getDefaultInstallerConfig()
	if err != nil {
		return nil, err
	}

	if installerConfig != nil {
		img.AdditionalDracutModules = installerConfig.AdditionalDracutModules
		img.AdditionalDrivers = installerConfig.AdditionalDrivers
	}

	instCust, err := customizations.GetInstaller()
	if err != nil {
		return nil, err
	}
	if instCust != nil && instCust.Modules != nil {
		img.AdditionalAnacondaModules = append(img.AdditionalAnacondaModules, instCust.Modules.Enable...)
		img.DisabledAnacondaModules = append(img.DisabledAnacondaModules, instCust.Modules.Disable...)
	}

	if len(img.Kickstart.Users)+len(img.Kickstart.Groups) > 0 {
		// only enable the users module if needed
		img.AdditionalAnacondaModules = append(img.AdditionalAnacondaModules, anaconda.ModuleUsers)
	}

	img.ISOLabel, err = t.ISOLabel()
	if err != nil {
		return nil, err
	}

	img.Product = t.Arch().Distro().Product()
	img.Variant = "edge"
	img.OSVersion = t.Arch().Distro().OsVersion()
	img.Release = fmt.Sprintf("%s %s", t.Arch().Distro().Product(), t.Arch().Distro().OsVersion())
	img.FIPS = customizations.GetFIPS()

	img.Filename = t.Filename()

	return img, nil
}

func EdgeRawImage(workload workload.Workload,
	t *ImageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	commit, err := makeOSTreePayloadCommit(options.OSTree, t.OSTreeRef())
	if err != nil {
		return nil, fmt.Errorf("%s: %s", t.Name(), err.Error())
	}
	img := image.NewOSTreeDiskImageFromCommit(commit)

	deploymentConfig, err := ostreeDeploymentCustomizations(t, customizations)
	if err != nil {
		return nil, err
	}
	img.OSTreeDeploymentCustomizations = deploymentConfig

	img.Platform = t.platform
	img.Workload = workload
	img.Remote = ostree.Remote{
		Name:       "rhel-edge",
		URL:        options.OSTree.URL,
		ContentURL: options.OSTree.ContentURL,
	}
	img.OSName = "rhel-edge"

	// TODO: move generation into LiveImage
	pt, err := t.GetPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}
	img.PartitionTable = pt

	img.Filename = t.Filename()
	img.Compression = t.Compression

	return img, nil
}

func EdgeSimplifiedInstallerImage(workload workload.Workload,
	t *ImageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	commit, err := makeOSTreePayloadCommit(options.OSTree, t.OSTreeRef())
	if err != nil {
		return nil, fmt.Errorf("%s: %s", t.Name(), err.Error())
	}
	rawImg := image.NewOSTreeDiskImageFromCommit(commit)

	deploymentConfig, err := ostreeDeploymentCustomizations(t, customizations)
	if err != nil {
		return nil, err
	}
	rawImg.OSTreeDeploymentCustomizations = deploymentConfig

	rawImg.Platform = t.platform
	rawImg.Workload = workload
	rawImg.Remote = ostree.Remote{
		Name:       "rhel-edge",
		URL:        options.OSTree.URL,
		ContentURL: options.OSTree.ContentURL,
	}
	rawImg.OSName = "rhel-edge"

	// TODO: move generation into LiveImage
	pt, err := t.GetPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}
	rawImg.PartitionTable = pt

	rawImg.Filename = t.Filename()

	img := image.NewOSTreeSimplifiedInstaller(rawImg, customizations.InstallationDevice)
	img.ExtraBasePackages = packageSets[InstallerPkgsKey]
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

	img.ISOLabel, err = t.ISOLabel()
	if err != nil {
		return nil, err
	}

	d := t.arch.distro
	img.Product = d.product
	img.Variant = "edge"
	img.OSName = "rhel-edge"
	img.OSVersion = d.osVersion

	installerConfig, err := t.getDefaultInstallerConfig()
	if err != nil {
		return nil, err
	}

	if installerConfig != nil {
		img.AdditionalDracutModules = installerConfig.AdditionalDracutModules
	}

	return img, nil
}

func ImageInstallerImage(workload workload.Workload,
	t *ImageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewAnacondaTarInstaller()

	img.Platform = t.platform
	img.Workload = workload

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[OSPkgsKey], options, containers, customizations)
	if err != nil {
		return nil, err
	}

	img.ExtraBasePackages = packageSets[InstallerPkgsKey]

	if t.Arch().Distro().Releasever() == "8" {
		// NOTE: RHEL 8 only supports the older Anaconda configs
		img.UseLegacyAnacondaConfig = true
	}

	img.Kickstart, err = kickstart.New(customizations)
	if err != nil {
		return nil, err
	}
	img.Kickstart.Language = &img.OSCustomizations.Language
	img.Kickstart.Keyboard = img.OSCustomizations.Keyboard
	img.Kickstart.Timezone = &img.OSCustomizations.Timezone

	installerConfig, err := t.getDefaultInstallerConfig()
	if err != nil {
		return nil, err
	}

	if installerConfig != nil {
		img.AdditionalDracutModules = installerConfig.AdditionalDracutModules
		img.AdditionalDrivers = installerConfig.AdditionalDrivers
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

	img.SquashfsCompression = "xz"

	// put the kickstart file in the root of the iso
	img.ISORootKickstart = true

	img.ISOLabel, err = t.ISOLabel()
	if err != nil {
		return nil, err
	}

	d := t.arch.distro
	img.Product = d.product
	img.OSVersion = d.osVersion
	img.Release = fmt.Sprintf("%s %s", d.product, d.osVersion)

	img.Filename = t.Filename()

	return img, nil
}

func TarImage(workload workload.Workload,
	t *ImageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewArchive()
	img.Platform = t.platform

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[OSPkgsKey], options, containers, customizations)
	if err != nil {
		return nil, err
	}

	img.Environment = t.Environment
	img.Workload = workload

	img.Filename = t.Filename()

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
