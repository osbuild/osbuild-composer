package rhel8

import (
	"errors"
	"sort"
	"strconv"

	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/crypt"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const name = "rhel-8"
const modulePlatformID = "platform:el8"

type RHEL8 struct {
	arches        map[string]arch
	imageTypes    map[string]imageType
	buildPackages []string
}

type arch struct {
	bootloaderPackages []string
	buildPackages      []string
	uefi               bool
}

type imageType struct {
	name             string
	mimeType         string
	packages         []string
	excludedPackages []string
	enabledServices  []string
	disabledServices []string
	bootable         bool
	defaultTarget    string
	kernelOptions    string
	defaultSize      uint64
	assembler        func(uefi bool, size uint64) *osbuild.Assembler
}

type rhel8Arch struct {
	name   string
	distro *RHEL8
	arch   *arch
}

type rhel8ImageType struct {
	name      string
	arch      *rhel8Arch
	imageType *imageType
}

func (a *rhel8Arch) Distro() distro.Distro {
	return a.distro
}

func (t *rhel8ImageType) Arch() distro.Arch {
	return t.arch
}

func (d *RHEL8) ListArches() []string {
	archs := make([]string, 0, len(d.arches))
	for name := range d.arches {
		archs = append(archs, name)
	}
	sort.Strings(archs)
	return archs
}

func (d *RHEL8) GetArch(arch string) (distro.Arch, error) {
	a, exists := d.arches[arch]
	if !exists {
		return nil, errors.New("invalid architecture: " + arch)
	}

	return &rhel8Arch{
		name:   arch,
		distro: d,
		arch:   &a,
	}, nil
}

func (a *rhel8Arch) Name() string {
	return a.name
}

func (a *rhel8Arch) ListImageTypes() []string {
	formats := make([]string, 0, len(a.distro.imageTypes))
	for name := range a.distro.imageTypes {
		formats = append(formats, name)
	}
	sort.Strings(formats)
	return formats
}

func (a *rhel8Arch) GetImageType(imageType string) (distro.ImageType, error) {
	t, exists := a.distro.imageTypes[imageType]
	if !exists {
		return nil, errors.New("invalid image type: " + imageType)
	}

	return &rhel8ImageType{
		name:      imageType,
		arch:      a,
		imageType: &t,
	}, nil
}

func (t *rhel8ImageType) Name() string {
	return t.name
}

func (t *rhel8ImageType) Filename() string {
	return t.imageType.name
}

func (t *rhel8ImageType) MIMEType() string {
	return t.imageType.mimeType
}

func (t *rhel8ImageType) Size(size uint64) uint64 {
	const MegaByte = 1024 * 1024
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.name == "vhd" && size%MegaByte != 0 {
		size = (size/MegaByte + 1) * MegaByte
	}
	if size == 0 {
		size = t.imageType.defaultSize
	}
	return size
}

func (t *rhel8ImageType) BasePackages() ([]string, []string) {
	packages := t.imageType.packages
	if t.imageType.bootable {
		packages = append(packages, t.arch.arch.bootloaderPackages...)
	}

	return packages, t.imageType.excludedPackages
}

func (t *rhel8ImageType) BuildPackages() []string {
	return append(t.arch.distro.buildPackages, t.arch.arch.buildPackages...)
}

func (t *rhel8ImageType) Manifest(c *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSpecs,
	buildPackageSpecs []rpmmd.PackageSpec) (*osbuild.Manifest, error) {
	pipeline, err := t.pipeline(c, repos, packageSpecs, buildPackageSpecs, options.Size)
	if err != nil {
		return nil, err
	}

	return &osbuild.Manifest{
		Sources:  *sources(append(packageSpecs, buildPackageSpecs...)),
		Pipeline: *pipeline,
	}, nil
}

func New() *RHEL8 {
	const GigaByte = 1024 * 1024 * 1024

	r := RHEL8{
		imageTypes: map[string]imageType{},
		buildPackages: []string{
			"dnf",
			"dosfstools",
			"e2fsprogs",
			"glibc",
			"policycoreutils",
			"python36",
			"qemu-img",
			"systemd",
			"tar",
			"xfsprogs",
			"xz",
		},
		arches: map[string]arch{
			"x86_64": {
				bootloaderPackages: []string{
					"dracut-config-generic",
					"grub2-pc",
				},
				buildPackages: []string{
					"grub2-pc",
				},
			},
			"aarch64": {
				bootloaderPackages: []string{
					"dracut-config-generic",
					"efibootmgr",
					"grub2-efi-aa64",
					"grub2-tools",
					"shim-aa64",
				},
				uefi: true,
			},
		},
	}

	r.imageTypes["ami"] = imageType{
		name:     "image.vhdx",
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
			"rng-tools",
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

			// TODO this cannot be removed, because the kernel (?)
			// depends on it. The ec2 kickstart force-removes it.
			// "linux-firmware",

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		defaultTarget: "multi-user.target",
		bootable:      true,
		kernelOptions: "ro console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto",
		defaultSize:   6 * GigaByte,
		assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("vhdx", "image.vhdx", uefi, size)
		},
	}

	r.imageTypes["qcow2"] = imageType{
		name:     "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packages: []string{
			"@core",
			"chrony",
			"dnf",
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
			"rhn-client-tools",
			"rhnlib",
			"rhnsd",
			"rhn-setup",
			"NetworkManager",
			"dhcp-client",
			"cockpit-ws",
			"cockpit-system",
			"subscription-manager-cockpit",
			"redhat-release",
			"redhat-release-eula",
			"rng-tools",
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

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		bootable:      true,
		kernelOptions: "console=ttyS0 console=ttyS0,115200n8 no_timer_check crashkernel=auto net.ifnames=0",
		defaultSize:   2 * GigaByte,
		assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("qcow2", "disk.qcow2", uefi, size)
		},
	}

	r.imageTypes["openstack"] = imageType{
		name:     "disk.qcow2",
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
		},
		bootable:      true,
		kernelOptions: "ro net.ifnames=0",
		defaultSize:   2 * GigaByte,
		assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("qcow2", "disk.qcow2", uefi, size)
		},
	}

	r.imageTypes["vhd"] = imageType{
		name:     "disk.vhd",
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

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		enabledServices: []string{
			"sshd",
			"waagent",
		},
		defaultTarget: "multi-user.target",
		bootable:      true,
		kernelOptions: "ro biosdevname=0 rootdelay=300 console=ttyS0 earlyprintk=ttyS0 net.ifnames=0",
		defaultSize:   2 * GigaByte,
		assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("vpc", "disk.vhd", uefi, size)
		},
	}

	r.imageTypes["vmdk"] = imageType{
		name:     "disk.vmdk",
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

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		bootable:      true,
		kernelOptions: "ro net.ifnames=0",
		defaultSize:   2 * GigaByte,
		assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("vmdk", "disk.vmdk", uefi, size)
		},
	}

	return &r
}

