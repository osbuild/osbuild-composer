package fedora

import (
	"fmt"
	"math/rand"
	"path"
	"path/filepath"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

func qcow2Pipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey], t.arch.distro.runner))

	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options, partitionTable)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, osbuild.QEMUFormatQCOW2, osbuild.QCOW2Options{Compat: "1.1"})
	pipelines = append(pipelines, *qemuPipeline)

	return pipelines, nil
}

func prependKernelCmdlineStage(pipeline *osbuild.Pipeline, kernelOptions string, pt *disk.PartitionTable) *osbuild.Pipeline {
	rootFs := pt.FindMountable("/")
	if rootFs == nil {
		panic("root filesystem must be defined for kernel-cmdline stage, this is a programming error")
	}
	rootFsUUID := rootFs.GetFSSpec().UUID
	kernelStage := osbuild.NewKernelCmdlineStage(osbuild.NewKernelCmdlineStageOptions(rootFsUUID, kernelOptions))
	pipeline.Stages = append([]*osbuild.Stage{kernelStage}, pipeline.Stages...)
	return pipeline
}

func vhdPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey], t.arch.distro.runner))

	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options, partitionTable)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, osbuild.QEMUFormatVPC, nil)
	pipelines = append(pipelines, *qemuPipeline)
	return pipelines, nil
}

func vmdkPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey], t.arch.distro.runner))

	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options, partitionTable)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, osbuild.QEMUFormatVMDK, osbuild.VMDKOptions{Subformat: osbuild.VMDKSubformatStreamOptimized})
	pipelines = append(pipelines, *qemuPipeline)
	return pipelines, nil
}

func openstackPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey], t.arch.distro.runner))

	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options, partitionTable)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, *treePipeline)

	diskfile := "disk.img"
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)

	qemuPipeline := qemuPipeline(imagePipeline.Name, diskfile, t.filename, osbuild.QEMUFormatQCOW2, nil)
	pipelines = append(pipelines, *qemuPipeline)
	return pipelines, nil
}

func ec2CommonPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions,
	repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec,
	rng *rand.Rand, diskfile string) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey], t.arch.distro.runner))

	partitionTable, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options, partitionTable)
	if err != nil {
		return nil, err
	}
	pipelines = append(pipelines, *treePipeline)

	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packageSetSpecs[osPkgsKey], customizations.GetKernel().Name)
	imagePipeline := liveImagePipeline(treePipeline.Name, diskfile, partitionTable, t.arch, kernelVer)
	pipelines = append(pipelines, *imagePipeline)
	return pipelines, nil
}

// ec2Pipelines returns pipelines which produce uncompressed EC2 images which are expected to use RHSM for content
func ec2Pipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	return ec2CommonPipelines(t, customizations, options, repos, packageSetSpecs, rng, t.Filename())
}

//makeISORootPath return a path that can be used to address files and folders in
//the root of the iso
func makeISORootPath(p string) string {
	fullpath := path.Join("/run/install/repo", p)
	return fmt.Sprintf("file://%s", fullpath)
}

func iotInstallerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey], t.arch.distro.runner))
	installerPackages := packageSetSpecs[installerPkgsKey]
	d := t.arch.distro
	archName := t.Arch().Name()
	kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(installerPackages, "kernel")
	ostreeRepoPath := "/ostree/repo"
	payloadStages := ostreePayloadStages(options, ostreeRepoPath)
	kickstartOptions, err := osbuild.NewKickstartStageOptions(kspath, "", customizations.GetUsers(), customizations.GetGroups(), makeISORootPath(ostreeRepoPath), options.OSTree.Ref, "fedora")
	if err != nil {
		return nil, err
	}
	ksUsers := len(customizations.GetUsers())+len(customizations.GetGroups()) > 0
	pipelines = append(pipelines, *anacondaTreePipeline(repos, installerPackages, kernelVer, archName, d.product, d.osVersion, "IoT", ksUsers))
	isolabel := fmt.Sprintf(d.isolabelTmpl, archName)
	pipelines = append(pipelines, *bootISOTreePipeline(kernelVer, archName, d.vendor, d.product, d.osVersion, isolabel, kickstartOptions, payloadStages))
	pipelines = append(pipelines, *bootISOPipeline(t.Filename(), d.isolabelTmpl, archName, false))
	return pipelines, nil
}

func iotCorePipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec) ([]osbuild.Pipeline, error) {
	pipelines := make([]osbuild.Pipeline, 0)
	pipelines = append(pipelines, *buildPipeline(repos, packageSetSpecs[buildPkgsKey], t.arch.distro.runner))

	treePipeline, err := osPipeline(t, repos, packageSetSpecs[osPkgsKey], customizations, options, nil)
	if err != nil {
		return nil, err
	}

	pipelines = append(pipelines, *treePipeline)
	pipelines = append(pipelines, *ostreeCommitPipeline(options, t.arch.distro.osVersion))

	return pipelines, nil
}

func iotCommitPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines, err := iotCorePipelines(t, customizations, options, repos, packageSetSpecs)
	if err != nil {
		return nil, err
	}
	tarPipeline := osbuild.Pipeline{
		Name:  "commit-archive",
		Build: "name:build",
	}
	tarPipeline.AddStage(tarStage("ostree-commit", t.Filename()))
	pipelines = append(pipelines, tarPipeline)
	return pipelines, nil
}

func iotContainerPipelines(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error) {
	pipelines, err := iotCorePipelines(t, customizations, options, repos, packageSetSpecs)
	if err != nil {
		return nil, err
	}

	nginxConfigPath := "/etc/nginx.conf"
	httpPort := "8080"
	pipelines = append(pipelines, *containerTreePipeline(repos, packageSetSpecs[containerPkgsKey], options, customizations, nginxConfigPath, httpPort))
	pipelines = append(pipelines, *containerPipeline(t, nginxConfigPath, httpPort))
	return pipelines, nil
}

func buildPipeline(repos []rpmmd.RepoConfig, buildPackageSpecs []rpmmd.PackageSpec, runner string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "build"
	p.Runner = runner
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(buildPackageSpecs)))
	p.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(true)))
	return p
}

