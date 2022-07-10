package fedora

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/platform"
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

	buildPipeline := manifest.NewBuild(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, workload, t, repos, packageSets[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	imagePipeline := manifest.NewRawImage(m, buildPipeline, treePipeline)
	qcow2Pipeline := manifest.NewQCOW2(m, buildPipeline, imagePipeline)
	qcow2Pipeline.Filename = t.filename
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

	buildPipeline := manifest.NewBuild(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, workload, t, repos, packageSets[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	imagePipeline := manifest.NewRawImage(m, buildPipeline, treePipeline)
	p := manifest.NewVPC(m, buildPipeline, imagePipeline)
	p.Filename = t.filename

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

	buildPipeline := manifest.NewBuild(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, workload, t, repos, packageSets[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	imagePipeline := manifest.NewRawImage(m, buildPipeline, treePipeline)
	p := manifest.NewVMDK(m, buildPipeline, imagePipeline)
	p.Filename = t.filename

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

	buildPipeline := manifest.NewBuild(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, workload, t, repos, packageSets[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	imagePipeline := manifest.NewRawImage(m, buildPipeline, treePipeline)
	p := manifest.NewQCOW2(m, buildPipeline, imagePipeline)
	p.Filename = t.filename

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

	buildPipeline := manifest.NewBuild(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, workload, t, repos, packageSets[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil
	}
	p := manifest.NewRawImage(m, buildPipeline, treePipeline)
	p.Filename = diskfile

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

	buildPipeline := manifest.NewBuild(m, t.arch.distro.runner, repos)

	d := t.arch.distro
	ksUsers := len(customizations.GetUsers())+len(customizations.GetGroups()) > 0

	anacondaTreePipeline := anacondaTreePipeline(m,
		buildPipeline,
		t.platform,
		repos,
		packageSets[installerPkgsKey],
		d.product,
		d.osVersion,
		"IoT",
		ksUsers)
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
	packageSets map[string]rpmmd.PackageSet) (*manifest.Build,
	*manifest.OSTreeCommit,
	error) {
	buildPipeline := manifest.NewBuild(m, t.arch.distro.runner, repos)
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
	p := manifest.NewTar(m, buildPipeline, &commitPipeline.Base, "commit-archive")
	p.Filename = t.Filename()

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
	containerTreePipeline := containerTreePipeline(m,
		buildPipeline,
		commitPipeline,
		t.platform,
		repos,
		packageSets[containerPkgsKey],
		options,
		customizations,
		nginxConfigPath,
		httpPort)
	containerPipeline(m, buildPipeline, containerTreePipeline, t, nginxConfigPath, httpPort)

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

	buildPipeline := manifest.NewBuild(m, t.arch.distro.runner, repos)
	treePipeline, err := osPipeline(m, buildPipeline, workload, t, repos, packageSets[osPkgsKey], customizations, options, rng)
	if err != nil {
		return err
	}
	ociPipeline := manifest.NewOCIContainer(m, buildPipeline, treePipeline)
	ociPipeline.Filename = t.Filename()

	return nil
}

func osPipeline(m *manifest.Manifest,
	buildPipeline *manifest.Build,
	workload workload.Workload,
	t *imageType,
	repos []rpmmd.RepoConfig,
	osPackageSet rpmmd.PackageSet,
	c *blueprint.Customizations,
	options distro.ImageOptions,
	rng *rand.Rand) (*manifest.OS, error) {
	pl := manifest.NewOS(m, buildPipeline, t.platform, repos)
	pl.Environment = t.environment
	pl.Workload = workload
	pl.OSCustomizations = osCustomizations(t, repos, osPackageSet, c, options)

	if t.bootable {
		var err error
		pt, err := t.getPartitionTable(c.GetFilesystems(), options, rng)
		if err != nil {
			return nil, err
		}
		pl.PartitionTable = pt
	}

	if t.rpmOstree {
		var parent *manifest.OSTreeParent
		if options.OSTree.Parent != "" && options.OSTree.URL != "" {
			parent = &manifest.OSTreeParent{
				Checksum: options.OSTree.Parent,
				URL:      options.OSTree.URL,
			}
		}
		pl.OSTree = &manifest.OSTree{
			Parent: parent,
		}
	}

	return pl, nil
}

func osCustomizations(
	t *imageType,
	repos []rpmmd.RepoConfig,
	osPackageSet rpmmd.PackageSet,
	c *blueprint.Customizations,
	options distro.ImageOptions) manifest.OSCustomizations {

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

	osc.GPGKeyFiles = imageConfig.GPGKeyFiles
	osc.ExcludeDocs = imageConfig.ExcludeDocs

	if !t.bootISO {
		// don't put users and groups in the payload of an installer
		// add them via kickstart instead
		osc.Groups = c.GetGroups()
		osc.Users = c.GetUsers()
	}

	osc.EnabledServices = imageConfig.EnabledServices
	osc.DisabledServices = imageConfig.DisabledServices
	osc.DefaultTarget = imageConfig.DefaultTarget

	osc.Firewall = c.GetFirewall()

	language, keyboard := c.GetPrimaryLocale()
	if language != nil {
		osc.Language = *language
	} else {
		osc.Language = imageConfig.Locale
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
	} else {
		osc.Timezone = imageConfig.Timezone
	}

	if len(ntpServers) > 0 {
		osc.NTPServers = ntpServers
	} else if imageConfig.TimeSynchronization != nil {
		osc.NTPServers = imageConfig.TimeSynchronization.Timeservers
	}

	if !imageConfig.NoSElinux {
		osc.SElinux = "targeted"
	}

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

func ostreeCommitPipeline(m *manifest.Manifest,
	buildPipeline *manifest.Build,
	treePipeline *manifest.OS,
	options distro.ImageOptions,
	osVersion string) *manifest.OSTreeCommit {
	p := manifest.NewOSTreeCommit(m, buildPipeline, treePipeline, options.OSTree.Ref)
	p.OSVersion = osVersion
	return p
}

func containerTreePipeline(m *manifest.Manifest,
	buildPipeline *manifest.Build,
	commitPipeline *manifest.OSTreeCommit,
	platform platform.Platform,
	repos []rpmmd.RepoConfig,
	containerPackageSet rpmmd.PackageSet,
	options distro.ImageOptions,
	c *blueprint.Customizations,
	nginxConfigPath,
	listenPort string) *manifest.OSTreeCommitServer {
	p := manifest.NewOSTreeCommitServer(m, buildPipeline, platform, repos, commitPipeline, nginxConfigPath, listenPort)
	p.ExtraPackages = containerPackageSet.Include
	p.ExtraRepos = containerPackageSet.Repositories
	language, _ := c.GetPrimaryLocale()
	if language != nil {
		p.Language = *language
	}
	return p
}

func containerPipeline(m *manifest.Manifest,
	buildPipeline *manifest.Build,
	treePipeline manifest.Tree,
	t *imageType,
	nginxConfigPath,
	listenPort string) *manifest.OCIContainer {
	p := manifest.NewOCIContainer(m, buildPipeline, treePipeline)
	p.Filename = t.Filename()
	p.Cmd = []string{"nginx", "-c", nginxConfigPath}
	p.ExposedPorts = []string{listenPort}
	return p
}

func anacondaTreePipeline(m *manifest.Manifest,
	buildPipeline *manifest.Build,
	pf platform.Platform,
	repos []rpmmd.RepoConfig,
	installerPackageSet rpmmd.PackageSet,
	product, osVersion, variant string,
	users bool) *manifest.Anaconda {
	p := manifest.NewAnaconda(m, buildPipeline, pf, repos, "kernel", product, osVersion)
	p.ExtraPackages = installerPackageSet.Include
	p.ExtraRepos = installerPackageSet.Repositories

	p.Users = users
	p.Variant = variant
	p.Biosdevname = (pf.GetArch() == platform.ARCH_X86_64)
	return p
}

func bootISOTreePipeline(m *manifest.Manifest,
	buildPipeline *manifest.Build,
	anacondaPipeline *manifest.Anaconda,
	options distro.ImageOptions,
	vendor,
	isoLabelTempl string,
	users []blueprint.UserCustomization,
	groups []blueprint.GroupCustomization) *manifest.ISOTree {
	p := manifest.NewISOTree(m, buildPipeline, anacondaPipeline, options.OSTree.Parent, options.OSTree.URL, options.OSTree.Ref, isoLabelTempl)
	p.Release = "202010217.n.0"
	p.OSName = "fedora"
	p.UEFIVendor = vendor
	p.Users = users
	p.Groups = groups

	return p
}

func bootISOPipeline(m *manifest.Manifest,
	buildPipeline *manifest.Build,
	treePipeline *manifest.ISOTree,
	filename string,
	isolinux bool) *manifest.ISO {
	p := manifest.NewISO(m, buildPipeline, treePipeline)
	p.ISOLinux = isolinux
	p.Filename = filename
	return p
}