func (r *RHEL8) Name() string {
	return name
}

func (r *RHEL8) ModulePlatformID() string {
	return modulePlatformID
}

func (r *RHEL8) BasePackages(outputFormat string, outputArchitecture string) ([]string, []string, error) {
	output, exists := r.imageTypes[outputFormat]
	if !exists {
		return nil, nil, errors.New("invalid output format: " + outputFormat)
	}

	packages := output.packages
	if output.bootable {
		arch, exists := r.arches[outputArchitecture]
		if !exists {
			return nil, nil, errors.New("invalid architecture: " + outputArchitecture)
		}

		packages = append(packages, arch.bootloaderPackages...)
	}

	return packages, output.excludedPackages, nil
}

func (r *RHEL8) BuildPackages(outputArchitecture string) ([]string, error) {
	arch, exists := r.arches[outputArchitecture]
	if !exists {
		return nil, errors.New("invalid architecture: " + outputArchitecture)
	}

	return append(r.buildPackages, arch.buildPackages...), nil
}

func sources(packages []rpmmd.PackageSpec) *osbuild.Sources {
	files := &osbuild.FilesSource{
		URLs: make(map[string]osbuild.FileSource),
	}
	for _, pkg := range packages {
		FileSource := osbuild.FileSource{
			URL: pkg.RemoteLocation,
		}
		if pkg.Secrets == "org.osbuild.rhsm" {
			FileSource.Secrets = &osbuild.Secret{
				Name: "org.osbuild.rhsm",
			}
		}
		files.URLs[pkg.Checksum] = FileSource
	}
	return &osbuild.Sources{
		"org.osbuild.files": files,
	}
}