func osPipeline(t *imageType,
	repos []rpmmd.RepoConfig,
	packages []rpmmd.PackageSpec,
	c *blueprint.Customizations,
	options distro.ImageOptions,
	pt *disk.PartitionTable) (*osbuild.Pipeline, error) {
	imageConfig := t.getDefaultImageConfig()
	p := new(osbuild.Pipeline)
	if t.rpmOstree {
		p.Name = "ostree-tree"
	} else {
		p.Name = "os"
	}
	p.Build = "name:build"

	if t.rpmOstree && options.OSTree.Parent != "" && options.OSTree.URL != "" {
		p.AddStage(osbuild.NewOSTreePasswdStage("org.osbuild.source", options.OSTree.Parent))
	}

	rpmOptions := osbuild.NewRPMStageOptions(repos)
	rpmOptions.GPGKeysFromTree = imageConfig.GPGKeyFiles
	p.AddStage(osbuild.NewRPMStage(rpmOptions, osbuild.NewRpmStageSourceFilesInputs(packages)))

	// If the /boot is on a separate partition, the prefix for the BLS stage must be ""
	if pt == nil || pt.FindMountable("/boot") == nil {
		p.AddStage(osbuild.NewFixBLSStage(&osbuild.FixBLSStageOptions{}))
	} else {
		p.AddStage(osbuild.NewFixBLSStage(&osbuild.FixBLSStageOptions{Prefix: common.StringToPtr("")}))
	}

	language, keyboard := c.GetPrimaryLocale()
	if language != nil {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: *language}))
	} else {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: imageConfig.Locale}))
	}
	if keyboard != nil {
		p.AddStage(osbuild.NewKeymapStage(&osbuild.KeymapStageOptions{Keymap: *keyboard}))
	} else if imageConfig.Keyboard != nil {
		p.AddStage(osbuild.NewKeymapStage(imageConfig.Keyboard))
	}

	if hostname := c.GetHostname(); hostname != nil {
		p.AddStage(osbuild.NewHostnameStage(&osbuild.HostnameStageOptions{Hostname: *hostname}))
	} else {
		p.AddStage(osbuild.NewHostnameStage(&osbuild.HostnameStageOptions{Hostname: "localhost.localdomain"}))
	}

	timezone, ntpServers := c.GetTimezoneSettings()
	if timezone != nil {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: *timezone}))
	} else {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: imageConfig.Timezone}))
	}

	if len(ntpServers) > 0 {
		p.AddStage(osbuild.NewChronyStage(&osbuild.ChronyStageOptions{Timeservers: ntpServers}))
	} else if imageConfig.TimeSynchronization != nil {
		p.AddStage(osbuild.NewChronyStage(imageConfig.TimeSynchronization))
	}

	if !t.bootISO {
		// don't put users and groups in the payload of an installer
		// add them via kickstart instead
		if groups := c.GetGroups(); len(groups) > 0 {
			p.AddStage(osbuild.NewGroupsStage(osbuild.NewGroupsStageOptions(groups)))
		}

		if userOptions, err := osbuild.NewUsersStageOptions(c.GetUsers(), false); err != nil {
			return nil, err
		} else if userOptions != nil {
			if t.rpmOstree {
				// for ostree, writing the key during user creation is
				// redundant and can cause issues so create users without keys
				// and write them on first boot
				userOptionsSansKeys, err := osbuild.NewUsersStageOptions(c.GetUsers(), true)
				if err != nil {
					return nil, err
				}
				p.AddStage(osbuild.NewUsersStage(userOptionsSansKeys))
				p.AddStage(osbuild.NewFirstBootStage(usersFirstBootOptions(userOptions)))
			} else {
				p.AddStage(osbuild.NewUsersStage(userOptions))
			}
		}
	}

	if services := c.GetServices(); services != nil || imageConfig.EnabledServices != nil ||
		imageConfig.DisabledServices != nil || imageConfig.DefaultTarget != "" {
		p.AddStage(osbuild.NewSystemdStage(systemdStageOptions(
			imageConfig.EnabledServices,
			imageConfig.DisabledServices,
			services,
			imageConfig.DefaultTarget,
		)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(firewallStageOptions(firewall)))
	}

	for _, sysconfigConfig := range imageConfig.Sysconfig {
		p.AddStage(osbuild.NewSysconfigStage(sysconfigConfig))
	}

	for _, systemdLogindConfig := range imageConfig.SystemdLogind {
		p.AddStage(osbuild.NewSystemdLogindStage(systemdLogindConfig))
	}

	for _, cloudInitConfig := range imageConfig.CloudInit {
		p.AddStage(osbuild.NewCloudInitStage(cloudInitConfig))
	}

	for _, modprobeConfig := range imageConfig.Modprobe {
		p.AddStage(osbuild.NewModprobeStage(modprobeConfig))
	}

	for _, dracutConfConfig := range imageConfig.DracutConf {
		p.AddStage(osbuild.NewDracutConfStage(dracutConfConfig))
	}

	for _, systemdUnitConfig := range imageConfig.SystemdUnit {
		p.AddStage(osbuild.NewSystemdUnitStage(systemdUnitConfig))
	}

	if authselectConfig := imageConfig.Authselect; authselectConfig != nil {
		p.AddStage(osbuild.NewAuthselectStage(authselectConfig))
	}

	if seLinuxConfig := imageConfig.SELinuxConfig; seLinuxConfig != nil {
		p.AddStage(osbuild.NewSELinuxConfigStage(seLinuxConfig))
	}

	if tunedConfig := imageConfig.Tuned; tunedConfig != nil {
		p.AddStage(osbuild.NewTunedStage(tunedConfig))
	}

	for _, tmpfilesdConfig := range imageConfig.Tmpfilesd {
		p.AddStage(osbuild.NewTmpfilesdStage(tmpfilesdConfig))
	}

	for _, pamLimitsConfConfig := range imageConfig.PamLimitsConf {
		p.AddStage(osbuild.NewPamLimitsConfStage(pamLimitsConfConfig))
	}

	for _, sysctldConfig := range imageConfig.Sysctld {
		p.AddStage(osbuild.NewSysctldStage(sysctldConfig))
	}

	for _, dnfConfig := range imageConfig.DNFConfig {
		p.AddStage(osbuild.NewDNFConfigStage(dnfConfig))
	}

	if sshdConfig := imageConfig.SshdConfig; sshdConfig != nil {
		p.AddStage((osbuild.NewSshdConfigStage(sshdConfig)))
	}

	if authConfig := imageConfig.Authconfig; authConfig != nil {
		p.AddStage(osbuild.NewAuthconfigStage(authConfig))
	}

	if pwQuality := imageConfig.PwQuality; pwQuality != nil {
		p.AddStage(osbuild.NewPwqualityConfStage(pwQuality))
	}

	if waConfig := imageConfig.WAAgentConfig; waConfig != nil {
		p.AddStage(osbuild.NewWAAgentConfStage(waConfig))
	}

	if pt != nil {
		kernelOptions := osbuild.GenImageKernelOptions(pt)
		if t.kernelOptions != "" {
			kernelOptions = append(kernelOptions, t.kernelOptions)
		}
		if bpKernel := c.GetKernel(); bpKernel.Append != "" {
			kernelOptions = append(kernelOptions, bpKernel.Append)
		}
		p = prependKernelCmdlineStage(p, strings.Join(kernelOptions, " "), pt)
		p.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(pt)))
		kernelVer := rpmmd.GetVerStrFromPackageSpecListPanic(packages, c.GetKernel().Name)
		bootloader := bootloaderConfigStage(t, *pt, kernelVer, false, false)

		if cfg := imageConfig.Grub2Config; cfg != nil {
			if grub2, ok := bootloader.Options.(*osbuild.GRUB2StageOptions); ok {
				// grub2.Config.Default is owned and set by `NewGrub2StageOptionsUnified`
				// and thus we need to preserve it
				if grub2.Config != nil {
					cfg.Default = grub2.Config.Default
				}
			}
		}

		p.AddStage(bootloader)
	}

	p.AddStage(osbuild.NewSELinuxStage(selinuxStageOptions(false)))

	if t.rpmOstree {
		p.AddStage(osbuild.NewOSTreePrepTreeStage(&osbuild.OSTreePrepTreeStageOptions{
			EtcGroupMembers: []string{
				// NOTE: We may want to make this configurable.
				"wheel", "docker",
			},
		}))
	}

	return p, nil
}

