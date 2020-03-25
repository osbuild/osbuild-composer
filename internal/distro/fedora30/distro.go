package fedora30

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

const name = "fedora-30"
const modulePlatformID = "platform:f30"

type Fedora30 struct {
	arches        map[string]arch
	imageTypes    map[string]imageType
	buildPackages []string
}

type arch struct {
	name               string
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
	kernelOptions    string
	bootable         bool
	defaultSize      uint64
	assembler        func(uefi bool, size uint64) *osbuild.Assembler
}

type fedora30Arch struct {
	name   string
	distro *Fedora30
	arch   *arch
}

type fedora30ImageType struct {
	name      string
	arch      *fedora30Arch
	imageType *imageType
}

func (d *Fedora30) ListArchs() []string {
	archs := make([]string, 0, len(d.arches))
	for name := range d.arches {
		archs = append(archs, name)
	}
	sort.Strings(archs)
	return archs
}

func (d *Fedora30) GetArch(arch string) (distro.Arch, error) {
	a, exists := d.arches[arch]
	if !exists {
		return nil, errors.New("invalid architecture: " + arch)
	}

	return &fedora30Arch{
		name:   arch,
		distro: d,
		arch:   &a,
	}, nil
}

func (a *fedora30Arch) Name() string {
	return a.name
}

func (a *fedora30Arch) ListImageTypes() []string {
	formats := make([]string, 0, len(a.distro.imageTypes))
	for name := range a.distro.imageTypes {
		formats = append(formats, name)
	}
	sort.Strings(formats)
	return formats
}

func (a *fedora30Arch) GetImageType(imageType string) (distro.ImageType, error) {
	t, exists := a.distro.imageTypes[imageType]
	if !exists {
		return nil, errors.New("invalid image type: " + imageType)
	}

	return &fedora30ImageType{
		name:      imageType,
		arch:      a,
		imageType: &t,
	}, nil
}

func (t *fedora30ImageType) Name() string {
	return t.name
}

func (t *fedora30ImageType) Filename() string {
	return t.imageType.name
}

func (t *fedora30ImageType) MIMEType() string {
	return t.imageType.mimeType
}

