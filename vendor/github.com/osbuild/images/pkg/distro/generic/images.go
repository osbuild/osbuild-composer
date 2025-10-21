package generic

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/anaconda"
	"github.com/osbuild/images/pkg/customizations/bootc"
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

func kernelOptions(t *imageType, c *blueprint.Customizations) []string {
	imageConfig := t.getDefaultImageConfig()

	kernelOptions := imageConfig.KernelOptions
	if bpKernel := c.GetKernel(); bpKernel.Append != "" {
		kernelOptions = append(kernelOptions, bpKernel.Append)
	}
	return kernelOptions
}

func osCustomizations(t *imageType, osPackageSet rpmmd.PackageSet, options distro.ImageOptions, containers []container.SourceSpec, bp *blueprint.Blueprint) (manifest.OSCustomizations, error) {
	c := bp.Customizations
	osc := manifest.OSCustomizations{}

	imageConfig := t.getDefaultImageConfig()
	if t.ImageTypeYAML.Bootable || t.ImageTypeYAML.RPMOSTree {
		// TODO: for now the only image types that define a default kernel are
		// ones that use UKIs and don't allow overriding, so this works.
		// However, if we ever need to specify default kernels for image types
		// that allow overriding, we will need to change c.GetKernel() to take
		// an argument as fallback or make it not return the standard "kernel"
		// when it's unset.
		osc.KernelName = c.GetKernel().Name
		if imageConfig.DefaultKernelName != nil {
			osc.KernelName = *imageConfig.DefaultKernelName
		}
		osc.KernelOptionsAppend = kernelOptions(t, c)
		if imageConfig.KernelOptionsBootloader != nil {
			osc.KernelOptionsBootloader = *imageConfig.KernelOptionsBootloader
		}
	}

	osc.FIPS = c.GetFIPS()

	osc.BasePackages = osPackageSet.Include
	osc.ExcludeBasePackages = osPackageSet.Exclude
	osc.ExtraBaseRepos = osPackageSet.Repositories
	// false here means bootable=false which means the kernel
	// package is excluded
	osc.BlueprintPackages = bp.GetPackagesEx(false)
	osc.BlueprintModules = bp.GetEnabledModules()
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
	if services := bp.Customizations.GetServices(); services != nil {
		osc.EnabledServices = append(osc.EnabledServices, services.Enabled...)
		osc.DisabledServices = append(osc.DisabledServices, services.Disabled...)
		osc.MaskedServices = append(osc.MaskedServices, services.Masked...)
	}

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
	} else if imageConfig.Hostname != nil {
		osc.Hostname = *imageConfig.Hostname
	}

	if imageConfig.InstallWeakDeps != nil {
		osc.InstallWeakDeps = *imageConfig.InstallWeakDeps
	}

	osc.InstallLangs = imageConfig.InstallLangs

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

	// Relabel the tree, unless the `NoSELinux` flag is explicitly set to `true`
	if imageConfig.NoSELinux == nil || imageConfig.NoSELinux != nil && !*imageConfig.NoSELinux {
		osc.SELinux = "targeted"
		osc.SELinuxForceRelabel = imageConfig.SELinuxForceRelabel
	}

	// XXX: move into pure YAML
	if strings.HasPrefix(t.Arch().Distro().Name(), "rhel-") && options.Facts != nil {
		osc.RHSMFacts = options.Facts
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

	var subscriptionStatus subscription.RHSMStatus
	if options.Subscription != nil {
		subscriptionStatus = subscription.RHSMConfigWithSubscription
		if options.Subscription.Proxy != "" {
			osc.InsightsClientConfig = &osbuild.InsightsClientConfigStageOptions{Config: osbuild.InsightsClientConfig{Proxy: options.Subscription.Proxy}}
		}
		osc.PermissiveRHC = imageConfig.PermissiveRHC
	} else {
		subscriptionStatus = subscription.RHSMConfigNoSubscription
	}
	if rhsmConfig, exists := imageConfig.RHSMConfig[subscriptionStatus]; exists {
		osc.RHSMConfig = rhsmConfig
	}

	if bpRhsmConfig := subscription.RHSMConfigFromBP(c.GetRHSM()); bpRhsmConfig != nil {
		osc.RHSMConfig = osc.RHSMConfig.Update(bpRhsmConfig)
	}

	dnfConfig, err := imageConfig.DNFConfigOptions(t.arch.distro.OsVersion())
	if err != nil {
		panic(fmt.Errorf("error creating dnf configs: %w", err))
	}
	if bpDnf := c.GetDNF(); bpDnf != nil && bpDnf.Config != nil {
		if dnfConfig == nil {
			dnfConfig = &osbuild.DNFConfigStageOptions{}
		}
		if bpDnf.Config.SetReleaseVer {
			// NOTE: currently this is either a no-op or an append (i.e. it
			// will never change the value of an existing releasever), but will
			// become useful if we ever support adding custom variables through
			// the blueprint
			dnfConfig.UpdateVar("releasever", t.arch.distro.OsVersion())
		}
	}

	osc.DNFConfig = dnfConfig

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
	osc.DNFAutomaticConfig = imageConfig.DNFAutomaticConfig
	osc.YUMConfig = imageConfig.YumConfig
	osc.SshdConfig = imageConfig.SshdConfig
	osc.AuthConfig = imageConfig.Authconfig
	osc.PwQuality = imageConfig.PwQuality
	osc.Subscription = options.Subscription
	osc.WAAgentConfig = imageConfig.WAAgentConfig
	osc.UdevRules = imageConfig.UdevRules
	osc.GCPGuestAgentConfig = imageConfig.GCPGuestAgentConfig
	osc.NetworkManager = imageConfig.NetworkManager

	if imageConfig.WSL != nil {
		osc.WSLConfig = osbuild.NewWSLConfStageOptions(imageConfig.WSL.Config)
		osc.WSLDistributionConfig = osbuild.NewWSLDistributionConfStageOptions(imageConfig.WSL.DistributionConfig)
	}

	osc.Files = append(osc.Files, imageConfig.Files...)
	osc.Directories = append(osc.Directories, imageConfig.Directories...)

	if imageConfig.NoBLS != nil {
		osc.NoBLS = *imageConfig.NoBLS
	}

	ca, err := c.GetCACerts()
	if err != nil {
		panic(fmt.Sprintf("unexpected error checking CA certs: %v", err))
	}
	if ca != nil {
		osc.CACerts = ca.PEMCerts
	}

	if imageConfig.InstallWeakDeps != nil {
		osc.InstallWeakDeps = *imageConfig.InstallWeakDeps
	}

	if imageConfig.MachineIdUninitialized != nil {
		osc.MachineIdUninitialized = *imageConfig.MachineIdUninitialized
	}

	if imageConfig.MountUnits != nil && *imageConfig.MountUnits {
		osc.MountConfiguration = osbuild.MOUNT_CONFIGURATION_UNITS
	}

	osc.VersionlockPackages = imageConfig.VersionlockPackages

	return osc, nil
}

