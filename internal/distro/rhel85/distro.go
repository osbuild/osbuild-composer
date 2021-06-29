package rhel85

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"sort"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const defaultName = "rhel-85"
const osVersion = "8.5"
const modulePlatformID = "platform:el8"
const ostreeRef = "rhel/8/%s/edge"

type distribution struct {
	name             string
	modulePlatformID string
	ostreeRef        string
	arches           map[string]distro.Arch
	packageSets      map[string]rpmmd.PackageSet
}

func (d *distribution) Name() string {
	return d.name
}

func (d *distribution) ModulePlatformID() string {
	return d.modulePlatformID
}

func (d *distribution) OSTreeRef() string {
	return d.ostreeRef
}

func (d *distribution) ListArches() []string {
	archNames := make([]string, 0, len(d.arches))
	for name := range d.arches {
		archNames = append(archNames, name)
	}
	sort.Strings(archNames)
	return archNames
}

func (d *distribution) GetArch(name string) (distro.Arch, error) {
	arch, exists := d.arches[name]
	if !exists {
		return nil, errors.New("invalid architecture: " + name)
	}
	return arch, nil
}

func (d *distribution) addArches(arches ...architecture) {
	if d.arches == nil {
		d.arches = map[string]distro.Arch{}
	}

	for _, a := range arches {
		d.arches[a.name] = &architecture{
			distro:     d,
			name:       a.name,
			imageTypes: a.imageTypes,
		}
	}
}

type architecture struct {
	distro      *distribution
	name        string
	imageTypes  map[string]distro.ImageType
	packageSets map[string]rpmmd.PackageSet
	legacy      string
	uefi        bool
}

func (a *architecture) Name() string {
	return a.name
}

func (a *architecture) ListImageTypes() []string {
	itNames := make([]string, 0, len(a.imageTypes))
	for name := range a.imageTypes {
		itNames = append(itNames, name)
	}
	sort.Strings(itNames)
	return itNames
}

func (a *architecture) GetImageType(name string) (distro.ImageType, error) {
	t, exists := a.imageTypes[name]
	if !exists {
		return nil, errors.New("invalid image type: " + name)
	}
	return t, nil
}

func (a *architecture) addImageTypes(imageTypes ...imageType) {
	if a.imageTypes == nil {
		a.imageTypes = map[string]distro.ImageType{}
	}
	for idx := range imageTypes {
		it := imageTypes[idx]
		it.arch = a
		a.imageTypes[it.name] = &it
	}
}

func (a *architecture) Distro() distro.Distro {
	return a.distro
}

type pipelinesFunc func(t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, repos []rpmmd.RepoConfig, packageSetSpecs map[string][]rpmmd.PackageSpec, rng *rand.Rand) ([]osbuild.Pipeline, error)

type imageType struct {
	arch             *architecture
	name             string
	filename         string
	mimeType         string
	packageSets      map[string]rpmmd.PackageSet
	enabledServices  []string
	disabledServices []string
	defaultTarget    string
	kernelOptions    string
	defaultSize      uint64
	exports          []string
	pipelines        pipelinesFunc

	// bootISO: installable ISO
	bootISO bool
	// rpmOstree: edge/ostree
	rpmOstree bool
	// bootable image
	bootable bool
}

func (t *imageType) Name() string {
	return t.name
}

