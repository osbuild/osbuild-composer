package rhel84

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"strings"

	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild1"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const defaultName = "rhel-84"
const defaultCentosName = "centos-8"
const releaseVersion = "8"
const modulePlatformID = "platform:el8"
const ostreeRef = "rhel/8/%s/edge"

type distribution struct {
	name             string
	modulePlatformID string
	ostreeRef        string
	arches           map[string]architecture
	buildPackages    []string
	isCentos         bool
}

type architecture struct {
	distro             *distribution
	name               string
	bootloaderPackages []string
	buildPackages      []string
	legacy             string
	uefi               bool
	imageTypes         map[string]distro.ImageType
}

type imageType struct {
	arch                    *architecture
	name                    string
	filename                string
	mimeType                string
	packages                []string
	excludedPackages        []string
	enabledServices         []string
	disabledServices        []string
	defaultTarget           string
	kernelOptions           string
	bootable                bool
	rpmOstree               bool
	defaultSize             uint64
	buildPipelines          []string
	payloadPipelines        []string
	exports                 []string
	partitionTableGenerator func(imageSize uint64, arch distro.Arch, rng *rand.Rand) disk.PartitionTable
	assembler               func(pt *disk.PartitionTable, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler
}

func (a *architecture) Distro() distro.Distro {
	return a.distro
}

func (t *imageType) Arch() distro.Arch {
	return t.arch
}

func (d *distribution) ListArches() []string {
	archs := make([]string, 0, len(d.arches))
	for name := range d.arches {
		archs = append(archs, name)
	}
	sort.Strings(archs)
	return archs
}

func (d *distribution) GetArch(arch string) (distro.Arch, error) {
	a, exists := d.arches[arch]
	if !exists {
		return nil, errors.New("invalid architecture: " + arch)
	}

	return &a, nil
}

func (d *distribution) addArches(arches ...architecture) {
	if d.arches == nil {
		d.arches = map[string]architecture{}
	}

	for _, a := range arches {
		d.arches[a.name] = architecture{
			distro:             d,
			name:               a.name,
			bootloaderPackages: a.bootloaderPackages,
			buildPackages:      a.buildPackages,
			uefi:               a.uefi,
			imageTypes:         a.imageTypes,
		}
	}
}

func (a *architecture) Name() string {
	return a.name
}

func (a *architecture) ListImageTypes() []string {
	formats := make([]string, 0, len(a.imageTypes))
	for name := range a.imageTypes {
		formats = append(formats, name)
	}
	sort.Strings(formats)
	return formats
}

func (a *architecture) GetImageType(imageType string) (distro.ImageType, error) {
	t, exists := a.imageTypes[imageType]
	if !exists {
		return nil, errors.New("invalid image type: " + imageType)
	}

	return t, nil
}

func (a *architecture) addImageTypes(imageTypes ...imageType) {
	if a.imageTypes == nil {
		a.imageTypes = map[string]distro.ImageType{}
	}
	for _, it := range imageTypes {
		a.imageTypes[it.name] = &imageType{
			arch:                    a,
			name:                    it.name,
			filename:                it.filename,
			mimeType:                it.mimeType,
			packages:                it.packages,
			excludedPackages:        it.excludedPackages,
			enabledServices:         it.enabledServices,
			disabledServices:        it.disabledServices,
			defaultTarget:           it.defaultTarget,
			kernelOptions:           it.kernelOptions,
			bootable:                it.bootable,
			rpmOstree:               it.rpmOstree,
			defaultSize:             it.defaultSize,
			buildPipelines:          it.buildPipelines,
			payloadPipelines:        it.payloadPipelines,
			exports:                 it.exports,
			partitionTableGenerator: it.partitionTableGenerator,
			assembler:               it.assembler,
		}
	}
}

// For the secondary implementation of image type.
// Temporary; for supporting the new Manifest schema, until everything is
// ported.
func (a *architecture) addS2ImageTypes(imageTypes ...imageTypeS2) {
	for _, it := range imageTypes {
		a.imageTypes[it.name] = &imageTypeS2{
			arch:                    a,
			name:                    it.name,
			filename:                it.filename,
			mimeType:                it.mimeType,
			packageSets:             it.packageSets,
			enabledServices:         it.enabledServices,
			disabledServices:        it.disabledServices,
			defaultTarget:           it.defaultTarget,
			kernelOptions:           it.kernelOptions,
			bootable:                it.bootable,
			rpmOstree:               it.rpmOstree,
			defaultSize:             it.defaultSize,
			bootISO:                 it.bootISO,
			buildPipelines:          it.buildPipelines,
			payloadPipelines:        it.payloadPipelines,
			exports:                 it.exports,
			pipelines:               it.pipelines,
			partitionTableGenerator: it.partitionTableGenerator,
		}
	}
}

func (t *imageType) Name() string {
	return t.name
}

func (t *imageType) Filename() string {
	return t.filename
}

func (t *imageType) MIMEType() string {
	return t.mimeType
}

func (t *imageType) OSTreeRef() string {
	if t.rpmOstree {
		return fmt.Sprintf(ostreeRef, t.Arch().Name())
	}
	return ""
}

func (t *imageType) Size(size uint64) uint64 {
	const MegaByte = 1024 * 1024
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.name == "vhd" && size%MegaByte != 0 {
		size = (size/MegaByte + 1) * MegaByte
	}
	if size == 0 {
		size = t.defaultSize
	}
	return size
}

func (t *imageType) PartitionType() string {
	return ""
}

func (t *imageType) Packages(bp blueprint.Blueprint) ([]string, []string) {
	packages := append(t.packages, bp.GetPackages()...)
	timezone, _ := bp.Customizations.GetTimezoneSettings()
	if timezone != nil {
		packages = append(packages, "chrony")
	}
	if t.bootable {
		packages = append(packages, t.arch.bootloaderPackages...)
	}

	if t.arch.distro.isCentos {
		// drop insights from centos, it's not available there
		packages = removePackage(packages, "insights-client")
	}

	// copy the list of excluded packages from the image type
	// and subtract any packages found in the blueprint (this
	// will not handle the issue with dependencies present in
	// the list of excluded packages, but it will create a
	// possibility of a workaround at least)
	excludedPackages := append([]string(nil), t.excludedPackages...)
	for _, pkg := range bp.GetPackages() {
		// removePackage is fine if the package doesn't exist
		excludedPackages = removePackage(excludedPackages, pkg)
	}

	return packages, excludedPackages
}

func (t *imageType) BuildPackages() []string {
	packages := append(t.arch.distro.buildPackages, t.arch.buildPackages...)
	if t.rpmOstree {
		packages = append(packages, "rpm-ostree")
	}
	return packages
}

func (t *imageType) PackageSets(bp blueprint.Blueprint, repos []rpmmd.RepoConfig) map[string][]rpmmd.PackageSet {
	includePackages, excludePackages := t.Packages(bp)
	return map[string][]rpmmd.PackageSet{
		"packages": {{
			Include:      includePackages,
			Exclude:      excludePackages,
			Repositories: repos,
		}},
		"build-packages": {{
			Include:      t.BuildPackages(),
			Repositories: repos,
		}},
	}
}

func (t *imageType) BuildPipelines() []string {
	if len(t.buildPipelines) > 0 {
		return t.buildPipelines
	}
	// fallback for v1 image types
	return distro.BuildPipelinesFallback()
}

func (t *imageType) PayloadPipelines() []string {
	if len(t.payloadPipelines) > 0 {
		return t.payloadPipelines
	}
	// fallback for v1 image types
	return distro.PayloadPipelinesFallback()
}

func (t *imageType) PayloadPackageSets() []string {
	return []string{"packages"}
}

func (t *imageType) PackageSetsChains() map[string][]string {
	return map[string][]string{}
}

func (t *imageType) Exports() []string {
	if len(t.exports) > 0 {
		return t.exports
	}
	// fallback for v1 image types
	return distro.ExportsFallback()
}

func (t *imageType) Manifest(c *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSpecSets map[string][]rpmmd.PackageSpec,
	seed int64) (distro.Manifest, error) {
	source := rand.NewSource(seed)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)
	pipeline, err := t.pipeline(c, options, repos, packageSpecSets["packages"], packageSpecSets["build-packages"], rng)
	if err != nil {
		return distro.Manifest{}, err
	}

	return json.Marshal(
		osbuild.Manifest{
			Sources:  *sources(append(packageSpecSets["packages"], packageSpecSets["build-packages"]...)),
			Pipeline: *pipeline,
		},
	)
}