func installerCustomizations(t *imageType, c *blueprint.Customizations) (manifest.InstallerCustomizations, error) {
	d := t.arch.distro
	isoLabel, err := t.ISOLabel()
	if err != nil {
		return manifest.InstallerCustomizations{}, err
	}

	isc := manifest.InstallerCustomizations{
		FIPS:                    c.GetFIPS(),
		UseLegacyAnacondaConfig: t.ImageTypeYAML.UseLegacyAnacondaConfig,
		Product:                 d.Product(),
		OSVersion:               d.OsVersion(),
		Release:                 fmt.Sprintf("%s %s", d.Product(), d.OsVersion()),
		Preview:                 d.DistroYAML.Preview,
		ISOLabel:                isoLabel,
		Variant:                 t.Variant,
	}

	installerConfig, err := t.getDefaultInstallerConfig()
	if err != nil {
		return isc, err
	}

	if installerConfig != nil {
		isc.EnabledAnacondaModules = append(isc.EnabledAnacondaModules, installerConfig.EnabledAnacondaModules...)
		isc.AdditionalDracutModules = append(isc.AdditionalDracutModules, installerConfig.AdditionalDracutModules...)
		isc.AdditionalDrivers = append(isc.AdditionalDrivers, installerConfig.AdditionalDrivers...)

		if menu := installerConfig.DefaultMenu; menu != nil {
			isc.DefaultMenu = *menu
		}

		if isoroot := installerConfig.ISORootfsType; isoroot != nil {
			isc.ISORootfsType = *isoroot
		}

		if isoboot := installerConfig.ISOBootType; isoboot != nil {
			isc.ISOBoot = *isoboot
		}

		for _, tmpl := range installerConfig.LoraxTemplates {
			isc.LoraxTemplates = append(isc.LoraxTemplates, manifest.InstallerLoraxTemplate{
				Path:        tmpl.Path,
				AfterDracut: tmpl.AfterDracut,
			})
		}

		if pkg := installerConfig.LoraxTemplatePackage; pkg != nil {
			isc.LoraxTemplatePackage = *pkg
		}
		if pkg := installerConfig.LoraxLogosPackage; pkg != nil {
			isc.LoraxLogosPackage = *pkg
		}
		if pkg := installerConfig.LoraxReleasePackage; pkg != nil {
			isc.LoraxReleasePackage = *pkg
		}
	}

	installerCust, err := c.GetInstaller()
	if err != nil {
		return isc, err
	}

	if installerCust != nil && installerCust.Modules != nil {
		isc.EnabledAnacondaModules = append(isc.EnabledAnacondaModules, installerCust.Modules.Enable...)
		isc.DisabledAnacondaModules = append(isc.DisabledAnacondaModules, installerCust.Modules.Disable...)
	}
	isc.KernelOptionsAppend = kernelOptions(t, c)

	return isc, nil
}

