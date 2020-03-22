package fedora31

import (
	"errors"
	"sort"
	"strconv"

	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/osbuild"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/crypt"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

type Fedora31 struct {
	arches        map[string]arch
	outputs       map[string]output
	buildPackages []string
}

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
	KernelOptions    string
	Bootable         bool
	DefaultSize      uint64
	Assembler        func(uefi bool, size uint64) *osbuild.Assembler
}

const Distro = common.Fedora31
const ModulePlatformID = "platform:f31"

func New() *Fedora31 {
	const GigaByte = 1024 * 1024 * 1024

	r := Fedora31{
		outputs: map[string]output{},
		buildPackages: []string{
			"dnf",
			"dosfstools",
			"e2fsprogs",
			"policycoreutils",
			"qemu-img",
			"systemd",
			"tar",
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
			"langpacks-en",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		Bootable:      false,
		DefaultSize:   2 * GigaByte,
		Assembler:     func(uefi bool, size uint64) *osbuild.Assembler { return r.rawFSAssembler("filesystem.img", size) },
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
		DefaultSize:   2 * GigaByte,
		Assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("raw", "disk.img", uefi, size)
		},
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
		DefaultSize:   2 * GigaByte,
		Assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("qcow2", "disk.qcow2", uefi, size)
		},
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
			"langpacks-en",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		KernelOptions: "ro biosdevname=0 net.ifnames=0",
		Bootable:      false,
		DefaultSize:   2 * GigaByte,
		Assembler:     func(uefi bool, size uint64) *osbuild.Assembler { return r.tarAssembler("root.tar.xz", "xz") },
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
			"initscripts",
			"glibc-all-langpacks",
			"dracut-config-generic",
		},
		ExcludedPackages: []string{
			"dracut-config-rescue",
		},
		EnabledServices: []string{
			"sshd",
			"waagent", // needed to run in Azure
		},
		DisabledServices: []string{
			"proc-sys-fs-binfmt_misc.mount",
			"loadmodules.service",
		},
		// These kernel parameters are required by Azure documentation
		KernelOptions: "ro biosdevname=0 rootdelay=300 console=ttyS0 earlyprintk=ttyS0 net.ifnames=0",
		Bootable:      true,
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
		DefaultSize:   2 * GigaByte,
		Assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("vmdk", "disk.vmdk", uefi, size)
		},
	}

	return &r
}

func (r *Fedora31) Name() string {
	name, exists := Distro.ToString()
	if !exists {
		panic("Fatal error, hardcoded distro value in fedora31 package is not valid!")
	}
	return name
}

func (r *Fedora31) Distribution() common.Distribution {
	return Distro
}

func (r *Fedora31) ModulePlatformID() string {
	return ModulePlatformID
}

func (r *Fedora31) ListOutputFormats() []string {
	formats := make([]string, 0, len(r.outputs))
	for name := range r.outputs {
		formats = append(formats, name)
	}
	sort.Strings(formats)
	return formats
}

func (r *Fedora31) FilenameFromType(outputFormat string) (string, string, error) {
	if output, exists := r.outputs[outputFormat]; exists {
		return output.Name, output.MimeType, nil
	}
	return "", "", errors.New("invalid output format: " + outputFormat)
}

