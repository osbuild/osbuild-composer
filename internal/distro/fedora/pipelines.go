package fedora

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/manifest"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func qcow2Pipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]manifest.Pipeline, error) {
	pipelines := make([]manifest.Pipeline, 0)

	buildPipeline := manifest.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	pipelines = append(pipelines, buildPipeline)

	treePipeline, err := osPipeline(&buildPipeline, t, repos, packageSetSpecs[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, treePipeline)

	imagePipeline := manifest.NewLiveImgPipeline(&buildPipeline, &treePipeline, "disk.img")
	pipelines = append(pipelines, imagePipeline)

	qcow2Pipeline := manifest.NewQCOW2Pipeline(&buildPipeline, &imagePipeline, t.filename)
	qcow2Pipeline.Compat = "1.1"
	pipelines = append(pipelines, qcow2Pipeline)

	return pipelines, nil
}

func vhdPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]manifest.Pipeline, error) {
	pipelines := make([]manifest.Pipeline, 0)

	buildPipeline := manifest.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	pipelines = append(pipelines, buildPipeline)

	treePipeline, err := osPipeline(&buildPipeline, t, repos, packageSetSpecs[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, treePipeline)

	imagePipeline := manifest.NewLiveImgPipeline(&buildPipeline, &treePipeline, "disk.img")
	pipelines = append(pipelines, imagePipeline)

	vpcPipeline := manifest.NewVPCPipeline(&buildPipeline, &imagePipeline, t.filename)
	pipelines = append(pipelines, vpcPipeline)
	return pipelines, nil
}

func vmdkPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]manifest.Pipeline, error) {
	pipelines := make([]manifest.Pipeline, 0)

	buildPipeline := manifest.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	pipelines = append(pipelines, buildPipeline)

	treePipeline, err := osPipeline(&buildPipeline, t, repos, packageSetSpecs[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, treePipeline)

	imagePipeline := manifest.NewLiveImgPipeline(&buildPipeline, &treePipeline, "disk.img")
	pipelines = append(pipelines, imagePipeline)

	vmdkPipeline := manifest.NewVMDKPipeline(&buildPipeline, &imagePipeline, t.filename)
	pipelines = append(pipelines, vmdkPipeline)
	return pipelines, nil
}

func openstackPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]manifest.Pipeline, error) {
	pipelines := make([]manifest.Pipeline, 0)

	buildPipeline := manifest.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	pipelines = append(pipelines, buildPipeline)

	treePipeline, err := osPipeline(&buildPipeline, t, repos, packageSetSpecs[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, treePipeline)

	imagePipeline := manifest.NewLiveImgPipeline(&buildPipeline, &treePipeline, "disk.img")
	pipelines = append(pipelines, imagePipeline)

	qcow2Pipeline := manifest.NewQCOW2Pipeline(&buildPipeline, &imagePipeline, t.filename)
	pipelines = append(pipelines, qcow2Pipeline)
	return pipelines, nil
}

func ec2CommonPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions,
	repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec,
	rng *rand.Rand, diskfile string) ([]manifest.Pipeline, error) {
	pipelines := make([]manifest.Pipeline, 0)

	buildPipeline := manifest.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	pipelines = append(pipelines, buildPipeline)

	treePipeline, err := osPipeline(&buildPipeline, t, repos, packageSetSpecs[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, treePipeline)

	imagePipeline := manifest.NewLiveImgPipeline(&buildPipeline, &treePipeline, diskfile)
	pipelines = append(pipelines, imagePipeline)
	return pipelines, nil
}

// ec2Pipelines returns pipelines which produce uncompressed EC2 images which are expected to use RHSM for content
func ec2Pipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]manifest.Pipeline, error) {
	return ec2CommonPipelines(t, customizations, options, repos, packageSetSpecs, rng, t.Filename())
}

func iotInstallerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]manifest.Pipeline, error) {
	pipelines := make([]manifest.Pipeline, 0)

	buildPipeline := manifest.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	pipelines = append(pipelines, buildPipeline)

	installerPackages := packageSetSpecs[installerPkgsKey]
	d := t.arch.distro
	ksUsers := len(customizations.GetUsers())+len(customizations.GetGroups()) > 0

	anacondaTreePipeline := anacondaTreePipeline(&buildPipeline, repos, installerPackages, t.Arch().Name(), d.product, d.osVersion, "IoT", ksUsers)
	isoTreePipeline := bootISOTreePipeline(&buildPipeline, &anacondaTreePipeline, options, d.vendor, d.isolabelTmpl, customizations.GetUsers(), customizations.GetGroups())
	isoPipeline := bootISOPipeline(&buildPipeline, &isoTreePipeline, t.Filename(), false)

	return append(pipelines, anacondaTreePipeline, isoTreePipeline, isoPipeline), nil
}

func iotCorePipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec) (*manifest.BuildPipeline, *manifest.OSPipeline, *manifest.OSTreeCommitPipeline, error) {
	buildPipeline := manifest.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	treePipeline, err := osPipeline(&buildPipeline, t, repos, packageSetSpecs[osPkgsKey], customizations, options, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	commitPipeline := ostreeCommitPipeline(&buildPipeline, &treePipeline, options, t.arch.distro.osVersion)

	return &buildPipeline, &treePipeline, &commitPipeline, nil
}

func iotCommitPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]manifest.Pipeline, error) {
	pipelines := make([]manifest.Pipeline, 0)

	buildPipeline, treePipeline, commitPipeline, err := iotCorePipelines(t, customizations, options, repos, packageSetSpecs)
	if err != nil {
		return nil, err
	}
	tarPipeline := manifest.NewTarPipeline(buildPipeline, &commitPipeline.BasePipeline, "commit-archive", t.Filename())
	pipelines = append(pipelines, buildPipeline, treePipeline, commitPipeline, tarPipeline)
	return pipelines, nil
}

func iotContainerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]manifest.Pipeline, error) {
	pipelines := make([]manifest.Pipeline, 0)

	buildPipeline, treePipeline, commitPipeline, err := iotCorePipelines(t, customizations, options, repos, packageSetSpecs)
	if err != nil {
		return nil, err
	}

	nginxConfigPath := "/etc/nginx.conf"
	httpPort := "8080"
	containerTreePipeline := containerTreePipeline(buildPipeline, commitPipeline, repos, packageSetSpecs[containerPkgsKey], options, customizations, nginxConfigPath, httpPort)
	containerPipeline := containerPipeline(buildPipeline, &containerTreePipeline.BasePipeline, t, nginxConfigPath, httpPort)

	pipelines = append(pipelines, buildPipeline, treePipeline, commitPipeline, containerTreePipeline, containerPipeline)
	return pipelines, nil
}

