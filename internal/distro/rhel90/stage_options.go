package rhel90

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/google/uuid"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/crypt"
	"github.com/osbuild/osbuild-composer/internal/disk"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
	"github.com/osbuild/osbuild-composer/internal/rpmmd"
)

const (
	kspath = "/osbuild.ks"
)

func rpmStageOptions(repos []rpmmd.RepoConfig) *osbuild.RPMStageOptions {
	var gpgKeys []string
	for _, repo := range repos {
		if repo.GPGKey == "" {
			continue
		}
		gpgKeys = append(gpgKeys, repo.GPGKey)
	}

	return &osbuild.RPMStageOptions{
		GPGKeys: gpgKeys,
		Exclude: &osbuild.Exclude{
			// NOTE: Make configurable?
			Docs: true,
		},
	}
}

// selinuxStageOptions returns the options for the org.osbuild.selinux stage.
// Setting the argument to 'true' relabels the '/usr/bin/cp' and '/usr/bin/tar'
// binaries with 'install_exec_t'. This should be set in the build root.
func selinuxStageOptions(labelcp bool) *osbuild.SELinuxStageOptions {
	options := &osbuild.SELinuxStageOptions{
		FileContexts: "etc/selinux/targeted/contexts/files/file_contexts",
	}
	if labelcp {
		options.Labels = map[string]string{
			"/usr/bin/cp":  "system_u:object_r:install_exec_t:s0",
			"/usr/bin/tar": "system_u:object_r:install_exec_t:s0",
		}
	}
	return options
}

