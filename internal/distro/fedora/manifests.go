package fedora

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func qcow2Manifest(m *manifest.Manifest,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSetChains map[string][]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, t, repos, packageSetChains[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	imagePipeline := manifest.NewLiveImgPipeline(m, buildPipeline, treePipeline, "disk.img")
	qcow2Pipeline := manifest.NewQCOW2Pipeline(m, buildPipeline, imagePipeline, t.filename)
	qcow2Pipeline.Compat = "1.1"

	return nil
}

func vhdManifest(m *manifest.Manifest,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSetChains map[string][]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, t, repos, packageSetChains[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	imagePipeline := manifest.NewLiveImgPipeline(m, buildPipeline, treePipeline, "disk.img")
	manifest.NewVPCPipeline(m, buildPipeline, imagePipeline, t.filename)

	return nil
}

func vmdkManifest(m *manifest.Manifest,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSetChains map[string][]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, t, repos, packageSetChains[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	imagePipeline := manifest.NewLiveImgPipeline(m, buildPipeline, treePipeline, "disk.img")
	manifest.NewVMDKPipeline(m, buildPipeline, imagePipeline, t.filename)

	return nil
}

func openstackManifest(m *manifest.Manifest,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSetChains map[string][]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, t, repos, packageSetChains[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	imagePipeline := manifest.NewLiveImgPipeline(m, buildPipeline, treePipeline, "disk.img")
	manifest.NewQCOW2Pipeline(m, buildPipeline, imagePipeline, t.filename)

	return nil
}

func ec2CommonManifest(m *manifest.Manifest,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSetChains map[string][]rpmmd.PackageSet,
	rng *rand.Rand,
	diskfile string) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, t, repos, packageSetChains[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil
	}
	manifest.NewLiveImgPipeline(m, buildPipeline, treePipeline, diskfile)

	return nil
}

// ec2Manifest returns a manifest which produce uncompressed EC2 images which are expected to use RHSM for content
func ec2Manifest(m *manifest.Manifest, t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions,
	repos []rpmmd.RepoConfig, packageSetChains map[string][]rpmmd.PackageSet,
	rng *rand.Rand) error {
	return ec2CommonManifest(m, t, customizations, options, repos, packageSetChains, rng, t.Filename())
}

func iotInstallerManifest(m *manifest.Manifest,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSetChains map[string][]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)

	d := t.arch.distro
	ksUsers := len(customizations.GetUsers())+len(customizations.GetGroups()) > 0

	anacondaTreePipeline := anacondaTreePipeline(m, buildPipeline, repos, packageSetChains[installerPkgsKey], t.Arch().Name(), d.product, d.osVersion, "IoT", ksUsers)
	isoTreePipeline := bootISOTreePipeline(m, buildPipeline, anacondaTreePipeline, options, d.vendor, d.isolabelTmpl, customizations.GetUsers(), customizations.GetGroups())
	bootISOPipeline(m, buildPipeline, isoTreePipeline, t.Filename(), false)

	return nil
}

func iotCorePipelines(m *manifest.Manifest,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSetChains map[string][]rpmmd.PackageSet) (*manifest.BuildPipeline,
	*manifest.OSTreeCommitPipeline,
	error) {
	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, t, repos, packageSetChains[osPkgsKey], customizations, options, nil)
	if err != nil {
		return nil, nil, err
	}
	commitPipeline := ostreeCommitPipeline(m, buildPipeline, treePipeline, options, t.arch.distro.osVersion)

	return buildPipeline, commitPipeline, nil
}

func iotCommitManifest(m *manifest.Manifest, t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions,
	repos []rpmmd.RepoConfig, packageSetChains map[string][]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline, commitPipeline, err := iotCorePipelines(m, t, customizations, options, repos, packageSetChains)
	if err != nil {
		return err
	}
	manifest.NewTarPipeline(m, buildPipeline, &commitPipeline.BasePipeline, "commit-archive", t.Filename())

	return nil
}

func iotContainerManifest(m *manifest.Manifest, t *imageType, customizations *blueprint.Customizations,
	options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetChains map[string][]rpmmd.PackageSet,
	rng *rand.Rand) error {
	buildPipeline, commitPipeline, err := iotCorePipelines(m, t, customizations, options, repos, packageSetChains)
	if err != nil {
		return err
	}

	nginxConfigPath := "/etc/nginx.conf"
	httpPort := "8080"
	containerTreePipeline := containerTreePipeline(m, buildPipeline, commitPipeline, repos, packageSetChains[containerPkgsKey], options, customizations, nginxConfigPath, httpPort)
	containerPipeline(m, buildPipeline, &containerTreePipeline.BasePipeline, t, nginxConfigPath, httpPort)

	return nil
}

func containerManifest(m *manifest.Manifest,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSetChains map[string][]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, t, repos, packageSetChains[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	manifest.NewOCIContainerPipeline(m, buildPipeline, &treePipeline.BasePipeline, t.Arch().Name(), t.Filename())

	return nil
}

func osPipeline(m *manifest.Manifest,
	buildPipeline *manifest.BuildPipeline,
	t *imageType,
	repos []rpmmd.RepoConfig,
	osChain []rpmmd.PackageSet,
	c *blueprint.Customizations,
	options distro.ImageOptions,
	rng *rand.Rand) (*manifest.OSPipeline, error) {

	imageConfig := t.getDefaultImageConfig()

	var arch manifest.Arch
	switch t.Arch().Name() {
	case distro.X86_64ArchName:
		arch = manifest.ARCH_X86_64
	case distro.Aarch64ArchName:
		arch = manifest.ARCH_AARCH64
	case distro.Ppc64leArchName:
		arch = manifest.ARCH_PPC64LE
	case distro.S390xArchName:
		arch = manifest.ARCH_S390X
	}

	pl := manifest.NewOSPipeline(m, buildPipeline, arch, repos)

	if t.bootable {
		var err error
		pt, err := t.getPartitionTable(c.GetFilesystems(), options, rng)
		if err != nil {
			return nil, err
		}
		pl.PartitionTable = pt

		if t.supportsUEFI() {
			pl.UEFIVendor = t.arch.distro.vendor
		}

		pl.BIOSPlatform = t.arch.legacy

		pl.KernelName = c.GetKernel().Name

		var kernelOptions []string
		if t.kernelOptions != "" {
			kernelOptions = append(kernelOptions, t.kernelOptions)
		}
		if bpKernel := c.GetKernel(); bpKernel.Append != "" {
			kernelOptions = append(kernelOptions, bpKernel.Append)
		}
		pl.KernelOptionsAppend = kernelOptions
	}

	if t.rpmOstree {
		var parent *manifest.OSPipelineOSTreeParent
		if options.OSTree.Parent != "" && options.OSTree.URL != "" {
			parent = &manifest.OSPipelineOSTreeParent{
				Checksum: options.OSTree.Parent,
				URL:      options.OSTree.URL,
			}
		}
		pl.OSTree = &manifest.OSPipelineOSTree{
			Parent: parent,
		}
	}

	if len(osChain) >= 1 {
		pl.ExtraBasePackages = osChain[0].Include
		pl.ExcludeBasePackages = osChain[0].Exclude
	}
	if len(osChain) >= 2 {
		pl.UserPackages = osChain[1].Include
		pl.UserRepos = osChain[1].Repositories
	}
	if len(osChain) > 2 {
		panic("unexpected number of package sets in os chain")
	}

	pl.GPGKeyFiles = imageConfig.GPGKeyFiles
	pl.ExcludeDocs = imageConfig.ExcludeDocs

	if !t.bootISO {
		// don't put users and groups in the payload of an installer
		// add them via kickstart instead
		pl.Groups = c.GetGroups()
		pl.Users = c.GetUsers()
	}

	services := &blueprint.ServicesCustomization{
		Enabled:  imageConfig.EnabledServices,
		Disabled: imageConfig.DisabledServices,
	}
	if extraServices := c.GetServices(); extraServices != nil {
		services.Enabled = append(services.Enabled, extraServices.Enabled...)
		services.Disabled = append(services.Disabled, extraServices.Disabled...)
	}
	pl.EnabledServices = services.Enabled
	pl.DisabledServices = services.Disabled
	pl.DefaultTarget = imageConfig.DefaultTarget

	pl.Firewall = c.GetFirewall()

	language, keyboard := c.GetPrimaryLocale()
	if language != nil {
		pl.Language = *language
	} else {
		pl.Language = imageConfig.Locale
	}
	if keyboard != nil {
		pl.Keyboard = keyboard
	} else if imageConfig.Keyboard != nil {
		pl.Keyboard = &imageConfig.Keyboard.Keymap
	}

	if hostname := c.GetHostname(); hostname != nil {
		pl.Hostname = *hostname
	}

	timezone, ntpServers := c.GetTimezoneSettings()
	if timezone != nil {
		pl.Timezone = *timezone
	} else {
		pl.Timezone = imageConfig.Timezone
	}

	if len(ntpServers) > 0 {
		pl.NTPServers = ntpServers
	} else if imageConfig.TimeSynchronization != nil {
		pl.NTPServers = imageConfig.TimeSynchronization.Timeservers
	}

	if imageConfig.NoSElinux {
		pl.SElinux = ""
	}

	pl.Grub2Config = imageConfig.Grub2Config
	pl.Sysconfig = imageConfig.Sysconfig
	pl.SystemdLogind = imageConfig.SystemdLogind
	pl.CloudInit = imageConfig.CloudInit
	pl.Modprobe = imageConfig.Modprobe
	pl.DracutConf = imageConfig.DracutConf
	pl.SystemdUnit = imageConfig.SystemdUnit
	pl.Authselect = imageConfig.Authselect
	pl.SELinuxConfig = imageConfig.SELinuxConfig
	pl.Tuned = imageConfig.Tuned
	pl.Tmpfilesd = imageConfig.Tmpfilesd
	pl.PamLimitsConf = imageConfig.PamLimitsConf
	pl.Sysctld = imageConfig.Sysctld
	pl.DNFConfig = imageConfig.DNFConfig
	pl.SshdConfig = imageConfig.SshdConfig
	pl.AuthConfig = imageConfig.Authconfig
	pl.PwQuality = imageConfig.PwQuality
	pl.WAAgentConfig = imageConfig.WAAgentConfig

	return pl, nil
}

func ostreeCommitPipeline(m *manifest.Manifest,
	buildPipeline *manifest.BuildPipeline,
	treePipeline *manifest.OSPipeline,
	options distro.ImageOptions,
	osVersion string) *manifest.OSTreeCommitPipeline {
	p := manifest.NewOSTreeCommitPipeline(m, buildPipeline, treePipeline, options.OSTree.Ref)
	p.OSVersion = osVersion
	return p
}

func containerTreePipeline(m *manifest.Manifest,
	buildPipeline *manifest.BuildPipeline,
	commitPipeline *manifest.OSTreeCommitPipeline,
	repos []rpmmd.RepoConfig,
	containerChain []rpmmd.PackageSet,
	options distro.ImageOptions,
	c *blueprint.Customizations,
	nginxConfigPath,
	listenPort string) *manifest.OSTreeCommitServerTreePipeline {
	p := manifest.NewOSTreeCommitServerTreePipeline(m, buildPipeline, repos, commitPipeline, nginxConfigPath, listenPort)
	if len(containerChain) >= 1 {
		p.ExtraPackages = containerChain[0].Include
	}
	if len(containerChain) > 2 {
		panic("unexpected number of package sets in os chain")
	}
	language, _ := c.GetPrimaryLocale()
	if language != nil {
		p.Language = *language
	}
	return p
}

func containerPipeline(m *manifest.Manifest,
	buildPipeline *manifest.BuildPipeline,
	treePipeline *manifest.BasePipeline,
	t *imageType,
	nginxConfigPath,
	listenPort string) *manifest.OCIContainerPipeline {
	p := manifest.NewOCIContainerPipeline(m, buildPipeline, treePipeline, t.Arch().Name(), t.Filename())
	p.Cmd = []string{"nginx", "-c", nginxConfigPath}
	p.ExposedPorts = []string{listenPort}
	return p
}

func anacondaTreePipeline(m *manifest.Manifest,
	buildPipeline *manifest.BuildPipeline,
	repos []rpmmd.RepoConfig,
	chain []rpmmd.PackageSet,
	arch, product, osVersion, variant string,
	users bool) *manifest.AnacondaPipeline {
	p := manifest.NewAnacondaPipeline(m, buildPipeline, repos, "kernel", arch, product, osVersion)
	if len(chain) >= 1 {
		p.ExtraPackages = chain[0].Include
	}
	if len(chain) > 1 {
		panic("unexpected number of package sets in installer chain")
	}
	p.Users = users
	p.Variant = variant
	p.Biosdevname = (arch == distro.X86_64ArchName)
	return p
}

func bootISOTreePipeline(m *manifest.Manifest,
	buildPipeline *manifest.BuildPipeline,
	anacondaPipeline *manifest.AnacondaPipeline,
	options distro.ImageOptions,
	vendor,
	isoLabelTempl string,
	users []blueprint.UserCustomization,
	groups []blueprint.GroupCustomization) *manifest.ISOTreePipeline {
	p := manifest.NewISOTreePipeline(m, buildPipeline, anacondaPipeline, options.OSTree.Parent, options.OSTree.URL, options.OSTree.Ref, isoLabelTempl)
	p.Release = "202010217.n.0"
	p.OSName = "fedora"
	p.UEFIVendor = vendor
	p.Users = users
	p.Groups = groups

	return p
}

func bootISOPipeline(m *manifest.Manifest,
	buildPipeline *manifest.BuildPipeline,
	treePipeline *manifest.ISOTreePipeline,
	filename string,
	isolinux bool) *manifest.ISOPipeline {
	p := manifest.NewISOPipeline(m, buildPipeline, treePipeline, filename)
	p.ISOLinux = isolinux
	return p
}