func osPipeline(buildPipeline *manifest.BuildPipeline,
	t *imageType,
	repos []rpmmd.RepoConfig,
	packages []rpmmd.PackageSpec,
	c *blueprint.Customizations,
	options distro.ImageOptions,
	rng *rand.Rand) (manifest.OSPipeline, error) {

	imageConfig := t.getDefaultImageConfig()

	var pt *disk.PartitionTable
	if t.bootable {
		// TODO: should there always be a partition table?
		var err error
		pt, err = t.getPartitionTable(c.GetFilesystems(), options, rng)
		if err != nil {
			return manifest.OSPipeline{}, err
		}
	}

	var bootLoader manifest.BootLoader
	if t.Arch().Name() == distro.S390xArchName {
		bootLoader = manifest.BOOTLOADER_ZIPL
	} else {
		bootLoader = manifest.BOOTLOADER_GRUB
	}

	var kernelName string
	if t.bootable {
		kernelName = c.GetKernel().Name
	}

	pl := manifest.NewOSPipeline(buildPipeline, t.rpmOstree, options.OSTree.Parent, options.OSTree.URL, repos, packages, pt, bootLoader, t.arch.legacy, kernelName)

	if t.supportsUEFI() {
		pl.UEFIVendor = t.arch.distro.vendor
	}

	var kernelOptions []string
	if t.kernelOptions != "" {
		kernelOptions = append(kernelOptions, t.kernelOptions)
	}
	if bpKernel := c.GetKernel(); bpKernel.Append != "" {
		kernelOptions = append(kernelOptions, bpKernel.Append)
	}
	pl.KernelOptionsAppend = kernelOptions

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

func ostreeCommitPipeline(buildPipeline *manifest.BuildPipeline, treePipeline *manifest.OSPipeline, options distro.ImageOptions, osVersion string) manifest.OSTreeCommitPipeline {
	p := manifest.NewOSTreeCommitPipeline(buildPipeline, treePipeline, options.OSTree.Ref)
	p.OSVersion = osVersion
	return p
}

func containerTreePipeline(buildPipeline *manifest.BuildPipeline, commitPipeline *manifest.OSTreeCommitPipeline, repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, options distro.ImageOptions, c *blueprint.Customizations, nginxConfigPath, listenPort string) manifest.OSTreeCommitServerTreePipeline {
	p := manifest.NewOSTreeCommitServerTreePipeline(buildPipeline, repos, packages, commitPipeline, nginxConfigPath, listenPort)
	language, _ := c.GetPrimaryLocale()
	if language != nil {
		p.Language = *language
	}
	return p
}

func containerPipeline(buildPipeline *manifest.BuildPipeline, treePipeline *manifest.BasePipeline, t *imageType, nginxConfigPath, listenPort string) manifest.OCIContainerPipeline {
	p := manifest.NewOCIContainerPipeline(buildPipeline, treePipeline, t.Arch().Name(), t.Filename())
	p.Cmd = []string{"nginx", "-c", nginxConfigPath}
	p.ExposedPorts = []string{listenPort}
	return p
}

func anacondaTreePipeline(buildPipeline *manifest.BuildPipeline, repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, arch, product, osVersion, variant string, users bool) manifest.AnacondaPipeline {
	p := manifest.NewAnacondaPipeline(buildPipeline, repos, packages, "kernel", arch, product, osVersion)
	p.Users = users
	p.Variant = variant
	p.Biosdevname = (arch == distro.X86_64ArchName)
	return p
}

func bootISOTreePipeline(buildPipeline *manifest.BuildPipeline, anacondaPipeline *manifest.AnacondaPipeline, options distro.ImageOptions, vendor, isoLabelTempl string, users []blueprint.UserCustomization, groups []blueprint.GroupCustomization) manifest.ISOTreePipeline {
	p := manifest.NewISOTreePipeline(buildPipeline, anacondaPipeline, options.OSTree.Parent, options.OSTree.URL, options.OSTree.Ref, isoLabelTempl)
	p.Release = "202010217.n.0"
	p.OSName = "fedora"
	p.UEFIVendor = vendor
	p.Users = users
	p.Groups = groups

	return p
}

func bootISOPipeline(buildPipeline *manifest.BuildPipeline, treePipeline *manifest.ISOTreePipeline, filename string, isolinux bool) manifest.ISOPipeline {
	p := manifest.NewISOPipeline(buildPipeline, treePipeline, filename)
	p.ISOLinux = isolinux
	return p
}

func containerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]manifest.Pipeline, error) {
	pipelines := make([]manifest.Pipeline, 0)

	buildPipeline := manifest.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	pipelines = append(pipelines, buildPipeline)

	treePipeline, err := osPipeline(buildPipeline, t, repos, packageSetSpecs[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, treePipeline)

	ociPipeline := manifest.NewOCIContainerPipeline(buildPipeline, &treePipeline.BasePipeline, t.Arch().Name(), t.Filename())
	pipelines = append(pipelines, ociPipeline)

	return pipelines, nil
}