func userStageOptions(users []blueprint.UserCustomization) (*osbuild.UsersStageOptions, error) {
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

func usersFirstBootOptions(usersStageOptions *osbuild.UsersStageOptions) *osbuild.FirstBootStageOptions {
	cmds := make([]string, 0, 3*len(usersStageOptions.Users)+1)
	// workaround for creating authorized_keys file for user
	varhome := filepath.Join("/var", "home")
	for name, user := range usersStageOptions.Users {
		if user.Key != nil {
			sshdir := filepath.Join(varhome, name, ".ssh")
			cmds = append(cmds, fmt.Sprintf("mkdir -p %s", sshdir))
			cmds = append(cmds, fmt.Sprintf("sh -c 'echo %q >> %q'", *user.Key, filepath.Join(sshdir, "authorized_keys")))
			cmds = append(cmds, fmt.Sprintf("chown %s:%s -Rc %s", name, name, sshdir))
		}
	}
	cmds = append(cmds, fmt.Sprintf("restorecon -rvF %s", varhome))
	options := &osbuild.FirstBootStageOptions{
		Commands:       cmds,
		WaitForNetwork: false,
	}

	return options
}

func groupStageOptions(groups []blueprint.GroupCustomization) *osbuild.GroupsStageOptions {
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

func firewallStageOptions(firewall *blueprint.FirewallCustomization) *osbuild.FirewallStageOptions {
	options := osbuild.FirewallStageOptions{
		Ports: firewall.Ports,
	}

	if firewall.Services != nil {
		options.EnabledServices = firewall.Services.Enabled
		options.DisabledServices = firewall.Services.Disabled
	}

	return &options
}

func systemdStageOptions(enabledServices, disabledServices []string, s *blueprint.ServicesCustomization, target string) *osbuild.SystemdStageOptions {
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

func buildStampStageOptions(arch string) *osbuild.BuildstampStageOptions {
	return &osbuild.BuildstampStageOptions{
		Arch:    arch,
		Product: "Red Hat Enterprise Linux",
		Version: osVersion,
		Variant: "edge",
		Final:   true,
	}
}

func anacondaStageOptions() *osbuild.AnacondaStageOptions {
	return &osbuild.AnacondaStageOptions{
		KickstartModules: []string{
			"org.fedoraproject.Anaconda.Modules.Network",
			"org.fedoraproject.Anaconda.Modules.Payloads",
			"org.fedoraproject.Anaconda.Modules.Storage",
		},
	}
}

func loraxScriptStageOptions(arch string) *osbuild.LoraxScriptStageOptions {
	return &osbuild.LoraxScriptStageOptions{
		Path:     "99-generic/runtime-postinstall.tmpl",
		BaseArch: arch,
	}
}

func dracutStageOptions(kernelVer string) *osbuild.DracutStageOptions {
	kernel := []string{kernelVer}
	modules := []string{
		"bash",
		"systemd",
		"fips",
		"systemd-initrd",
		"modsign",
		"nss-softokn",
		"rdma",
		"rngd",
		"i18n",
		"convertfs",
		"network-manager",
		"network",
		"ifcfg",
		"url-lib",
		"drm",
		"plymouth",
		"prefixdevname",
		"prefixdevname-tools",
		"anaconda",
		"crypt",
		"dm",
		"dmsquash-live",
		"kernel-modules",
		"kernel-modules-extra",
		"kernel-network-modules",
		"livenet",
		"lvm",
		"mdraid",
		"multipath",
		"qemu",
		"qemu-net",
		"fcoe",
		"fcoe-uefi",
		"iscsi",
		"lunmask",
		"nfs",
		"resume",
		"rootfs-block",
		"terminfo",
		"udev-rules",
		"biosdevname",
		"dracut-systemd",
		"pollcdrom",
		"usrmount",
		"base",
		"fs-lib",
		"img-lib",
		"shutdown",
		"uefi-lib",
	}
	return &osbuild.DracutStageOptions{
		Kernel:  kernel,
		Modules: modules,
		Install: []string{"/.buildstamp"},
	}
}

func tarKickstartStageOptions(tarURL string) *osbuild.KickstartStageOptions {
	return &osbuild.KickstartStageOptions{
		Path: kspath,
		LiveIMG: &osbuild.LiveIMG{
			URL: tarURL,
		},
	}
}

func ostreeKickstartStageOptions(ostreeURL, ostreeRef string) *osbuild.KickstartStageOptions {
	return &osbuild.KickstartStageOptions{
		Path: kspath,
		OSTree: &osbuild.OSTreeOptions{
			OSName: "rhel",
			URL:    ostreeURL,
			Ref:    ostreeRef,
			GPG:    false,
		},
	}
}

func bootISOMonoStageOptions(kernelVer string, arch string) *osbuild.BootISOMonoStageOptions {
	comprOptions := new(osbuild.FSCompressionOptions)
	if bcj := osbuild.BCJOption(arch); bcj != "" {
		comprOptions.BCJ = bcj
	}
	isolabel := fmt.Sprintf("RHEL-9-0-0-BaseOS-%s", arch)
	return &osbuild.BootISOMonoStageOptions{
		Product: osbuild.Product{
			Name:    "Red Hat Enterprise Linux",
			Version: osVersion,
		},
		ISOLabel:   isolabel,
		Kernel:     kernelVer,
		KernelOpts: fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", isolabel, kspath),
		EFI: osbuild.EFI{
			Architectures: []string{
				"IA32",
				"X64",
			},
			Vendor: "redhat",
		},
		ISOLinux: osbuild.ISOLinux{
			Enabled: true,
			Debug:   false,
		},
		Templates: "80-rhel",
		RootFS: osbuild.RootFS{
			Size: 9216,
			Compression: osbuild.FSCompression{
				Method:  "xz",
				Options: comprOptions,
			},
		},
	}
}

func discinfoStageOptions(arch string) *osbuild.DiscinfoStageOptions {
	return &osbuild.DiscinfoStageOptions{
		BaseArch: arch,
		Release:  "202010217.n.0",
	}
}

func xorrisofsStageOptions(filename string, arch string) *osbuild.XorrisofsStageOptions {
	return &osbuild.XorrisofsStageOptions{
		Filename: filename,
		VolID:    fmt.Sprintf("RHEL-9-0-0-BaseOS-%s", arch),
		SysID:    "LINUX",
		Boot: osbuild.XorrisofsBoot{
			Image:   "isolinux/isolinux.bin",
			Catalog: "isolinux/boot.cat",
		},
		EFI:          "images/efiboot.img",
		IsohybridMBR: "/usr/share/syslinux/isohdpfx.bin",
	}
}

func grub2StageOptions(rootPartition *disk.Partition, bootPartition *disk.Partition, kernelOptions string,
	kernel *blueprint.KernelCustomization, kernelVer string, uefi bool, legacy string) *osbuild.GRUB2StageOptions {
	if rootPartition == nil {
		panic("root partition must be defined for grub2 stage, this is a programming error")
	}

	stageOptions := osbuild.GRUB2StageOptions{
		RootFilesystemUUID: uuid.MustParse(rootPartition.Filesystem.UUID),
		KernelOptions:      kernelOptions,
		Legacy:             legacy,
	}

	if bootPartition != nil {
		bootFsUUID := uuid.MustParse(bootPartition.Filesystem.UUID)
		stageOptions.BootFilesystemUUID = &bootFsUUID
	}

	if uefi {
		stageOptions.UEFI = &osbuild.GRUB2UEFI{
			Vendor: "redhat",
		}
	}

	if !uefi {
		stageOptions.Legacy = legacy
	}

	if kernel != nil {
		if kernel.Append != "" {
			stageOptions.KernelOptions += " " + kernel.Append
		}
		stageOptions.SavedEntry = "ffffffffffffffffffffffffffffffff-" + kernelVer
	}

	return &stageOptions
}

// sfdiskStageOptions creates the options and devices properties for an
// org.osbuild.sfdisk stage based on a partition table description
func sfdiskStageOptions(pt *disk.PartitionTable) *osbuild.SfdiskStageOptions {
	partitions := make([]osbuild.Partition, len(pt.Partitions))
	for idx, p := range pt.Partitions {
		partitions[idx] = osbuild.Partition{
			Bootable: p.Bootable,
			Size:     p.Size,
			Start:    p.Start,
			Type:     p.Type,
			UUID:     p.UUID,
		}
	}
	stageOptions := &osbuild.SfdiskStageOptions{
		Label:      pt.Type,
		UUID:       pt.UUID,
		Partitions: partitions,
	}

	return stageOptions
}

// copyFSTreeOptions creates the options, inputs, devices, and mounts properties
// for an org.osbuild.copy stage for a given source tree using a partition
// table description to define the mounts
func copyFSTreeOptions(inputName, inputPipeline string, pt *disk.PartitionTable, device *osbuild.Device) (
	*osbuild.CopyStageOptions,
	*osbuild.Devices,
	*osbuild.Mounts,
) {
	// assume loopback device for simplicity since it's the only one currently supported
	// panic if the conversion fails
	devOptions, ok := device.Options.(*osbuild.LoopbackDeviceOptions)
	if !ok {
		panic("copyStageOptions: failed to convert device options to loopback options")
	}

	devices := make(map[string]osbuild.Device, len(pt.Partitions))
	mounts := make([]osbuild.Mount, 0, len(pt.Partitions))
	for _, p := range pt.Partitions {
		if p.Filesystem == nil {
			// no filesystem for partition (e.g., BIOS boot)
			continue
		}
		name := filepath.Base(p.Filesystem.Mountpoint)
		if name == "/" {
			name = "root"
		}
		devices[name] = *osbuild.NewLoopbackDevice(
			&osbuild.LoopbackDeviceOptions{
				Filename: devOptions.Filename,
				Start:    p.Start,
				Size:     p.Size,
			},
		)
		var mount *osbuild.Mount
		switch p.Filesystem.Type {
		case "xfs":
			mount = osbuild.NewXfsMount(name, name, p.Filesystem.Mountpoint)
		case "vfat":
			mount = osbuild.NewFATMount(name, name, p.Filesystem.Mountpoint)
		case "ext4":
			mount = osbuild.NewExt4Mount(name, name, p.Filesystem.Mountpoint)
		case "btrfs":
			mount = osbuild.NewBtrfsMount(name, name, p.Filesystem.Mountpoint)
		default:
			panic("unknown fs type " + p.Type)
		}
		mounts = append(mounts, *mount)
	}

	// sort the mounts, using < should just work because:
	// - a parent directory should be always before its children:
	//   / < /boot
	// - the order of siblings doesn't matter
	sort.Slice(mounts, func(i, j int) bool {
		return mounts[i].Target < mounts[j].Target
	})

	stageMounts := osbuild.Mounts(mounts)
	stageDevices := osbuild.Devices(devices)

	options := osbuild.CopyStageOptions{
		Paths: []osbuild.CopyStagePath{
			{
				From: fmt.Sprintf("input://%s/", inputName),
				To:   "mount://root/",
			},
		},
	}

	return &options, &stageDevices, &stageMounts
}

func grub2InstStageOptions(filename string, pt *disk.PartitionTable, platform string) *osbuild.Grub2InstStageOptions {
	bootPartIndex := pt.BootPartitionIndex()
	if bootPartIndex == -1 {
		panic("failed to find boot or root partition for grub2.inst stage")
	}
	core := osbuild.CoreMkImage{
		Type:       "mkimage",
		PartLabel:  pt.Type,
		Filesystem: pt.Partitions[bootPartIndex].Filesystem.Type,
	}

	prefix := osbuild.PrefixPartition{
		Type:      "partition",
		PartLabel: pt.Type,
		Number:    uint(bootPartIndex),
		Path:      "/boot/grub2",
	}

	return &osbuild.Grub2InstStageOptions{
		Filename: filename,
		Platform: platform,
		Location: pt.Partitions[0].Start,
		Core:     core,
		Prefix:   prefix,
	}
}

func ziplInstStageOptions(kernel string, pt *disk.PartitionTable) *osbuild.ZiplInstStageOptions {
	bootPartIndex := pt.BootPartitionIndex()
	if bootPartIndex == -1 {
		panic("failed to find boot or root partition for zipl.inst stage")
	}

	return &osbuild.ZiplInstStageOptions{
		Kernel:   kernel,
		Location: pt.Partitions[bootPartIndex].Start,
	}
}

func qemuStageOptions(filename, format, compat string) *osbuild.QEMUStageOptions {
	var options osbuild.QEMUFormatOptions
	switch format {
	case "qcow2":
		options = osbuild.Qcow2Options{
			Type:   "qcow2",
			Compat: compat,
		}
	case "vpc":
		options = osbuild.VPCOptions{
			Type: "vpc",
		}
	case "vmdk":
		options = osbuild.VMDKOptions{
			Type: "vmdk",
		}
	default:
		panic("unknown format in qemu stage: " + format)
	}

	return &osbuild.QEMUStageOptions{
		Filename: filename,
		Format:   options,
	}
}

func kernelCmdlineStageOptions(rootUUID string, kernelOptions string) *osbuild.KernelCmdlineStageOptions {
	return &osbuild.KernelCmdlineStageOptions{
		RootFsUUID: rootUUID,
		KernelOpts: kernelOptions,
	}
}