func (d *distribution) Name() string {
	return d.name
}

func (d *distribution) Releasever() string {
	return releaseVersion
}

func (d *distribution) ModulePlatformID() string {
	return d.modulePlatformID
}

func (d *distribution) OSTreeRef() string {
	return d.ostreeRef
}

func (d *distribution) isRHEL() bool {
	return strings.HasPrefix(d.name, "rhel")
}

func sources(packages []rpmmd.PackageSpec) *osbuild.Sources {
	files := &osbuild.FilesSource{
		URLs: make(map[string]osbuild.FileSource),
	}
	for _, pkg := range packages {
		fileSource := osbuild.FileSource{
			URL: pkg.RemoteLocation,
		}
		if pkg.Secrets == "org.osbuild.rhsm" {
			fileSource.Secrets = &osbuild.Secret{
				Name: "org.osbuild.rhsm",
			}
		}
		files.URLs[pkg.Checksum] = fileSource
	}
	return &osbuild.Sources{
		"org.osbuild.files": files,
	}
}

func (t *imageType) pipeline(c *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, rng *rand.Rand) (*osbuild.Pipeline, error) {

	imageSize := t.Size(options.Size)

	if kernelOpts := c.GetKernel(); kernelOpts != nil && kernelOpts.Append != "" && t.rpmOstree {
		return nil, fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := c.GetFilesystems()

	if mountpoints != nil && t.rpmOstree {
		return nil, fmt.Errorf("Custom mountpoints are not supported for ostree types")
	}

	invalidMountpoints := []string{}
	for _, m := range mountpoints {
		if m.Mountpoint != "/" {
			invalidMountpoints = append(invalidMountpoints, m.Mountpoint)
		} else {
			if m.MinSize > imageSize {
				imageSize = m.MinSize
			}
		}
	}

	if len(invalidMountpoints) > 0 {
		return nil, fmt.Errorf("The following custom mountpoints are not supported %+q", invalidMountpoints)
	}

	var pt *disk.PartitionTable
	if t.partitionTableGenerator != nil {
		table := t.partitionTableGenerator(imageSize, t.arch, rng)
		pt = &table
	}

	p := &osbuild.Pipeline{}
	if t.arch.distro.isCentos {
		p.SetBuild(t.buildPipeline(repos, *t.arch, buildPackageSpecs), "org.osbuild.centos8")
	} else {
		p.SetBuild(t.buildPipeline(repos, *t.arch, buildPackageSpecs), "org.osbuild.rhel84")
	}

	if t.arch.Name() == distro.S390xArchName && t.bootable {
		if pt == nil {
			panic("s390x image must have a partition table, this is a programming error")
		}

		rootFs := pt.FindMountable("/")
		if rootFs == nil {
			panic("s390x image must have a root filesystem, this is a programming error")
		}

		p.AddStage(osbuild.NewKernelCmdlineStage(&osbuild.KernelCmdlineStageOptions{
			RootFsUUID: rootFs.GetFSSpec().UUID,
			KernelOpts: t.kernelOptions,
		}))
	}

	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(*t.arch, repos, packageSpecs)))
	p.AddStage(osbuild.NewFixBLSStage())

	if pt != nil {
		p.AddStage(osbuild.NewFSTabStage(osbuild.NewFSTabStageOptions(pt)))
	}

	if t.bootable {
		if t.arch.Name() != distro.S390xArchName {
			p.AddStage(osbuild.NewGRUB2Stage(t.grub2StageOptions(pt, t.kernelOptions, c.GetKernel(), packageSpecs, t.arch.uefi, t.arch.legacy)))
		}
	}

	// TODO support setting all languages and install corresponding langpack-* package
	language, keyboard := c.GetPrimaryLocale()

	if language != nil {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: *language}))
	} else {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US.UTF-8"}))
	}

	if keyboard != nil {
		p.AddStage(osbuild.NewKeymapStage(&osbuild.KeymapStageOptions{Keymap: *keyboard}))
	}

	if hostname := c.GetHostname(); hostname != nil {
		p.AddStage(osbuild.NewHostnameStage(&osbuild.HostnameStageOptions{Hostname: *hostname}))
	}

	timezone, ntpServers := c.GetTimezoneSettings()

	if timezone != nil {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: *timezone}))
	} else {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: "America/New_York"}))
	}

	if len(ntpServers) > 0 {
		p.AddStage(osbuild.NewChronyStage(&osbuild.ChronyStageOptions{Timeservers: ntpServers}))
	}

	if groups := c.GetGroups(); len(groups) > 0 {
		p.AddStage(osbuild.NewGroupsStage(osbuild.NewGroupsStageOptions(groups)))
	}

	if userOptions, err := osbuild.NewUsersStageOptions(c.GetUsers()); err != nil {
		return nil, err
	} else if userOptions != nil {
		p.AddStage(osbuild.NewUsersStage(userOptions))
	}

	if services := c.GetServices(); services != nil || t.enabledServices != nil || t.disabledServices != nil || t.defaultTarget != "" {
		p.AddStage(osbuild.NewSystemdStage(t.systemdStageOptions(t.enabledServices, t.disabledServices, services, t.defaultTarget)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(t.firewallStageOptions(firewall)))
	}

	if t.arch.Name() == distro.S390xArchName {
		p.AddStage(osbuild.NewZiplStage(&osbuild.ZiplStageOptions{}))
	}

	p.AddStage(osbuild.NewSELinuxStage(t.selinuxStageOptions()))

	// These are the current defaults for the sysconfig stage. This can be changed to be image type exclusive if different configs are needed.
	p.AddStage(osbuild.NewSysconfigStage(&osbuild.SysconfigStageOptions{
		Kernel: osbuild.SysconfigKernelOptions{
			UpdateDefault: true,
			DefaultKernel: "kernel",
		},
		Network: osbuild.SysconfigNetworkOptions{
			Networking: true,
			NoZeroConf: true,
		},
	}))

	if t.rpmOstree {
		p.AddStage(osbuild.NewRPMOSTreeStage(&osbuild.RPMOSTreeStageOptions{
			EtcGroupMembers: []string{
				// NOTE: We may want to make this configurable.
				"wheel", "docker",
			},
		}))
	}

	if options.Subscription != nil {
		commands := []string{
			fmt.Sprintf("/usr/sbin/subscription-manager register --org=%s --activationkey=%s --serverurl %s --baseurl %s", options.Subscription.Organization, options.Subscription.ActivationKey, options.Subscription.ServerUrl, options.Subscription.BaseUrl),
		}
		if options.Subscription.Insights {
			commands = append(commands, "/usr/bin/insights-client --register")
		}

		p.AddStage(osbuild.NewFirstBootStage(&osbuild.FirstBootStageOptions{
			Commands:       commands,
			WaitForNetwork: true,
		},
		))
	} else {
		// RHSM DNF plugins should be by default disabled on RHEL Guest KVM images
		if t.Name() == "qcow2" {
			p.AddStage(osbuild.NewRHSMStage(&osbuild.RHSMStageOptions{
				DnfPlugins: &osbuild.RHSMStageOptionsDnfPlugins{
					ProductID: &osbuild.RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
					SubscriptionManager: &osbuild.RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
				},
			}))
		}
	}

	p.Assembler = t.assembler(pt, options, t.arch)

	return p, nil
}

