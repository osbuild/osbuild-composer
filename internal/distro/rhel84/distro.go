package rhel84

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/osbuild/osbuild-composer/internal/disk"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/crypt"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const name = "rhel-84"
const modulePlatformID = "platform:el8"

type distribution struct {
	arches        map[string]architecture
	imageTypes    map[string]imageType
	buildPackages []string
}

type architecture struct {
	distro             *distribution
	name               string
	bootloaderPackages []string
	buildPackages      []string
	legacy             string
	uefi               bool
	imageTypes         map[string]imageType
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
	partitionTableGenerator func(imageOptions distro.ImageOptions, arch distro.Arch) disk.PartitionTable
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

func (d *distribution) setArches(arches ...architecture) {
	d.arches = map[string]architecture{}
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

	return &t, nil
}

func (a *architecture) setImageTypes(imageTypes ...imageType) {
	a.imageTypes = map[string]imageType{}
	for _, it := range imageTypes {
		a.imageTypes[it.name] = imageType{
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

	return packages, t.excludedPackages
}

func (t *imageType) BuildPackages() []string {
	packages := append(t.arch.distro.buildPackages, t.arch.buildPackages...)
	if t.rpmOstree {
		packages = append(packages, "rpm-ostree")
	}
	return packages
}

func (t *imageType) Manifest(c *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSpecs,
	buildPackageSpecs []rpmmd.PackageSpec) (distro.Manifest, error) {
	pipeline, err := t.pipeline(c, options, repos, packageSpecs, buildPackageSpecs)
	if err != nil {
		return distro.Manifest{}, err
	}

	return json.Marshal(
		osbuild.Manifest{
			Sources:  *sources(append(packageSpecs, buildPackageSpecs...)),
			Pipeline: *pipeline,
		},
	)
}

func (d *distribution) Name() string {
	return name
}

func (d *distribution) ModulePlatformID() string {
	return modulePlatformID
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
	var pt *disk.PartitionTable
	if t.partitionTableGenerator != nil {
		table := t.partitionTableGenerator(options, t.arch)
		pt = &table
	}

	p := &osbuild.Pipeline{}
	p.SetBuild(t.buildPipeline(repos, *t.arch, buildPackageSpecs), "org.osbuild.rhel84")

	if t.arch.Name() == "s390x" {
		if pt == nil {
			panic("s390x image must have a partition table, this is a programming error")
		}

		rootPartition := pt.RootPartition()
		if rootPartition == nil {
			panic("s390x image must have a root partition, this is a programming error")
		}

		p.AddStage(osbuild.NewKernelCmdlineStage(&osbuild.KernelCmdlineStageOptions{
			RootFsUUID: rootPartition.UUID,
			KernelOpts: "net.ifnames=0 crashkernel=auto",
		}))
	}

	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(*t.arch, repos, packageSpecs)))
	p.AddStage(osbuild.NewFixBLSStage())

	if pt != nil {
		p.AddStage(osbuild.NewFSTabStage(pt.FSTabStageOptions()))
	}

	if t.bootable {
		if t.arch.Name() != "s390x" {
			p.AddStage(osbuild.NewGRUB2Stage(t.grub2StageOptions(pt, t.kernelOptions, c.GetKernel(), t.arch.uefi, t.arch.legacy)))
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
	}

	timezone, ntpServers := c.GetTimezoneSettings()

	if timezone != nil {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{Zone: *timezone}))
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

	if services := c.GetServices(); services != nil || t.enabledServices != nil {
		p.AddStage(osbuild.NewSystemdStage(t.systemdStageOptions(t.enabledServices, t.disabledServices, services, t.defaultTarget)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(t.firewallStageOptions(firewall)))
	}

	if t.arch.Name() == "s390x" {
		p.AddStage(osbuild.NewZiplStage(&osbuild.ZiplStageOptions{}))
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

func (t *imageType) grub2StageOptions(pt *disk.PartitionTable, kernelOptions string, kernel *blueprint.KernelCustomization, uefi bool, legacy string) *osbuild.GRUB2StageOptions {
	if pt == nil {
		panic("partition table must be defined for grub2 stage, this is a programming error")
	}
	rootPartition := pt.RootPartition()
	if rootPartition == nil {
		panic("root partition must be defined for grub2 stage, this is a programming error")
	}

	id := uuid.MustParse(rootPartition.Filesystem.UUID)

	if kernel != nil {
		kernelOptions += " " + kernel.Append
	}

	var uefiOptions *osbuild.GRUB2UEFI
	if uefi {
		uefiOptions = &osbuild.GRUB2UEFI{
			Vendor: "redhat",
		}
	}

	if !uefi {
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

func defaultPartitionTable(imageOptions distro.ImageOptions, arch distro.Arch) disk.PartitionTable {
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
						UUID:         "efe8afea-c0a8-45dc-8e6e-499279f6fa5d",
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
						UUID:         "efe8afea-c0a8-45dc-8e6e-499279f6fa5d",
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
						UUID:         "efe8afea-c0a8-45dc-8e6e-499279f6fa5d",
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
						UUID:         "efe8afea-c0a8-45dc-8e6e-499279f6fa5d",
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

func tarAssembler(filename, compression string) *osbuild.Assembler {
	return osbuild.NewTarAssembler(
		&osbuild.TarAssemblerOptions{
			Filename:    filename,
			Compression: compression,
		})
}

func ostreeCommitAssembler(options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
	ref := options.OSTree.Ref
	if ref == "" {
		ref = fmt.Sprintf("rhel/8/%s/edge", arch.Name())
	}
	return osbuild.NewOSTreeCommitAssembler(
		&osbuild.OSTreeCommitAssemblerOptions{
			Ref:    ref,
			Parent: options.OSTree.Parent,
			Tar: osbuild.OSTreeCommitAssemblerTarOptions{
				Filename: "commit.tar",
			},
		},
	)
}

// New creates a new distro object, defining the supported architectures and image types
func New() distro.Distro {
	const GigaByte = 1024 * 1024 * 1024

	edgeImgTypeX86_64 := imageType{
		name:     "rhel-edge-commit",
		filename: "commit.tar",
		mimeType: "application/x-tar",
		packages: []string{
			"redhat-release", // TODO: is this correct for Edge?
			"glibc", "glibc-minimal-langpack", "nss-altfiles",
			"kernel",
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
			"subscription-manager",
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
			"kernel",
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
			"subscription-manager",
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
			"kernel",
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
			return qemuAssembler(pt, "raw", "image.raw", options, arch)
		},
	}

	qcow2ImageType := imageType{
		name:     "qcow2",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packages: []string{
			"@core",
			"chrony",
			"dnf",
			"dosfstools",
			"kernel",
			"yum",
			"nfs-utils",
			"dnf-utils",
			"cloud-init",
			"python3-jsonschema",
			"qemu-guest-agent",
			"cloud-utils-growpart",
			"dracut-norescue",
			"tar",
			"tcpdump",
			"rsync",
			"dnf-plugin-spacewalk",
			"NetworkManager",
			"dhcp-client",
			"cockpit-ws",
			"cockpit-system",
			"subscription-manager-cockpit",
			"redhat-release",
			"redhat-release-eula",
			"insights-client",
			// TODO: rh-amazon-rhui-client
		},
		excludedPackages: []string{
			"dracut-config-rescue",
			"aic94xx-firmware",
			"alsa-firmware",
			"alsa-lib",
			"alsa-tools-firmware",
			"firewalld",
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
			"langpacks-*",
			"langpacks-en",
			"biosdevname",
			"plymouth",
			"iprutils",
			"langpacks-en",
			"fedora-release",
			"fedora-repos",
			"rng-tools",

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		kernelOptions:           "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0 crashkernel=auto",
		bootable:                true,
		defaultSize:             10 * GigaByte,
		partitionTableGenerator: defaultPartitionTable,
		assembler: func(pt *disk.PartitionTable, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler(pt, "qcow2", "disk.qcow2", options, arch)
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
			"kernel",
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
			return qemuAssembler(pt, "qcow2", "disk.qcow2", options, arch)
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
			"kernel",
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
			return qemuAssembler(pt, "vpc", "disk.vhd", options, arch)
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
			"kernel",
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
			return qemuAssembler(pt, "vmdk", "disk.vmdk", options, arch)
		},
	}

	r := distribution{
		imageTypes: map[string]imageType{},
		buildPackages: []string{
			"dnf",
			"dosfstools",
			"e2fsprogs",
			"glibc",
			"policycoreutils",
			"python36",
			"qemu-img",
			"selinux-policy-targeted",
			"systemd",
			"tar",
			"xfsprogs",
			"xz",
		},
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
	x8664.setImageTypes(
		amiImgType,
		edgeImgTypeX86_64,
		qcow2ImageType,
		openstackImgType,
		tarImgType,
		vhdImgType,
		vmdkImgType,
	)

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
	aarch64.setImageTypes(
		amiImgType,
		edgeImgTypeAarch64,
		qcow2ImageType,
		openstackImgType,
		tarImgType,
	)

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
	ppc64le.setImageTypes(
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
	s390x.setImageTypes(
		tarImgType,
		qcow2ImageType,
	)

	r.setArches(x8664, aarch64, ppc64le, s390x)

	return &r
}
