package fedora

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
	"github.com/osbuild/osbuild-composer/internal/workload"
)

func qcow2Manifest(m *manifest.Manifest,
	workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, workload, t, repos, packageSets[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	imagePipeline := manifest.NewLiveImgPipeline(m, buildPipeline, treePipeline, "disk.img")
	qcow2Pipeline := manifest.NewQCOW2Pipeline(m, buildPipeline, imagePipeline, t.filename)
	qcow2Pipeline.Compat = "1.1"

	return nil
}

func vhdManifest(m *manifest.Manifest,
	workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, workload, t, repos, packageSets[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	imagePipeline := manifest.NewLiveImgPipeline(m, buildPipeline, treePipeline, "disk.img")
	manifest.NewVPCPipeline(m, buildPipeline, imagePipeline, t.filename)

	return nil
}

func vmdkManifest(m *manifest.Manifest,
	workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, workload, t, repos, packageSets[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	imagePipeline := manifest.NewLiveImgPipeline(m, buildPipeline, treePipeline, "disk.img")
	manifest.NewVMDKPipeline(m, buildPipeline, imagePipeline, t.filename)

	return nil
}

func openstackManifest(m *manifest.Manifest,
	workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, workload, t, repos, packageSets[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	imagePipeline := manifest.NewLiveImgPipeline(m, buildPipeline, treePipeline, "disk.img")
	manifest.NewQCOW2Pipeline(m, buildPipeline, imagePipeline, t.filename)

	return nil
}

func ec2CommonManifest(m *manifest.Manifest,
	workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand,
	diskfile string) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, workload, t, repos, packageSets[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil
	}
	manifest.NewLiveImgPipeline(m, buildPipeline, treePipeline, diskfile)

	return nil
}

// ec2Manifest returns a manifest which produce uncompressed EC2 images which are expected to use RHSM for content
func ec2Manifest(m *manifest.Manifest,
	workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) error {
	return ec2CommonManifest(m, workload, t, customizations, options, repos, packageSets, rng, t.Filename())
}

func iotInstallerManifest(m *manifest.Manifest,
	workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)

	d := t.arch.distro
	ksUsers := len(customizations.GetUsers())+len(customizations.GetGroups()) > 0

	anacondaTreePipeline := anacondaTreePipeline(m, buildPipeline, repos, packageSets[installerPkgsKey], t.Arch().Name(), d.product, d.osVersion, "IoT", ksUsers)
	isoTreePipeline := bootISOTreePipeline(m, buildPipeline, anacondaTreePipeline, options, d.vendor, d.isolabelTmpl, customizations.GetUsers(), customizations.GetGroups())
	bootISOPipeline(m, buildPipeline, isoTreePipeline, t.Filename(), false)

	return nil
}

func iotCorePipelines(m *manifest.Manifest,
	workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string]rpmmd.PackageSet) (*manifest.BuildPipeline,
	*manifest.OSTreeCommitPipeline,
	error) {
	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, workload, t, repos, packageSets[osPkgsKey], customizations, options, nil)
	if err != nil {
		return nil, nil, err
	}
	commitPipeline := ostreeCommitPipeline(m, buildPipeline, treePipeline, options, t.arch.distro.osVersion)

	return buildPipeline, commitPipeline, nil
}

func iotCommitManifest(m *manifest.Manifest,
	workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline, commitPipeline, err := iotCorePipelines(m, workload, t, customizations, options, repos, packageSets)
	if err != nil {
		return err
	}
	manifest.NewTarPipeline(m, buildPipeline, &commitPipeline.BasePipeline, "commit-archive", t.Filename())

	return nil
}

func iotContainerManifest(m *manifest.Manifest,
	workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) error {
	buildPipeline, commitPipeline, err := iotCorePipelines(m, workload, t, customizations, options, repos, packageSets)
	if err != nil {
		return err
	}

	nginxConfigPath := "/etc/nginx.conf"
	httpPort := "8080"
	containerTreePipeline := containerTreePipeline(m, buildPipeline, commitPipeline, repos, packageSets[containerPkgsKey], options, customizations, nginxConfigPath, httpPort)
	containerPipeline(m, buildPipeline, &containerTreePipeline.BasePipeline, t, nginxConfigPath, httpPort)

	return nil
}

func containerManifest(m *manifest.Manifest,
	workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSets map[string]rpmmd.PackageSet,
	rng *rand.Rand) error {

	buildPipeline := manifest.NewBuildPipeline(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, workload, t, repos, packageSets[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	manifest.NewOCIContainerPipeline(m, buildPipeline, &treePipeline.BasePipeline, t.Arch().Name(), t.Filename())

	return nil
}

func osPipeline(m *manifest.Manifest,
	buildPipeline *manifest.BuildPipeline,
	workload workload.Workload,
	t *imageType,
	repos []rpmmd.RepoConfig,
	osPackageSet rpmmd.PackageSet,
	c *blueprint.Customizations,
	options distro.ImageOptions,
	rng *rand.Rand) (*manifest.OSPipeline, error) {

	imageConfig := t.getDefaultImageConfig()

	pl := manifest.NewOSPipeline(m, buildPipeline, t.platform, repos)
	pl.Environment = t.environment
	pl.Workload = workload

	if t.bootable {
		var err error
		pt, err := t.getPartitionTable(c.GetFilesystems(), options, rng)
		if err != nil {
			return nil, err
		}
		pl.PartitionTable = pt
	}

	if t.bootable || t.rpmOstree {
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

	pl.ExtraBasePackages = osPackageSet.Include
	pl.ExcludeBasePackages = osPackageSet.Exclude
	pl.ExtraBaseRepos = osPackageSet.Repositories

	pl.GPGKeyFiles = imageConfig.GPGKeyFiles
	pl.ExcludeDocs = imageConfig.ExcludeDocs

	if !t.bootISO {
		// don't put users and groups in the payload of an installer
		// add them via kickstart instead
		pl.Groups = c.GetGroups()
		pl.Users = c.GetUsers()
	}

	pl.EnabledServices = imageConfig.EnabledServices
	pl.DisabledServices = imageConfig.DisabledServices
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
	containerPackageSet rpmmd.PackageSet,
	options distro.ImageOptions,
	c *blueprint.Customizations,
	nginxConfigPath,
	listenPort string) *manifest.OSTreeCommitServerTreePipeline {
	p := manifest.NewOSTreeCommitServerTreePipeline(m, buildPipeline, repos, commitPipeline, nginxConfigPath, listenPort)
	p.ExtraPackages = containerPackageSet.Include
	p.ExtraRepos = containerPackageSet.Repositories
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
	installerPackageSet rpmmd.PackageSet,
	arch, product, osVersion, variant string,
	users bool) *manifest.AnacondaPipeline {
	p := manifest.NewAnacondaPipeline(m, buildPipeline, repos, "kernel", arch, product, osVersion)
	p.ExtraPackages = installerPackageSet.Include
	p.ExtraRepos = installerPackageSet.Repositories

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