func (t *imageType) buildPipeline(repos []rpmmd.RepoConfig, arch architecture, buildPackageSpecs []rpmmd.PackageSpec) *osbuild.Pipeline {
	p := &osbuild.Pipeline{}
	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(arch, repos, buildPackageSpecs)))
	p.AddStage(osbuild.NewSELinuxStage(t.selinuxStageOptions()))
	return p
}

func (t *imageType) rpmStageOptions(arch architecture, repos []rpmmd.RepoConfig, specs []rpmmd.PackageSpec) *osbuild.RPMStageOptions {
	var gpgKeys []string
	for _, repo := range repos {
		if repo.GPGKey == "" {
			continue
		}
		gpgKeys = append(gpgKeys, repo.GPGKey)
	}

	var packages []osbuild.RPMPackage
	for _, spec := range specs {
		pkg := osbuild.RPMPackage{
			Checksum: spec.Checksum,
			CheckGPG: spec.CheckGPG,
		}
		packages = append(packages, pkg)
	}

	return &osbuild.RPMStageOptions{
		GPGKeys:  gpgKeys,
		Packages: packages,
	}
}

func (t *imageType) firewallStageOptions(firewall *blueprint.FirewallCustomization) *osbuild.FirewallStageOptions {
	options := osbuild.FirewallStageOptions{
		Ports: firewall.Ports,
	}

	if firewall.Services != nil {
		options.EnabledServices = firewall.Services.Enabled
		options.DisabledServices = firewall.Services.Disabled
	}

	return &options
}

func (t *imageType) systemdStageOptions(enabledServices, disabledServices []string, s *blueprint.ServicesCustomization, target string) *osbuild.SystemdStageOptions {
	if s != nil {
		enabledServices = append(enabledServices, s.Enabled...)
		disabledServices = append(disabledServices, s.Disabled...)
	}
	return &osbuild.SystemdStageOptions{
		EnabledServices:  enabledServices,
		DisabledServices: disabledServices,
		DefaultTarget:    target,
	}
}

func (t *imageType) grub2StageOptions(pt *disk.PartitionTable, kernelOptions string, kernel *blueprint.KernelCustomization, packages []rpmmd.PackageSpec, uefi bool, legacy string) *osbuild.GRUB2StageOptions {
	if pt == nil {
		panic("partition table must be defined for grub2 stage, this is a programming error")
	}
	rootFs := pt.FindMountable("/")
	if rootFs == nil {
		panic("root filesystem must be defined for grub2 stage, this is a programming error")
	}

	stageOptions := osbuild.GRUB2StageOptions{
		RootFilesystemUUID: uuid.MustParse(rootFs.GetFSSpec().UUID),
		KernelOptions:      kernelOptions,
		Legacy:             legacy,
	}

	if uefi {
		var vendor string
		if t.arch.distro.isCentos {
			vendor = "centos"
		} else {
			vendor = "redhat"
		}
		stageOptions.UEFI = &osbuild.GRUB2UEFI{
			Vendor: vendor,
		}
	}

	if !uefi {
		stageOptions.Legacy = t.arch.legacy
	}

	if kernel != nil {
		if kernel.Append != "" {
			stageOptions.KernelOptions += " " + kernel.Append
		}
		for _, pkg := range packages {
			if pkg.Name == kernel.Name {
				stageOptions.SavedEntry = "ffffffffffffffffffffffffffffffff-" + pkg.Version + "-" + pkg.Release + "." + pkg.Arch
				break
			}
		}
	}

	return &stageOptions
}

