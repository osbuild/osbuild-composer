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
	Assembler        *pipeline.Assembler
}

func init() {
	r := RHEL82{
		outputs: map[string]output{},
	}

	r.outputs["ami"] = output{
		Name:     "image.ami",
		MimeType: "application/x-qemu-disk",
		Packages: []string{
			"@Core",
			"chrony",
			"kernel",
			"selinux-policy-targeted",
			"grub2-pc",
			"dracut-config-generic",
			"cloud-init",
			"checkpolicy",
			"net-tools",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",

			// TODO setfiles failes because of usr/sbin/timedatex. Exlude until
			// https://errata.devel.redhat.com/advisory/47339 lands
			"timedatex",
		},
		IncludeFSTab: true,
		Assembler:    r.qemuAssembler("raw", "image.ami"),
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
		IncludeFSTab: false,
		Assembler:    r.rawFSAssembler("filesystem.img"),
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
		IncludeFSTab: true,
		Assembler:    r.qemuAssembler("raw", "disk.img"),
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
		IncludeFSTab: true,
		Assembler:    r.qemuAssembler("qcow2", "image.qcow2"),
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
		IncludeFSTab: true,
		Assembler:    r.qemuAssembler("qcow2", "image.qcow2"),
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
		IncludeFSTab: false,
		Assembler:    r.tarAssembler("root.tar.xz", "xz"),
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
		IncludeFSTab: true,
		Assembler:    r.qemuAssembler("vhd", "image.vhd"),
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
		IncludeFSTab: true,
		Assembler:    r.qemuAssembler("vmdk", "disk.vmdk"),
	}

	distro.Register("rhel-8.2", &r)
}

func (r *RHEL82) Repositories() []rpmmd.RepoConfig {
	return []rpmmd.RepoConfig{
		{
			Id:       "baseos",
			Name:     "BaseOS",
			BaseURL:  "http://download-ipv4.eng.brq.redhat.com/rhel-8/nightly/RHEL-8/RHEL-8.2.0-20191117.n.0/compose/BaseOS/x86_64/os",
			Checksum: "sha256:4699a755326e5af71cd069dc9d9289e7d0433ab0acc42ee33b93054fd0e980e7",
		},
		{
			Id:       "appstream",
			Name:     "AppStream",
			BaseURL:  "http://download-ipv4.eng.brq.redhat.com/rhel-8/nightly/RHEL-8/RHEL-8.2.0-20191117.n.0/compose/AppStream/x86_64/os",
			Checksum: "sha256:212f10ee3fb8265f38837a1e867e4218556e9bc71fa1d38827a088413974a949",
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
	p.AddStage(pipeline.NewGRUB2Stage(r.grub2StageOptions(b.GetKernel())))

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
		p.AddStage(pipeline.NewSystemdStage(r.systemdStageOptions(services)))
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

func (r *RHEL82) systemdStageOptions(s *blueprint.ServicesCustomization) *pipeline.SystemdStageOptions {
	return &pipeline.SystemdStageOptions{
		EnabledServices:  s.Enabled,
		DisabledServices: s.Disabled,
	}
}

func (r *RHEL82) fsTabStageOptions() *pipeline.FSTabStageOptions {
	id, err := uuid.Parse("0bd700f8-090f-4556-b797-b340297ea1bd")
	if err != nil {
		panic("invalid UUID")
	}
	options := pipeline.FSTabStageOptions{}
	options.AddFilesystem(id, "xfs", "/", "defaults", 1, 1)
	return &options
}

func (r *RHEL82) grub2StageOptions(kernel *blueprint.KernelCustomization) *pipeline.GRUB2StageOptions {
	id, err := uuid.Parse("0bd700f8-090f-4556-b797-b340297ea1bd")
	if err != nil {
		panic("invalid UUID")
	}
	kernelOptions := "ro biosdevname=0 net.ifnames=0"

	if kernel != nil {
		kernelOptions += " " + kernel.Append
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

func (r *RHEL82) qemuAssembler(format string, filename string) *pipeline.Assembler {
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
			Size:               3221225472,
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