func (t *imageType) Arch() distro.Arch {
	return t.arch
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

func (t *imageType) PackageSets(bp blueprint.Blueprint) map[string]rpmmd.PackageSet {
	// merge package sets that appear in the image type (or are enabled by
	// flags) with the package sets of the same name from the distro and arch
	mergedSets := make(map[string]rpmmd.PackageSet)

	imageSets := t.packageSets
	distroSets := t.arch.distro.packageSets
	archSets := t.arch.packageSets
	for name := range imageSets {
		mergedSets[name] = imageSets[name].Append(archSets[name]).Append(distroSets[name])
	}

	if _, hasPackages := imageSets["packages"]; !hasPackages {
		// should this be possible??
		mergedSets["packages"] = rpmmd.PackageSet{}
	}

	// build is usually not defined on the image type
	// so handle it explicitly
	if _, hasBuild := imageSets["build"]; !hasBuild {
		buildSet := archSets["build"].Append(distroSets["build"])
		if t.rpmOstree {
			buildSet.Include = append(buildSet.Include, "rpm-ostree")
		}
		mergedSets["build"] = buildSet
	}

	// package sets from flags
	if t.bootable {
		mergedSets["packages"] = mergedSets["packages"].Append(archSets["boot"]).Append(distroSets["boot"])
	}

	// blueprint packages
	bpPackages := bp.GetPackages()
	timezone, _ := bp.Customizations.GetTimezoneSettings()
	if timezone != nil {
		bpPackages = append(bpPackages, "chrony")
	}
	mergedSets["packages"] = mergedSets["packages"].Append(rpmmd.PackageSet{Include: bpPackages})
	return mergedSets

}

func (t *imageType) Exports() []string {
	if len(t.exports) > 0 {
		return t.exports
	}
	return []string{"assembler"}
}

// local type for ostree commit metadata used to define commit sources
type ostreeCommit struct {
	Checksum string
	URL      string
}

func (t *imageType) Manifest(customizations *blueprint.Customizations,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	packageSpecSets map[string][]rpmmd.PackageSpec,
	seed int64) (distro.Manifest, error) {

	if err := t.checkOptions(customizations, options); err != nil {
		return distro.Manifest{}, err
	}

	source := rand.NewSource(seed)
	rng := rand.New(source)

	pipelines, err := t.pipelines(t, customizations, options, repos, packageSpecSets, rng)
	if err != nil {
		return distro.Manifest{}, err
	}

	// flatten spec sets for sources
	allPackageSpecs := make([]rpmmd.PackageSpec, 0)
	for _, specs := range packageSpecSets {
		allPackageSpecs = append(allPackageSpecs, specs...)
	}

	var commits []ostreeCommit
	if t.bootISO && options.OSTree.Parent != "" && options.OSTree.URL != "" {
		commits = []ostreeCommit{{Checksum: options.OSTree.Parent, URL: options.OSTree.URL}}
	}
	return json.Marshal(
		osbuild.Manifest{
			Version:   "2",
			Pipelines: pipelines,
			Sources:   t.sources(allPackageSpecs, commits),
		},
	)
}

func (t *imageType) sources(packages []rpmmd.PackageSpec, ostreeCommits []ostreeCommit) osbuild.Sources {
	sources := osbuild.Sources{}
	curl := &osbuild.CurlSource{
		Items: make(map[string]osbuild.CurlSourceItem),
	}
	for _, pkg := range packages {
		item := new(osbuild.URLWithSecrets)
		item.URL = pkg.RemoteLocation
		if pkg.Secrets == "org.osbuild.rhsm" {
			item.Secrets = &osbuild.URLSecrets{
				Name: "org.osbuild.rhsm",
			}
		}
		curl.Items[pkg.Checksum] = item
	}
	if len(curl.Items) > 0 {
		sources["org.osbuild.curl"] = curl
	}

	ostree := &osbuild.OSTreeSource{
		Items: make(map[string]osbuild.OSTreeSourceItem),
	}
	for _, commit := range ostreeCommits {
		item := new(osbuild.OSTreeSourceItem)
		item.Remote.URL = commit.URL
		ostree.Items[commit.Checksum] = *item
	}
	if len(ostree.Items) > 0 {
		sources["org.osbuild.ostree"] = ostree
	}
	return sources
}

// checkOptions checks the validity and compatibility of options and customizations for the image type.
func (t *imageType) checkOptions(customizations *blueprint.Customizations, options distro.ImageOptions) error {
	if t.bootISO && t.rpmOstree {
		if options.OSTree.Parent == "" {
			return fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.name)
		}
		if customizations != nil {
			return fmt.Errorf("boot ISO image type %q does not support blueprint customizations", t.name)
		}
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.rpmOstree {
		return fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	return nil
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

	rd := &distribution{
		name:             name,
		modulePlatformID: modulePlatformID,
		ostreeRef:        ostreeRef,
		packageSets: map[string]rpmmd.PackageSet{
			"build": buildPackageSet(),
		},
	}

	// Shared Package sets
	edgeCommitCommonPkgSet := rpmmd.PackageSet{
		Include: []string{
			"redhat-release",
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
			"audit",
			"podman", "container-selinux", "skopeo", "criu",
			"slirp4netns", "fuse-overlayfs",
			"clevis", "clevis-dracut", "clevis-luks",
			"greenboot", "greenboot-grub2", "greenboot-rpm-ostree-grub2", "greenboot-reboot", "greenboot-status",
		},
		Exclude: []string{"rng-tools"},
	}
	edgeBuildPkgSet := rpmmd.PackageSet{
		Include: []string{
			"dnf", "dosfstools", "e2fsprogs", "efibootmgr", "genisoimage",
			"grub2-efi-ia32-cdboot", "grub2-efi-x64", "grub2-efi-x64-cdboot",
			"grub2-pc", "grub2-pc-modules", "grub2-tools", "grub2-tools-efi",
			"grub2-tools-extra", "grub2-tools-minimal", "isomd5sum",
			"lorax-templates-generic", "lorax-templates-rhel",
			"policycoreutils", "python36", "python3-iniparse", "qemu-img",
			"rpm-ostree", "selinux-policy-targeted", "shim-ia32", "shim-x64",
			"squashfs-tools", "syslinux", "syslinux-nonlinux", "systemd",
			"tar", "xfsprogs", "xorriso", "xz",
		},
		Exclude: nil,
	}
	edgeInstallerPkgSet := rpmmd.PackageSet{
		Include: []string{
			"aajohan-comfortaa-fonts", "abattis-cantarell-fonts",
			"alsa-firmware", "alsa-tools-firmware", "anaconda",
			"anaconda-dracut", "anaconda-install-env-deps", "anaconda-widgets",
			"audit", "bind-utils", "biosdevname", "bitmap-fangsongti-fonts",
			"bzip2", "cryptsetup", "curl", "dbus-x11", "dejavu-sans-fonts",
			"dejavu-sans-mono-fonts", "device-mapper-persistent-data",
			"dmidecode", "dnf", "dracut-config-generic", "dracut-network",
			"dump", "efibootmgr", "ethtool", "ftp", "gdb-gdbserver", "gdisk",
			"gfs2-utils", "glibc-all-langpacks",
			"google-noto-sans-cjk-ttc-fonts", "grub2-efi-ia32-cdboot",
			"grub2-efi-x64-cdboot", "grub2-tools", "grub2-tools-efi",
			"grub2-tools-extra", "grub2-tools-minimal", "grubby",
			"gsettings-desktop-schemas", "hdparm", "hexedit", "hostname",
			"initscripts", "ipmitool", "iwl1000-firmware", "iwl100-firmware",
			"iwl105-firmware", "iwl135-firmware", "iwl2000-firmware",
			"iwl2030-firmware", "iwl3160-firmware", "iwl3945-firmware",
			"iwl4965-firmware", "iwl5000-firmware", "iwl5150-firmware",
			"iwl6000-firmware", "iwl6000g2a-firmware", "iwl6000g2b-firmware",
			"iwl6050-firmware", "iwl7260-firmware", "jomolhari-fonts",
			"kacst-farsi-fonts", "kacst-qurn-fonts", "kbd", "kbd-misc",
			"kdump-anaconda-addon", "kernel", "khmeros-base-fonts", "less",
			"libblockdev-lvm-dbus", "libertas-sd8686-firmware",
			"libertas-sd8787-firmware", "libertas-usb8388-firmware",
			"libertas-usb8388-olpc-firmware", "libibverbs",
			"libreport-plugin-bugzilla", "libreport-plugin-reportuploader",
			"libreport-rhel-anaconda-bugzilla", "librsvg2", "linux-firmware",
			"lklug-fonts", "lohit-assamese-fonts", "lohit-bengali-fonts",
			"lohit-devanagari-fonts", "lohit-gujarati-fonts",
			"lohit-gurmukhi-fonts", "lohit-kannada-fonts", "lohit-odia-fonts",
			"lohit-tamil-fonts", "lohit-telugu-fonts", "lsof", "madan-fonts",
			"memtest86+", "metacity", "mtr", "mt-st", "net-tools", "nfs-utils",
			"nmap-ncat", "nm-connection-editor", "nss-tools",
			"openssh-clients", "openssh-server", "oscap-anaconda-addon",
			"ostree", "pciutils", "perl-interpreter", "pigz", "plymouth",
			"prefixdevname", "python3-pyatspi", "rdma-core",
			"redhat-release-eula", "rng-tools", "rpcbind", "rpm-ostree",
			"rsync", "rsyslog", "selinux-policy-targeted", "sg3_utils",
			"shim-ia32", "shim-x64", "sil-abyssinica-fonts",
			"sil-padauk-fonts", "sil-scheherazade-fonts", "smartmontools",
			"smc-meera-fonts", "spice-vdagent", "strace", "syslinux",
			"systemd", "system-storage-manager", "tar",
			"thai-scalable-waree-fonts", "tigervnc-server-minimal",
			"tigervnc-server-module", "udisks2", "udisks2-iscsi", "usbutils",
			"vim-minimal", "volume_key", "wget", "xfsdump", "xfsprogs",
			"xorg-x11-drivers", "xorg-x11-fonts-misc", "xorg-x11-server-utils",
			"xorg-x11-server-Xorg", "xorg-x11-xauth", "xz",
		},
		Exclude: nil,
	}
	edgeCommitX86PkgSet := rpmmd.PackageSet{
		Include: append(edgeCommitCommonPkgSet.Include,
			// x86 specific
			"grub2", "grub2-efi-x64", "efibootmgr", "shim-x64",
			"microcode_ctl", "iwl1000-firmware", "iwl100-firmware",
			"iwl105-firmware", "iwl135-firmware", "iwl2000-firmware",
			"iwl2030-firmware", "iwl3160-firmware", "iwl5000-firmware",
			"iwl5150-firmware", "iwl6000-firmware", "iwl6050-firmware",
			"iwl7260-firmware"),
		Exclude: edgeCommitCommonPkgSet.Exclude,
	}
	edgeCommitAarch64PkgSet := rpmmd.PackageSet{
		Include: append(edgeCommitCommonPkgSet.Include,
			// aarch64 specific
			"grub2-efi-aa64", "efibootmgr", "shim-aa64",
			"iwl7260-firmware"),
		Exclude: edgeCommitCommonPkgSet.Exclude,
	}

	// Shared Services
	edgeServices := []string{
		"NetworkManager.service", "firewalld.service", "sshd.service",
	}

	// Image Definitions
	edgeCommitImgTypeX86_64 := imageType{
		name:     "edge-commit",
		filename: "commit.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]rpmmd.PackageSet{
			"build":    edgeBuildPkgSet,
			"packages": edgeCommitX86PkgSet,
		},
		enabledServices: edgeServices,
		rpmOstree:       true,
		pipelines:       edgeCommitPipelines,
		exports:         []string{"commit-archive"},
	}
	edgeOCIImgTypeX86_64 := imageType{
		name:     "edge-container",
		filename: "container.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]rpmmd.PackageSet{
			"build":     edgeBuildPkgSet,
			"packages":  edgeCommitX86PkgSet,
			"container": {Include: []string{"httpd"}},
		},
		enabledServices: edgeServices,
		rpmOstree:       true,
		bootISO:         false,
		pipelines:       edgeContainerPipelines,
		exports:         []string{"container"},
	}
	edgeInstallerImgTypeX86_64 := imageType{
		name:     "edge-installer",
		filename: "installer.iso",
		mimeType: "application/x-iso9660-image",
		packageSets: map[string]rpmmd.PackageSet{
			"build":     edgeBuildPkgSet,
			"packages":  edgeCommitX86PkgSet,
			"installer": edgeInstallerPkgSet,
		},
		enabledServices: edgeServices,
		rpmOstree:       true,
		bootISO:         true,
		pipelines:       edgeInstallerPipelines,
		exports:         []string{"bootiso"},
	}

	x86_64 := architecture{
		name:   "x86_64",
		distro: rd,
		packageSets: map[string]rpmmd.PackageSet{
			"boot": x8664BootPackageSet(),
		},
		legacy: "i386-pc",
		uefi:   true,
	}

	qcow2ImageType := imageType{
		name:          "qcow2",
		filename:      "disk.qcow2",
		mimeType:      "application/x-qemu-disk",
		defaultTarget: "multi-user.target",
		kernelOptions: "console=tty0 console=ttyS0,115200n8 no_timer_check net.ifnames=0 crashkernel=auto",
		packageSets: map[string]rpmmd.PackageSet{
			"build":    x8664BuildPackageSet(),
			"packages": qcow2CommonPkgSet(),
		},
		bootable:    true,
		defaultSize: 10 * GigaByte,
		pipelines:   qcow2Pipelines,
		exports:     []string{"qcow2"},
	}

	tarImgType := imageType{
		name:     "tar",
		filename: "root.tar.xz",
		mimeType: "application/x-tar",
		packageSets: map[string]rpmmd.PackageSet{
			"packages": {
				Include: []string{"policycoreutils", "selinux-policy-targeted"},
				Exclude: []string{"rng-tools"},
			},
		},
		pipelines: tarPipelines,
		exports:   []string{"root-tar"},
	}
	tarInstallerImgTypeX86_64 := imageType{
		name:     "tar-installer",
		filename: "installer.iso",
		mimeType: "application/x-iso9660-image",
		packageSets: map[string]rpmmd.PackageSet{
			"build": installerBuildPackageSet(),
			"packages": {
				Include: []string{"lvm2", "policycoreutils", "selinux-policy-targeted"},
				Exclude: []string{"rng-tools"},
			},
			"installer": installerPackageSet(),
		},
		rpmOstree: false,
		bootISO:   true,
		pipelines: tarInstallerPipelines,
		exports:   []string{"bootiso"},
	}

	edgeCommitImgTypeAarch64 := imageType{
		name:     "edge-commit",
		filename: "commit.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]rpmmd.PackageSet{
			"build":    edgeBuildPkgSet,
			"packages": edgeCommitAarch64PkgSet,
		},
		enabledServices: edgeServices,
		rpmOstree:       true,
		pipelines:       edgeCommitPipelines,
		exports:         []string{"commit-archive"},
	}
	edgeOCIImgTypeAarch64 := imageType{
		name:     "edge-container",
		filename: "container.tar",
		mimeType: "application/x-tar",
		packageSets: map[string]rpmmd.PackageSet{
			"build":     edgeBuildPkgSet,
			"packages":  edgeCommitAarch64PkgSet,
			"container": {Include: []string{"httpd"}},
		},
		enabledServices: edgeServices,
		rpmOstree:       true,
		pipelines:       edgeContainerPipelines,
		exports:         []string{"container"},
	}
	edgeInstallerImgTypeAarch64 := imageType{
		name:     "edge-installer",
		filename: "installer.iso",
		mimeType: "application/x-iso9660-image",
		packageSets: map[string]rpmmd.PackageSet{
			"build":     edgeBuildPkgSet,
			"packages":  edgeCommitX86PkgSet,
			"installer": edgeInstallerPkgSet,
		},
		enabledServices: edgeServices,
		rpmOstree:       true,
		bootISO:         true,
		pipelines:       edgeInstallerPipelines,
		exports:         []string{"bootiso"},
	}
	x86_64.addImageTypes(qcow2ImageType, tarImgType, tarInstallerImgTypeX86_64, edgeCommitImgTypeX86_64, edgeInstallerImgTypeX86_64, edgeOCIImgTypeX86_64)

	aarch64 := architecture{
		name:   "aarch64",
		distro: rd,
		packageSets: map[string]rpmmd.PackageSet{
			"boot": aarch64BootPackageSet(),
		},
	}
	aarch64.addImageTypes(qcow2ImageType, tarImgType, edgeCommitImgTypeAarch64, edgeOCIImgTypeAarch64, edgeInstallerImgTypeAarch64)

	ppc64le := architecture{
		distro: rd,
		name:   "ppc64le",
		packageSets: map[string]rpmmd.PackageSet{
			"boot": ppc64leBootPackageSet(),
		},
		legacy: "powerpc-ieee1275",
		uefi:   false,
	}
	ppc64le.addImageTypes(qcow2ImageType, tarImgType)

	s390x := architecture{
		distro: rd,
		name:   "s390x",
		packageSets: map[string]rpmmd.PackageSet{
			"boot": s390xBootPackageSet(),
		},
		uefi: false,
	}
	s390x.addImageTypes(qcow2ImageType, tarImgType)

	rd.addArches(x86_64, aarch64, ppc64le, s390x)
	return rd
}