func (t *imageType) selinuxStageOptions() *osbuild.SELinuxStageOptions {
	return &osbuild.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
}

func defaultPartitionTable(imageSize uint64, arch distro.Arch, rng *rand.Rand) disk.PartitionTable {
	if arch.Name() == distro.X86_64ArchName {
		return disk.PartitionTable{
			Size: imageSize,
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: "gpt",
			Partitions: []disk.Partition{
				{
					Bootable: true,
					Size:     2048,
					Start:    2048,
					Type:     "21686148-6449-6E6F-744E-656564454649",
					UUID:     "FAC7F1FB-3E8D-4137-A512-961DE09A5549",
				},
				{
					Start: 4096,
					Size:  204800,
					Type:  "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
					UUID:  "68B2905B-DF3E-4FB3-80FA-49D1E773AA33",
					Payload: &disk.Filesystem{
						Type:         "vfat",
						UUID:         "7B77-95E7",
						Mountpoint:   "/boot/efi",
						FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
						FSTabFreq:    0,
						FSTabPassNo:  2,
					},
				},
				{
					Start: 208896,
					Type:  "0FC63DAF-8483-4772-8E79-3D69D8477DE4",
					UUID:  "6264D520-3FB9-423F-8AB8-7A0A8E3D3562",
					Payload: &disk.Filesystem{
						Type:         "xfs",
						UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
						Label:        "root",
						Mountpoint:   "/",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
			},
		}
	} else if arch.Name() == distro.Aarch64ArchName {
		return disk.PartitionTable{
			Size: imageSize,
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: "gpt",
			Partitions: []disk.Partition{
				{
					Start: 2048,
					Size:  204800,
					Type:  "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
					UUID:  "68B2905B-DF3E-4FB3-80FA-49D1E773AA33",
					Payload: &disk.Filesystem{
						Type:         "vfat",
						UUID:         "7B77-95E7",
						Mountpoint:   "/boot/efi",
						FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
						FSTabFreq:    0,
						FSTabPassNo:  2,
					},
				},
				{
					Start: 206848,
					Type:  "0FC63DAF-8483-4772-8E79-3D69D8477DE4",
					UUID:  "6264D520-3FB9-423F-8AB8-7A0A8E3D3562",
					Payload: &disk.Filesystem{
						Type:         "xfs",
						UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
						Label:        "root",
						Mountpoint:   "/",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
			},
		}
	} else if arch.Name() == distro.Ppc64leArchName {
		return disk.PartitionTable{
			Size: imageSize,
			UUID: "0x14fc63d2",
			Type: "dos",
			Partitions: []disk.Partition{
				{
					Size:     8192,
					Type:     "41",
					Bootable: true,
				},
				{
					Start: 10240,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
						Mountpoint:   "/",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
			},
		}
	} else if arch.Name() == distro.S390xArchName {
		return disk.PartitionTable{
			Size: imageSize,
			UUID: "0x14fc63d2",
			Type: "dos",
			Partitions: []disk.Partition{
				{
					Start:    2048,
					Bootable: true,
					Payload: &disk.Filesystem{
						Type:         "xfs",
						UUID:         uuid.Must(newRandomUUIDFromReader(rng)).String(),
						Mountpoint:   "/",
						FSTabOptions: "defaults",
						FSTabFreq:    0,
						FSTabPassNo:  0,
					},
				},
			},
		}
	}

	panic("unknown arch: " + arch.Name())
}

func qemuAssembler(pt *disk.PartitionTable, format string, filename string, imageOptions distro.ImageOptions, arch distro.Arch, qcow2Compat string, vmdkSubformat osbuild.VMDKSubformat) *osbuild.Assembler {
	options := osbuild.NewQEMUAssemblerOptions(pt)

	options.Format = format
	options.Filename = filename
	options.Qcow2Compat = qcow2Compat
	options.VMDKSubformat = vmdkSubformat

	if arch.Name() == "x86_64" {
		options.Bootloader = &osbuild.QEMUBootloader{
			Type: "grub2",
		}
	} else if arch.Name() == distro.Ppc64leArchName {
		options.Bootloader = &osbuild.QEMUBootloader{
			Type:     "grub2",
			Platform: "powerpc-ieee1275",
		}
	} else if arch.Name() == distro.S390xArchName {
		options.Bootloader = &osbuild.QEMUBootloader{
			Type: "zipl",
		}
	}
	return osbuild.NewQEMUAssembler(&options)
}

func tarAssembler(filename, compression string) *osbuild.Assembler {
	return osbuild.NewTarAssembler(
		&osbuild.TarAssemblerOptions{
			Filename:    filename,
			Compression: compression,
		})
}

func ostreeCommitAssembler(options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
	return osbuild.NewOSTreeCommitAssembler(
		&osbuild.OSTreeCommitAssemblerOptions{
			Ref:    options.OSTree.Ref,
			Parent: options.OSTree.Parent,
			Tar: osbuild.OSTreeCommitAssemblerTarOptions{
				Filename: "commit.tar",
			},
		},
	)
}

func newRandomUUIDFromReader(r io.Reader) (uuid.UUID, error) {
	var id uuid.UUID
	_, err := io.ReadFull(r, id[:])
	if err != nil {
		return uuid.Nil, err
	}
	id[6] = (id[6] & 0x0f) | 0x40 // Version 4
	id[8] = (id[8] & 0x3f) | 0x80 // Variant is 10
	return id, nil
}

func removePackage(packages []string, packageToRemove string) []string {
	for i, pkg := range packages {
		if pkg == packageToRemove {
			// override the package with the last one from the list
			packages[i] = packages[len(packages)-1]

			// drop the last package from the slice
			return packages[:len(packages)-1]
		}
	}
	return packages
}

// New creates a new distro object, defining the supported architectures and image types
func New() distro.Distro {
	return newDistro(defaultName, modulePlatformID, ostreeRef, false)
}

func NewCentos() distro.Distro {
	return newDistro(defaultCentosName, modulePlatformID, ostreeRef, true)
}

func NewHostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	return newDistro(name, modulePlatformID, ostreeRef, false)
}

func NewCentosHostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	return newDistro(name, modulePlatformID, ostreeRef, true)
}

func newDistro(name, modulePlatformID, ostreeRef string, isCentos bool) distro.Distro {
	const GigaByte = 1024 * 1024 * 1024

	edgeImgTypeX86_64 := imageType{
		name:     "rhel-edge-commit",
		filename: "commit.tar",
		mimeType: "application/x-tar",
		packages: []string{
			"redhat-release", // TODO: is this correct for Edge?
			"glibc", "glibc-minimal-langpack", "nss-altfiles",
			"dracut-config-generic", "dracut-network",
			"basesystem", "bash", "platform-python",
			"shadow-utils", "chrony", "setup", "shadow-utils",
			"sudo", "systemd", "coreutils", "util-linux",
			"curl", "vim-minimal",
			"rpm", "rpm-ostree", "polkit",
			"lvm2", "cryptsetup", "pinentry",
			"e2fsprogs", "dosfstools",
			"keyutils", "gnupg2",
			"attr", "xz", "gzip",
			"firewalld", "iptables",
			"NetworkManager", "NetworkManager-wifi", "NetworkManager-wwan",
			"wpa_supplicant",
			"dnsmasq", "traceroute",
			"hostname", "iproute", "iputils",
			"openssh-clients", "procps-ng", "rootfiles",
			"openssh-server", "passwd",
			"policycoreutils", "policycoreutils-python-utils",
			"selinux-policy-targeted", "setools-console",
			"less", "tar", "rsync",
			"fwupd", "usbguard",
			"bash-completion", "tmux",
			"ima-evm-utils",
			"audit",
			"podman", "container-selinux", "skopeo", "criu",
			"slirp4netns", "fuse-overlayfs",
			"clevis", "clevis-dracut", "clevis-luks",
			"greenboot", "greenboot-grub2", "greenboot-rpm-ostree-grub2", "greenboot-reboot", "greenboot-status",
			// x86 specific
			"grub2", "grub2-efi-x64", "efibootmgr", "shim-x64", "microcode_ctl",
			"iwl1000-firmware", "iwl100-firmware", "iwl105-firmware", "iwl135-firmware",
			"iwl2000-firmware", "iwl2030-firmware", "iwl3160-firmware", "iwl5000-firmware",
			"iwl5150-firmware", "iwl6000-firmware", "iwl6050-firmware", "iwl7260-firmware",
		},
		excludedPackages: []string{
			"rng-tools",
		},
		enabledServices: []string{
			"NetworkManager.service", "firewalld.service", "sshd.service",
			"greenboot-grub2-set-counter", "greenboot-grub2-set-success", "greenboot-healthcheck",
			"greenboot-rpm-ostree-grub2-check-fallback", "greenboot-status", "greenboot-task-runner",
			"redboot-auto-reboot", "redboot-task-runner",
		},
		rpmOstree: true,
		assembler: func(pt *disk.PartitionTable, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return ostreeCommitAssembler(options, arch)
		},
	}
	edgeImgTypeAarch64 := imageType{
		name:     "rhel-edge-commit",
		filename: "commit.tar",
		mimeType: "application/x-tar",
		packages: []string{
			"redhat-release", // TODO: is this correct for Edge?
			"glibc", "glibc-minimal-langpack", "nss-altfiles",
			"dracut-config-generic", "dracut-network",
			"basesystem", "bash", "platform-python",
			"shadow-utils", "chrony", "setup", "shadow-utils",
			"sudo", "systemd", "coreutils", "util-linux",
			"curl", "vim-minimal",
			"rpm", "rpm-ostree", "polkit",
			"lvm2", "cryptsetup", "pinentry",
			"e2fsprogs", "dosfstools",
			"keyutils", "gnupg2",
			"attr", "xz", "gzip",
			"firewalld", "iptables",
			"NetworkManager", "NetworkManager-wifi", "NetworkManager-wwan",
			"wpa_supplicant",
			"dnsmasq", "traceroute",
			"hostname", "iproute", "iputils",
			"openssh-clients", "procps-ng", "rootfiles",
			"openssh-server", "passwd",
			"policycoreutils", "policycoreutils-python-utils",
			"selinux-policy-targeted", "setools-console",
			"less", "tar", "rsync",
			"fwupd", "usbguard",
			"bash-completion", "tmux",
			"ima-evm-utils",
			"audit",
			"podman", "container-selinux", "skopeo", "criu",
			"slirp4netns", "fuse-overlayfs",
			"clevis", "clevis-dracut", "clevis-luks",
			"greenboot", "greenboot-grub2", "greenboot-rpm-ostree-grub2", "greenboot-reboot", "greenboot-status",
			// aarch64 specific
			"grub2-efi-aa64", "efibootmgr", "shim-aa64",
			"iwl7260-firmware",
		},
		excludedPackages: []string{
			"rng-tools",
		},
		enabledServices: []string{
			"NetworkManager.service", "firewalld.service", "sshd.service",
			"greenboot-grub2-set-counter", "greenboot-grub2-set-success", "greenboot-healthcheck",
			"greenboot-rpm-ostree-grub2-check-fallback", "greenboot-status", "greenboot-task-runner",
			"redboot-auto-reboot", "redboot-task-runner",
		},
		rpmOstree: true,
		assembler: func(pt *disk.PartitionTable, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return ostreeCommitAssembler(options, arch)
		},
	}
	amiImgType := imageType{
		name:     "ami",
		filename: "image.raw",
		mimeType: "application/octet-stream",
		packages: []string{
			"checkpolicy",
			"chrony",
			"cloud-init",
			"cloud-init",
			"cloud-utils-growpart",
			"@core",
			"dhcp-client",
			"gdisk",
			"insights-client",
			"langpacks-en",
			"net-tools",
			"NetworkManager",
			"redhat-release",
			"redhat-release-eula",
			"rsync",
			"selinux-policy-targeted",
			"tar",
			"yum-utils",

			// TODO this doesn't exist in BaseOS or AppStream
			// "rh-amazon-rhui-client",
		},
		excludedPackages: []string{
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-lib",
			"alsa-tools-firmware",
			"biosdevname",
			"dracut-config-rescue",
			"firewalld",
			"iprutils",
			"ivtv-firmware",
			"iwl1000-firmware",
			"iwl100-firmware",
			"iwl105-firmware",
			"iwl135-firmware",
			"iwl2000-firmware",
			"iwl2030-firmware",
			"iwl3160-firmware",
			"iwl3945-firmware",
			"iwl4965-firmware",
			"iwl5000-firmware",
			"iwl5150-firmware",
			"iwl6000-firmware",
			"iwl6000g2a-firmware",
			"iwl6000g2b-firmware",
			"iwl6050-firmware",
			"iwl7260-firmware",
			"libertas-sd8686-firmware",
			"libertas-sd8787-firmware",
			"libertas-usb8388-firmware",
			"plymouth",
			"rng-tools",

			// TODO this cannot be removed, because the kernel (?)
			// depends on it. The ec2 kickstart force-removes it.
			// "linux-firmware",

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		defaultTarget:           "multi-user.target",
		kernelOptions:           "console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto",
		bootable:                true,
		defaultSize:             6 * GigaByte,
		partitionTableGenerator: defaultPartitionTable,
		assembler: func(pt *disk.PartitionTable, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler(pt, "raw", "image.raw", options, arch, "", "")
		},
	}

	qcow2ImageType := imageType{
		name:     "qcow2",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packages: []string{
			"@core",
			"authselect-compat",
			"chrony",
			"cloud-init",
			"cloud-utils-growpart",
			"cockpit-system",
			"cockpit-ws",
			"dhcp-client",
			"dnf",
			"dnf-utils",
			"dosfstools",
			"dracut-norescue",
			"insights-client",
			"NetworkManager",
			"net-tools",
			"nfs-utils",
			"oddjob",
			"oddjob-mkhomedir",
			"psmisc",
			"python3-jsonschema",
			"qemu-guest-agent",
			"redhat-release",
			"redhat-release-eula",
			"rsync",
			"subscription-manager-cockpit",
			"tar",
			"tcpdump",
			"yum",
		},
		excludedPackages: []string{
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-lib",
			"alsa-tools-firmware",
			"biosdevname",
			"dnf-plugin-spacewalk",
			"dracut-config-rescue",
			"fedora-release",
			"fedora-repos",
			"firewalld",
			"fwupd",
			"iprutils",
			"ivtv-firmware",
			"iwl100-firmware",
			"iwl1000-firmware",
			"iwl105-firmware",
			"iwl135-firmware",
			"iwl2000-firmware",
			"iwl2030-firmware",
			"iwl3160-firmware",
			"iwl3945-firmware",
			"iwl4965-firmware",
			"iwl5000-firmware",
			"iwl5150-firmware",
			"iwl6000-firmware",
			"iwl6000g2a-firmware",
			"iwl6000g2b-firmware",
			"iwl6050-firmware",
			"iwl7260-firmware",
			"langpacks-*",
			"langpacks-en",
			"langpacks-en",
			"libertas-sd8686-firmware",
			"libertas-sd8787-firmware",
			"libertas-usb8388-firmware",
			"nss",
			"plymouth",
			"rng-tools",
			"udisks2",
		},
		defaultTarget:           "multi-user.target",
		kernelOptions:           "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0 crashkernel=auto",
		bootable:                true,
		defaultSize:             10 * GigaByte,
		partitionTableGenerator: defaultPartitionTable,
		assembler: func(pt *disk.PartitionTable, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			// guest images of RHEL 8 must be bootable with older QEMUs.
			const qcow2Compat = "0.10"
			return qemuAssembler(pt, "qcow2", "disk.qcow2", options, arch, qcow2Compat, "")
		},
	}

	openstackImgType := imageType{
		name:     "openstack",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packages: []string{
			// Defaults
			"@Core",
			"langpacks-en",

			// From the lorax kickstart
			"selinux-policy-targeted",
			"cloud-init",
			"qemu-guest-agent",
			"spice-vdagent",
		},
		excludedPackages: []string{
			"dracut-config-rescue",
			"rng-tools",
		},
		kernelOptions:           "ro net.ifnames=0",
		bootable:                true,
		defaultSize:             4 * GigaByte,
		partitionTableGenerator: defaultPartitionTable,
		assembler: func(pt *disk.PartitionTable, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler(pt, "qcow2", "disk.qcow2", options, arch, "", "")
		},
	}

	tarImgType := imageType{
		name:     "tar",
		filename: "root.tar.xz",
		mimeType: "application/x-tar",
		packages: []string{
			"policycoreutils",
			"selinux-policy-targeted",
		},
		excludedPackages: []string{
			"rng-tools",
		},
		bootable:      false,
		kernelOptions: "ro net.ifnames=0",
		assembler: func(pt *disk.PartitionTable, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return tarAssembler("root.tar.xz", "xz")
		},
	}

	vhdImgType := imageType{
		name:     "vhd",
		filename: "disk.vhd",
		mimeType: "application/x-vhd",
		packages: []string{
			// Defaults
			"@Core",
			"langpacks-en",

			// From the lorax kickstart
			"selinux-policy-targeted",
			"chrony",
			"WALinuxAgent",
			"python3",
			"net-tools",
			"cloud-init",
			"cloud-utils-growpart",
			"gdisk",
		},
		excludedPackages: []string{
			"dracut-config-rescue",
			"rng-tools",

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		enabledServices: []string{
			"sshd",
			"waagent",
		},
		defaultTarget:           "multi-user.target",
		kernelOptions:           "ro biosdevname=0 rootdelay=300 console=ttyS0 earlyprintk=ttyS0 net.ifnames=0",
		bootable:                true,
		defaultSize:             4 * GigaByte,
		partitionTableGenerator: defaultPartitionTable,
		assembler: func(pt *disk.PartitionTable, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler(pt, "vpc", "disk.vhd", options, arch, "", "")
		},
	}

	vmdkImgType := imageType{
		name:     "vmdk",
		filename: "disk.vmdk",
		mimeType: "application/x-vmdk",
		packages: []string{
			"@core",
			"chrony",
			"firewalld",
			"langpacks-en",
			"open-vm-tools",
			"selinux-policy-targeted",
		},
		excludedPackages: []string{
			"dracut-config-rescue",
			"rng-tools",

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		kernelOptions:           "ro net.ifnames=0",
		bootable:                true,
		defaultSize:             4 * GigaByte,
		partitionTableGenerator: defaultPartitionTable,
		assembler: func(pt *disk.PartitionTable, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler(pt, "vmdk", "disk.vmdk", options, arch, "", osbuild.VMDKSubformatStreamOptimized)
		},
	}

	r := distribution{
		buildPackages: []string{
			"dnf",
			"dosfstools",
			"e2fsprogs",
			"glibc",
			"policycoreutils",
			"python36",
			"python3-iniparse", // dependency of org.osbuild.rhsm stage
			"qemu-img",
			"selinux-policy-targeted",
			"systemd",
			"tar",
			"xfsprogs",
			"xz",
		},
		isCentos:         isCentos,
		name:             name,
		modulePlatformID: modulePlatformID,
		ostreeRef:        ostreeRef,
	}
	x8664 := architecture{
		distro: &r,
		name:   "x86_64",
		bootloaderPackages: []string{
			"dracut-config-generic",
			"grub2-pc",
			"grub2-efi-x64",
			"shim-x64",
		},
		buildPackages: []string{
			"grub2-pc",
		},
		legacy: "i386-pc",
		uefi:   true,
	}

	edgeOCIImgTypeX86_64 := imageTypeS2{
		name:     "rhel-edge-container",
		filename: "rhel84-container.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]rpmmd.PackageSet{
			"packages": {
				Include: edgeImgTypeX86_64.packages,
				Exclude: edgeImgTypeX86_64.excludedPackages,
			},
			"container": {Include: []string{"httpd"}},
		},
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-tree", "ostree-commit", "container-tree", "assembler"},
		exports:          []string{"assembler"},
		enabledServices:  edgeImgTypeX86_64.enabledServices,
		rpmOstree:        true,
		bootISO:          false,
		pipelines:        edgePipelines,
	}

	edgeBuildPkgs := []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"grub2-pc",
		"policycoreutils",
		"python36",
		"python3-iniparse",
		"qemu-img",
		"rpm-ostree",
		"systemd",
		"tar",
		"xfsprogs",
		"xz",
		"selinux-policy-targeted",
		"genisoimage",
		"isomd5sum",
		"xorriso",
		"syslinux",
		"lorax-templates-generic",
		"lorax-templates-rhel",
		"syslinux-nonlinux",
		"squashfs-tools",
		"grub2-pc-modules",
		"grub2-tools",
		"grub2-efi-x64",
		"shim-x64",
		"efibootmgr",
		"grub2-tools-minimal",
		"grub2-tools-extra",
		"grub2-tools-efi",
		"grub2-efi-x64",
		"grub2-efi-x64-cdboot",
		"shim-ia32",
		"grub2-efi-ia32-cdboot",
	}

	edgeInstallerPkgs := []string{
		"anaconda",
		"anaconda-widgets",
		"kdump-anaconda-addon",
		"anaconda-install-env-deps",
		"oscap-anaconda-addon",
		"redhat-release-eula",
		"dnf",
		"rpm-ostree",
		"ostree",
		"ostree",
		"pigz",
		"kernel",
		"kernel-modules",
		"kernel-modules-extra",
		"grubby",
		"iwl100-firmware",
		"iwl1000-firmware",
		"iwl105-firmware",
		"iwl135-firmware",
		"iwl2000-firmware",
		"iwl2030-firmware",
		"iwl3160-firmware",
		"iwl3945-firmware",
		"iwl4965-firmware",
		"iwl5000-firmware",
		"iwl5150-firmware",
		"iwl6000-firmware",
		"iwl6000g2a-firmware",
		"iwl6000g2b-firmware",
		"iwl6050-firmware",
		"iwl7260-firmware",
		"libertas-sd8686-firmware",
		"libertas-sd8787-firmware",
		"libertas-usb8388-firmware",
		"libertas-usb8388-olpc-firmware",
		"linux-firmware",
		"alsa-firmware",
		"alsa-tools-firmware",
		"glibc-all-langpacks",
		"grub2-tools-efi",
		"efibootmgr",
		"shim-x64",
		"grub2-efi-x64-cdboot",
		"shim-ia32",
		"grub2-efi-ia32-cdboot",
		"biosdevname",
		"memtest86+",
		"syslinux",
		"grub2-tools",
		"grub2-tools-minimal",
		"grub2-tools-extra",
		"plymouth",
		"anaconda-dracut",
		"dracut-network",
		"dracut-config-generic",
		"initscripts",
		"cryptsetup",
		"rpcbind",
		"kbd",
		"kbd-misc",
		"tar",
		"xz",
		"curl",
		"bzip2",
		"systemd",
		"systemd",
		"rsyslog",
		"xorg-x11-drivers",
		"xorg-x11-server-Xorg",
		"xorg-x11-server-utils",
		"xorg-x11-xauth",
		"dbus-x11",
		"metacity",
		"metacity",
		"gsettings-desktop-schemas",
		"gsettings-desktop-schemas",
		"nm-connection-editor",
		"librsvg2",
		"librsvg2",
		"xfsprogs",
		"xfsprogs",
		"gfs2-utils",
		"system-storage-manager",
		"device-mapper-persistent-data",
		"xfsdump",
		"udisks2",
		"udisks2-iscsi",
		"hostname",
		"libblockdev-lvm-dbus",
		"libblockdev-lvm-dbus",
		"volume_key",
		"nss-tools",
		"selinux-policy-targeted",
		"audit",
		"ethtool",
		"openssh-server",
		"nfs-utils",
		"openssh-clients",
		"tigervnc-server-minimal",
		"tigervnc-server-module",
		"net-tools",
		"nmap-ncat",
		"prefixdevname",
		"pciutils",
		"usbutils",
		"ipmitool",
		"mt-st",
		"smartmontools",
		"hdparm",
		"libibverbs",
		"libibverbs",
		"rdma-core",
		"rdma-core",
		"rng-tools",
		"dmidecode",
		"bitmap-fangsongti-fonts",
		"dejavu-sans-fonts",
		"dejavu-sans-mono-fonts",
		"kacst-farsi-fonts",
		"kacst-qurn-fonts",
		"lklug-fonts",
		"lohit-assamese-fonts",
		"lohit-bengali-fonts",
		"lohit-devanagari-fonts",
		"lohit-gujarati-fonts",
		"lohit-gurmukhi-fonts",
		"lohit-kannada-fonts",
		"lohit-odia-fonts",
		"lohit-tamil-fonts",
		"lohit-telugu-fonts",
		"madan-fonts",
		"smc-meera-fonts",
		"thai-scalable-waree-fonts",
		"sil-abyssinica-fonts",
		"xorg-x11-fonts-misc",
		"aajohan-comfortaa-fonts",
		"abattis-cantarell-fonts",
		"sil-scheherazade-fonts",
		"jomolhari-fonts",
		"khmeros-base-fonts",
		"sil-padauk-fonts",
		"google-noto-sans-cjk-ttc-fonts",
		"gdb-gdbserver",
		"libreport-plugin-bugzilla",
		"libreport-plugin-reportuploader",
		"libreport-rhel-anaconda-bugzilla",
		"python3-pyatspi",
		"vim-minimal",
		"strace",
		"lsof",
		"dump",
		"xz",
		"less",
		"rsync",
		"bind-utils",
		"ftp",
		"mtr",
		"wget",
		"spice-vdagent",
		"gdisk",
		"hexedit",
		"sg3_utils",
		"perl-interpreter",
	}
	edgeInstImgTypeX86_64 := imageTypeS2{
		name:     "rhel-edge-installer",
		filename: "rhel84-boot.iso",
		mimeType: "application/x-iso9660-image",
		packageSets: map[string]rpmmd.PackageSet{
			"build": {
				Include: edgeBuildPkgs,
			},
			"packages": {
				Include: edgeImgTypeX86_64.packages,
				Exclude: edgeImgTypeX86_64.excludedPackages,
			},
			"installer": {Include: edgeInstallerPkgs},
		},
		enabledServices:  edgeImgTypeX86_64.enabledServices,
		rpmOstree:        true,
		bootISO:          true,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"anaconda-tree", "bootiso-tree", "assembler"},
		exports:          []string{"assembler"},
		pipelines:        edgePipelines,
	}

	edgeOCIImgTypeAarch64 := imageTypeS2{
		name:     "rhel-edge-container",
		filename: "rhel84-container.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]rpmmd.PackageSet{
			"packages": {
				Include: edgeImgTypeAarch64.packages,
				Exclude: edgeImgTypeAarch64.excludedPackages,
			},
			"container": {Include: []string{"httpd"}},
		},
		enabledServices:  edgeImgTypeAarch64.enabledServices,
		rpmOstree:        true,
		bootISO:          false,
		buildPipelines:   []string{"build"},
		payloadPipelines: []string{"ostree-tree", "ostree-commit", "container-tree", "assembler"},
		exports:          []string{"assembler"},
		pipelines:        edgePipelines,
	}

	// This image type does not take the disabled / enabled service definitions
	// from this structure definition, but rather from distro.ImageConfig instance
	// defined in the gcePipelines() function. The same applies to the default
	// target.
	gceImgType := imageTypeS2{
		name:     "gce",
		filename: "image.tar.gz",
		mimeType: "application/gzip",
		packageSets: map[string]rpmmd.PackageSet{
			"packages": getGcePackageSet(),
		},
		kernelOptions:           "net.ifnames=0 biosdevname=0 scsi_mod.use_blk_mq=Y crashkernel=auto console=ttyS0,38400n8d",
		bootable:                true,
		defaultSize:             20 * GigaByte,
		pipelines:               gceByosPipelines,
		buildPipelines:          []string{"build"},
		payloadPipelines:        []string{"os", "image", "archive"},
		exports:                 []string{"archive"},
		partitionTableGenerator: defaultPartitionTable,
	}

	gceRhuiImgType := imageTypeS2{
		name:     "gce-rhui",
		filename: "image.tar.gz",
		mimeType: "application/gzip",
		packageSets: map[string]rpmmd.PackageSet{
			"packages": getGceRhuiPackageSet(),
		},
		kernelOptions:           "net.ifnames=0 biosdevname=0 scsi_mod.use_blk_mq=Y crashkernel=auto console=ttyS0,38400n8d",
		bootable:                true,
		defaultSize:             20 * GigaByte,
		pipelines:               gceRhuiPipelines,
		buildPipelines:          []string{"build"},
		payloadPipelines:        []string{"os", "image", "archive"},
		exports:                 []string{"archive"},
		partitionTableGenerator: defaultPartitionTable,
	}

	x8664.addImageTypes(
		amiImgType,
		qcow2ImageType,
		openstackImgType,
		tarImgType,
		vhdImgType,
		vmdkImgType,
	)

	x8664.addS2ImageTypes(gceImgType, gceRhuiImgType)

	if !isCentos {
		x8664.addImageTypes(edgeImgTypeX86_64)
		x8664.addS2ImageTypes(edgeOCIImgTypeX86_64, edgeInstImgTypeX86_64)
	}

	aarch64 := architecture{
		distro: &r,
		name:   "aarch64",
		bootloaderPackages: []string{
			"dracut-config-generic",
			"efibootmgr",
			"grub2-efi-aa64",
			"grub2-tools",
			"shim-aa64",
		},
		uefi: true,
	}
	aarch64.addImageTypes(
		amiImgType,
		qcow2ImageType,
		openstackImgType,
		tarImgType,
	)

	if !isCentos {
		aarch64.addImageTypes(edgeImgTypeAarch64)
		aarch64.addS2ImageTypes(edgeOCIImgTypeAarch64)
	}

	ppc64le := architecture{
		distro: &r,
		name:   "ppc64le",
		bootloaderPackages: []string{
			"dracut-config-generic",
			"powerpc-utils",
			"grub2-ppc64le",
			"grub2-ppc64le-modules",
		},
		buildPackages: []string{
			"grub2-ppc64le",
			"grub2-ppc64le-modules",
		},
		legacy: "powerpc-ieee1275",
		uefi:   false,
	}
	ppc64le.addImageTypes(
		qcow2ImageType,
		tarImgType,
	)

	s390x := architecture{
		distro: &r,
		name:   "s390x",
		bootloaderPackages: []string{
			"dracut-config-generic",
			"s390utils-base",
		},
		uefi: false,
	}
	s390x.addImageTypes(
		tarImgType,
		qcow2ImageType,
	)

	r.addArches(x8664, aarch64, ppc64le)

	if !isCentos {
		r.addArches(s390x)
	}

	return &r
}