func ostreeCommitPipeline(options distro.ImageOptions, osVersion string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "ostree-commit"
	p.Build = "name:build"
	p.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: "/repo"}))

	commitStageInput := new(osbuild.OSTreeCommitStageInput)
	commitStageInput.Type = "org.osbuild.tree"
	commitStageInput.Origin = "org.osbuild.pipeline"
	commitStageInput.References = osbuild.OSTreeCommitStageReferences{"name:ostree-tree"}

	p.AddStage(osbuild.NewOSTreeCommitStage(
		&osbuild.OSTreeCommitStageOptions{
			Ref:       options.OSTree.Ref,
			OSVersion: osVersion,
			Parent:    options.OSTree.Parent,
		},
		&osbuild.OSTreeCommitStageInputs{Tree: commitStageInput}),
	)
	return p
}

func tarStage(source, filename string) *osbuild.Stage {
	tree := new(osbuild.TarStageInput)
	tree.Type = "org.osbuild.tree"
	tree.Origin = "org.osbuild.pipeline"
	tree.References = []string{"name:" + source}
	return osbuild.NewTarStage(&osbuild.TarStageOptions{Filename: filename}, &osbuild.TarStageInputs{Tree: tree})
}

func containerTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, options distro.ImageOptions, c *blueprint.Customizations, nginxConfigPath, listenPort string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "container-tree"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(packages)))
	language, _ := c.GetPrimaryLocale()
	if language != nil {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: *language}))
	} else {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US"}))
	}

	htmlRoot := "/usr/share/nginx/html"
	repoPath := filepath.Join(htmlRoot, "repo")
	p.AddStage(osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: repoPath}))

	p.AddStage(osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: repoPath},
		osbuild.NewOstreePullStageInputs("org.osbuild.pipeline", "name:ostree-commit", options.OSTree.Ref),
	))

	// make nginx log and lib directories world writeable, otherwise nginx can't start in
	// an unprivileged container
	p.AddStage(osbuild.NewChmodStage(chmodStageOptions("/var/log/nginx", "a+rwX", true)))
	p.AddStage(osbuild.NewChmodStage(chmodStageOptions("/var/lib/nginx", "a+rwX", true)))

	p.AddStage(osbuild.NewNginxConfigStage(nginxConfigStageOptions(nginxConfigPath, htmlRoot, listenPort)))
	return p
}

