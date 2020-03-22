package rhel81

import (
	"errors"
	"sort"
	"strconv"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/osbuild"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/crypt"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type arch struct {
	Name               string
	BootloaderPackages []string
	BuildPackages      []string
	UEFI               bool
}

type output struct {
	Name             string
	MimeType         string
	Packages         []string
	ExcludedPackages []string
	EnabledServices  []string
	DisabledServices []string
	Bootable         bool
	DefaultTarget    string
	KernelOptions    string
	DefaultSize      uint64
	Assembler        func(uefi bool, size uint64) *osbuild.Assembler
}

const Distro = common.RHEL81
const ModulePlatformID = "platform:el8"

type RHEL81 struct {
	arches        map[string]arch
	outputs       map[string]output
	buildPackages []string
}

type RHEL81Arch struct {
	name   string
	distro *RHEL81
	arch   *arch
}

type RHEL81ImageType struct {
	name   string
	arch   *RHEL81Arch
	output *output
}

func (d *RHEL81) GetArch(arch string) (distro.Arch, error) {
	a, exists := d.arches[arch]
	if !exists {
		return nil, errors.New("invalid architecture: " + arch)
	}

	return &RHEL81Arch{
		name:   arch,
		distro: d,
		arch:   &a,
	}, nil
}

func (a *RHEL81Arch) Name() string {
	return a.name
}

func (a *RHEL81Arch) ListImageTypes() []string {
	return a.distro.ListOutputFormats()
}

func (a *RHEL81Arch) GetImageType(imageType string) (distro.ImageType, error) {
	t, exists := a.distro.outputs[imageType]
	if !exists {
		return nil, errors.New("invalid image type: " + imageType)
	}

	return &RHEL81ImageType{
		name:   imageType,
		arch:   a,
		output: &t,
	}, nil
}

func (t *RHEL81ImageType) Name() string {
	return t.name
}

func (t *RHEL81ImageType) Filename() string {
	return t.output.Name
}

func (t *RHEL81ImageType) MIMEType() string {
	return t.output.MimeType
}

func (t *RHEL81ImageType) Size(size uint64) uint64 {
	return t.arch.distro.GetSizeForOutputType(t.name, size)
}

func (t *RHEL81ImageType) BasePackages() ([]string, []string) {
	packages := t.output.Packages
	if t.output.Bootable {
		packages = append(packages, t.arch.arch.BootloaderPackages...)
	}

	return packages, t.output.ExcludedPackages
}

func (t *RHEL81ImageType) BuildPackages() []string {
	return append(t.arch.distro.buildPackages, t.arch.arch.BuildPackages...)
}

func (t *RHEL81ImageType) Manifest(c *blueprint.Customizations,
	repos []rpmmd.RepoConfig,
	packageSpecs,
	buildPackageSpecs []rpmmd.PackageSpec,
	size uint64) (*osbuild.Manifest, error) {
	pipeline, err := t.arch.distro.pipeline(c, repos, packageSpecs, buildPackageSpecs, t.arch.name, t.name, size)
	if err != nil {
		return nil, err
	}

	return &osbuild.Manifest{
		Sources:  *t.arch.distro.sources(append(packageSpecs, buildPackageSpecs...)),
		Pipeline: *pipeline,
	}, nil
}

func New() *RHEL81 {
	const GigaByte = 1024 * 1024 * 1024

	r := RHEL81{
		outputs: map[string]output{},
		buildPackages: []string{
			"dnf",
			"dosfstools",
			"dracut-config-generic",
			"e2fsprogs",
			"glibc",
			"policycoreutils",
			"python36",
			"qemu-img",
			"systemd",
			"tar",
			"xfsprogs",
		},
		arches: map[string]arch{
			"x86_64": arch{
				Name: "x86_64",
				BootloaderPackages: []string{
					"grub2-pc",
				},
				BuildPackages: []string{
					"grub2-pc",
				},
			},
			"aarch64": arch{
				Name: "aarch64",
				BootloaderPackages: []string{
					"dracut-config-generic",
					"efibootmgr",
					"grub2-efi-aa64",
					"grub2-tools",
					"shim-aa64",
				},
				UEFI: true,
			},
		},
	}

	r.outputs["ami"] = output{
		Name:     "image.raw.xz",
		MimeType: "application/octet-stream",
		Packages: []string{
			"checkpolicy",
			"chrony",
			"cloud-init",
			"cloud-init",
			"cloud-utils-growpart",
			"@core",
			"dhcp-client",
			"dracut-config-generic",
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
		ExcludedPackages: []string{
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
		DefaultTarget: "multi-user.target",
		Bootable:      true,
		KernelOptions: "ro console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto",
		DefaultSize:   6 * GigaByte,
		Assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("raw.xz", "image.raw.xz", uefi, size)
		},
	}

	r.outputs["ext4-filesystem"] = output{
		Name:     "filesystem.img",
		MimeType: "application/octet-stream",
		Packages: []string{
			"policycoreutils",
			"selinux-policy-targeted",
			"kernel",
			"firewalld",
			"chrony",
			"dracut-config-generic",
			"langpacks-en",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		Bootable:      false,
		KernelOptions: "ro net.ifnames=0",
		DefaultSize:   2 * GigaByte,
		Assembler:     func(uefi bool, size uint64) *osbuild.Assembler { return r.rawFSAssembler("filesystem.img", size) },
	}

	r.outputs["partitioned-disk"] = output{
		Name:     "disk.img",
		MimeType: "application/octet-stream",
		Packages: []string{
			"@core",
			"chrony",
			"dracut-config-generic",
			"firewalld",
			"kernel",
			"langpacks-en",
			"selinux-policy-targeted",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		Bootable:      true,
		KernelOptions: "ro net.ifnames=0",
		DefaultSize:   2 * GigaByte,
		Assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("raw", "disk.img", uefi, size)
		},
	}

	r.outputs["qcow2"] = output{
		Name:     "disk.qcow2",
		MimeType: "application/x-qemu-disk",
		Packages: []string{
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
			"dracut-config-generic",
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
		ExcludedPackages: []string{
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
		Bootable:      true,
		KernelOptions: "console=ttyS0 console=ttyS0,115200n8 no_timer_check crashkernel=auto net.ifnames=0",
		DefaultSize:   2 * GigaByte,
		Assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("qcow2", "disk.qcow2", uefi, size)
		},
	}

	r.outputs["openstack"] = output{
		Name:     "disk.qcow2",
		MimeType: "application/x-qemu-disk",
		Packages: []string{
			// Defaults
			"@Core",
			"langpacks-en",

			// Don't run dracut in host-only mode, in order to pull in
			// the hv_vmbus, hv_netvsc and hv_storvsc modules into the initrd.
			"dracut-config-generic",

			// From the lorax kickstart
			"kernel",
			"selinux-policy-targeted",
			"cloud-init",
			"qemu-guest-agent",
			"spice-vdagent",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		Bootable:      true,
		KernelOptions: "ro net.ifnames=0",
		DefaultSize:   2 * GigaByte,
		Assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("qcow2", "disk.qcow2", uefi, size)
		},
	}

	r.outputs["tar"] = output{
		Name:     "root.tar.xz",
		MimeType: "application/x-tar",
		Packages: []string{
			"policycoreutils",
			"selinux-policy-targeted",
			"kernel",
			"firewalld",
			"chrony",
			"dracut-config-generic",
			"langpacks-en",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		Bootable:      false,
		KernelOptions: "ro net.ifnames=0",
		Assembler:     func(uefi bool, size uint64) *osbuild.Assembler { return r.tarAssembler("root.tar.xz", "xz") },
	}

	r.outputs["vhd"] = output{
		Name:     "disk.vhd",
		MimeType: "application/x-vhd",
		Packages: []string{
			// Defaults
			"@Core",
			"langpacks-en",

			// Don't run dracut in host-only mode, in order to pull in
			// the hv_vmbus, hv_netvsc and hv_storvsc modules into the initrd.
			"dracut-config-generic",

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
		ExcludedPackages: []string{
			"dracut-config-rescue",

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		EnabledServices: []string{
			"sshd",
			"waagent",
		},
		DefaultTarget: "multi-user.target",
		Bootable:      true,
		KernelOptions: "ro biosdevname=0 rootdelay=300 console=ttyS0 earlyprintk=ttyS0 net.ifnames=0",
		DefaultSize:   2 * GigaByte,
		Assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("vpc", "disk.vhd", uefi, size)
		},
	}

	r.outputs["vmdk"] = output{
		Name:     "disk.vmdk",
		MimeType: "application/x-vmdk",
		Packages: []string{
			"@core",
			"chrony",
			"dracut-config-generic",
			"firewalld",
			"kernel",
			"langpacks-en",
			"open-vm-tools",
			"selinux-policy-targeted",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		Bootable:      true,
		KernelOptions: "ro net.ifnames=0",
		DefaultSize:   2 * GigaByte,
		Assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("vmdk", "disk.vmdk", uefi, size)
		},
	}

	return &r
}

func (r *RHEL81) Name() string {
	name, exists := Distro.ToString()
	if !exists {
		panic("Fatal error, hardcoded distro value in rhel81 package is not valid!")
	}
	return name
}

func (r *RHEL81) Distribution() common.Distribution {
	return Distro
}

func (r *RHEL81) ModulePlatformID() string {
	return ModulePlatformID
}

func (r *RHEL81) ListOutputFormats() []string {
	formats := make([]string, 0, len(r.outputs))
	for name := range r.outputs {
		formats = append(formats, name)
	}
	sort.Strings(formats)
	return formats
}

func (r *RHEL81) FilenameFromType(outputFormat string) (string, string, error) {
	if output, exists := r.outputs[outputFormat]; exists {
		return output.Name, output.MimeType, nil
	}
	return "", "", errors.New("invalid output format: " + outputFormat)
}

func (r *RHEL81) GetSizeForOutputType(outputFormat string, size uint64) uint64 {
	const MegaByte = 1024 * 1024
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if outputFormat == "vhd" && size%MegaByte != 0 {
		size = (size/MegaByte + 1) * MegaByte
	}
	if size == 0 {
		size = r.outputs[outputFormat].DefaultSize
	}
	return size
}

func (r *RHEL81) BasePackages(outputFormat string, outputArchitecture string) ([]string, []string, error) {
	output, exists := r.outputs[outputFormat]
	if !exists {
		return nil, nil, errors.New("invalid output format: " + outputFormat)
	}

	packages := output.Packages
	if output.Bootable {
		arch, exists := r.arches[outputArchitecture]
		if !exists {
			return nil, nil, errors.New("invalid architecture: " + outputArchitecture)
		}

		packages = append(packages, arch.BootloaderPackages...)
	}

	return packages, output.ExcludedPackages, nil
}

func (r *RHEL81) BuildPackages(outputArchitecture string) ([]string, error) {
	arch, exists := r.arches[outputArchitecture]
	if !exists {
		return nil, errors.New("invalid architecture: " + outputArchitecture)
	}

	return append(r.buildPackages, arch.BuildPackages...), nil
}

func (r *RHEL81) pipeline(c *blueprint.Customizations, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, outputArchitecture, outputFormat string, size uint64) (*osbuild.Pipeline, error) {
	output, exists := r.outputs[outputFormat]
	if !exists {
		return nil, errors.New("invalid output format: " + outputFormat)
	}

	arch, exists := r.arches[outputArchitecture]
	if !exists {
		return nil, errors.New("invalid architecture: " + outputArchitecture)
	}

	p := &osbuild.Pipeline{}
	p.SetBuild(r.buildPipeline(repos, arch, buildPackageSpecs), "org.osbuild.rhel81")

	p.AddStage(osbuild.NewRPMStage(r.rpmStageOptions(arch, repos, packageSpecs)))
	p.AddStage(osbuild.NewFixBLSStage())

	if output.Bootable {
		p.AddStage(osbuild.NewFSTabStage(r.fsTabStageOptions(arch.UEFI)))
	}

	kernelOptions := output.KernelOptions
	if kernel := c.GetKernel(); kernel != nil {
		kernelOptions += " " + kernel.Append
	}
	p.AddStage(osbuild.NewGRUB2Stage(r.grub2StageOptions(kernelOptions, arch.UEFI)))

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
		options, err := r.userStageOptions(users)
		if err != nil {
			return nil, err
		}
		p.AddStage(osbuild.NewUsersStage(options))
	}

	if groups := c.GetGroups(); len(groups) > 0 {
		p.AddStage(osbuild.NewGroupsStage(r.groupStageOptions(groups)))
	}

	if services := c.GetServices(); services != nil || output.EnabledServices != nil {
		p.AddStage(osbuild.NewSystemdStage(r.systemdStageOptions(output.EnabledServices, output.DisabledServices, services, output.DefaultTarget)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(r.firewallStageOptions(firewall)))
	}

	p.AddStage(osbuild.NewSELinuxStage(r.selinuxStageOptions()))

	p.Assembler = output.Assembler(arch.UEFI, size)

	return p, nil
}

func (r *RHEL81) sources(packages []rpmmd.PackageSpec) *osbuild.Sources {
	files := &osbuild.FilesSource{
		URLs: make(map[string]string),
	}
	for _, pkg := range packages {
		files.URLs[pkg.Checksum] = pkg.RemoteLocation
	}
	return &osbuild.Sources{
		"org.osbuild.files": files,
	}
}

func (r *RHEL81) Manifest(c *blueprint.Customizations, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, outputArchitecture, outputFormat string, size uint64) (*osbuild.Manifest, error) {
	pipeline, err := r.pipeline(c, repos, packageSpecs, buildPackageSpecs, outputArchitecture, outputFormat, size)
	if err != nil {
		return nil, err
	}

	return &osbuild.Manifest{
		Sources:  *r.sources(append(packageSpecs, buildPackageSpecs...)),
		Pipeline: *pipeline,
	}, nil
}

func (r *RHEL81) Runner() string {
	return "org.osbuild.rhel81"
}

func (r *RHEL81) buildPipeline(repos []rpmmd.RepoConfig, arch arch, buildPackageSpecs []rpmmd.PackageSpec) *osbuild.Pipeline {
	p := &osbuild.Pipeline{}
	p.AddStage(osbuild.NewRPMStage(r.rpmStageOptions(arch, repos, buildPackageSpecs)))
	return p
}

func (r *RHEL81) rpmStageOptions(arch arch, repos []rpmmd.RepoConfig, specs []rpmmd.PackageSpec) *osbuild.RPMStageOptions {
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

func (r *RHEL81) userStageOptions(users []blueprint.UserCustomization) (*osbuild.UsersStageOptions, error) {
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

func (r *RHEL81) groupStageOptions(groups []blueprint.GroupCustomization) *osbuild.GroupsStageOptions {
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

func (r *RHEL81) firewallStageOptions(firewall *blueprint.FirewallCustomization) *osbuild.FirewallStageOptions {
	options := osbuild.FirewallStageOptions{
		Ports: firewall.Ports,
	}

	if firewall.Services != nil {
		options.EnabledServices = firewall.Services.Enabled
		options.DisabledServices = firewall.Services.Disabled
	}

	return &options
}

func (r *RHEL81) systemdStageOptions(enabledServices, disabledServices []string, s *blueprint.ServicesCustomization, target string) *osbuild.SystemdStageOptions {
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

func (r *RHEL81) fsTabStageOptions(uefi bool) *osbuild.FSTabStageOptions {
	options := osbuild.FSTabStageOptions{}
	options.AddFilesystem("0bd700f8-090f-4556-b797-b340297ea1bd", "xfs", "/", "defaults", 0, 0)
	if uefi {
		options.AddFilesystem("46BB-8120", "vfat", "/boot/efi", "umask=0077,shortname=winnt", 0, 2)
	}
	return &options
}

func (r *RHEL81) grub2StageOptions(kernelOptions string, uefi bool) *osbuild.GRUB2StageOptions {
	id, err := uuid.Parse("0bd700f8-090f-4556-b797-b340297ea1bd")
	if err != nil {
		panic("invalid UUID")
	}

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

func (r *RHEL81) selinuxStageOptions() *osbuild.SELinuxStageOptions {
	return &osbuild.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
}

func (r *RHEL81) qemuAssembler(format string, filename string, uefi bool, size uint64) *osbuild.Assembler {
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

func (r *RHEL81) tarAssembler(filename, compression string) *osbuild.Assembler {
	return osbuild.NewTarAssembler(
		&osbuild.TarAssemblerOptions{
			Filename:    filename,
			Compression: compression,
		})
}

func (r *RHEL81) rawFSAssembler(filename string, size uint64) *osbuild.Assembler {
	id, err := uuid.Parse("0bd700f8-090f-4556-b797-b340297ea1bd")
	if err != nil {
		panic("invalid UUID")
	}
	return osbuild.NewRawFSAssembler(
		&osbuild.RawFSAssemblerOptions{
			Filename:           filename,
			RootFilesystemUUDI: id,
			Size:               size,
			FilesystemType:     "xfs",
		})
}
