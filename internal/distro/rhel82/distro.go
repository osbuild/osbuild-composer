package rhel82

import (
	"sort"
	"strconv"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/crypt"
	"github.com/osbuild/osbuild-composer/internal/distro"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type RHEL82 struct {
	outputs map[string]output
}

type output struct {
	Name             string
	MimeType         string
	Packages         []string
	ExcludedPackages []string
	IncludeFSTab     bool
	DefaultTarget    string
	KernelOptions    string
	Assembler        *pipeline.Assembler
}

func init() {
	const GigaByte = 1024 * 1024 * 1024

	r := RHEL82{
		outputs: map[string]output{},
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
			"grub2",
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
		IncludeFSTab:  true,
		KernelOptions: "ro console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto",
		Assembler:     r.qemuAssembler("raw.xz", "image.raw.xz", 6 * GigaByte),
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
			"langpacks-en",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		IncludeFSTab:  false,
		KernelOptions: "ro net.ifnames=0",
		Assembler:     r.rawFSAssembler("filesystem.img"),
	}

	r.outputs["partitioned-disk"] = output{
		Name:     "disk.img",
		MimeType: "application/octet-stream",
		Packages: []string{
			"@core",
			"chrony",
			"firewalld",
			"grub2-pc",
			"kernel",
			"langpacks-en",
			"selinux-policy-targeted",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		IncludeFSTab:  true,
		KernelOptions: "ro net.ifnames=0",
		Assembler:     r.qemuAssembler("raw", "disk.img", 3*GigaByte),
	}

	r.outputs["qcow2"] = output{
		Name:     "image.qcow2",
		MimeType: "application/x-qemu-disk",
		Packages: []string{
			"kernel-core",
			"chrony",
			"polkit",
			"systemd-udev",
			"selinux-policy-targeted",
			"grub2-pc",
			"langpacks-en",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
			"etables",
			"firewalld",
			"gobject-introspection",
			"plymouth",
		},
		IncludeFSTab:  true,
		KernelOptions: "ro net.ifnames=0",
		Assembler:     r.qemuAssembler("qcow2", "image.qcow2", 3*GigaByte),
	}

	r.outputs["openstack"] = output{
		Name:     "image.qcow2",
		MimeType: "application/x-qemu-disk",
		Packages: []string{
			"@Core",
			"chrony",
			"kernel",
			"selinux-policy-targeted",
			"grub2-pc",
			"spice-vdagent",
			"qemu-guest-agent",
			"xen-libs",
			"langpacks-en",
			"cloud-init",
			"libdrm",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		IncludeFSTab:  true,
		KernelOptions: "ro net.ifnames=0",
		Assembler:     r.qemuAssembler("qcow2", "image.qcow2", 3*GigaByte),
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
			"langpacks-en",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		IncludeFSTab:  false,
		KernelOptions: "ro net.ifnames=0",
		Assembler:     r.tarAssembler("root.tar.xz", "xz"),
	}

	r.outputs["vhd"] = output{
		Name:     "image.vhd",
		MimeType: "application/x-vhd",
		Packages: []string{
			"@Core",
			"chrony",
			"kernel",
			"selinux-policy-targeted",
			"grub2-pc",
			"langpacks-en",
			"net-tools",
			"ntfsprogs",
			"WALinuxAgent",
			"libxcrypt-compat",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		IncludeFSTab:  true,
		KernelOptions: "ro net.ifnames=0",
		Assembler:     r.qemuAssembler("vhd", "image.vhd", 3*GigaByte),
	}

	r.outputs["vmdk"] = output{
		Name:     "disk.vmdk",
		MimeType: "application/x-vmdk",
		Packages: []string{
			"@core",
			"chrony",
			"firewalld",
			"grub2-pc",
			"kernel",
			"langpacks-en",
			"open-vm-tools",
			"selinux-policy-targeted",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		IncludeFSTab:  true,
		KernelOptions: "ro net.ifnames=0",
		Assembler:     r.qemuAssembler("vmdk", "disk.vmdk", 3*GigaByte),
	}

	distro.Register("rhel-8.2", &r)
}

func (r *RHEL82) Repositories() []rpmmd.RepoConfig {
	return []rpmmd.RepoConfig{
		{
			Id:       "baseos",
			Name:     "BaseOS",
			BaseURL:  "http://download-ipv4.eng.brq.redhat.com/rhel-8/nightly/RHEL-8/RHEL-8.2.0-20191125.n.1/compose/BaseOS/x86_64/os",
			Checksum: "sha256:30b905ab1538243de69e019573443b2a1e4edad7c1f7d32aa5a4fb014ff98060",
		},
		{
			Id:       "appstream",
			Name:     "AppStream",
			BaseURL:  "http://download-ipv4.eng.brq.redhat.com/rhel-8/nightly/RHEL-8/RHEL-8.2.0-20191125.n.1/compose/AppStream/x86_64/os",
			Checksum: "sha256:afd86d5b664ec87e209c5ff3cf011bcc6a40578394191c1d889b4ead17a072ae",
		},
	}
}

func (r *RHEL82) ListOutputFormats() []string {
	formats := make([]string, 0, len(r.outputs))
	for name := range r.outputs {
		formats = append(formats, name)
	}
	sort.Strings(formats)
	return formats
}

func (r *RHEL82) FilenameFromType(outputFormat string) (string, string, error) {
	if output, exists := r.outputs[outputFormat]; exists {
		return output.Name, output.MimeType, nil
	}
	return "", "", &distro.InvalidOutputFormatError{outputFormat}
}

func (r *RHEL82) Pipeline(b *blueprint.Blueprint, outputFormat string) (*pipeline.Pipeline, error) {
	output, exists := r.outputs[outputFormat]
	if !exists {
		return nil, &distro.InvalidOutputFormatError{outputFormat}
	}

	p := &pipeline.Pipeline{}
	p.SetBuild(r.buildPipeline(), "org.osbuild.rhel82")

	packages := append(output.Packages, b.GetPackages()...)
	p.AddStage(pipeline.NewDNFStage(r.dnfStageOptions(packages, output.ExcludedPackages)))
	p.AddStage(pipeline.NewFixBLSStage())

	if output.IncludeFSTab {
		p.AddStage(pipeline.NewFSTabStage(r.fsTabStageOptions()))
	}

	kernelOptions := output.KernelOptions
	if kernel := b.GetKernel(); kernel != nil {
		kernelOptions += " " + kernel.Append
	}
	p.AddStage(pipeline.NewGRUB2Stage(r.grub2StageOptions(kernelOptions)))

	// TODO support setting all languages and install corresponding langpack-* package
	language, keyboard := b.GetPrimaryLocale()

	if language != nil {
		p.AddStage(pipeline.NewLocaleStage(&pipeline.LocaleStageOptions{*language}))
	} else {
		p.AddStage(pipeline.NewLocaleStage(&pipeline.LocaleStageOptions{"en_US"}))
	}

	if keyboard != nil {
		p.AddStage(pipeline.NewKeymapStage(&pipeline.KeymapStageOptions{*keyboard}))
	}

	if hostname := b.GetHostname(); hostname != nil {
		p.AddStage(pipeline.NewHostnameStage(&pipeline.HostnameStageOptions{*hostname}))
	}

	timezone, ntpServers := b.GetTimezoneSettings()

	// TODO install chrony when this is set?
	if timezone != nil {
		p.AddStage(pipeline.NewTimezoneStage(&pipeline.TimezoneStageOptions{*timezone}))
	}

	if len(ntpServers) > 0 {
		p.AddStage(pipeline.NewChronyStage(&pipeline.ChronyStageOptions{ntpServers}))
	}

	if users := b.GetUsers(); len(users) > 0 {
		options, err := r.userStageOptions(users)
		if err != nil {
			return nil, err
		}
		p.AddStage(pipeline.NewUsersStage(options))
	}

	if groups := b.GetGroups(); len(groups) > 0 {
		p.AddStage(pipeline.NewGroupsStage(r.groupStageOptions(groups)))
	}

	if services := b.GetServices(); services != nil {
		p.AddStage(pipeline.NewSystemdStage(r.systemdStageOptions(services, output.DefaultTarget)))
	}

	if firewall := b.GetFirewall(); firewall != nil {
		p.AddStage(pipeline.NewFirewallStage(r.firewallStageOptions(firewall)))
	}

	p.AddStage(pipeline.NewSELinuxStage(r.selinuxStageOptions()))
	p.Assembler = output.Assembler

	return p, nil
}

func (r *RHEL82) buildPipeline() *pipeline.Pipeline {
	packages := []string{
		"dnf",
		"dracut-config-generic",
		"e2fsprogs",
		"glibc",
		"grub2-pc",
		"policycoreutils",
		"python36",
		"qemu-img",
		"systemd",
		"tar",
		"xfsprogs",
	}
	p := &pipeline.Pipeline{}
	p.AddStage(pipeline.NewDNFStage(r.dnfStageOptions(packages, nil)))
	return p
}

func (r *RHEL82) dnfStageOptions(packages, excludedPackages []string) *pipeline.DNFStageOptions {
	options := &pipeline.DNFStageOptions{
		ReleaseVersion:   "8",
		BaseArchitecture: "x86_64",
		ModulePlatformId: "platform:el8",
	}
	for _, repo := range r.Repositories() {
		options.AddRepository(&pipeline.DNFRepository{
			BaseURL:    repo.BaseURL,
			MetaLink:   repo.Metalink,
			MirrorList: repo.MirrorList,
			Checksum:   repo.Checksum,
		})
	}

	for _, pkg := range packages {
		options.AddPackage(pkg)
	}

	for _, pkg := range excludedPackages {
		options.ExcludePackage(pkg)
	}

	return options
}

func (r *RHEL82) userStageOptions(users []blueprint.UserCustomization) (*pipeline.UsersStageOptions, error) {
	options := pipeline.UsersStageOptions{
		Users: make(map[string]pipeline.UsersStageOptionsUser),
	}

	for _, c := range users {
		if c.Password != nil && !crypt.PasswordIsCrypted(*c.Password) {
			cryptedPassword, err := crypt.CryptSHA512(*c.Password)
			if err != nil {
				return nil, err
			}

			c.Password = &cryptedPassword
		}

		user := pipeline.UsersStageOptionsUser{
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

func (r *RHEL82) groupStageOptions(groups []blueprint.GroupCustomization) *pipeline.GroupsStageOptions {
	options := pipeline.GroupsStageOptions{
		Groups: map[string]pipeline.GroupsStageOptionsGroup{},
	}

	for _, group := range groups {
		groupData := pipeline.GroupsStageOptionsGroup{
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

func (r *RHEL82) firewallStageOptions(firewall *blueprint.FirewallCustomization) *pipeline.FirewallStageOptions {
	options := pipeline.FirewallStageOptions{
		Ports: firewall.Ports,
	}

	if firewall.Services != nil {
		options.EnabledServices = firewall.Services.Enabled
		options.DisabledServices = firewall.Services.Disabled
	}

	return &options
}

func (r *RHEL82) systemdStageOptions(s *blueprint.ServicesCustomization, target string) *pipeline.SystemdStageOptions {
	return &pipeline.SystemdStageOptions{
		EnabledServices:  s.Enabled,
		DisabledServices: s.Disabled,
		DefaultTarget:    target,
	}
}

func (r *RHEL82) fsTabStageOptions() *pipeline.FSTabStageOptions {
	id, err := uuid.Parse("0bd700f8-090f-4556-b797-b340297ea1bd")
	if err != nil {
		panic("invalid UUID")
	}
	options := pipeline.FSTabStageOptions{}
	options.AddFilesystem(id, "xfs", "/", "defaults", 0, 0)
	return &options
}

func (r *RHEL82) grub2StageOptions(kernelOptions string) *pipeline.GRUB2StageOptions {
	id, err := uuid.Parse("0bd700f8-090f-4556-b797-b340297ea1bd")
	if err != nil {
		panic("invalid UUID")
	}
	return &pipeline.GRUB2StageOptions{
		RootFilesystemUUID: id,
		KernelOptions:      kernelOptions,
	}
}

func (r *RHEL82) selinuxStageOptions() *pipeline.SELinuxStageOptions {
	return &pipeline.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
}

func (r *RHEL82) qemuAssembler(format string, filename string, size uint64) *pipeline.Assembler {
	id, err := uuid.Parse("0bd700f8-090f-4556-b797-b340297ea1bd")
	if err != nil {
		panic("invalid UUID")
	}
	return pipeline.NewQEMUAssembler(
		&pipeline.QEMUAssemblerOptions{
			Format:             format,
			Filename:           filename,
			PTUUID:             "0x14fc63d2",
			RootFilesystemUUDI: id,
			Size:               size,
			RootFilesystemType: "xfs",
		})
}

func (r *RHEL82) tarAssembler(filename, compression string) *pipeline.Assembler {
	return pipeline.NewTarAssembler(
		&pipeline.TarAssemblerOptions{
			Filename: filename,
		})
}

func (r *RHEL82) rawFSAssembler(filename string) *pipeline.Assembler {
	id, err := uuid.Parse("0bd700f8-090f-4556-b797-b340297ea1bd")
	if err != nil {
		panic("invalid UUID")
	}
	return pipeline.NewRawFSAssembler(
		&pipeline.RawFSAssemblerOptions{
			Filename:           filename,
			RootFilesystemUUDI: id,
			Size:               3221225472,
			FilesystemType:     "xfs",
		})
}
