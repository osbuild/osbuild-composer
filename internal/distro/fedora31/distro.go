package fedora31

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

const name = "fedora-31"
const modulePlatformID = "platform:f31"

type Fedora31 struct {
	arches        map[string]arch
	buildPackages []string
}

type imageType struct {
	arch             *arch
	name             string
	filename         string
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

type arch struct {
	distro             *Fedora31
	name               string
	bootloaderPackages []string
	buildPackages      []string
	uefi               bool
	imageTypes         map[string]imageType
}

func (a *arch) Distro() distro.Distro {
	return a.distro
}

func (t *imageType) Arch() distro.Arch {
	return t.arch
}

func (d *Fedora31) ListArches() []string {
	archs := make([]string, 0, len(d.arches))
	for name := range d.arches {
		archs = append(archs, name)
	}
	sort.Strings(archs)
	return archs
}

func (d *Fedora31) GetArch(arch string) (distro.Arch, error) {
	a, exists := d.arches[arch]
	if !exists {
		return nil, errors.New("invalid architecture: " + arch)
	}

	return &a, nil
}

func (d *Fedora31) setArches(arches ...arch) {
	d.arches = map[string]arch{}
	for _, a := range arches {
		d.arches[a.name] = arch{
			distro:             d,
			name:               a.name,
			bootloaderPackages: a.bootloaderPackages,
			buildPackages:      a.buildPackages,
			uefi:               a.uefi,
			imageTypes:         a.imageTypes,
		}
	}
}

func (a *arch) Name() string {
	return a.name
}

func (a *arch) ListImageTypes() []string {
	formats := make([]string, 0, len(a.imageTypes))
	for name := range a.imageTypes {
		formats = append(formats, name)
	}
	sort.Strings(formats)
	return formats
}

func (a *arch) GetImageType(imageType string) (distro.ImageType, error) {
	t, exists := a.imageTypes[imageType]
	if !exists {
		return nil, errors.New("invalid image type: " + imageType)
	}

	return &t, nil
}

func (a *arch) setImageTypes(imageTypes ...imageType) {
	a.imageTypes = map[string]imageType{}
	for _, it := range imageTypes {
		a.imageTypes[it.name] = imageType{
			arch:             a,
			name:             it.name,
			filename:         it.filename,
			mimeType:         it.mimeType,
			packages:         it.packages,
			excludedPackages: it.excludedPackages,
			enabledServices:  it.enabledServices,
			disabledServices: it.disabledServices,
			kernelOptions:    it.kernelOptions,
			bootable:         it.bootable,
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

func (t *imageType) BasePackages() ([]string, []string) {
	packages := t.packages
	if t.bootable {
		packages = append(packages, t.arch.bootloaderPackages...)
	}

	return packages, t.excludedPackages
}

func (t *imageType) BuildPackages() []string {
	return append(t.arch.distro.buildPackages, t.arch.buildPackages...)
}

func (t *imageType) Manifest(c *blueprint.Customizations,
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

func New() *Fedora31 {
	const GigaByte = 1024 * 1024 * 1024

	amiImgType := imageType{
		name:     "ami",
		filename: "image.vhdx",
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
			return qemuAssembler("vhdx", "image.vhdx", uefi, size)
		},
	}

	qcow2ImageType := imageType{
		name:     "qcow2",
		filename: "disk.qcow2",
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
			return qemuAssembler("qcow2", "disk.qcow2", uefi, size)
		},
	}

	openstackImgType := imageType{
		name:     "openstack",
		filename: "disk.qcow2",
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
			return qemuAssembler("qcow2", "disk.qcow2", uefi, size)
		},
	}

	vhdImgType := imageType{
		name:     "vhd",
		filename: "disk.vhd",
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
			return qemuAssembler("vpc", "disk.vhd", uefi, size)
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
		},
		kernelOptions: "ro biosdevname=0 net.ifnames=0",
		bootable:      true,
		defaultSize:   2 * GigaByte,
		assembler: func(uefi bool, size uint64) *osbuild.Assembler {
			return qemuAssembler("vmdk", "disk.vmdk", uefi, size)
		},
	}

	r := Fedora31{
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
	}
	x8664 := arch{
		distro: &r,
		name:   "x86_64",
		bootloaderPackages: []string{
			"dracut-config-generic",
			"grub2-pc",
		},
		buildPackages: []string{
			"grub2-pc",
		},
	}
	x8664.setImageTypes(
		amiImgType,
		qcow2ImageType,
		openstackImgType,
		vhdImgType,
		vmdkImgType,
	)

	aarch64 := arch{
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
		qcow2ImageType,
		openstackImgType,
	)

	r.setArches(x8664, aarch64)

	return &r
}

func (r *Fedora31) Name() string {
	return name
}

func (r *Fedora31) ModulePlatformID() string {
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
		files.URLs[pkg.Checksum] = fileSource
	}
	return &osbuild.Sources{
		"org.osbuild.files": files,
	}
}