func (t *rhel8ImageType) pipeline(c *blueprint.Customizations, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, size uint64) (*osbuild.Pipeline, error) {
	p := &osbuild.Pipeline{}
	p.SetBuild(t.buildPipeline(repos, *t.arch.arch, buildPackageSpecs), "org.osbuild.rhel82")

	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(*t.arch.arch, repos, packageSpecs)))
	p.AddStage(osbuild.NewFixBLSStage())

	if t.imageType.bootable {
		p.AddStage(osbuild.NewFSTabStage(t.fsTabStageOptions(t.arch.arch.uefi)))
	}

	kernelOptions := t.imageType.kernelOptions
	if kernel := c.GetKernel(); kernel != nil {
		kernelOptions += " " + kernel.Append
	}
	p.AddStage(osbuild.NewGRUB2Stage(t.grub2StageOptions(kernelOptions, t.arch.arch.uefi)))

	// TODO support setting all languages and install corresponding langpack-* package
	language, keyboard := c.GetPrimaryLocale()

	if language != nil {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{*language}))
	} else {
		p.AddStage(osbuild.NewLocaleStage(&osbuild.LocaleStageOptions{"en_US"}))
	}

	if keyboard != nil {
		p.AddStage(osbuild.NewKeymapStage(&osbuild.KeymapStageOptions{*keyboard}))
	}

	if hostname := c.GetHostname(); hostname != nil {
		p.AddStage(osbuild.NewHostnameStage(&osbuild.HostnameStageOptions{*hostname}))
	}

	timezone, ntpServers := c.GetTimezoneSettings()

	// TODO install chrony when this is set?
	if timezone != nil {
		p.AddStage(osbuild.NewTimezoneStage(&osbuild.TimezoneStageOptions{*timezone}))
	}

	if len(ntpServers) > 0 {
		p.AddStage(osbuild.NewChronyStage(&osbuild.ChronyStageOptions{ntpServers}))
	}

	if users := c.GetUsers(); len(users) > 0 {
		options, err := t.userStageOptions(users)
		if err != nil {
			return nil, err
		}
		p.AddStage(osbuild.NewUsersStage(options))
	}

	if groups := c.GetGroups(); len(groups) > 0 {
		p.AddStage(osbuild.NewGroupsStage(t.groupStageOptions(groups)))
	}

	if services := c.GetServices(); services != nil || t.imageType.enabledServices != nil {
		p.AddStage(osbuild.NewSystemdStage(t.systemdStageOptions(t.imageType.enabledServices, t.imageType.disabledServices, services, t.imageType.defaultTarget)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(t.firewallStageOptions(firewall)))
	}

	p.AddStage(osbuild.NewSELinuxStage(t.selinuxStageOptions()))

	p.Assembler = t.imageType.assembler(t.arch.arch.uefi, size)

	return p, nil
}

func (r *rhel8ImageType) buildPipeline(repos []rpmmd.RepoConfig, arch arch, buildPackageSpecs []rpmmd.PackageSpec) *osbuild.Pipeline {
	p := &osbuild.Pipeline{}
	p.AddStage(osbuild.NewRPMStage(r.rpmStageOptions(arch, repos, buildPackageSpecs)))
	return p
}

func (r *rhel8ImageType) rpmStageOptions(arch arch, repos []rpmmd.RepoConfig, specs []rpmmd.PackageSpec) *osbuild.RPMStageOptions {
	var gpgKeys []string
	for _, repo := range repos {
		if repo.GPGKey == "" {
			continue
		}
		gpgKeys = append(gpgKeys, repo.GPGKey)
	}

	var packages []string
	for _, spec := range specs {
		packages = append(packages, spec.Checksum)
	}

	return &osbuild.RPMStageOptions{
		GPGKeys:  gpgKeys,
		Packages: packages,
	}
}
func (r *rhel8ImageType) userStageOptions(users []blueprint.UserCustomization) (*osbuild.UsersStageOptions, error) {
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

		if c.UID != nil {
			uid := strconv.Itoa(*c.UID)
			user.UID = &uid
		}

		if c.GID != nil {
			gid := strconv.Itoa(*c.GID)
			user.GID = &gid
		}

		options.Users[c.Name] = user
	}

	return &options, nil
}