func containerPipeline(t *imageType, nginxConfigPath, listenPort string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "container"
	p.Build = "name:build"
	options := &osbuild.OCIArchiveStageOptions{
		Architecture: t.Arch().Name(),
		Filename:     t.Filename(),
		Config: &osbuild.OCIArchiveConfig{
			Cmd:          []string{"nginx", "-c", nginxConfigPath},
			ExposedPorts: []string{listenPort},
		},
	}
	baseInput := new(osbuild.OCIArchiveStageInput)
	baseInput.Type = "org.osbuild.tree"
	baseInput.Origin = "org.osbuild.pipeline"
	baseInput.References = []string{"name:container-tree"}
	inputs := &osbuild.OCIArchiveStageInputs{Base: baseInput}
	p.AddStage(osbuild.NewOCIArchiveStage(options, inputs))
	return p
}

func ostreePayloadStages(options distro.ImageOptions, ostreeRepoPath string) []*osbuild.Stage {
	stages := make([]*osbuild.Stage, 0)

	// ostree commit payload
	stages = append(stages, osbuild.NewOSTreeInitStage(&osbuild.OSTreeInitStageOptions{Path: ostreeRepoPath}))
	stages = append(stages, osbuild.NewOSTreePullStage(
		&osbuild.OSTreePullStageOptions{Repo: ostreeRepoPath},
		osbuild.NewOstreePullStageInputs("org.osbuild.source", options.OSTree.Parent, options.OSTree.Ref),
	))

	return stages
}

func anacondaTreePipeline(repos []rpmmd.RepoConfig, packages []rpmmd.PackageSpec, kernelVer, arch, product, osVersion, variant string, users bool) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "anaconda-tree"
	p.Build = "name:build"
	p.AddStage(osbuild.NewRPMStage(osbuild.NewRPMStageOptions(repos), osbuild.NewRpmStageSourceFilesInputs(packages)))
	p.AddStage(osbuild.NewBuildstampStage(buildStampStageOptions(arch, product, osVersion, variant)))
	p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US.UTF-8"}))

	rootPassword := ""
	rootUser := osbuild.UsersStageOptionsUser{
		Password: &rootPassword,
	}

	installUID := 0
	installGID := 0
	installHome := "/root"
	installShell := "/usr/libexec/anaconda/run-anaconda"
	installPassword := ""
	installUser := osbuild.UsersStageOptionsUser{
		UID:      &installUID,
		GID:      &installGID,
		Home:     &installHome,
		Shell:    &installShell,
		Password: &installPassword,
	}
	usersStageOptions := &osbuild.UsersStageOptions{
		Users: map[string]osbuild.UsersStageOptionsUser{
			"root":    rootUser,
			"install": installUser,
		},
	}

	p.AddStage(osbuild.NewUsersStage(usersStageOptions))
	p.AddStage(osbuild.NewAnacondaStage(osbuild.NewAnacondaStageOptions(users)))
	p.AddStage(osbuild.NewLoraxScriptStage(loraxScriptStageOptions(arch)))
	p.AddStage(osbuild.NewDracutStage(dracutStageOptions(kernelVer, arch, []string{
		"anaconda",
		"rdma",
		"rngd",
		"multipath",
		"fcoe",
		"fcoe-uefi",
		"iscsi",
		"lunmask",
		"nfs",
	})))
	p.AddStage(osbuild.NewSELinuxConfigStage(&osbuild.SELinuxConfigStageOptions{State: osbuild.SELinuxStatePermissive}))

	return p
}

