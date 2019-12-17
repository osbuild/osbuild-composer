package fedora30

import (
	"errors"
	"log"
	"sort"
	"strconv"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/crypt"
	"github.com/osbuild/osbuild-composer/internal/pipeline"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type Fedora30 struct {
	arches  map[string]arch
	outputs map[string]output
}

type arch struct {
	Name               string
	BootloaderPackages []string
	BuildPackages      []string
	UEFI               bool
	Repositories       []rpmmd.RepoConfig
}

type output struct {
	Name             string
	MimeType         string
	Packages         []string
	ExcludedPackages []string
	EnabledServices  []string
	DisabledServices []string
	KernelOptions    string
	Bootable         bool
	Assembler        func(uefi bool) *pipeline.Assembler
}

const Name = "fedora-30"

func New(confPaths []string) *Fedora30 {
	r := Fedora30{
		arches:  map[string]arch{},
		outputs: map[string]output{},
	}

	repoMap, err := rpmmd.LoadRepositories(confPaths, Name)
	if err != nil {
		log.Printf("Could not load repository data for %s: %s", Name, err.Error())
		return nil
	}

	repos, exists := repoMap["x86_64"]
	if !exists {
		log.Printf("Could not load architecture-specific repository data for x86_64 (%s): %s", Name, err.Error())
	} else {
		r.arches["x86_64"] = arch{
			Name: "x86_64",
			BootloaderPackages: []string{
				"grub2-pc",
			},
			BuildPackages: []string{
				"grub2-pc",
			},
			Repositories: repos,
		}
	}

	repos, exists = repoMap["aarch64"]
	if !exists {
		log.Printf("Could not load architecture-specific repository data for x86_64 (%s): %s", Name, err.Error())
	} else {
		r.arches["aarch64"] = arch{
			Name: "aarch64",
			BootloaderPackages: []string{
				"dracut-config-generic",
				"efibootmgr",
				"grub2-efi-aa64",
				"grub2-tools",
				"shim-aa64",
			},
			UEFI:         true,
			Repositories: repos,
		}
	}

	r.outputs["ami"] = output{
		Name:     "image.raw.xz",
		MimeType: "application/octet-stream",
		Packages: []string{
			"@Core",
			"chrony",
			"kernel",
			"selinux-policy-targeted",
			"langpacks-en",
			"libxcrypt-compat",
			"xfsprogs",
			"cloud-init",
			"checkpolicy",
			"net-tools",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		EnabledServices: []string{
			"cloud-init.service",
		},
		KernelOptions: "ro no_timer_check console=ttyS0,115200n8 console=tty1 biosdevname=0 net.ifnames=0 console=ttyS0,115200",
		Bootable:      true,
		Assembler:     func(uefi bool) *pipeline.Assembler { return r.qemuAssembler("raw.xz", "image.raw.xz", uefi) },
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
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		Bootable:      false,
		Assembler:     func(uefi bool) *pipeline.Assembler { return r.rawFSAssembler("filesystem.img") },
	}

	r.outputs["partitioned-disk"] = output{
		Name:     "disk.img",
		MimeType: "application/octet-stream",
		Packages: []string{
			"@core",
			"chrony",
			"firewalld",
			"kernel",
			"langpacks-en",
			"selinux-policy-targeted",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		Bootable:      true,
		Assembler:     func(uefi bool) *pipeline.Assembler { return r.qemuAssembler("raw", "disk.img", uefi) },
	}

	r.outputs["qcow2"] = output{
		Name:     "disk.qcow2",
		MimeType: "application/x-qemu-disk",
		Packages: []string{
			"kernel-core",
			"@Fedora Cloud Server",
			"chrony",
			"polkit",
			"systemd-udev",
			"selinux-policy-targeted",
			"langpacks-en",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
			"etables",
			"firewalld",
			"gobject-introspection",
			"plymouth",
		},
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		Bootable:      true,
		Assembler:     func(uefi bool) *pipeline.Assembler { return r.qemuAssembler("qcow2", "disk.qcow2", uefi) },
	}

	r.outputs["openstack"] = output{
		Name:     "disk.qcow2",
		MimeType: "application/x-qemu-disk",
		Packages: []string{
			"@Core",
			"chrony",
			"kernel",
			"selinux-policy-targeted",
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
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		Bootable:      true,
		Assembler:     func(uefi bool) *pipeline.Assembler { return r.qemuAssembler("qcow2", "disk.qcow2", uefi) },
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
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		Bootable:      false,
		Assembler:     func(uefi bool) *pipeline.Assembler { return r.tarAssembler("root.tar.xz", "xz") },
	}

	r.outputs["vhd"] = output{
		Name:     "disk.vhd",
		MimeType: "application/x-vhd",
		Packages: []string{
			"@Core",
			"chrony",
			"kernel",
			"selinux-policy-targeted",
			"langpacks-en",
			"net-tools",
			"ntfsprogs",
			"WALinuxAgent",
			"libxcrypt-compat",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		Bootable:      true,
		Assembler:     func(uefi bool) *pipeline.Assembler { return r.qemuAssembler("vpc", "disk.vhd", uefi) },
	}

	r.outputs["vmdk"] = output{
		Name:     "disk.vmdk",
		MimeType: "application/x-vmdk",
		Packages: []string{
			"@core",
			"chrony",
			"firewalld",
			"kernel",
			"langpacks-en",
			"open-vm-tools",
			"selinux-policy-targeted",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		Bootable:      true,
		Assembler:     func(uefi bool) *pipeline.Assembler { return r.qemuAssembler("vmdk", "disk.vmdk", uefi) },
	}

	return &r
}

func (r *Fedora30) Name() string {
	return Name
}

func (r *Fedora30) Repositories(arch string) []rpmmd.RepoConfig {
	return r.arches[arch].Repositories
}

func (r *Fedora30) ListOutputFormats() []string {
	formats := make([]string, 0, len(r.outputs))
	for name := range r.outputs {
		formats = append(formats, name)
	}
	sort.Strings(formats)
	return formats
}

func (r *Fedora30) FilenameFromType(outputFormat string) (string, string, error) {
	if output, exists := r.outputs[outputFormat]; exists {
		return output.Name, output.MimeType, nil
	}
	return "", "", errors.New("invalid output format: " + outputFormat)
}

func (r *Fedora30) Pipeline(b *blueprint.Blueprint, additionalRepos []rpmmd.RepoConfig, checksums map[string]string, outputArchitecture, outputFormat string) (*pipeline.Pipeline, error) {
	output, exists := r.outputs[outputFormat]
	if !exists {
		return nil, errors.New("invalid output format: " + outputFormat)
	}

	arch, exists := r.arches[outputArchitecture]
	if !exists {
		return nil, errors.New("invalid architecture: " + outputArchitecture)
	}

	p := &pipeline.Pipeline{}
	p.SetBuild(r.buildPipeline(arch, checksums), "org.osbuild.fedora30")

	packages := append(output.Packages, b.GetPackages()...)
	if output.Bootable {
		packages = append(packages, arch.BootloaderPackages...)
	}
	p.AddStage(pipeline.NewDNFStage(r.dnfStageOptions(arch, additionalRepos, checksums, packages, output.ExcludedPackages)))
	p.AddStage(pipeline.NewFixBLSStage())

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

	if output.Bootable {
		p.AddStage(pipeline.NewFSTabStage(r.fsTabStageOptions(arch.UEFI)))
	}
	p.AddStage(pipeline.NewGRUB2Stage(r.grub2StageOptions(output.KernelOptions, b.GetKernel(), arch.UEFI)))

	if services := b.GetServices(); services != nil || output.EnabledServices != nil {
		p.AddStage(pipeline.NewSystemdStage(r.systemdStageOptions(output.EnabledServices, output.DisabledServices, services)))
	}

	if firewall := b.GetFirewall(); firewall != nil {
		p.AddStage(pipeline.NewFirewallStage(r.firewallStageOptions(firewall)))
	}

	p.AddStage(pipeline.NewSELinuxStage(r.selinuxStageOptions()))
	p.Assembler = output.Assembler(arch.UEFI)

	return p, nil
}

func (r *Fedora30) Runner() string {
	return "org.osbuild.fedora30"
}

func (r *Fedora30) buildPipeline(arch arch, checksums map[string]string) *pipeline.Pipeline {
	packages := []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"policycoreutils",
		"qemu-img",
		"systemd",
		"tar",
	}
	packages = append(packages, arch.BuildPackages...)
	p := &pipeline.Pipeline{}
	p.AddStage(pipeline.NewDNFStage(r.dnfStageOptions(arch, nil, checksums, packages, nil)))
	return p
}

func (r *Fedora30) dnfStageOptions(arch arch, additionalRepos []rpmmd.RepoConfig, checksums map[string]string, packages, excludedPackages []string) *pipeline.DNFStageOptions {
	options := &pipeline.DNFStageOptions{
		ReleaseVersion:   "30",
		BaseArchitecture: arch.Name,
		ModulePlatformId: "platform:f30",
	}

	for _, repo := range append(arch.Repositories, additionalRepos...) {
		options.AddRepository(&pipeline.DNFRepository{
			BaseURL:    repo.BaseURL,
			MetaLink:   repo.Metalink,
			MirrorList: repo.MirrorList,
			GPGKey:     repo.GPGKey,
			Checksum:   checksums[repo.Id],
		})
	}

	sort.Strings(packages)
	for _, pkg := range packages {
		options.AddPackage(pkg)
	}

	sort.Strings(excludedPackages)
	for _, pkg := range excludedPackages {
		options.ExcludePackage(pkg)
	}

	return options
}

func (r *Fedora30) userStageOptions(users []blueprint.UserCustomization) (*pipeline.UsersStageOptions, error) {
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

func (r *Fedora30) groupStageOptions(groups []blueprint.GroupCustomization) *pipeline.GroupsStageOptions {
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

func (r *Fedora30) firewallStageOptions(firewall *blueprint.FirewallCustomization) *pipeline.FirewallStageOptions {
	options := pipeline.FirewallStageOptions{
		Ports: firewall.Ports,
	}

	if firewall.Services != nil {
		options.EnabledServices = firewall.Services.Enabled
		options.DisabledServices = firewall.Services.Disabled
	}

	return &options
}

func (r *Fedora30) systemdStageOptions(enabledServices, disabledServices []string, s *blueprint.ServicesCustomization) *pipeline.SystemdStageOptions {
	if s != nil {
		enabledServices = append(enabledServices, s.Enabled...)
		enabledServices = append(disabledServices, s.Disabled...)
	}
	return &pipeline.SystemdStageOptions{
		EnabledServices:  enabledServices,
		DisabledServices: disabledServices,
	}
}

func (r *Fedora30) fsTabStageOptions(uefi bool) *pipeline.FSTabStageOptions {
	options := pipeline.FSTabStageOptions{}
	options.AddFilesystem("76a22bf4-f153-4541-b6c7-0332c0dfaeac", "ext4", "/", "defaults", 1, 1)
	if uefi {
		options.AddFilesystem("46BB-8120", "vfat", "/boot/efi", "umask=0077,shortname=winnt", 0, 2)
	}
	return &options
}

func (r *Fedora30) grub2StageOptions(kernelOptions string, kernel *blueprint.KernelCustomization, uefi bool) *pipeline.GRUB2StageOptions {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}

	if kernel != nil {
		kernelOptions += " " + kernel.Append
	}

	var uefiOptions *pipeline.GRUB2UEFI
	if uefi {
		uefiOptions = &pipeline.GRUB2UEFI{
			Vendor: "fedora",
		}
	}

	return &pipeline.GRUB2StageOptions{
		RootFilesystemUUID: id,
		KernelOptions:      kernelOptions,
		Legacy:             !uefi,
		UEFI:               uefiOptions,
	}
}

func (r *Fedora30) selinuxStageOptions() *pipeline.SELinuxStageOptions {
	return &pipeline.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
}

func (r *Fedora30) qemuAssembler(format string, filename string, uefi bool) *pipeline.Assembler {
	var options pipeline.QEMUAssemblerOptions
	if uefi {
		fstype := uuid.MustParse("C12A7328-F81F-11D2-BA4B-00A0C93EC93B")
		options = pipeline.QEMUAssemblerOptions{
			Format:   format,
			Filename: filename,
			Size:     3222274048,
			PTUUID:   "8DFDFF87-C96E-EA48-A3A6-9408F1F6B1EF",
			PTType:   "gpt",
			Partitions: []pipeline.QEMUPartition{
				{
					Start: 2048,
					Size:  972800,
					Type:  &fstype,
					Filesystem: pipeline.QEMUFilesystem{
						Type:       "vfat",
						UUID:       "46BB-8120",
						Label:      "EFI System Partition",
						Mountpoint: "/boot/efi",
					},
				},
				{
					Start: 976896,
					Filesystem: pipeline.QEMUFilesystem{
						Type:       "ext4",
						UUID:       "76a22bf4-f153-4541-b6c7-0332c0dfaeac",
						Mountpoint: "/",
					},
				},
			},
		}
	} else {
		options = pipeline.QEMUAssemblerOptions{
			Format:   format,
			Filename: filename,
			Size:     3222274048,
			PTUUID:   "0x14fc63d2",
			PTType:   "mbr",
			Partitions: []pipeline.QEMUPartition{
				{
					Start:    2048,
					Bootable: true,
					Filesystem: pipeline.QEMUFilesystem{
						Type:       "ext4",
						UUID:       "76a22bf4-f153-4541-b6c7-0332c0dfaeac",
						Mountpoint: "/",
					},
				},
			},
		}
	}
	return pipeline.NewQEMUAssembler(&options)
}

func (r *Fedora30) tarAssembler(filename, compression string) *pipeline.Assembler {
	return pipeline.NewTarAssembler(
		&pipeline.TarAssemblerOptions{
			Filename:    filename,
			Compression: compression,
		})
}

func (r *Fedora30) rawFSAssembler(filename string) *pipeline.Assembler {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}
	return pipeline.NewRawFSAssembler(
		&pipeline.RawFSAssemblerOptions{
			Filename:           filename,
			RootFilesystemUUDI: id,
			Size:               3222274048,
		})
}
