package fedora

import (
	"fmt"
	"math/rand"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/container"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/fdo"
	"github.com/osbuild/osbuild-composer/internal/ignition"
	"github.com/osbuild/osbuild-composer/internal/image"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/osbuild"
	"github.com/osbuild/osbuild-composer/internal/ostree"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/users"
	"github.com/osbuild/osbuild-composer/internal/workload"
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

	if oscapConfig := c.GetOpenSCAP(); oscapConfig != nil {
		if t.rpmOstree {
			panic("unexpected oscap options for ostree image type")
		}
		osc.OpenSCAPConfig = osbuild.NewOscapRemediationStageOptions(
			osbuild.OscapConfig{
				Datastream: oscapConfig.DataStream,
				ProfileID:  oscapConfig.ProfileID,
			},
		)
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

	return osc
}

// IMAGES

func liveImage(workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewLiveImage()
	img.Platform = t.platform
	img.OSCustomizations = osCustomizations(t, packageSets[osPkgsKey], containers, customizations)
	img.Environment = t.environment
	img.Workload = workload
	// TODO: move generation into LiveImage
	pt, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}
	img.PartitionTable = pt

	img.Filename = t.Filename()

	return img, nil
}

func containerImage(workload workload.Workload,
	t *imageType,
	c *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {
	img := image.NewBaseContainer()

	img.Platform = t.platform
	img.OSCustomizations = osCustomizations(t, packageSets[osPkgsKey], containers, c)
	img.Environment = t.environment
	img.Workload = workload

	img.Filename = t.Filename()

	return img, nil
}

func liveInstallerImage(workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewAnacondaLiveInstaller()

	distro := t.Arch().Distro()

	// If the live installer is generated for Fedora 39 or higher then we enable the web ui
	// kernel options. This is a temporary thing as the check for this should really lie with
	// anaconda and their `liveinst` script to determine which frontend to start.
	if common.VersionLessThan(distro.Releasever(), "39") {
		img.AdditionalKernelOpts = []string{}
	} else {
		img.AdditionalKernelOpts = []string{"inst.webui"}
	}

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
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewAnacondaTarInstaller()

	// Enable anaconda-webui for Fedora > 38
	distro := t.Arch().Distro()
	if strings.HasPrefix(distro.Name(), "fedora") && !common.VersionLessThan(distro.Releasever(), "38") {
		img.AdditionalAnacondaModules = []string{
			"org.fedoraproject.Anaconda.Modules.Security",
			"org.fedoraproject.Anaconda.Modules.Timezone",
			"org.fedoraproject.Anaconda.Modules.Localization",
		}
		img.AdditionalKernelOpts = []string{"inst.webui", "inst.webui.remote"}
	}
	img.AdditionalAnacondaModules = append(img.AdditionalAnacondaModules, "org.fedoraproject.Anaconda.Modules.Users")

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
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	parentCommit, commitRef := makeOSTreeParentCommit(options.OSTree, t.OSTreeRef())
	img := image.NewOSTreeArchive(commitRef)

	distro := t.Arch().Distro()

	img.Platform = t.platform
	img.OSCustomizations = osCustomizations(t, packageSets[osPkgsKey], containers, customizations)
	if strings.HasPrefix(distro.Name(), "fedora") && !common.VersionLessThan(distro.Releasever(), "38") {
		img.OSCustomizations.EnabledServices = append(img.OSCustomizations.EnabledServices, "ignition-firstboot-complete.service", "coreos-ignition-write-issues.service")
	}
	img.Environment = t.environment
	img.Workload = workload
	img.OSTreeParent = parentCommit
	img.OSVersion = t.arch.distro.osVersion
	img.Filename = t.Filename()

	return img, nil
}

func iotContainerImage(workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	parentCommit, commitRef := makeOSTreeParentCommit(options.OSTree, t.OSTreeRef())
	img := image.NewOSTreeContainer(commitRef)

	distro := t.Arch().Distro()

	img.Platform = t.platform
	img.OSCustomizations = osCustomizations(t, packageSets[osPkgsKey], containers, customizations)
	if strings.HasPrefix(distro.Name(), "fedora") && !common.VersionLessThan(distro.Releasever(), "38") {
		img.OSCustomizations.EnabledServices = append(img.OSCustomizations.EnabledServices, "ignition-firstboot-complete.service", "coreos-ignition-write-issues.service")
	}
	img.ContainerLanguage = img.OSCustomizations.Language
	img.Environment = t.environment
	img.Workload = workload
	img.OSTreeParent = parentCommit
	img.OSVersion = t.arch.distro.osVersion
	img.ExtraContainerPackages = packageSets[containerPkgsKey]
	img.Filename = t.Filename()

	return img, nil
}

func iotInstallerImage(workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
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
	img.OSName = "fedora"
	img.OSVersion = d.osVersion
	img.Release = fmt.Sprintf("%s %s", d.product, d.osVersion)

	img.Filename = t.Filename()

	return img, nil
}

func iotQcow2Image(workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	commit, err := makeOSTreePayloadCommit(options.OSTree, t.OSTreeRef())
	if err != nil {
		return nil, fmt.Errorf("%s: %s", t.Name(), err.Error())
	}
	img := image.NewOSTreeImage(commit)

	distro := t.Arch().Distro()

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

	// Set sysroot read-only only for Fedora 37+
	if strings.HasPrefix(distro.Name(), "fedora") && !common.VersionLessThan(distro.Releasever(), "37") {
		img.SysrootReadOnly = true
		img.KernelOptionsAppend = append(img.KernelOptionsAppend, "rw")
	}

	img.Platform = t.platform
	img.Workload = workload

	img.Remote = ostree.Remote{
		Name:        "fedora-iot",
		URL:         "https://ostree.fedoraproject.org/iot",
		ContentURL:  "mirrorlist=https://ostree.fedoraproject.org/iot/mirrorlist",
		GPGKeyPaths: []string{"/etc/pki/rpm-gpg/"},
	}
	img.OSName = "fedora-iot"

	if strings.HasPrefix(distro.Name(), "fedora") && !common.VersionLessThan(distro.Releasever(), "38") {
		img.Ignition = true
		img.IgnitionPlatform = "qemu"
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

	return img, nil
}

func iotRawImage(workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	commit, err := makeOSTreePayloadCommit(options.OSTree, t.OSTreeRef())
	if err != nil {
		return nil, fmt.Errorf("%s: %s", t.Name(), err.Error())
	}
	img := image.NewOSTreeImage(commit)
	img.Compression = "xz"

	distro := t.Arch().Distro()

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

	// Set sysroot read-only only for Fedora 37+
	if strings.HasPrefix(distro.Name(), "fedora") && !common.VersionLessThan(distro.Releasever(), "37") {
		img.SysrootReadOnly = true
		img.KernelOptionsAppend = append(img.KernelOptionsAppend, "rw")
	}

	img.Platform = t.platform
	img.Workload = workload

	img.Remote = ostree.Remote{
		Name:        "fedora-iot",
		URL:         "https://ostree.fedoraproject.org/iot",
		ContentURL:  "mirrorlist=https://ostree.fedoraproject.org/iot/mirrorlist",
		GPGKeyPaths: []string{"/etc/pki/rpm-gpg/"},
	}
	img.OSName = "fedora-iot"

	if strings.HasPrefix(distro.Name(), "fedora") && !common.VersionLessThan(distro.Releasever(), "38") {
		img.Ignition = true
		img.IgnitionPlatform = "metal"
		if bpIgnition := customizations.GetIgnition(); bpIgnition != nil && bpIgnition.FirstBoot != nil && bpIgnition.FirstBoot.ProvisioningURL != "" {
			img.KernelOptionsAppend = append(img.KernelOptionsAppend, "ignition.config.url="+bpIgnition.FirstBoot.ProvisioningURL)
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
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	commit, err := makeOSTreePayloadCommit(options.OSTree, t.OSTreeRef())
	if err != nil {
		return nil, fmt.Errorf("%s: %s", t.Name(), err.Error())
	}
	rawImg := image.NewOSTreeImage(commit)
	rawImg.Compression = "xz"

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
		Name:       "fedora-iot",
		URL:        options.OSTree.URL,
		ContentURL: options.OSTree.ContentURL,
	}
	rawImg.OSName = "fedora"

	if !common.VersionLessThan(t.arch.distro.osVersion, "38") {
		rawImg.Ignition = true
		rawImg.IgnitionPlatform = "metal"
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