func (r *rhel8ImageType) groupStageOptions(groups []blueprint.GroupCustomization) *osbuild.GroupsStageOptions {
	options := osbuild.GroupsStageOptions{
		Groups: map[string]osbuild.GroupsStageOptionsGroup{},
	}

	for _, group := range groups {
		groupData := osbuild.GroupsStageOptionsGroup{
			Name: group.Name,
		}
		if group.GID != nil {
			gid := strconv.Itoa(*group.GID)
			groupData.GID = &gid
		}

		options.Groups[group.Name] = groupData
	}

	return &options
}

func (r *rhel8ImageType) firewallStageOptions(firewall *blueprint.FirewallCustomization) *osbuild.FirewallStageOptions {
	options := osbuild.FirewallStageOptions{
		Ports: firewall.Ports,
	}

	if firewall.Services != nil {
		options.EnabledServices = firewall.Services.Enabled
		options.DisabledServices = firewall.Services.Disabled
	}

	return &options
}

func (r *rhel8ImageType) systemdStageOptions(enabledServices, disabledServices []string, s *blueprint.ServicesCustomization, target string) *osbuild.SystemdStageOptions {
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

func (r *rhel8ImageType) fsTabStageOptions(uefi bool) *osbuild.FSTabStageOptions {
	options := osbuild.FSTabStageOptions{}
	options.AddFilesystem("0bd700f8-090f-4556-b797-b340297ea1bd", "xfs", "/", "defaults", 0, 0)
	if uefi {
		options.AddFilesystem("46BB-8120", "vfat", "/boot/efi", "umask=0077,shortname=winnt", 0, 2)
	}
	return &options
}

func (r *rhel8ImageType) grub2StageOptions(kernelOptions string, uefi bool) *osbuild.GRUB2StageOptions {
	id := uuid.MustParse("0bd700f8-090f-4556-b797-b340297ea1bd")

	var uefiOptions *osbuild.GRUB2UEFI
	if uefi {
		uefiOptions = &osbuild.GRUB2UEFI{
			Vendor: "redhat",
		}
	}

	return &osbuild.GRUB2StageOptions{
		RootFilesystemUUID: id,
		KernelOptions:      kernelOptions,
		Legacy:             !uefi,
		UEFI:               uefiOptions,
	}
}

func (r *rhel8ImageType) selinuxStageOptions() *osbuild.SELinuxStageOptions {
	return &osbuild.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
}

func (r *RHEL8) qemuAssembler(format string, filename string, uefi bool, size uint64) *osbuild.Assembler {
	var options osbuild.QEMUAssemblerOptions
	if uefi {
		fstype := uuid.MustParse("C12A7328-F81F-11D2-BA4B-00A0C93EC93B")
		options = osbuild.QEMUAssemblerOptions{
			Format:   format,
			Filename: filename,
			Size:     size,
			PTUUID:   "8DFDFF87-C96E-EA48-A3A6-9408F1F6B1EF",
			PTType:   "gpt",
			Partitions: []osbuild.QEMUPartition{
				{
					Start: 2048,
					Size:  972800,
					Type:  &fstype,
					Filesystem: osbuild.QEMUFilesystem{
						Type:       "vfat",
						UUID:       "46BB-8120",
						Label:      "EFI System Partition",
						Mountpoint: "/boot/efi",
					},
				},
				{
					Start: 976896,
					Filesystem: osbuild.QEMUFilesystem{
						Type:       "xfs",
						UUID:       "0bd700f8-090f-4556-b797-b340297ea1bd",
						Mountpoint: "/",
					},
				},
			},
		}
	} else {
		options = osbuild.QEMUAssemblerOptions{
			Format:   format,
			Filename: filename,
			Size:     size,
			PTUUID:   "0x14fc63d2",
			PTType:   "mbr",
			Partitions: []osbuild.QEMUPartition{
				{
					Start:    2048,
					Bootable: true,
					Filesystem: osbuild.QEMUFilesystem{
						Type:       "xfs",
						UUID:       "0bd700f8-090f-4556-b797-b340297ea1bd",
						Mountpoint: "/",
					},
				},
			},
		}
	}
	return osbuild.NewQEMUAssembler(&options)
}