func (r *Fedora31) GetSizeForOutputType(outputFormat string, size uint64) uint64 {
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

func (r *Fedora31) BasePackages(outputFormat string, outputArchitecture string) ([]string, []string, error) {
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

func (r *Fedora31) BuildPackages(outputArchitecture string) ([]string, error) {
	arch, exists := r.arches[outputArchitecture]
	if !exists {
		return nil, errors.New("invalid architecture: " + outputArchitecture)
	}

	return append(r.buildPackages, arch.BuildPackages...), nil
}

func (r *Fedora31) pipeline(c *blueprint.Customizations, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, outputArchitecture, outputFormat string, size uint64) (*osbuild.Pipeline, error) {
	output, exists := r.outputs[outputFormat]
	if !exists {
		return nil, errors.New("invalid output format: " + outputFormat)
	}

	arch, exists := r.arches[outputArchitecture]
	if !exists {
		return nil, errors.New("invalid architecture: " + outputArchitecture)
	}

	p := &osbuild.Pipeline{}
	p.SetBuild(r.buildPipeline(repos, arch, buildPackageSpecs), "org.osbuild.fedora31")

	p.AddStage(osbuild.NewRPMStage(r.rpmStageOptions(arch, repos, packageSpecs)))
	p.AddStage(osbuild.NewFixBLSStage())

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

	if output.Bootable {
		p.AddStage(osbuild.NewFSTabStage(r.fsTabStageOptions(arch.UEFI)))
	}
	p.AddStage(osbuild.NewGRUB2Stage(r.grub2StageOptions(output.KernelOptions, c.GetKernel(), arch.UEFI)))

	if services := c.GetServices(); services != nil || output.EnabledServices != nil {
		p.AddStage(osbuild.NewSystemdStage(r.systemdStageOptions(output.EnabledServices, output.DisabledServices, services)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(r.firewallStageOptions(firewall)))
	}

	p.AddStage(osbuild.NewSELinuxStage(r.selinuxStageOptions()))

	p.Assembler = output.Assembler(arch.UEFI, size)

	return p, nil
}

func (r *Fedora31) sources(packages []rpmmd.PackageSpec) *osbuild.Sources {
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

func (r *Fedora31) Manifest(c *blueprint.Customizations, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, outputArchitecture, outputFormat string, size uint64) (*osbuild.Manifest, error) {
	pipeline, err := r.pipeline(c, repos, packageSpecs, buildPackageSpecs, outputArchitecture, outputFormat, size)
	if err != nil {
		return nil, err
	}

	return &osbuild.Manifest{
		Sources:  *r.sources(append(packageSpecs, buildPackageSpecs...)),
		Pipeline: *pipeline,
	}, nil
}

func (r *Fedora31) Runner() string {
	return "org.osbuild.fedora31"
}

func (r *Fedora31) buildPipeline(repos []rpmmd.RepoConfig, arch arch, buildPackageSpecs []rpmmd.PackageSpec) *osbuild.Pipeline {
	p := &osbuild.Pipeline{}
	p.AddStage(osbuild.NewRPMStage(r.rpmStageOptions(arch, repos, buildPackageSpecs)))
	return p
}

func (r *Fedora31) rpmStageOptions(arch arch, repos []rpmmd.RepoConfig, specs []rpmmd.PackageSpec) *osbuild.RPMStageOptions {
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

func (r *Fedora31) userStageOptions(users []blueprint.UserCustomization) (*osbuild.UsersStageOptions, error) {
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

func (r *Fedora31) groupStageOptions(groups []blueprint.GroupCustomization) *osbuild.GroupsStageOptions {
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

func (r *Fedora31) firewallStageOptions(firewall *blueprint.FirewallCustomization) *osbuild.FirewallStageOptions {
	options := osbuild.FirewallStageOptions{
		Ports: firewall.Ports,
	}

	if firewall.Services != nil {
		options.EnabledServices = firewall.Services.Enabled
		options.DisabledServices = firewall.Services.Disabled
	}

	return &options
}

func (r *Fedora31) systemdStageOptions(enabledServices, disabledServices []string, s *blueprint.ServicesCustomization) *osbuild.SystemdStageOptions {
	if s != nil {
		enabledServices = append(enabledServices, s.Enabled...)
		disabledServices = append(disabledServices, s.Disabled...)
	}
	return &osbuild.SystemdStageOptions{
		EnabledServices:  enabledServices,
		DisabledServices: disabledServices,
	}
}

func (r *Fedora31) fsTabStageOptions(uefi bool) *osbuild.FSTabStageOptions {
	options := osbuild.FSTabStageOptions{}
	options.AddFilesystem("76a22bf4-f153-4541-b6c7-0332c0dfaeac", "ext4", "/", "defaults", 1, 1)
	if uefi {
		options.AddFilesystem("46BB-8120", "vfat", "/boot/efi", "umask=0077,shortname=winnt", 0, 2)
	}
	return &options
}

func (r *Fedora31) grub2StageOptions(kernelOptions string, kernel *blueprint.KernelCustomization, uefi bool) *osbuild.GRUB2StageOptions {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}

	if kernel != nil {
		kernelOptions += " " + kernel.Append
	}

	var uefiOptions *osbuild.GRUB2UEFI
	if uefi {
		uefiOptions = &osbuild.GRUB2UEFI{
			Vendor: "fedora",
		}
	}

	return &osbuild.GRUB2StageOptions{
		RootFilesystemUUID: id,
		KernelOptions:      kernelOptions,
		Legacy:             !uefi,
		UEFI:               uefiOptions,
	}
}

func (r *Fedora31) selinuxStageOptions() *osbuild.SELinuxStageOptions {
	return &osbuild.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
}

func (r *Fedora31) qemuAssembler(format string, filename string, uefi bool, size uint64) *osbuild.Assembler {
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
			Size:     size,
			PTUUID:   "0x14fc63d2",
			PTType:   "mbr",
			Partitions: []osbuild.QEMUPartition{
				{
					Start:    2048,
					Bootable: true,
					Filesystem: osbuild.QEMUFilesystem{
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

func (r *Fedora31) tarAssembler(filename, compression string) *osbuild.Assembler {
	return osbuild.NewTarAssembler(
		&osbuild.TarAssemblerOptions{
			Filename:    filename,
			Compression: compression,
		})
}

func (r *Fedora31) rawFSAssembler(filename string, size uint64) *osbuild.Assembler {
	id, err := uuid.Parse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	if err != nil {
		panic("invalid UUID")
	}
	return osbuild.NewRawFSAssembler(
		&osbuild.RawFSAssemblerOptions{
			Filename:           filename,
			RootFilesystemUUDI: id,
			Size:               size,
		})
}
