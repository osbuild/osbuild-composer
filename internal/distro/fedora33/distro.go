package fedora33

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild1"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/crypt"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const (
	// package set names

	// build package set name
	buildPkgsKey = "build-packages"

	// main/common os image package set name
	osPkgsKey = "packages"

	// blueprint package set name
	blueprintPkgsKey = "blueprint"

	nameTmpl             = "fedora-%s"
	modulePlatformIDTmpl = "platform:f%s"

	// The second format value is intentionally escaped, because the
	// format string is substituted in two steps:
	//  1. on distribution level when being created
	//  2. on image level
	ostreeRefTmpl = "fedora/%s/%%s/iot"

	// Supported Fedora versions
	f33ReleaseVersion = "33"
	f34ReleaseVersion = "34"
	f35ReleaseVersion = "35"
	f36ReleaseVersion = "36"
)

type distribution struct {
	name             string
	releaseVersion   string
	modulePlatformID string
	ostreeRefTmpl    string
	arches           map[string]architecture
}

type architecture struct {
	distro             *distribution
	name               string
	bootloaderPackages []string
	legacy             string
	bootType           distro.BootType
	imageTypes         map[string]imageType
}

type packageSetFunc func(t *imageType) rpmmd.PackageSet

type imageType struct {
	arch             *architecture
	name             string
	filename         string
	mimeType         string
	packageSets      map[string]packageSetFunc
	excludedPackages map[string]packageSetFunc
	enabledServices  []string
	disabledServices []string
	kernelOptions    string
	bootable         bool
	rpmOstree        bool
	defaultSize      uint64
	assembler        func(bootType distro.BootType, options distro.ImageOptions, arch distro.Arch, imageSize uint64) *osbuild.Assembler
}