func (t *imageType) pipeline(c *blueprint.Customizations, repos []rpmmd.RepoConfig, packageSpecs, buildPackageSpecs []rpmmd.PackageSpec, size uint64) (*osbuild.Pipeline, error) {
	p := &osbuild.Pipeline{}
	p.SetBuild(t.buildPipeline(repos, *t.arch, buildPackageSpecs), "org.osbuild.fedora31")

	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(repos, packageSpecs)))
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

	if t.bootable {
		p.AddStage(osbuild.NewFSTabStage(t.fsTabStageOptions(t.arch.uefi)))
		p.AddStage(osbuild.NewGRUB2Stage(t.grub2StageOptions(t.kernelOptions, c.GetKernel(), t.arch.uefi)))
	}

	if services := c.GetServices(); services != nil || t.enabledServices != nil {
		p.AddStage(osbuild.NewSystemdStage(t.systemdStageOptions(t.enabledServices, t.disabledServices, services)))
	}

	if firewall := c.GetFirewall(); firewall != nil {
		p.AddStage(osbuild.NewFirewallStage(t.firewallStageOptions(firewall)))
	}

	p.AddStage(osbuild.NewSELinuxStage(t.selinuxStageOptions()))

	p.Assembler = t.assembler(t.arch.uefi, size)

	return p, nil
}

func (r *imageType) buildPipeline(repos []rpmmd.RepoConfig, arch arch, buildPackageSpecs []rpmmd.PackageSpec) *osbuild.Pipeline {
	p := &osbuild.Pipeline{}
	p.AddStage(osbuild.NewRPMStage(r.rpmStageOptions(repos, buildPackageSpecs)))
	return p
}

func (r *imageType) rpmStageOptions(repos []rpmmd.RepoConfig, specs []rpmmd.PackageSpec) *osbuild.RPMStageOptions {
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

func (r *imageType) userStageOptions(users []blueprint.UserCustomization) (*osbuild.UsersStageOptions, error) {
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

func (r *imageType) groupStageOptions(groups []blueprint.GroupCustomization) *osbuild.GroupsStageOptions {
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

func (r *imageType) firewallStageOptions(firewall *blueprint.FirewallCustomization) *osbuild.FirewallStageOptions {
	options := osbuild.FirewallStageOptions{
		Ports: firewall.Ports,
	}

	if firewall.Services != nil {
		options.EnabledServices = firewall.Services.Enabled
		options.DisabledServices = firewall.Services.Disabled
	}

	return &options
}

func (r *imageType) systemdStageOptions(enabledServices, disabledServices []string, s *blueprint.ServicesCustomization) *osbuild.SystemdStageOptions {
	if s != nil {
		enabledServices = append(enabledServices, s.Enabled...)
		disabledServices = append(disabledServices, s.Disabled...)
	}
	return &osbuild.SystemdStageOptions{
		EnabledServices:  enabledServices,
		DisabledServices: disabledServices,
	}
}

func (r *imageType) fsTabStageOptions(uefi bool) *osbuild.FSTabStageOptions {
	options := osbuild.FSTabStageOptions{}
	options.AddFilesystem("76a22bf4-f153-4541-b6c7-0332c0dfaeac", "ext4", "/", "defaults", 1, 1)
	if uefi {
		options.AddFilesystem("46BB-8120", "vfat", "/boot/efi", "umask=0077,shortname=winnt", 0, 2)
	}
	return &options
}

func (r *imageType) grub2StageOptions(kernelOptions string, kernel *blueprint.KernelCustomization, uefi bool) *osbuild.GRUB2StageOptions {
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

func (r *imageType) selinuxStageOptions() *osbuild.SELinuxStageOptions {
	return &osbuild.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
}

func qemuAssembler(format string, filename string, uefi bool, size uint64) *osbuild.Assembler {
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
					UUID:  "02C1E068-1D2F-4DA3-91FD-8DD76A955C9D",
					Filesystem: osbuild.QEMUFilesystem{
						Type:       "vfat",
						UUID:       "46BB-8120",
						Label:      "EFI System Partition",
						Mountpoint: "/boot/efi",
					},
				},
				{
					Start: 976896,
					UUID:  "8D760010-FAAE-46D1-9E5B-4A2EAC5030CD",
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
