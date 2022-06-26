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

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(&buildPipeline, &treePipeline, diskfile, t.arch)
	pipelines = append(pipelines, imagePipeline.Serialize())

	qemuPipeline := qemuPipeline(&buildPipeline, &imagePipeline, diskfile, t.filename, osbuild.QEMUFormatQCOW2, osbuild.QCOW2Options{Compat: "1.1"})
	pipelines = append(pipelines, qemuPipeline.Serialize())

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

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(&buildPipeline, &treePipeline, diskfile, t.arch)
	pipelines = append(pipelines, imagePipeline.Serialize())

	qemuPipeline := qemuPipeline(&buildPipeline, &imagePipeline, diskfile, t.filename, osbuild.QEMUFormatVPC, nil)
	pipelines = append(pipelines, qemuPipeline.Serialize())
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

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(&buildPipeline, &treePipeline, diskfile, t.arch)
	pipelines = append(pipelines, imagePipeline.Serialize())

	qemuPipeline := qemuPipeline(&buildPipeline, &imagePipeline, diskfile, t.filename, osbuild.QEMUFormatVMDK, osbuild.VMDKOptions{Subformat: osbuild.VMDKSubformatStreamOptimized})
	pipelines = append(pipelines, qemuPipeline.Serialize())
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

	diskfile := "disk.img"
	imagePipeline := liveImagePipeline(&buildPipeline, &treePipeline, diskfile, t.arch)
	pipelines = append(pipelines, imagePipeline.Serialize())

	qemuPipeline := qemuPipeline(&buildPipeline, &imagePipeline, diskfile, t.filename, osbuild.QEMUFormatQCOW2, nil)
	pipelines = append(pipelines, qemuPipeline.Serialize())
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

	imagePipeline := liveImagePipeline(&buildPipeline, &treePipeline, diskfile, t.arch)
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
	tarPipeline := pipeline.NewTarPipeline(buildPipeline, &commitPipeline.Pipeline, "commit-archive")
	tarPipeline.Filename = t.Filename()
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

	pl := pipeline.NewOSPipeline(buildPipeline, t.rpmOstree, repos, packages, pt, c.GetKernel().Name)

	if t.Arch().Name() == distro.S390xArchName {
		pl.BootLoader = pipeline.BOOTLOADER_ZIPL
	} else {
		pl.BootLoader = pipeline.BOOTLOADER_GRUB
	}

	pl.UEFI = t.supportsUEFI()
	pl.GRUBLegacy = t.arch.legacy
	pl.Vendor = t.arch.distro.vendor

	var kernelOptions []string
	if t.kernelOptions != "" {
		kernelOptions = append(kernelOptions, t.kernelOptions)
	}
	if bpKernel := c.GetKernel(); bpKernel.Append != "" {
		kernelOptions = append(kernelOptions, bpKernel.Append)
	}
	pl.KernelOptionsAppend = kernelOptions

	pl.OSTreeParent = options.OSTree.Parent
	pl.OSTreeURL = options.OSTree.URL
	pl.GPGKeyFiles = imageConfig.GPGKeyFiles

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
	p.Parent = options.OSTree.Parent
	return p
}

func containerTreePipeline(buildPipeline *pipeline.BuildPipeline, commitPipeline *pipeline.OSTreeCommitPipeline, repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, options distro.ImageOptions, c *blueprint.Customizations, nginxConfigPath, listenPort string) pipeline.OSTreeCommitServerTreePipeline {
	p := pipeline.NewOSTreeCommitServerTreePipeline(buildPipeline, commitPipeline)
	p.Repos = repos
	p.PackageSpecs = packages
	p.NginxConfigPath = nginxConfigPath
	p.ListenPort = listenPort
	language, _ := c.GetPrimaryLocale()
	if language != nil {
		p.Language = *language
	}
	return p
}

func containerPipeline(buildPipeline *pipeline.BuildPipeline, treePipeline *pipeline.Pipeline, t *imageType, nginxConfigPath, listenPort string) pipeline.OCIContainerPipeline {
	p := pipeline.NewOCIContainerPipeline(buildPipeline, treePipeline)
	p.Architecture = t.Arch().Name()
	p.Filename = t.Filename()
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
	p.Vendor = vendor
	p.Users = users
	p.Groups = groups
	p.OSTreeRef = options.OSTree.Ref
	p.OSTreeParent = options.OSTree.Parent

	return p
}

func bootISOPipeline(buildPipeline *pipeline.BuildPipeline, treePipeline *pipeline.ISOTreePipeline, filename string, isolinux bool) pipeline.ISOPipeline {
	p := pipeline.NewISOPipeline(buildPipeline, treePipeline)
	p.Filename = filename
	p.ISOLinux = isolinux
	return p
}

func liveImagePipeline(buildPipeline *pipeline.BuildPipeline, treePipeline *pipeline.OSPipeline, outputFilename string, arch *architecture) pipeline.LiveImgPipeline {
	p := pipeline.NewLiveImgPipeline(buildPipeline, treePipeline)

	p.Filename = outputFilename

	if arch.name == distro.S390xArchName {
		p.BootLoader = pipeline.BOOTLOADER_ZIPL
	} else {
		p.BootLoader = pipeline.BOOTLOADER_GRUB
		p.GRUBLegacy = arch.legacy
	}

	return p
}

func qemuPipeline(buildPipeline *pipeline.BuildPipeline, imagePipeline *pipeline.LiveImgPipeline, inputFilename, outputFilename string, format osbuild.QEMUFormat, formatOptions osbuild.QEMUFormatOptions) pipeline.QemuPipeline {
	p := pipeline.NewQemuPipeline(buildPipeline, imagePipeline, string(format))
	p.InputFilename = inputFilename
	p.OutputFilename = outputFilename
	p.Format = format
	p.FormatOptions = formatOptions

	return p
}
