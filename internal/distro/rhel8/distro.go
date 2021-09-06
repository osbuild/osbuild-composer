package rhel8

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

const defaultName = "rhel-8"
const releaseVersion = "8"
const modulePlatformID = "platform:el8"
const ostreeRef = "rhel/8/%s/edge"

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
	defaultTarget    string
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
			defaultTarget:    it.defaultTarget,
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

	// create a slice for storing
	// invalid mountpoints in order to return
	// a detailed message
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
	p.SetBuild(t.buildPipeline(repos, *t.arch, buildPackageSpecs), "org.osbuild.rhel82")

	if t.arch.Name() == "s390x" {
		p.AddStage(osbuild.NewKernelCmdlineStage(&osbuild.KernelCmdlineStageOptions{
			RootFsUUID: "0bd700f8-090f-4556-b797-b340297ea1bd",
			KernelOpts: "net.ifnames=0 crashkernel=auto",
		}))
	}

	p.AddStage(osbuild.NewRPMStage(t.rpmStageOptions(*t.arch, repos, packageSpecs)))
	p.AddStage(osbuild.NewFixBLSStage())

	if t.bootable {
		p.AddStage(osbuild.NewFSTabStage(t.fsTabStageOptions(t.arch.uefi)))
		if t.arch.Name() != "s390x" {
			p.AddStage(osbuild.NewGRUB2Stage(t.grub2StageOptions(t.kernelOptions, c.GetKernel(), t.arch.uefi)))
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

	if services := c.GetServices(); services != nil || t.enabledServices != nil || t.disabledServices != nil || t.defaultTarget != "" {
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
			fmt.Sprintf("/usr/sbin/subscription-manager register --org=%s --activationkey=%s --serverurl %s --baseurl %s", options.Subscription.Organization, options.Subscription.ActivationKey, options.Subscription.ServerUrl, options.Subscription.BaseUrl),
		}
		if options.Subscription.Insights {
			commands = append(commands, "/usr/bin/insights-client --register")
		}

		p.AddStage(osbuild.NewFirstBootStage(&osbuild.FirstBootStageOptions{
			Commands:       commands,
			WaitForNetwork: true,
		},
		))
	} else {
		// RHSM DNF plugins should be by default disabled on RHEL Guest KVM images
		if t.Name() == "qcow2" {
			p.AddStage(osbuild.NewRHSMStage(&osbuild.RHSMStageOptions{
				DnfPlugins: &osbuild.RHSMStageOptionsDnfPlugins{
					ProductID: &osbuild.RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
					SubscriptionManager: &osbuild.RHSMStageOptionsDnfPlugin{
						Enabled: false,
					},
				},
			}))
		}
	}

	p.Assembler = t.assembler(t.arch.uefi, options, t.arch)

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

func (t *imageType) fsTabStageOptions(uefi bool) *osbuild.FSTabStageOptions {
	options := osbuild.FSTabStageOptions{}
	options.AddFilesystem("0bd700f8-090f-4556-b797-b340297ea1bd", "xfs", "/", "defaults", 0, 0)
	if uefi {
		options.AddFilesystem("46BB-8120", "vfat", "/boot/efi", "umask=0077,shortname=winnt", 0, 2)
	}
	return &options
}

func (t *imageType) grub2StageOptions(kernelOptions string, kernel *blueprint.KernelCustomization, uefi bool) *osbuild.GRUB2StageOptions {
	id := uuid.MustParse("0bd700f8-090f-4556-b797-b340297ea1bd")

	if kernel != nil && kernel.Append != "" {
		kernelOptions += " " + kernel.Append
	}

	var uefiOptions *osbuild.GRUB2UEFI
	if uefi {
		uefiOptions = &osbuild.GRUB2UEFI{
			Vendor: "redhat",
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

func qemuAssembler(format string, filename string, uefi bool, imageOptions distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
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
					Filesystem: &osbuild.QEMUFilesystem{
						Type:       "vfat",
						UUID:       "46BB-8120",
						Label:      "EFI System Partition",
						Mountpoint: "/boot/efi",
					},
				},
				{
					Start: 976896,
					Filesystem: &osbuild.QEMUFilesystem{
						Type:       "xfs",
						UUID:       "0bd700f8-090f-4556-b797-b340297ea1bd",
						Mountpoint: "/",
					},
				},
			},
		}
	} else {
		if arch.Name() == "ppc64le" {
			options = osbuild.QEMUAssemblerOptions{
				Bootloader: &osbuild.QEMUBootloader{
					Type:     "grub2",
					Platform: "powerpc-ieee1275",
				},
				Format:   format,
				Filename: filename,
				Size:     imageOptions.Size,
				PTUUID:   "0x14fc63d2",
				PTType:   "dos",
				Partitions: []osbuild.QEMUPartition{
					{
						Size:     8192,
						Type:     "41",
						Bootable: true,
					},
					{
						Start: 10240,
						Filesystem: &osbuild.QEMUFilesystem{
							Type:       "xfs",
							UUID:       "0bd700f8-090f-4556-b797-b340297ea1bd",
							Mountpoint: "/",
						},
					},
				},
			}
		} else if arch.Name() == "s390x" {
			options = osbuild.QEMUAssemblerOptions{
				Bootloader: &osbuild.QEMUBootloader{
					Type: "zipl",
				},
				Format:   format,
				Filename: filename,
				Size:     imageOptions.Size,
				PTUUID:   "0x14fc63d2",
				PTType:   "dos",
				Partitions: []osbuild.QEMUPartition{
					{
						Start:    2048,
						Bootable: true,
						Filesystem: &osbuild.QEMUFilesystem{
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
				Size:     imageOptions.Size,
				PTUUID:   "0x14fc63d2",
				PTType:   "mbr",
				Partitions: []osbuild.QEMUPartition{
					{
						Start:    2048,
						Bootable: true,
						Filesystem: &osbuild.QEMUFilesystem{
							Type:       "xfs",
							UUID:       "0bd700f8-090f-4556-b797-b340297ea1bd",
							Mountpoint: "/",
						},
					},
				},
			}
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

func NewHostDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	return newDistro(name, modulePlatformID, ostreeRef)
}

func newDistro(name, modulePlatformID, ostreeRef string) distro.Distro {
	const GigaByte = 1024 * 1024 * 1024

	edgeImgTypeX86_64 := imageType{
		name:     "rhel-edge-commit",
		filename: "commit.tar",
		mimeType: "application/x-tar",
		packages: []string{
			"redhat-release", // TODO: is this correct for Edge?
			"glibc", "glibc-minimal-langpack", "nss-altfiles",
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
			"audit", "rng-tools",
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
			"subscription-manager",
		},
		enabledServices: []string{
			"NetworkManager.service", "firewalld.service", "rngd.service", "sshd.service",
			"greenboot-grub2-set-counter", "greenboot-grub2-set-success", "greenboot-healthcheck",
			"greenboot-rpm-ostree-grub2-check-fallback", "greenboot-status", "greenboot-task-runner",
			"redboot-auto-reboot", "redboot-task-runner",
		},
		rpmOstree: true,
		assembler: func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
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
			"audit", "rng-tools",
			"podman", "container-selinux", "skopeo", "criu",
			"slirp4netns", "fuse-overlayfs",
			"clevis", "clevis-dracut", "clevis-luks",
			"greenboot", "greenboot-grub2", "greenboot-rpm-ostree-grub2", "greenboot-reboot", "greenboot-status",
			// aarch64 specific
			"grub2-efi-aa64", "efibootmgr", "shim-aa64",
			"iwl7260-firmware",
		},
		excludedPackages: []string{
			"subscription-manager",
		},
		enabledServices: []string{
			"NetworkManager.service", "firewalld.service", "rngd.service", "sshd.service",
			"greenboot-grub2-set-counter", "greenboot-grub2-set-success", "greenboot-healthcheck",
			"greenboot-rpm-ostree-grub2-check-fallback", "greenboot-status", "greenboot-task-runner",
			"redboot-auto-reboot", "redboot-task-runner",
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
			"checkpolicy",
			"chrony",
			"cloud-init",
			"cloud-init",
			"cloud-utils-growpart",
			"@core",
			"dhcp-client",
			"gdisk",
			"insights-client",
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
		kernelOptions: "ro console=ttyS0,115200n8 console=tty0 net.ifnames=0 rd.blacklist=nouveau nvme_core.io_timeout=4294967295 crashkernel=auto",
		bootable:      true,
		defaultSize:   6 * GigaByte,
		assembler: func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler("raw", "image.raw", uefi, options, arch)
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
		kernelOptions: "console=ttyS0 console=ttyS0,115200n8 no_timer_check crashkernel=auto net.ifnames=0",
		bootable:      true,
		defaultSize:   4 * GigaByte,
		assembler: func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler("qcow2", "disk.qcow2", uefi, options, arch)
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
			"selinux-policy-targeted",
			"cloud-init",
			"qemu-guest-agent",
			"spice-vdagent",
		},
		excludedPackages: []string{
			"dracut-config-rescue",
		},
		kernelOptions: "ro net.ifnames=0",
		bootable:      true,
		defaultSize:   4 * GigaByte,
		assembler: func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler("qcow2", "disk.qcow2", uefi, options, arch)
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
		bootable:      false,
		kernelOptions: "ro net.ifnames=0",
		assembler: func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
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
		kernelOptions: "ro biosdevname=0 rootdelay=300 console=ttyS0 earlyprintk=ttyS0 net.ifnames=0",
		bootable:      true,
		defaultSize:   4 * GigaByte,
		assembler: func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler("vpc", "disk.vhd", uefi, options, arch)
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
		kernelOptions: "ro net.ifnames=0",
		bootable:      true,
		defaultSize:   4 * GigaByte,
		assembler: func(uefi bool, options distro.ImageOptions, arch distro.Arch) *osbuild.Assembler {
			return qemuAssembler("vmdk", "disk.vmdk", uefi, options, arch)
		},
	}

	r := distribution{
		buildPackages: []string{
			"dnf",
			"dosfstools",
			"e2fsprogs",
			"glibc",
			"policycoreutils",
			"python36",
			"python3-iniparse", // dependency of org.osbuild.rhsm stage
			"qemu-img",
			"selinux-policy-targeted",
			"systemd",
			"tar",
			"xfsprogs",
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