func ostreeDeploymentCustomizations(
	t *imageType,
	c *blueprint.Customizations) (manifest.OSTreeDeploymentCustomizations, error) {

	if !t.ImageTypeYAML.RPMOSTree || !t.ImageTypeYAML.Bootable {
		return manifest.OSTreeDeploymentCustomizations{}, fmt.Errorf("ostree deployment customizations are only supported for bootable rpm-ostree images")
	}
	deploymentConf := manifest.OSTreeDeploymentCustomizations{}

	imageConfig := t.getDefaultImageConfig()
	kernelOptions := imageConfig.KernelOptions
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

func diskImage(t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	payloadRepos []rpmmd.RepoConfig,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewDiskImage(t.platform, t.Filename())

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], options, containers, bp)
	if err != nil {
		return nil, err
	}
	img.OSCustomizations.PayloadRepos = payloadRepos

	img.Environment = &t.ImageTypeYAML.Environment
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

	img.VPCForceSize = t.ImageTypeYAML.DiskImageVPCForceSize

	if img.OSCustomizations.NoBLS {
		img.OSProduct = t.Arch().Distro().Product()
		img.OSVersion = t.Arch().Distro().OsVersion()
		img.OSNick = t.Arch().Distro().Codename()
	}

	if t.ImageTypeYAML.DiskImagePartTool != nil {
		img.PartTool = *t.ImageTypeYAML.DiskImagePartTool
	}

	return img, nil
}

func tarImage(t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	payloadRepos []rpmmd.RepoConfig,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {
	img := image.NewArchive(t.platform, t.Filename())

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], options, containers, bp)
	if err != nil {
		return nil, err
	}
	img.OSCustomizations.PayloadRepos = payloadRepos

	d := t.arch.distro

	img.Environment = &t.ImageTypeYAML.Environment
	img.Compression = t.ImageTypeYAML.Compression
	img.OSVersion = d.OsVersion()

	return img, nil
}

func containerImage(t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	payloadRepos []rpmmd.RepoConfig,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {
	img := image.NewBaseContainer(t.platform, t.Filename())

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], options, containers, bp)
	if err != nil {
		return nil, err
	}
	img.OSCustomizations.PayloadRepos = payloadRepos
	img.Environment = &t.ImageTypeYAML.Environment

	return img, nil
}

func liveInstallerImage(t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	payloadRepos []rpmmd.RepoConfig,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewAnacondaLiveInstaller(t.platform, t.Filename())

	img.ExtraBasePackages = packageSets[installerPkgsKey]

	imgConfig := t.getDefaultImageConfig()
	if locale := imgConfig.Locale; locale != nil {
		img.Locale = *locale
	}

	var err error
	img.InstallerCustomizations, err = installerCustomizations(t, bp.Customizations)
	if err != nil {
		return nil, err
	}

	return img, nil
}

func imageInstallerImage(t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	payloadRepos []rpmmd.RepoConfig,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	customizations := bp.Customizations

	img := image.NewAnacondaTarInstaller(t.platform, t.Filename())

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], options, containers, bp)
	if err != nil {
		return nil, err
	}
	img.OSCustomizations.PayloadRepos = payloadRepos

	img.Kickstart, err = kickstart.New(customizations)
	if err != nil {
		return nil, err
	}
	img.Kickstart.Language = &img.OSCustomizations.Language
	img.Kickstart.Keyboard = img.OSCustomizations.Keyboard
	img.Kickstart.Timezone = &img.OSCustomizations.Timezone

	img.ExtraBasePackages = packageSets[installerPkgsKey]

	img.InstallerCustomizations, err = installerCustomizations(t, bp.Customizations)
	if err != nil {
		return nil, err
	}

	installerConfig, err := t.getDefaultInstallerConfig()
	if err != nil {
		return nil, err
	}

	// XXX these bits should move into the `installerCustomization` function
	// XXX directly
	img.InstallerCustomizations.EnabledAnacondaModules = append(img.InstallerCustomizations.EnabledAnacondaModules, anaconda.ModuleUsers)

	if img.Kickstart.Unattended {
		img.InstallerCustomizations.KernelOptionsAppend = append(installerConfig.KickstartUnattendedExtraKernelOpts, img.InstallerCustomizations.KernelOptionsAppend...)
	}

	img.RootfsCompression = "xz" // This also triggers using the bcj filter

	return img, nil
}

