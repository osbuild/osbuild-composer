package fedora

import (
	"math/rand"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func qcow2Pipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)

	buildPipeline := pipeline.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	pipelines = append(pipelines, buildPipeline.Serialize())

	treePipeline, err := osPipeline(&buildPipeline, t, repos, packageSetSpecs[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, treePipeline.Serialize())

	imagePipeline := pipeline.NewLiveImgPipeline(&buildPipeline, &treePipeline, "disk.img")
	pipelines = append(pipelines, imagePipeline.Serialize())

	qcow2Pipeline := pipeline.NewQCOW2Pipeline(&buildPipeline, &imagePipeline, t.filename)
	qcow2Pipeline.Compat = "1.1"
	pipelines = append(pipelines, qcow2Pipeline.Serialize())

	return pipelines, nil
}

func vhdPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)

	buildPipeline := pipeline.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	pipelines = append(pipelines, buildPipeline.Serialize())

	treePipeline, err := osPipeline(&buildPipeline, t, repos, packageSetSpecs[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, treePipeline.Serialize())

	imagePipeline := pipeline.NewLiveImgPipeline(&buildPipeline, &treePipeline, "disk.img")
	pipelines = append(pipelines, imagePipeline.Serialize())

	vpcPipeline := pipeline.NewVPCPipeline(&buildPipeline, &imagePipeline, t.filename)
	pipelines = append(pipelines, vpcPipeline.Serialize())
	return pipelines, nil
}

func vmdkPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)

	buildPipeline := pipeline.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	pipelines = append(pipelines, buildPipeline.Serialize())

	treePipeline, err := osPipeline(&buildPipeline, t, repos, packageSetSpecs[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, treePipeline.Serialize())

	imagePipeline := pipeline.NewLiveImgPipeline(&buildPipeline, &treePipeline, "disk.img")
	pipelines = append(pipelines, imagePipeline.Serialize())

	vmdkPipeline := pipeline.NewVMDKPipeline(&buildPipeline, &imagePipeline, t.filename)
	pipelines = append(pipelines, vmdkPipeline.Serialize())
	return pipelines, nil
}

func openstackPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)

	buildPipeline := pipeline.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	pipelines = append(pipelines, buildPipeline.Serialize())

	treePipeline, err := osPipeline(&buildPipeline, t, repos, packageSetSpecs[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, treePipeline.Serialize())

	imagePipeline := pipeline.NewLiveImgPipeline(&buildPipeline, &treePipeline, "disk.img")
	pipelines = append(pipelines, imagePipeline.Serialize())

	qcow2Pipeline := pipeline.NewQCOW2Pipeline(&buildPipeline, &imagePipeline, t.filename)
	pipelines = append(pipelines, qcow2Pipeline.Serialize())
	return pipelines, nil
}

func ec2CommonPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions,
	repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec,
	rng *rand.Rand, diskfile string) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)

	buildPipeline := pipeline.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	pipelines = append(pipelines, buildPipeline.Serialize())

	treePipeline, err := osPipeline(&buildPipeline, t, repos, packageSetSpecs[osPkgsKey], customizations, options, rng)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, treePipeline.Serialize())

	imagePipeline := pipeline.NewLiveImgPipeline(&buildPipeline, &treePipeline, diskfile)
	pipelines = append(pipelines, imagePipeline.Serialize())
	return pipelines, nil
}

// ec2Pipelines returns pipelines which produce uncompressed EC2 images which are expected to use RHSM for content
func ec2Pipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	return ec2CommonPipelines(t, customizations, options, repos, packageSetSpecs, rng, t.Filename())
}

func iotInstallerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)

	buildPipeline := pipeline.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	pipelines = append(pipelines, buildPipeline.Serialize())

	installerPackages := packageSetSpecs[installerPkgsKey]
	d := t.arch.distro
	ksUsers := len(customizations.GetUsers())+len(customizations.GetGroups()) > 0

	anacondaTreePipeline := anacondaTreePipeline(&buildPipeline, repos, installerPackages, t.Arch().Name(), d.product, d.osVersion, "IoT", ksUsers)
	isoTreePipeline := bootISOTreePipeline(&buildPipeline, &anacondaTreePipeline, options, d.vendor, d.isolabelTmpl, customizations.GetUsers(), customizations.GetGroups())
	isoPipeline := bootISOPipeline(&buildPipeline, &isoTreePipeline, t.Filename(), false)

	return append(pipelines, anacondaTreePipeline.Serialize(), isoTreePipeline.Serialize(), isoPipeline.Serialize()), nil
}

func iotCorePipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec) (*pipeline.BuildPipeline, *pipeline.OSPipeline, *pipeline.OSTreeCommitPipeline, error) {
	buildPipeline := pipeline.NewBuildPipeline(t.arch.distro.runner, repos, packageSetSpecs[buildPkgsKey])
	treePipeline, err := osPipeline(&buildPipeline, t, repos, packageSetSpecs[osPkgsKey], customizations, options, nil)
	if err != nil {
		return nil, nil, nil, err
	}
	commitPipeline := ostreeCommitPipeline(&buildPipeline, &treePipeline, options, t.arch.distro.osVersion)

	return &buildPipeline, &treePipeline, &commitPipeline, nil
}

func iotCommitPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)

	buildPipeline, treePipeline, commitPipeline, err := iotCorePipelines(t, customizations, options, repos, packageSetSpecs)
	if err != nil {
		return nil, err
	}
	tarPipeline := pipeline.NewTarPipeline(buildPipeline, &commitPipeline.Pipeline, "commit-archive", t.Filename())
	pipelines = append(pipelines, buildPipeline.Serialize(), treePipeline.Serialize(), commitPipeline.Serialize(), tarPipeline.Serialize())
	return pipelines, nil
}

func iotContainerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)

	buildPipeline, treePipeline, commitPipeline, err := iotCorePipelines(t, customizations, options, repos, packageSetSpecs)
	if err != nil {
		return nil, err
	}

	nginxConfigPath := "/etc/nginx.conf"
	httpPort := "8080"
	containerTreePipeline := containerTreePipeline(buildPipeline, commitPipeline, repos, packageSetSpecs[containerPkgsKey], options, customizations, nginxConfigPath, httpPort)
	containerPipeline := containerPipeline(buildPipeline, &containerTreePipeline.Pipeline, t, nginxConfigPath, httpPort)

	pipelines = append(pipelines, buildPipeline.Serialize(), treePipeline.Serialize(), commitPipeline.Serialize(), containerTreePipeline.Serialize(), containerPipeline.Serialize())
	return pipelines, nil
}

func osPipeline(buildPipeline *pipeline.BuildPipeline,
	t *imageType,
	repos []rpmmd.RepoConfig,
	packages []rpmmd.PackageSpec,
	c *blueprint.Customizations,
	options distro.ImageOptions,
	rng *rand.Rand) (pipeline.OSPipeline, error) {

	imageConfig := t.getDefaultImageConfig()

	var pt *disk.PartitionTable
	if t.bootable {
		// TODO: should there always be a partition table?
		var err error
		pt, err = t.getPartitionTable(c.GetFilesystems(), options, rng)
		if err != nil {
			return pipeline.OSPipeline{}, err
		}
	}

	var bootLoader pipeline.BootLoader
	if t.Arch().Name() == distro.S390xArchName {
		bootLoader = pipeline.BOOTLOADER_ZIPL
	} else {
		bootLoader = pipeline.BOOTLOADER_GRUB
	}

	var kernelName string
	if t.bootable {
		kernelName = c.GetKernel().Name
	}

	pl := pipeline.NewOSPipeline(buildPipeline, t.rpmOstree, options.OSTree.Parent, repos, packages, pt, bootLoader, t.arch.legacy, kernelName)

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

func ostreeCommitPipeline(buildPipeline *pipeline.BuildPipeline, treePipeline *pipeline.OSPipeline, options distro.ImageOptions, osVersion string) pipeline.OSTreeCommitPipeline {
	p := pipeline.NewOSTreeCommitPipeline(buildPipeline, treePipeline, options.OSTree.Ref)
	p.OSVersion = osVersion
	return p
}

func containerTreePipeline(buildPipeline *pipeline.BuildPipeline, commitPipeline *pipeline.OSTreeCommitPipeline, repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, options distro.ImageOptions, c *blueprint.Customizations, nginxConfigPath, listenPort string) pipeline.OSTreeCommitServerTreePipeline {
	p := pipeline.NewOSTreeCommitServerTreePipeline(buildPipeline, repos, packages, commitPipeline, nginxConfigPath, listenPort)
	language, _ := c.GetPrimaryLocale()
	if language != nil {
		p.Language = *language
	}
	return p
}

func containerPipeline(buildPipeline *pipeline.BuildPipeline, treePipeline *pipeline.Pipeline, t *imageType, nginxConfigPath, listenPort string) pipeline.OCIContainerPipeline {
	p := pipeline.NewOCIContainerPipeline(buildPipeline, treePipeline, t.Arch().Name(), t.Filename())
	p.Cmd = []string{"nginx", "-c", nginxConfigPath}
	p.ExposedPorts = []string{listenPort}
	return p
}

func anacondaTreePipeline(buildPipeline *pipeline.BuildPipeline, repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, arch, product, osVersion, variant string, users bool) pipeline.AnacondaPipeline {
	p := pipeline.NewAnacondaPipeline(buildPipeline, repos, packages, "kernel", arch, product, osVersion)
	p.Users = users
	p.Variant = variant
	p.Biosdevname = (arch == distro.X86_64ArchName)
	return p
}

func bootISOTreePipeline(buildPipeline *pipeline.BuildPipeline, anacondaPipeline *pipeline.AnacondaPipeline, options distro.ImageOptions, vendor, isoLabelTempl string, users []blueprint.UserCustomization, groups []blueprint.GroupCustomization) pipeline.ISOTreePipeline {
	p := pipeline.NewISOTreePipeline(buildPipeline, anacondaPipeline, isoLabelTempl)
	p.Release = "202010217.n.0"
	p.OSName = "fedora"
	p.UEFIVendor = vendor
	p.Users = users
	p.Groups = groups
	p.OSTreeRef = options.OSTree.Ref
	p.OSTreeParent = options.OSTree.Parent

	return p
}

func bootISOPipeline(buildPipeline *pipeline.BuildPipeline, treePipeline *pipeline.ISOTreePipeline, filename string, isolinux bool) pipeline.ISOPipeline {
	p := pipeline.NewISOPipeline(buildPipeline, treePipeline, filename)
	p.ISOLinux = isolinux
	return p
}
