package rhel90

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"sort"

	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild1"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/crypt"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const defaultName = "rhel-90"
const releaseVersion = "9"
const modulePlatformID = "platform:el9"
const ostreeRef = "rhel/9/%s/edge"

type distribution struct {
	name             string
	modulePlatformID string
	ostreeRef        string
	arches           map[string]architecture
	buildPackages    []string
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
	defaultSize             uint64
	partitionTableGenerator func(imageOptions distro.ImageOptions, arch distro.Arch, rng *rand.Rand) disk.PartitionTable
	assembler               func(pt *disk.PartitionTable, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler
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

func (t *imageType) PackageSets(bp blueprint.Blueprint) map[string]rpmmd.PackageSet {
	includePackages, excludePackages := t.Packages(bp)
	return map[string]rpmmd.PackageSet{
		"packages": {
			Include: includePackages,
			Exclude: excludePackages,
		},
		"build-packages": {
			Include: t.BuildPackages(),
		},
	}
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
			defaultSize:             it.defaultSize,
			partitionTableGenerator: it.partitionTableGenerator,
			assembler:               it.assembler,
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

func (t *imageType) Packages(bp blueprint.Blueprint) ([]string, []string) {
	packages := append(t.packages, bp.GetPackages()...)
	timezone, _ := bp.Customizations.GetTimezoneSettings()
	if timezone != nil {
		packages = append(packages, "chrony")
	}
	if t.bootable {
		packages = append(packages, t.arch.bootloaderPackages...)
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
	return packages
}

func (t *imageType) Exports() []string {
	return []string{"assembler"}
}

func (t *imageType) Manifest(c *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSpecSets map[string][]rpmmd.PackageSpec,
	seed int64) (distro.Manifest, error) {
	source := rand.NewSource(seed)
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

	mountpoints := c.GetFilesystems()

	// only allow root mountpoint for the time-being
	invalidMountpoints := []string{}
	for _, m := range mountpoints {
		if m.Mountpoint != "/" {
			invalidMountpoints = append(invalidMountpoints, m.Mountpoint)
		}
	}

	if len(invalidMountpoints) > 0 {
		return nil, fmt.Errorf("The following custom mountpoints are not supported %+q", invalidMountpoints)
	}

	var pt *disk.PartitionTable
	if t.partitionTableGenerator != nil {
		table := t.partitionTableGenerator(options, t.arch, rng)
		pt = &table
	}

	p := &osbuild.Pipeline{}
	p.SetBuild(t.buildPipeline(repos, *t.arch, buildPackageSpecs), "org.osbuild.rhel90")

	if pt == nil {
		panic("Image must have a partition table, this is a programming error")
	}

	rootPartition := pt.RootPartition()
	if rootPartition == nil {
		panic("Image must have a root partition, this is a programming error")
	}

	p.AddStage(osbuild.NewKernelCmdlineStage(&osbuild.KernelCmdlineStageOptions{
		RootFsUUID: rootPartition.Filesystem.UUID,
		KernelOpts: t.kernelOptions,
	}))

	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(*t.arch, repos, packageSpecs)))
	p.AddStage(osbuild.NewFixBLSStage())

	p.AddStage(osbuild.NewResolvConfStage(t.resolvConfOptions()))

	if pt != nil {
		p.AddStage(osbuild.NewFSTabStage(pt.FSTabStageOptions()))
	}

	if t.bootable {
		if t.arch.Name() != "s390x" {
			p.AddStage(osbuild.NewGRUB2Stage(t.grub2StageOptions(pt, t.kernelOptions, c.GetKernel(), packageSpecs, t.arch.uefi, t.arch.legacy)))
		}
	}

	// TODO support setting all languages and install corresponding langpack-* package
	language, keyboard := c.GetPrimaryLocale()

	if language != nil {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: *language}))
	} else {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{Language: "en_US"}))
	}

	if keyboard != nil {
		p.AddStage(osbuild.NewKeymapStage(&osbuild.KeymapStageOptions{Keymap: *keyboard}))
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
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: "America/New_York"}))
	}

	if len(ntpServers) > 0 {
		p.AddStage(osbuild.NewChronyStage(&osbuild.ChronyStageOptions{Timeservers: ntpServers}))
	}

	if groups := c.GetGroups(); len(groups) > 0 {
		p.AddStage(osbuild.NewGroupsStage(t.groupStageOptions(groups)))
	}

	if users := c.GetUsers(); len(users) > 0 {
		options, err := t.userStageOptions(users)
		if err != nil {
			return nil, err
		}
		p.AddStage(osbuild.NewUsersStage(options))
	}

	if services := c.GetServices(); services != nil || t.enabledServices != nil || t.disabledServices != nil || t.defaultTarget != "" {
		p.AddStage(osbuild.NewSystemdStage(t.systemdStageOptions(t.enabledServices, t.disabledServices, services, t.defaultTarget)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(t.firewallStageOptions(firewall)))
	}

	if t.arch.Name() == "s390x" {
		p.AddStage(osbuild.NewZiplStage(&osbuild.ZiplStageOptions{}))
	}

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

	if options.Subscription != nil {
		commands := []string{
			fmt.Sprintf("/usr/sbin/subscription-manager register --org=%d --activationkey=%s --serverurl %s --baseurl %s", options.Subscription.Organization, options.Subscription.ActivationKey, options.Subscription.ServerUrl, options.Subscription.BaseUrl),
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

	// SELinux stage should be the last so everything has the right label.
	p.AddStage(osbuild.NewSELinuxStage(t.selinuxStageOptions()))

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

func (t *imageType) userStageOptions(users []blueprint.UserCustomization) (*osbuild.UsersStageOptions, error) {
	options := osbuild.UsersStageOptions{
		Users: make(map[string]osbuild.UsersStageOptionsUser),
	}

	for _, c := range users {
		if c.Password != nil && !crypt.PasswordIsCrypted(*c.Password) {
			cryptedPassword, err := crypt.CryptSHA512(*c.Password)
			if err != nil {
				return nil, err
			}

			c.Password = &cryptedPassword
		}

		user := osbuild.UsersStageOptionsUser{
			Groups:      c.Groups,
			Description: c.Description,
			Home:        c.Home,
			Shell:       c.Shell,
			Password:    c.Password,
			Key:         c.Key,
		}

		user.UID = c.UID
		user.GID = c.GID

		options.Users[c.Name] = user
	}

	return &options, nil
}

func (t *imageType) resolvConfOptions() *osbuild.ResolvConfStageOptions {
	return &osbuild.ResolvConfStageOptions{}
}

func (t *imageType) groupStageOptions(groups []blueprint.GroupCustomization) *osbuild.GroupsStageOptions {
	options := osbuild.GroupsStageOptions{
		Groups: map[string]osbuild.GroupsStageOptionsGroup{},
	}

	for _, group := range groups {
		groupData := osbuild.GroupsStageOptionsGroup{
			Name: group.Name,
		}
		groupData.GID = group.GID

		options.Groups[group.Name] = groupData
	}

	return &options
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
	rootPartition := pt.RootPartition()
	if rootPartition == nil {
		panic("root partition must be defined for grub2 stage, this is a programming error")
	}

	stageOptions := osbuild.GRUB2StageOptions{
		RootFilesystemUUID: uuid.MustParse(rootPartition.Filesystem.UUID),
		KernelOptions:      kernelOptions,
		Legacy:             legacy,
	}

	if uefi {
		vendor := "redhat"
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

func defaultPartitionTable(imageOptions distro.ImageOptions, arch distro.Arch, rng *rand.Rand) disk.PartitionTable {
	if arch.Name() == "x86_64" {
		return disk.PartitionTable{
			Size: imageOptions.Size,
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
					Filesystem: &disk.Filesystem{
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
					Filesystem: &disk.Filesystem{
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
	} else if arch.Name() == "aarch64" {
		return disk.PartitionTable{
			Size: imageOptions.Size,
			UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
			Type: "gpt",
			Partitions: []disk.Partition{
				{
					Start: 2048,
					Size:  204800,
					Type:  "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
					UUID:  "68B2905B-DF3E-4FB3-80FA-49D1E773AA33",
					Filesystem: &disk.Filesystem{
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
					Filesystem: &disk.Filesystem{
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
	} else if arch.Name() == "ppc64le" {
		return disk.PartitionTable{
			Size: imageOptions.Size,
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
					Filesystem: &disk.Filesystem{
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
	} else if arch.Name() == "s390x" {
		return disk.PartitionTable{
			Size: imageOptions.Size,
			UUID: "0x14fc63d2",
			Type: "dos",
			Partitions: []disk.Partition{
				{
					Start:    2048,
					Bootable: true,
					Filesystem: &disk.Filesystem{
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

func qemuAssembler(pt *disk.PartitionTable, format string, filename string, imageOptions distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
	options := pt.QEMUAssemblerOptions()

	options.Format = format
	options.Filename = filename

	if arch.Name() == "x86_64" {
		options.Bootloader = &osbuild.QEMUBootloader{
			Type: "grub2",
		}
	} else if arch.Name() == "ppc64le" {
		options.Bootloader = &osbuild.QEMUBootloader{
			Type:     "grub2",
			Platform: "powerpc-ieee1275",
		}
	} else if arch.Name() == "s390x" {
		options.Bootloader = &osbuild.QEMUBootloader{
			Type: "zipl",
		}
	}
	return osbuild.NewQEMUAssembler(&options)
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

func New() distro.Distro {
	return newDistro(defaultName, modulePlatformID, ostreeRef)
}

func NewHostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	return newDistro(name, modulePlatformID, ostreeRef)
}

func newDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	const GigaByte = 1024 * 1024 * 1024

	qcow2ImageType := imageType{
		name:             "qcow2",
		filename:         "disk.qcow2",
		mimeType:         "application/x-qemu-disk",
		packages:         packages.Qcow2.Include,
		excludedPackages: packages.Qcow2.Exclude,
		enabledServices: []string{
			"cloud-init.service",
			"cloud-config.service",
			"cloud-final.service",
			"cloud-init-local.service",
		},
		defaultTarget:           "multi-user.target",
		kernelOptions:           "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0 crashkernel=auto",
		bootable:                true,
		defaultSize:             10 * GigaByte,
		partitionTableGenerator: defaultPartitionTable,
		assembler: func(pt *disk.PartitionTable, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler(pt, "qcow2", "disk.qcow2", options, arch)
		},
	}

	r := distribution{
		buildPackages:    packages.GenericBuild,
		name:             name,
		modulePlatformID: modulePlatformID,
		ostreeRef:        ostreeRef,
	}
	x8664 := architecture{
		distro:             &r,
		name:               "x86_64",
		bootloaderPackages: packages.Bootloader.x8664,
		buildPackages:      packages.Build.x8664,
		legacy:             "i386-pc",
		uefi:               true,
	}
	x8664.addImageTypes(
		qcow2ImageType,
	)

	aarch64 := architecture{
		distro:             &r,
		name:               "aarch64",
		bootloaderPackages: packages.Bootloader.aarch64,
		buildPackages:      packages.Build.aarch64,
		uefi:               true,
	}
	aarch64.addImageTypes(
		qcow2ImageType,
	)

	ppc64le := architecture{
		distro:             &r,
		name:               "ppc64le",
		bootloaderPackages: packages.Bootloader.ppc64le,
		buildPackages:      packages.Build.ppc64le,
		legacy:             "powerpc-ieee1275",
		uefi:               false,
	}
	ppc64le.addImageTypes(
		qcow2ImageType,
	)

	s390x := architecture{
		distro:             &r,
		name:               "s390x",
		bootloaderPackages: packages.Bootloader.s390x,
		buildPackages:      packages.Build.s390x,
		uefi:               false,
	}
	s390x.addImageTypes(
		qcow2ImageType,
	)

	r.addArches(x8664, aarch64, ppc64le, s390x)

	return &r
}