func iotCommitImage(t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	payloadRepos []rpmmd.RepoConfig,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	parentCommit, commitRef := makeOSTreeParentCommit(options.OSTree, t.OSTreeRef())
	img := image.NewOSTreeArchive(t.platform, t.Filename(), commitRef)

	d := t.arch.distro

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], options, containers, bp)
	if err != nil {
		return nil, err
	}
	img.OSCustomizations.PayloadRepos = payloadRepos

	imgConfig := t.getDefaultImageConfig()
	img.OSCustomizations.Presets = imgConfig.Presets
	if imgConfig.InstallWeakDeps != nil {
		img.InstallWeakDeps = *imgConfig.InstallWeakDeps
	}

	img.Environment = &t.ImageTypeYAML.Environment
	img.OSTreeParent = parentCommit
	img.OSVersion = d.OsVersion()

	return img, nil
}

func bootableContainerImage(t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	payloadRepos []rpmmd.RepoConfig,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	parentCommit, commitRef := makeOSTreeParentCommit(options.OSTree, t.OSTreeRef())
	img := image.NewOSTreeArchive(t.platform, t.Filename(), commitRef)

	d := t.arch.distro

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], options, containers, bp)
	if err != nil {
		return nil, err
	}
	img.OSCustomizations.PayloadRepos = payloadRepos

	img.Environment = &t.ImageTypeYAML.Environment
	img.OSTreeParent = parentCommit
	img.OSVersion = d.OsVersion()
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

func iotContainerImage(t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	payloadRepos []rpmmd.RepoConfig,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	parentCommit, commitRef := makeOSTreeParentCommit(options.OSTree, t.OSTreeRef())
	img := image.NewOSTreeContainer(t.platform, t.Filename(), commitRef)
	d := t.arch.distro

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], options, containers, bp)
	if err != nil {
		return nil, err
	}

	imgConfig := t.getDefaultImageConfig()
	img.OSCustomizations.Presets = imgConfig.Presets
	if imgConfig.InstallWeakDeps != nil {
		img.OSCustomizations.InstallWeakDeps = *imgConfig.InstallWeakDeps
	}
	img.OSCustomizations.PayloadRepos = payloadRepos

	img.ContainerLanguage = img.OSCustomizations.Language
	img.Environment = &t.ImageTypeYAML.Environment
	img.OSTreeParent = parentCommit
	img.OSVersion = d.OsVersion()
	img.ExtraContainerPackages = packageSets[containerPkgsKey]

	return img, nil
}

func iotInstallerImage(t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	payloadRepos []rpmmd.RepoConfig,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	commit, err := makeOSTreePayloadCommit(options.OSTree, t.OSTreeRef())
	if err != nil {
		return nil, fmt.Errorf("%s: %s", t.Name(), err.Error())
	}

	img := image.NewAnacondaOSTreeInstaller(t.platform, t.Filename(), commit)

	customizations := bp.Customizations
	img.ExtraBasePackages = packageSets[installerPkgsKey]
	img.Kickstart, err = kickstart.New(customizations)
	if err != nil {
		return nil, err
	}
	img.Kickstart.OSTree = &kickstart.OSTree{
		OSName: t.OSTree.Name,
		Remote: t.OSTree.RemoteName,
	}
	img.Kickstart.Path = osbuild.KickstartPathOSBuild
	img.Kickstart.Language, img.Kickstart.Keyboard = customizations.GetPrimaryLocale()
	// ignore ntp servers - we don't currently support setting these in the
	// kickstart though kickstart does support setting them
	img.Kickstart.Timezone, _ = customizations.GetTimezoneSettings()

	img.InstallerCustomizations, err = installerCustomizations(t, bp.Customizations)
	if err != nil {
		return nil, err
	}

	// XXX these bits should move into the `installerCustomization` function
	// XXX directly
	if len(img.Kickstart.Users)+len(img.Kickstart.Groups) > 0 {
		// only enable the users module if needed
		img.InstallerCustomizations.EnabledAnacondaModules = append(img.InstallerCustomizations.EnabledAnacondaModules, anaconda.ModuleUsers)
	}

	img.RootfsCompression = "xz" // This also triggers using the bcj filter
	imgConfig := t.getDefaultImageConfig()
	if locale := imgConfig.Locale; locale != nil {
		img.Locale = *locale
	}

	return img, nil
}

