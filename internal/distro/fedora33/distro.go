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

const defaultName = "fedora-33"
const releaseVersion = "33"
const modulePlatformID = "platform:f33"
const ostreeRef = "fedora/33/%s/iot"

const f34Name = "fedora-34"
const f34modulePlatformID = "platform:f34"
const f34ostreeRef = "fedora/34/%s/iot"

const f35Name = "fedora-35"
const f35modulePlatformID = "platform:f35"
const f35ostreeRef = "fedora/35/%s/iot"

const f36Name = "fedora-36"
const f36modulePlatformID = "platform:f36"
const f36ostreeRef = "fedora/36/%s/iot"

type distribution struct {
	name             string
	modulePlatformID string
	ostreeRef        string
	arches           map[string]architecture
	buildPackages    []string
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
	arch             *architecture
	name             string
	filename         string
	mimeType         string
	packages         []string
	excludedPackages []string
	enabledServices  []string
	disabledServices []string
	kernelOptions    string
	bootable         bool
	rpmOstree        bool
	defaultSize      uint64
	assembler        func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler
}

func removePackage(packages []string, packageToRemove string) []string {
	for i, pkg := range packages {
		if pkg == packageToRemove {
			// override the package with the last one from the list
			packages[i] = packages[len(packages)-1]

			// drop the last package from the slice
			return packages[:len(packages)-1]
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
		return fmt.Sprintf(ostreeRef, t.arch.name)
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

func (t *imageType) Packages(bp blueprint.Blueprint) ([]string, []string) {
	packages := append(t.packages, bp.GetPackages()...)
	timezone, _ := bp.Customizations.GetTimezoneSettings()
	if timezone != nil {
		packages = append(packages, "chrony")
	}
	if t.bootable {
		packages = append(packages, t.arch.bootloaderPackages...)
	}

	// copy the list of excluded packages from the image type
	// and subtract any packages found in the blueprint (this
	// will not handle the issue with dependencies present in
	// the list of excluded packages, but it will create a
	// possibility of a workaround at least)
	excludedPackages := append([]string(nil), t.excludedPackages...)
	for _, pkg := range bp.GetPackages() {
		// removePackage is fine if the package doesn't exist
		excludedPackages = removePackage(excludedPackages, pkg)
	}

	return packages, excludedPackages
}

func (t *imageType) BuildPackages() []string {
	packages := append(t.arch.distro.buildPackages, t.arch.buildPackages...)
	if t.rpmOstree {
		packages = append(packages, "rpm-ostree")
	}
	return packages
}

func (t *imageType) PackageSets(bp blueprint.Blueprint) map[string]rpmmd.PackageSet {
	includePackages, excludePackages := t.Packages(bp)
	return map[string]rpmmd.PackageSet{
		"packages": {
			Include: includePackages,
			Exclude: excludePackages,
		},
		"build-packages": {
			Include: t.BuildPackages(),
		},
	}
}

func (t *imageType) BuildPipelines() []string {
	return distro.BuildPipelinesFallback()
}

func (t *imageType) PayloadPipelines() []string {
	return distro.PayloadPipelinesFallback()
}

func (t *imageType) Exports() []string {
	return distro.ExportsFallback()
}

func (t *imageType) Manifest(c *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSpecSets map[string][]rpmmd.PackageSpec,
	seed int64) (distro.Manifest, error) {
	pipeline, err := t.pipeline(c, options, repos, packageSpecSets["packages"], packageSpecSets["build-packages"])
	if err != nil {
		return distro.Manifest{}, err
	}

	return json.Marshal(
		osbuild.Manifest{
			Sources:  *sources(append(packageSpecSets["packages"], packageSpecSets["build-packages"]...)),
			Pipeline: *pipeline,
		},
	)
}

func (d *distribution) Name() string {
	return d.name
}

func (d *distribution) Releasever() string {
	return releaseVersion
}

func (d *distribution) ModulePlatformID() string {
	return d.modulePlatformID
}

func (d *distribution) OSTreeRef() string {
	return d.ostreeRef
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
		p.AddStage(osbuild.NewFSTabStage(t.fsTabStageOptions(t.arch.uefi)))
		p.AddStage(osbuild.NewGRUB2Stage(t.grub2StageOptions(t.kernelOptions, c.GetKernel(), t.arch.uefi)))
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

	p.Assembler = t.assembler(t.arch.uefi, options, t.arch)

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

func (t *imageType) fsTabStageOptions(uefi bool) *osbuild.FSTabStageOptions {
	options := osbuild.FSTabStageOptions{}
	options.AddFilesystem("76a22bf4-f153-4541-b6c7-0332c0dfaeac", "ext4", "/", "defaults", 1, 1)
	if uefi {
		options.AddFilesystem("46BB-8120", "vfat", "/boot/efi", "umask=0077,shortname=winnt", 0, 2)
	}
	return &options
}

func (t *imageType) grub2StageOptions(kernelOptions string, kernel *blueprint.KernelCustomization, uefi bool) *osbuild.GRUB2StageOptions {
	id := uuid.MustParse("76a22bf4-f153-4541-b6c7-0332c0dfaeac")

	if kernel != nil && kernel.Append != "" {
		kernelOptions += " " + kernel.Append
	}

	var uefiOptions *osbuild.GRUB2UEFI
	if uefi {
		uefiOptions = &osbuild.GRUB2UEFI{
			Vendor: "fedora",
		}
	}

	var legacy string
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

func qemuAssembler(format string, filename string, uefi bool, imageOptions distro.ImageOptions) *osbuild.Assembler {
	var options osbuild.QEMUAssemblerOptions
	if uefi {
		options = osbuild.QEMUAssemblerOptions{
			Format:   format,
			Filename: filename,
			Size:     imageOptions.Size,
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
						Label:      "EFI System Partition",
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
			Size:     imageOptions.Size,
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
func New() distro.Distro {
	return newDistro(defaultName, modulePlatformID, ostreeRef)
}

func NewF34() distro.Distro {
	return newDistro(f34Name, f34modulePlatformID, f34ostreeRef)
}

func NewF35() distro.Distro {
	return newDistro(f35Name, f35modulePlatformID, f35ostreeRef)
}

func NewF36() distro.Distro {
	return newDistro(f36Name, f36modulePlatformID, f36ostreeRef)
}

func NewHostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	return newDistro(name, modulePlatformID, ostreeRef)
}

func newDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	const GigaByte = 1024 * 1024 * 1024

	iotImgType := imageType{
		name:     "fedora-iot-commit",
		filename: "commit.tar",
		mimeType: "application/x-tar",
		packages: []string{
			"fedora-release-iot",
			"glibc", "glibc-minimal-langpack", "nss-altfiles",
			"sssd-client", "libsss_sudo", "shadow-utils",
			"dracut-config-generic", "dracut-network",
			"rpm-ostree", "polkit", "lvm2",
			"cryptsetup", "pinentry",
			"keyutils", "cracklib-dicts",
			"e2fsprogs", "xfsprogs", "dosfstools",
			"gnupg2",
			"basesystem", "python3", "bash",
			"xz", "gzip",
			"coreutils", "which", "curl",
			"firewalld", "iptables",
			"NetworkManager", "NetworkManager-wifi", "NetworkManager-wwan",
			"wpa_supplicant", "iwd", "tpm2-pkcs11",
			"dnsmasq", "traceroute",
			"hostname", "iproute", "iputils",
			"openssh-clients", "openssh-server", "passwd",
			"policycoreutils", "procps-ng", "rootfiles", "rpm",
			"selinux-policy-targeted", "setup", "shadow-utils",
			"sudo", "systemd", "util-linux", "vim-minimal",
			"less", "tar",
			"fwupd", "usbguard",
			"greenboot", "greenboot-grub2", "greenboot-rpm-ostree-grub2", "greenboot-reboot", "greenboot-status",
			"ignition", "zezere-ignition",
			"rsync", "attr",
			"ima-evm-utils",
			"bash-completion",
			"tmux", "screen",
			"policycoreutils-python-utils",
			"setools-console",
			"audit", "rng-tools", "chrony",
			"bluez", "bluez-libs", "bluez-mesh",
			"kernel-tools", "libgpiod-utils",
			"podman", "container-selinux", "skopeo", "criu",
			"slirp4netns", "fuse-overlayfs",
			"clevis", "clevis-dracut", "clevis-luks", "clevis-pin-tpm2",
			"parsec", "dbus-parsec",
			// x86 specific
			"grub2", "grub2-efi-x64", "efibootmgr", "shim-x64", "microcode_ctl",
			"iwl1000-firmware", "iwl100-firmware", "iwl105-firmware", "iwl135-firmware",
			"iwl2000-firmware", "iwl2030-firmware", "iwl3160-firmware", "iwl5000-firmware",
			"iwl5150-firmware", "iwl6000-firmware", "iwl6050-firmware", "iwl7260-firmware",
		},
		enabledServices: []string{
			"NetworkManager.service", "firewalld.service", "rngd.service", "sshd.service",
			"zezere_ignition.timer", "zezere_ignition_banner.service",
			"greenboot-grub2-set-counter", "greenboot-grub2-set-success", "greenboot-healthcheck", "greenboot-rpm-ostree-grub2-check-fallback",
			"greenboot-status", "greenboot-task-runner", "redboot-auto-reboot", "redboot-task-runner",
			"parsec", "dbus-parsec",
		},
		rpmOstree: true,
		assembler: func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return ostreeCommitAssembler(options, arch)
		},
	}

	amiImgType := imageType{
		name:     "ami",
		filename: "image.raw",
		mimeType: "application/octet-stream",
		packages: []string{
			"@Core",
			"chrony",
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
			"geolite2-city",
			"geolite2-country",
			"zram-generator-defaults",
		},
		enabledServices: []string{
			"cloud-init.service",
		},
		kernelOptions: "ro no_timer_check console=ttyS0,115200n8 console=tty1 biosdevname=0 net.ifnames=0 console=ttyS0,115200",
		bootable:      true,
		defaultSize:   6 * GigaByte,
		assembler: func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler("raw", "image.raw", uefi, options)
		},
	}

	qcow2ImageType := imageType{
		name:     "qcow2",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packages: []string{
			"@Fedora Cloud Server",
			"chrony",
			"systemd-udev",
			"selinux-policy-targeted",
			"langpacks-en",
		},
		excludedPackages: []string{
			"dracut-config-rescue",
			"etables",
			"firewalld",
			"geolite2-city",
			"geolite2-country",
			"gobject-introspection",
			"plymouth",
			"zram-generator-defaults",
		},
		enabledServices: []string{
			"cloud-init.service",
			"cloud-config.service",
			"cloud-final.service",
			"cloud-init-local.service",
		},
		bootable:    true,
		defaultSize: 2 * GigaByte,
		assembler: func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler("qcow2", "disk.qcow2", uefi, options)
		},
	}

	openstackImgType := imageType{
		name:     "openstack",
		filename: "disk.qcow2",
		mimeType: "application/x-qemu-disk",
		packages: []string{
			"@Core",
			"chrony",
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
			"geolite2-city",
			"geolite2-country",
			"zram-generator-defaults",
		},
		enabledServices: []string{
			"cloud-init.service",
			"cloud-config.service",
			"cloud-final.service",
			"cloud-init-local.service",
		},
		bootable:    true,
		defaultSize: 2 * GigaByte,
		assembler: func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler("qcow2", "disk.qcow2", uefi, options)
		},
	}

	vhdImgType := imageType{
		name:     "vhd",
		filename: "disk.vhd",
		mimeType: "application/x-vhd",
		packages: []string{
			"@Core",
			"chrony",
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
			"geolite2-city",
			"geolite2-country",
			"zram-generator-defaults",
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
		assembler: func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler("vpc", "disk.vhd", uefi, options)
		},
	}

	vmdkImgType := imageType{
		name:     "vmdk",
		filename: "disk.vmdk",
		mimeType: "application/x-vmdk",
		packages: []string{
			"@Fedora Cloud Server",
			"chrony",
			"systemd-udev",
			"selinux-policy-targeted",
			"langpacks-en",
		},
		excludedPackages: []string{
			"dracut-config-rescue",
			"etables",
			"firewalld",
			"geolite2-city",
			"geolite2-country",
			"gobject-introspection",
			"plymouth",
			"zram-generator-defaults",
		},
		enabledServices: []string{
			"cloud-init.service",
			"cloud-config.service",
			"cloud-final.service",
			"cloud-init-local.service",
		},
		bootable:    true,
		defaultSize: 2 * GigaByte,
		assembler: func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler("vmdk", "disk.vmdk", uefi, options)
		},
	}

	r := distribution{
		buildPackages: []string{
			"dnf",
			"dosfstools",
			"e2fsprogs",
			"policycoreutils",
			"qemu-img",
			"selinux-policy-targeted",
			"systemd",
			"tar",
			"xz",
		},
		name:             name,
		modulePlatformID: modulePlatformID,
		ostreeRef:        ostreeRef,
	}
	x8664 := architecture{
		distro: &r,
		name:   "x86_64",
		bootloaderPackages: []string{
			"dracut-config-generic",
			"grub2-pc",
		},
		buildPackages: []string{
			"grub2-pc",
		},
		legacy: "i386-pc",
	}
	x8664.setImageTypes(
		iotImgType,
		amiImgType,
		qcow2ImageType,
		openstackImgType,
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
		qcow2ImageType,
		openstackImgType,
	)

	r.setArches(x8664, aarch64)

	return &r
}