func (t *fedora30ImageType) Size(size uint64) uint64 {
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

func (t *fedora30ImageType) BasePackages() ([]string, []string) {
	packages := t.imageType.packages
	if t.imageType.bootable {
		packages = append(packages, t.arch.arch.bootloaderPackages...)
	}

	return packages, t.imageType.excludedPackages
}

func (t *fedora30ImageType) BuildPackages() []string {
	return append(t.arch.distro.buildPackages, t.arch.arch.buildPackages...)
}

func (t *fedora30ImageType) Manifest(c *blueprint.Customizations,
	repos []rpmmd.RepoConfig,
	packageSpecs,
	buildPackageSpecs []rpmmd.PackageSpec,
	size uint64) (*osbuild.Manifest, error) {
	pipeline, err := t.pipeline(c, repos, packageSpecs, buildPackageSpecs, size)
	if err != nil {
		return nil, err
	}

	return &osbuild.Manifest{
		Sources:  *sources(append(packageSpecs, buildPackageSpecs...)),
		Pipeline: *pipeline,
	}, nil
}

func New() *Fedora30 {
	const GigaByte = 1024 * 1024 * 1024

	r := Fedora30{
		imageTypes: map[string]imageType{},
		buildPackages: []string{
			"dnf",
			"dosfstools",
			"e2fsprogs",
			"policycoreutils",
			"qemu-img",
			"systemd",
			"tar",
			"xz",
		},
		arches: map[string]arch{
			"x86_64": arch{
				name: "x86_64",
				bootloaderPackages: []string{
					"grub2-pc",
				},
				buildPackages: []string{
					"grub2-pc",
				},
			},
			"aarch64": arch{
				name: "aarch64",
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
		name:     "image.raw.xz",
		mimeType: "application/octet-stream",
		packages: []string{
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
		excludedPackages: []string{
			"dracut-config-rescue",
		},
		enabledServices: []string{
			"cloud-init.service",
		},
		kernelOptions: "ro no_timer_check console=ttyS0,115200n8 console=tty1 biosdevname=0 net.ifnames=0 console=ttyS0,115200",
		bootable:      true,
		defaultSize:   6 * GigaByte,
		assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("raw.xz", "image.raw.xz", uefi, size)
		},
	}

	r.imageTypes["ext4-filesystem"] = imageType{
		name:     "filesystem.img",
		mimeType: "application/octet-stream",
		packages: []string{
			"policycoreutils",
			"selinux-policy-targeted",
			"kernel",
			"firewalld",
			"chrony",
			"langpacks-en",
		},
		excludedPackages: []string{
			"dracut-config-rescue",
		},
		kernelOptions: "ro biosdevname=0 net.ifnames=0",
		bootable:      false,
		defaultSize:   2 * GigaByte,
		assembler:     func(uefi bool, size uint64) *osbuild.Assembler { return r.rawFSAssembler("filesystem.img", size) },
	}

	r.imageTypes["partitioned-disk"] = imageType{
		name:     "disk.img",
		mimeType: "application/octet-stream",
		packages: []string{
			"@core",
			"chrony",
			"firewalld",
			"kernel",
			"langpacks-en",
			"selinux-policy-targeted",
		},
		excludedPackages: []string{
			"dracut-config-rescue",
		},
		kernelOptions: "ro biosdevname=0 net.ifnames=0",
		bootable:      true,
		defaultSize:   2 * GigaByte,
		assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("raw", "disk.img", uefi, size)
		},
	}

	r.imageTypes["qcow2"] = imageType{
		name:     "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packages: []string{
			"kernel-core",
			"@Fedora Cloud Server",
			"chrony",
			"polkit",
			"systemd-udev",
			"selinux-policy-targeted",
			"langpacks-en",
		},
		excludedPackages: []string{
			"dracut-config-rescue",
			"etables",
			"firewalld",
			"gobject-introspection",
			"plymouth",
		},
		kernelOptions: "ro biosdevname=0 net.ifnames=0",
		bootable:      true,
		defaultSize:   2 * GigaByte,
		assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("qcow2", "disk.qcow2", uefi, size)
		},
	}

	r.imageTypes["openstack"] = imageType{
		name:     "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packages: []string{
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
		excludedPackages: []string{
			"dracut-config-rescue",
		},
		kernelOptions: "ro biosdevname=0 net.ifnames=0",
		bootable:      true,
		defaultSize:   2 * GigaByte,
		assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("qcow2", "disk.qcow2", uefi, size)
		},
	}

	r.imageTypes["tar"] = imageType{
		name:     "root.tar.xz",
		mimeType: "application/x-tar",
		packages: []string{
			"policycoreutils",
			"selinux-policy-targeted",
			"kernel",
			"firewalld",
			"chrony",
			"langpacks-en",
		},
		excludedPackages: []string{
			"dracut-config-rescue",
		},
		kernelOptions: "ro biosdevname=0 net.ifnames=0",
		bootable:      false,
		defaultSize:   2 * GigaByte,
		assembler:     func(uefi bool, size uint64) *osbuild.Assembler { return r.tarAssembler("root.tar.xz", "xz") },
	}

	r.imageTypes["vhd"] = imageType{
		name:     "disk.vhd",
		mimeType: "application/x-vhd",
		packages: []string{
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
		excludedPackages: []string{
			"dracut-config-rescue",
		},
		enabledServices: []string{
			"sshd",
			"waagent", // needed to run in Azure
		},
		disabledServices: []string{
			"proc-sys-fs-binfmt_misc.mount",
			"loadmodules.service",
		},
		// These kernel parameters are required by Azure documentation
		kernelOptions: "ro biosdevname=0 rootdelay=300 console=ttyS0 earlyprintk=ttyS0 net.ifnames=0",
		bootable:      true,
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
		},
		kernelOptions: "ro biosdevname=0 net.ifnames=0",
		bootable:      true,
		defaultSize:   2 * GigaByte,
		assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return r.qemuAssembler("vmdk", "disk.vmdk", uefi, size)
		},
	}

	return &r
}

func (r *Fedora30) Name() string {
	return name
}

func (r *Fedora30) ModulePlatformID() string {
	return modulePlatformID
}

func (r *Fedora30) FilenameFromType(outputFormat string) (string, string, error) {
	if output, exists := r.imageTypes[outputFormat]; exists {
		return output.name, output.mimeType, nil
	}
	return "", "", errors.New("invalid output format: " + outputFormat)
}