func iotImage(t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	payloadRepos []rpmmd.RepoConfig,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	commit, err := makeOSTreePayloadCommit(options.OSTree, t.OSTreeRef())
	if err != nil {
		return nil, fmt.Errorf("%s: %s", t.Name(), err.Error())
	}
	img := image.NewOSTreeDiskImageFromCommit(t.platform, t.Filename(), commit)

	customizations := bp.Customizations
	deploymentConfig, err := ostreeDeploymentCustomizations(t, customizations)
	if err != nil {
		return nil, err
	}
	img.OSTreeDeploymentCustomizations = deploymentConfig
	img.OSCustomizations.PayloadRepos = payloadRepos

	img.Remote = ostree.Remote{
		Name: t.ImageTypeYAML.OSTree.RemoteName,
	}
	// XXX: can we do better?
	if t.ImageTypeYAML.UseOstreeRemotes {
		img.Remote.URL = options.OSTree.URL
		img.Remote.ContentURL = options.OSTree.ContentURL
	}

	img.OSName = t.ImageTypeYAML.OSTree.Name

	// TODO: move generation into LiveImage
	pt, err := t.getPartitionTable(customizations, options, rng)
	if err != nil {
		return nil, err
	}
	img.PartitionTable = pt

	img.Compression = t.ImageTypeYAML.Compression

	return img, nil
}

func iotSimplifiedInstallerImage(t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	payloadRepos []rpmmd.RepoConfig,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	commit, err := makeOSTreePayloadCommit(options.OSTree, t.OSTreeRef())
	if err != nil {
		return nil, fmt.Errorf("%s: %s", t.Name(), err.Error())
	}
	rawImg := image.NewOSTreeDiskImageFromCommit(t.platform, t.Filename(), commit)

	customizations := bp.Customizations
	deploymentConfig, err := ostreeDeploymentCustomizations(t, customizations)
	if err != nil {
		return nil, err
	}
	rawImg.OSTreeDeploymentCustomizations = deploymentConfig

	rawImg.OSCustomizations.PayloadRepos = payloadRepos
	rawImg.Remote = ostree.Remote{
		Name: t.OSTree.RemoteName,
	}
	if t.ImageTypeYAML.UseOstreeRemotes {
		rawImg.Remote.URL = options.OSTree.URL
		rawImg.Remote.ContentURL = options.OSTree.ContentURL
	}
	rawImg.OSName = t.OSTree.Name

	// TODO: move generation into LiveImage
	pt, err := t.getPartitionTable(customizations, options, rng)
	if err != nil {
		return nil, err
	}
	rawImg.PartitionTable = pt

	// XXX: can we take platform/filename in NewOSTreeSimplifiedInstaller from rawImg instead?
	img := image.NewOSTreeSimplifiedInstaller(t.platform, t.Filename(), rawImg, customizations.InstallationDevice)
	img.ExtraBasePackages = packageSets[installerPkgsKey]
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
	img.InstallerCustomizations, err = installerCustomizations(t, bp.Customizations)
	if err != nil {
		return nil, err
	}
	img.OSName = t.OSTree.Name

	return img, nil
}

// Make an Anaconda installer boot.iso
func netinstImage(t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	payloadRepos []rpmmd.RepoConfig,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	customizations := bp.Customizations

	img := image.NewAnacondaNetInstaller(t.platform, t.Filename())
	language, _ := customizations.GetPrimaryLocale()
	if language != nil {
		img.Language = *language
	}

	img.ExtraBasePackages = packageSets[installerPkgsKey]

	var err error
	img.InstallerCustomizations, err = installerCustomizations(t, bp.Customizations)
	if err != nil {
		return nil, err
	}

	img.RootfsCompression = "xz" // This also triggers using the bcj filter

	return img, nil
}

func pxeTarImage(t *imageType,
	bp *blueprint.Blueprint,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	payloadRepos []rpmmd.RepoConfig,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {
	img := image.NewPXETar(t.platform, t.Filename())

	var err error
	img.OSCustomizations, err = osCustomizations(t, packageSets[osPkgsKey], options, containers, bp)
	if err != nil {
		return nil, err
	}
	img.OSCustomizations.PayloadRepos = payloadRepos

	d := t.arch.distro

	img.Environment = &t.ImageTypeYAML.Environment
	img.Compression = t.ImageTypeYAML.Compression
	img.OSVersion = d.OsVersion()

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