func bootISOTreePipeline(kernelVer, arch, vendor, product, osVersion, isolabel string, ksOptions *osbuild.KickstartStageOptions, payloadStages []*osbuild.Stage) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "bootiso-tree"
	p.Build = "name:build"

	p.AddStage(osbuild.NewBootISOMonoStage(bootISOMonoStageOptions(kernelVer, arch, vendor, product, osVersion, isolabel), osbuild.NewBootISOMonoStagePipelineTreeInputs("anaconda-tree")))
	p.AddStage(osbuild.NewKickstartStage(ksOptions))
	p.AddStage(osbuild.NewDiscinfoStage(discinfoStageOptions(arch)))

	for _, stage := range payloadStages {
		p.AddStage(stage)
	}

	return p
}
func bootISOPipeline(filename, isolabel, arch string, isolinux bool) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "bootiso"
	p.Build = "name:build"

	p.AddStage(osbuild.NewXorrisofsStage(xorrisofsStageOptions(filename, isolabel, arch, isolinux), osbuild.NewXorrisofsStagePipelineTreeInputs("bootiso-tree")))
	p.AddStage(osbuild.NewImplantisomd5Stage(&osbuild.Implantisomd5StageOptions{Filename: filename}))

	return p
}

func liveImagePipeline(inputPipelineName string, outputFilename string, pt *disk.PartitionTable, arch *architecture, kernelVer string) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = "image"
	p.Build = "name:build"

	for _, stage := range osbuild.GenImagePrepareStages(pt, outputFilename, osbuild.PTSfdisk) {
		p.AddStage(stage)
	}

	inputName := "root-tree"
	copyOptions, copyDevices, copyMounts := osbuild.GenCopyFSTreeOptions(inputName, inputPipelineName, outputFilename, pt)
	copyInputs := osbuild.NewCopyStagePipelineTreeInputs(inputName, inputPipelineName)
	p.AddStage(osbuild.NewCopyStage(copyOptions, copyInputs, copyDevices, copyMounts))

	for _, stage := range osbuild.GenImageFinishStages(pt, outputFilename) {
		p.AddStage(stage)
	}

	loopback := osbuild.NewLoopbackDevice(&osbuild.LoopbackDeviceOptions{Filename: outputFilename})
	p.AddStage(bootloaderInstStage(outputFilename, pt, arch, kernelVer, copyDevices, copyMounts, loopback))
	return p
}

func qemuPipeline(inputPipelineName, inputFilename, outputFilename string, format osbuild.QEMUFormat, formatOptions osbuild.QEMUFormatOptions) *osbuild.Pipeline {
	p := new(osbuild.Pipeline)
	p.Name = string(format)
	p.Build = "name:build"

	qemuStage := osbuild.NewQEMUStage(
		osbuild.NewQEMUStageOptions(outputFilename, format, formatOptions),
		osbuild.NewQemuStagePipelineFilesInputs(inputPipelineName, inputFilename),
	)
	p.AddStage(qemuStage)
	return p
}

func bootloaderConfigStage(t *imageType, partitionTable disk.PartitionTable, kernelVer string, install, greenboot bool) *osbuild.Stage {
	if t.Arch().Name() == distro.S390xArchName {
		return osbuild.NewZiplStage(new(osbuild.ZiplStageOptions))
	}

	uefi := t.supportsUEFI()
	legacy := t.arch.legacy

	options := osbuild.NewGrub2StageOptionsUnified(&partitionTable, kernelVer, uefi, legacy, t.arch.distro.vendor, install)
	options.Greenboot = greenboot

	return osbuild.NewGRUB2Stage(options)
}

func bootloaderInstStage(filename string, pt *disk.PartitionTable, arch *architecture, kernelVer string, devices *osbuild.Devices, mounts *osbuild.Mounts, disk *osbuild.Device) *osbuild.Stage {
	platform := arch.legacy
	if platform != "" {
		return osbuild.NewGrub2InstStage(osbuild.NewGrub2InstStageOption(filename, pt, platform))
	}

	if arch.name == distro.S390xArchName {
		return osbuild.NewZiplInstStage(osbuild.NewZiplInstStageOptions(kernelVer, pt), disk, devices, mounts)
	}

	return nil
}