func removePackage(packages []string, packageToRemove string) []string {
	for i, pkg := range packages {
		if pkg == packageToRemove {
			return append(packages[:i], packages[i+1:]...)
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

func (d *distribution) setArches(arches ...architecture) {
	d.arches = map[string]architecture{}
	for _, a := range arches {
		d.arches[a.name] = architecture{
			distro:             d,
			name:               a.name,
			bootloaderPackages: a.bootloaderPackages,
			bootType:           a.bootType,
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

	return &t, nil
}

func (a *architecture) setImageTypes(imageTypes ...imageType) {
	a.imageTypes = map[string]imageType{}
	for _, it := range imageTypes {
		a.imageTypes[it.name] = imageType{
			arch:             a,
			name:             it.name,
			filename:         it.filename,
			mimeType:         it.mimeType,
			packageSets:      it.packageSets,
			excludedPackages: it.excludedPackages,
			enabledServices:  it.enabledServices,
			disabledServices: it.disabledServices,
			kernelOptions:    it.kernelOptions,
			bootable:         it.bootable,
			rpmOstree:        it.rpmOstree,
			defaultSize:      it.defaultSize,
			assembler:        it.assembler,
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
		return fmt.Sprintf(t.Arch().Distro().OSTreeRef(), t.Arch().Name())
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

func (t *imageType) getPackages(name string) rpmmd.PackageSet {
	getter := t.packageSets[name]
	if getter == nil {
		return rpmmd.PackageSet{}
	}

	return getter(t)
}

func (t *imageType) PackageSets(bp blueprint.Blueprint) map[string]rpmmd.PackageSet {
	// merge package sets that appear in the image type with the package sets
	// of the same name from the distro and arch
	mergedSets := make(map[string]rpmmd.PackageSet)

	imageSets := t.packageSets

	for name := range imageSets {
		mergedSets[name] = t.getPackages(name)
	}

	if _, hasPackages := imageSets[osPkgsKey]; !hasPackages {
		// should this be possible??
		mergedSets[osPkgsKey] = rpmmd.PackageSet{}
	}

	// every image type must define a 'build' package set
	if _, hasBuild := imageSets[buildPkgsKey]; !hasBuild {
		panic(fmt.Sprintf("'%s' image type has no '%s' package set defined", t.name, buildPkgsKey))
	}

	if t.bootable {
		switch t.arch.Name() {
		case distro.X86_64ArchName:
			mergedSets[osPkgsKey] = mergedSets[osPkgsKey].Append(rpmmd.PackageSet{Include: t.arch.bootloaderPackages})
		case distro.Aarch64ArchName:
			mergedSets[osPkgsKey] = mergedSets[osPkgsKey].Append(rpmmd.PackageSet{Include: t.arch.bootloaderPackages})
		}

	}
	// blueprint packages
	bpPackages := bp.GetPackages()
	timezone, _ := bp.Customizations.GetTimezoneSettings()
	if timezone != nil {
		bpPackages = append(bpPackages, "chrony")
	}

	// depsolve bp packages separately
	// bp packages aren't restricted by exclude lists
	mergedSets[blueprintPkgsKey] = rpmmd.PackageSet{Include: bpPackages}

	// copy the list of excluded packages from the image type
	// and subtract any packages found in the blueprint (this
	// will not handle the issue with dependencies present in
	// the list of excluded packages, but it will create a
	// possibility of a workaround at least)
	excludedPackages := mergedSets[osPkgsKey].Exclude
	for _, pkg := range mergedSets[osPkgsKey].Append(mergedSets[blueprintPkgsKey]).Include {
		// removePackage is fine if the package doesn't exist
		excludedPackages = removePackage(excludedPackages, pkg)
	}
	mergedSets[osPkgsKey] = rpmmd.PackageSet{Include: mergedSets[osPkgsKey].Include, Exclude: excludedPackages}

	// Add kernel package if none defined
	kc := bp.Customizations.GetKernel()
	mergedSets[osPkgsKey] = mergedSets[osPkgsKey].Append(rpmmd.PackageSet{Include: []string{kc.Name}})

	return mergedSets
}

func (t *imageType) BuildPipelines() []string {
	return distro.BuildPipelinesFallback()
}

func (t *imageType) PayloadPipelines() []string {
	return distro.PayloadPipelinesFallback()
}

func (t *imageType) PayloadPackageSets() []string {
	return []string{"packages"}
}

func (t *imageType) Exports() []string {
	return distro.ExportsFallback()
}

func (t *imageType) Manifest(c *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSpecSets map[string][]rpmmd.PackageSpec,
	seed int64) (distro.Manifest, error) {
	pipeline, err := t.pipeline(c, options, repos, packageSpecSets[osPkgsKey], packageSpecSets[buildPkgsKey])
	if err != nil {
		return distro.Manifest{}, err
	}

	return json.Marshal(
		osbuild.Manifest{
			Sources:  *sources(append(packageSpecSets[osPkgsKey], packageSpecSets[buildPkgsKey]...)),
			Pipeline: *pipeline,
		},
	)
}

func (d *distribution) Name() string {
	return d.name
}

func (d *distribution) Releasever() string {
	return d.releaseVersion
}

func (d *distribution) ModulePlatformID() string {
	return d.modulePlatformID
}

func (d *distribution) OSTreeRef() string {
	return d.ostreeRefTmpl
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

func (t *imageType) pipeline(c *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec) (*osbuild.Pipeline, error) {

	// if options.Size is 0, this will be the default size of the image type
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

	p := &osbuild.Pipeline{}
	p.SetBuild(t.buildPipeline(repos, *t.arch, buildPackageSpecs), "org.osbuild.fedora33")

	p.AddStage(osbuild.NewKernelCmdlineStage(t.kernelCmdlineStageOptions()))
	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(*t.arch, repos, packageSpecs)))

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
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: "UTC"}))
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

	if t.bootable {
		p.AddStage(osbuild.NewFSTabStage(t.fsTabStageOptions(t.arch.bootType)))
		p.AddStage(osbuild.NewGRUB2Stage(t.grub2StageOptions(t.kernelOptions, c.GetKernel())))
	}
	p.AddStage(osbuild.NewFixBLSStage())

	if services := c.GetServices(); services != nil || t.enabledServices != nil {
		p.AddStage(osbuild.NewSystemdStage(t.systemdStageOptions(t.enabledServices, t.disabledServices, services)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(t.firewallStageOptions(firewall)))
	}

	p.AddStage(osbuild.NewSELinuxStage(t.selinuxStageOptions()))

	if t.rpmOstree {
		p.AddStage(osbuild.NewRPMOSTreeStage(&osbuild.RPMOSTreeStageOptions{
			EtcGroupMembers: []string{
				// NOTE: We may want to make this configurable.
				"wheel", "docker",
			},
		}))
	}

	p.Assembler = t.assembler(t.arch.bootType, options, t.arch, imageSize)

	return p, nil
}

func (t *imageType) buildPipeline(repos []rpmmd.RepoConfig, arch architecture, buildPackageSpecs []rpmmd.PackageSpec) *osbuild.Pipeline {
	p := &osbuild.Pipeline{}
	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(arch, repos, buildPackageSpecs)))

	selinuxOptions := osbuild.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
		Labels: map[string]string{
			"/usr/bin/cp": "system_u:object_r:install_exec_t:s0",
		},
	}

	p.AddStage(osbuild.NewSELinuxStage(&selinuxOptions))
	return p
}

func (t *imageType) kernelCmdlineStageOptions() *osbuild.KernelCmdlineStageOptions {
	return &osbuild.KernelCmdlineStageOptions{
		RootFsUUID: "76a22bf4-f153-4541-b6c7-0332c0dfaeac",
		KernelOpts: "ro no_timer_check net.ifnames=0 console=tty1 console=ttyS0,115200n8",
	}
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

func (t *imageType) systemdStageOptions(enabledServices, disabledServices []string, s *blueprint.ServicesCustomization) *osbuild.SystemdStageOptions {
	if s != nil {
		enabledServices = append(enabledServices, s.Enabled...)
		disabledServices = append(disabledServices, s.Disabled...)
	}
	return &osbuild.SystemdStageOptions{
		EnabledServices:  enabledServices,
		DisabledServices: disabledServices,
	}
}

func (t *imageType) fsTabStageOptions(bootType distro.BootType) *osbuild.FSTabStageOptions {
	options := osbuild.FSTabStageOptions{}
	options.AddFilesystem("76a22bf4-f153-4541-b6c7-0332c0dfaeac", "ext4", "/", "defaults", 1, 1)
	if bootType == distro.UEFIBootType {
		options.AddFilesystem("46BB-8120", "vfat", "/boot/efi", "umask=0077,shortname=winnt", 0, 2)
	}
	return &options
}

func (t *imageType) grub2StageOptions(kernelOptions string, kernel *blueprint.KernelCustomization) *osbuild.GRUB2StageOptions {
	id := uuid.MustParse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")

	if kernel != nil && kernel.Append != "" {
		kernelOptions += " " + kernel.Append
	}

	var uefiOptions *osbuild.GRUB2UEFI
	if t.arch.bootType == distro.UEFIBootType {
		uefiOptions = &osbuild.GRUB2UEFI{
			Vendor: "fedora",
		}
	}

	var legacy string
	if t.arch.bootType == distro.LegacyBootType {
		legacy = t.arch.legacy
	}

	return &osbuild.GRUB2StageOptions{
		RootFilesystemUUID: id,
		KernelOptions:      kernelOptions,
		Legacy:             legacy,
		UEFI:               uefiOptions,
	}
}

func (t *imageType) selinuxStageOptions() *osbuild.SELinuxStageOptions {
	return &osbuild.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
}

func qemuAssembler(format string, filename string, bootType distro.BootType, imageSize uint64) *osbuild.Assembler {
	var options osbuild.QEMUAssemblerOptions
	if bootType == distro.UEFIBootType {
		options = osbuild.QEMUAssemblerOptions{
			Format:   format,
			Filename: filename,
			Size:     imageSize,
			PTUUID:   "8DFDFF87-C96E-EA48-A3A6-9408F1F6B1EF",
			PTType:   "gpt",
			Partitions: []osbuild.QEMUPartition{
				{
					Start: 2048,
					Size:  972800,
					Type:  "C12A7328-F81F-11D2-BA4B-00A0C93EC93B",
					UUID:  "02C1E068-1D2F-4DA3-91FD-8DD76A955C9D",
					Filesystem: &osbuild.QEMUFilesystem{
						Type:       "vfat",
						UUID:       "46BB-8120",
						Label:      "EFI-SYSTEM",
						Mountpoint: "/boot/efi",
					},
				},
				{
					Start: 976896,
					UUID:  "8D760010-FAAE-46D1-9E5B-4A2EAC5030CD",
					Filesystem: &osbuild.QEMUFilesystem{
						Type:       "ext4",
						UUID:       "76a22bf4-f153-4541-b6c7-0332c0dfaeac",
						Mountpoint: "/",
					},
				},
			},
		}
	} else {
		options = osbuild.QEMUAssemblerOptions{
			Format:   format,
			Filename: filename,
			Size:     imageSize,
			PTUUID:   "0x14fc63d2",
			PTType:   "mbr",
			Partitions: []osbuild.QEMUPartition{
				{
					Start:    2048,
					Bootable: true,
					Filesystem: &osbuild.QEMUFilesystem{
						Type:       "ext4",
						UUID:       "76a22bf4-f153-4541-b6c7-0332c0dfaeac",
						Mountpoint: "/",
					},
				},
			},
		}
	}
	return osbuild.NewQEMUAssembler(&options)
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

// New creates a new distro object, defining the supported architectures and image types
func NewF33() distro.Distro {
	return newDefaultDistro(f33ReleaseVersion)
}

func NewF34() distro.Distro {
	return newDefaultDistro(f34ReleaseVersion)
}

func NewF35() distro.Distro {
	return newDefaultDistro(f35ReleaseVersion)
}

func NewF36() distro.Distro {
	return newDefaultDistro(f36ReleaseVersion)
}

func NewHostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	return newDistro(name, modulePlatformID, ostreeRef)
}

func newDefaultDistro(releaseVersion string) distro.Distro {
	return newDistro(
		fmt.Sprintf(nameTmpl, releaseVersion),
		fmt.Sprintf(modulePlatformIDTmpl, releaseVersion),
		fmt.Sprintf(ostreeRefTmpl, releaseVersion),
	)
}

func newDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	const GigaByte = 1024 * 1024 * 1024

	r := distribution{
		name:             name,
		modulePlatformID: modulePlatformID,
		ostreeRefTmpl:    ostreeRef,
	}

	x86_64 := architecture{
		distro: &r,
		name:   distro.X86_64ArchName,
		bootloaderPackages: []string{
			"dracut-config-generic",
			"grub2-pc",
		},
		legacy:   "i386-pc",
		bootType: distro.LegacyBootType,
	}

	aarch64 := architecture{
		distro: &r,
		name:   distro.Aarch64ArchName,
		bootloaderPackages: []string{
			"dracut-config-generic", "efibootmgr", "grub2-efi-aa64",
			"grub2-tools", "shim-aa64"},
		bootType: distro.UEFIBootType,
	}

	iotServices := []string{
		"NetworkManager.service", "firewalld.service", "rngd.service", "sshd.service",
		"zezere_ignition.timer", "zezere_ignition_banner.service",
		"greenboot-grub2-set-counter", "greenboot-grub2-set-success", "greenboot-healthcheck", "greenboot-rpm-ostree-grub2-check-fallback",
		"greenboot-status", "greenboot-task-runner", "redboot-auto-reboot", "redboot-task-runner",
		"parsec", "dbus-parsec"}

	iotImgType := imageType{
		name:     "fedora-iot-commit",
		filename: "commit.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: iotBuildPackageSet,
			osPkgsKey:    iotCommitPackageSet,
		},
		enabledServices: iotServices,
		rpmOstree:       true,
		assembler: func(bootType distro.BootType, options distro.ImageOptions, arch distro.Arch, imageSize uint64) *osbuild.Assembler {
			return ostreeCommitAssembler(options, arch)
		},
	}

	ec2EnabledServices := []string{
		"cloud-init.service",
	}

	amiImgType := imageType{
		name:     "ami",
		filename: "image.raw",
		mimeType: "application/octet-stream",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    ec2CorePackageSet,
		},
		enabledServices: ec2EnabledServices,
		kernelOptions:   "ro no_timer_check console=ttyS0,115200n8 console=tty1 biosdevname=0 net.ifnames=0 console=ttyS0,115200",
		bootable:        true,
		defaultSize:     6 * GigaByte,
		assembler: func(bootType distro.BootType, options distro.ImageOptions, arch distro.Arch, imageSize uint64) *osbuild.Assembler {
			return qemuAssembler("raw", "image.raw", bootType, imageSize)
		},
	}

	qcow2Services := []string{
		"cloud-init.service",
		"cloud-config.service",
		"cloud-final.service",
		"cloud-init-local.service",
	}

	qcow2ImageType := imageType{
		name:     "qcow2",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    qcow2PackageSet,
		},
		enabledServices: qcow2Services,
		bootable:        true,
		defaultSize:     2 * GigaByte,
		assembler: func(bootType distro.BootType, options distro.ImageOptions, arch distro.Arch, imageSize uint64) *osbuild.Assembler {
			return qemuAssembler("qcow2", "disk.qcow2", bootType, imageSize)
		},
	}

	openStackServices := []string{
		"cloud-init.service",
		"cloud-config.service",
		"cloud-final.service",
		"cloud-init-local.service",
	}

	openstackImgType := imageType{
		name:     "openstack",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    openStackPackageSet,
		},

		enabledServices: openStackServices,
		bootable:        true,
		defaultSize:     2 * GigaByte,
		assembler: func(bootType distro.BootType, options distro.ImageOptions, arch distro.Arch, imageSize uint64) *osbuild.Assembler {
			return qemuAssembler("qcow2", "disk.qcow2", bootType, options.Size)
		},
	}

	vhdEnabledServices := []string{
		"sshd",
		"waagent", // needed to run in Azure
	}

	vhdDisabledServices := []string{
		"proc-sys-fs-binfmt_misc.mount",
		"loadmodules.service",
	}

	vhdImgType := imageType{
		name:     "vhd",
		filename: "disk.vhd",
		mimeType: "application/x-vhd",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    vhdPackageSet,
		},
		enabledServices:  vhdEnabledServices,
		disabledServices: vhdDisabledServices,
		// These kernel parameters are required by Azure documentation
		kernelOptions: "ro biosdevname=0 rootdelay=300 console=ttyS0 earlyprintk=ttyS0 net.ifnames=0",
		bootable:      true,
		defaultSize:   2 * GigaByte,
		assembler: func(bootType distro.BootType, options distro.ImageOptions, arch distro.Arch, imageSize uint64) *osbuild.Assembler {
			return qemuAssembler("vpc", "disk.vhd", bootType, imageSize)
		},
	}

	vmdkServices := []string{
		"cloud-init.service",
		"cloud-config.service",
		"cloud-final.service",
		"cloud-init-local.service",
	}

	vmdkImgType := imageType{
		name:     "vmdk",
		filename: "disk.vmdk",
		mimeType: "application/x-vmdk",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    vmdkPackageSet,
		},
		enabledServices: vmdkServices,
		bootable:        true,
		defaultSize:     2 * GigaByte,
		assembler: func(bootType distro.BootType, options distro.ImageOptions, arch distro.Arch, imageSize uint64) *osbuild.Assembler {
			return qemuAssembler("vmdk", "disk.vmdk", bootType, options.Size)
		},
	}

	ociImageServices := []string{
		"cloud-init.service",
		"cloud-config.service",
		"cloud-final.service",
		"cloud-init-local.service",
	}

	ociImageType := imageType{
		name:     "oci",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packageSets: map[string]packageSetFunc{
			buildPkgsKey: distroBuildPackageSet,
			osPkgsKey:    ociImagePackageSet,
		},
		enabledServices: ociImageServices,
		bootable:        true,
		defaultSize:     2 * GigaByte,
		assembler: func(bootType distro.BootType, options distro.ImageOptions, arch distro.Arch, imageSize uint64) *osbuild.Assembler {
			return qemuAssembler("qcow2", "disk.qcow2", bootType, options.Size)
		},
	}

	x86_64.setImageTypes(
		iotImgType,
		amiImgType,
		qcow2ImageType,
		openstackImgType,
		vhdImgType,
		vmdkImgType,
		ociImageType,
	)

	aarch64.setImageTypes(
		iotImgType,
		amiImgType,
		qcow2ImageType,
		openstackImgType,
		ociImageType,
	)

	r.setArches(x86_64, aarch64)

	return &r
}