func sources(packages []rpmmd.PackageSpec) *osbuild.Sources {
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

func (t *fedora30ImageType) pipeline(c *blueprint.Customizations, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, size uint64) (*osbuild.Pipeline, error) {
	p := &osbuild.Pipeline{}
	p.SetBuild(t.buildPipeline(repos, *t.arch.arch, buildPackageSpecs), "org.osbuild.fedora30")

	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(*t.arch.arch, repos, packageSpecs)))
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
		options, err := t.userStageOptions(users)
		if err != nil {
			return nil, err
		}
		p.AddStage(osbuild.NewUsersStage(options))
	}

	if groups := c.GetGroups(); len(groups) > 0 {
		p.AddStage(osbuild.NewGroupsStage(t.groupStageOptions(groups)))
	}

	if t.imageType.bootable {
		p.AddStage(osbuild.NewFSTabStage(t.fsTabStageOptions(t.arch.arch.uefi)))
	}
	p.AddStage(osbuild.NewGRUB2Stage(t.grub2StageOptions(t.imageType.kernelOptions, c.GetKernel(), t.arch.arch.uefi)))

	if services := c.GetServices(); services != nil || t.imageType.enabledServices != nil {
		p.AddStage(osbuild.NewSystemdStage(t.systemdStageOptions(t.imageType.enabledServices, t.imageType.disabledServices, services)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(t.firewallStageOptions(firewall)))
	}

	p.AddStage(osbuild.NewSELinuxStage(t.selinuxStageOptions()))

	p.Assembler = t.imageType.assembler(t.arch.arch.uefi, size)

	return p, nil
}

func (r *fedora30ImageType) buildPipeline(repos []rpmmd.RepoConfig, arch arch, packageSpecs []rpmmd.PackageSpec) *osbuild.Pipeline {
	p := &osbuild.Pipeline{}
	p.AddStage(osbuild.NewRPMStage(r.rpmStageOptions(arch, repos, packageSpecs)))
	return p
}

func (r *fedora30ImageType) rpmStageOptions(arch arch, repos []rpmmd.RepoConfig, specs []rpmmd.PackageSpec) *osbuild.RPMStageOptions {
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

func (r *fedora30ImageType) userStageOptions(users []blueprint.UserCustomization) (*osbuild.UsersStageOptions, error) {
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

func (r *fedora30ImageType) groupStageOptions(groups []blueprint.GroupCustomization) *osbuild.GroupsStageOptions {
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

func (r *fedora30ImageType) firewallStageOptions(firewall *blueprint.FirewallCustomization) *osbuild.FirewallStageOptions {
	options := osbuild.FirewallStageOptions{
		Ports: firewall.Ports,
	}

	if firewall.Services != nil {
		options.EnabledServices = firewall.Services.Enabled
		options.DisabledServices = firewall.Services.Disabled
	}

	return &options
}

func (r *fedora30ImageType) systemdStageOptions(enabledServices, disabledServices []string, s *blueprint.ServicesCustomization) *osbuild.SystemdStageOptions {
	if s != nil {
		enabledServices = append(enabledServices, s.Enabled...)
		disabledServices = append(disabledServices, s.Disabled...)
	}
	return &osbuild.SystemdStageOptions{
		EnabledServices:  enabledServices,
		DisabledServices: disabledServices,
	}
}

func (r *fedora30ImageType) fsTabStageOptions(uefi bool) *osbuild.FSTabStageOptions {
	options := osbuild.FSTabStageOptions{}
	options.AddFilesystem("76a22bf4-f153-4541-b6c7-0332c0dfaeac", "ext4", "/", "defaults", 1, 1)
	if uefi {
		options.AddFilesystem("46BB-8120", "vfat", "/boot/efi", "umask=0077,shortname=winnt", 0, 2)
	}
	return &options
}

func (r *fedora30ImageType) grub2StageOptions(kernelOptions string, kernel *blueprint.KernelCustomization, uefi bool) *osbuild.GRUB2StageOptions {
	id := uuid.MustParse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")

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

func (r *fedora30ImageType) selinuxStageOptions() *osbuild.SELinuxStageOptions {
	return &osbuild.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
}

func (r *Fedora30) qemuAssembler(format string, filename string, uefi bool, size uint64) *osbuild.Assembler {
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

func (r *Fedora30) tarAssembler(filename, compression string) *osbuild.Assembler {
	return osbuild.NewTarAssembler(
		&osbuild.TarAssemblerOptions{
			Filename:    filename,
			Compression: compression,
		})
}

func (r *Fedora30) rawFSAssembler(filename string, size uint64) *osbuild.Assembler {
	id := uuid.MustParse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")
	return osbuild.NewRawFSAssembler(
		&osbuild.RawFSAssemblerOptions{
			Filename:           filename,
			RootFilesystemUUID: id,
			Size:               size,
		})
}
