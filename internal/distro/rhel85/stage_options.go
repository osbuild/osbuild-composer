package rhel85

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/osbuild/osbuild-composer/internal/blueprint"
	"github.com/osbuild/osbuild-composer/internal/common"
	"github.com/osbuild/osbuild-composer/internal/distro"
	osbuild "github.com/osbuild/osbuild-composer/internal/osbuild2"
)

const (
	kspath = "/osbuild.ks"
)

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

func dracutStageOptions(kernelVer, arch string, additionalModules []string) *osbuild.DracutStageOptions {
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
		"dracut-systemd",
		"pollcdrom",
		"usrmount",
		"base",
		"fs-lib",
		"img-lib",
		"shutdown",
		"uefi-lib",
	}

	if arch == distro.X86_64ArchName {
		modules = append(modules, "biosdevname")
	}

	modules = append(modules, additionalModules...)
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
	isolabel := fmt.Sprintf("RHEL-8-5-0-BaseOS-%s", arch)

	var architectures []string

	if arch == distro.X86_64ArchName {
		architectures = []string{"IA32", "X64"}
	} else if arch == distro.Aarch64ArchName {
		architectures = []string{"AA64"}
	} else {
		panic("unsupported architecture")
	}

	return &osbuild.BootISOMonoStageOptions{
		Product: osbuild.Product{
			Name:    "Red Hat Enterprise Linux",
			Version: osVersion,
		},
		ISOLabel:   isolabel,
		Kernel:     kernelVer,
		KernelOpts: fmt.Sprintf("inst.ks=hd:LABEL=%s:%s", isolabel, kspath),
		EFI: osbuild.EFI{
			Architectures: architectures,
			Vendor:        "redhat",
		},
		ISOLinux: osbuild.ISOLinux{
			Enabled: arch == distro.X86_64ArchName,
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

func xorrisofsStageOptions(filename string, arch string, isolinux bool) *osbuild.XorrisofsStageOptions {
	options := &osbuild.XorrisofsStageOptions{
		Filename: filename,
		VolID:    fmt.Sprintf("RHEL-8-5-0-BaseOS-%s", arch),
		SysID:    "LINUX",
		EFI:      "images/efiboot.img",
	}

	if isolinux {
		options.Boot = &osbuild.XorrisofsBoot{
			Image:   "isolinux/isolinux.bin",
			Catalog: "isolinux/boot.cat",
		}

		options.IsohybridMBR = "/usr/share/syslinux/isohdpfx.bin"
	}

	return options
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

func nginxConfigStageOptions(path, htmlRoot, listen string) *osbuild.NginxConfigStageOptions {
	// configure nginx to work in an unprivileged container
	cfg := &osbuild.NginxConfig{
		Listen: listen,
		Root:   htmlRoot,
		Daemon: common.BoolToPtr(false),
		PID:    "/tmp/nginx.pid",
	}
	return &osbuild.NginxConfigStageOptions{
		Path:   path,
		Config: cfg,
	}
}

func chmodStageOptions(path, mode string, recursive bool) *osbuild.ChmodStageOptions {
	return &osbuild.ChmodStageOptions{
		Items: map[string]osbuild.ChmodStagePathOptions{
			path: {Mode: mode, Recursive: recursive},
		},
	}
}

func ostreeConfigStageOptions(repo string, readOnly bool) *osbuild.OSTreeConfigStageOptions {
	return &osbuild.OSTreeConfigStageOptions{
		Repo: repo,
		Config: &osbuild.OSTreeConfig{
			Sysroot: &osbuild.SysrootOptions{
				ReadOnly:   common.BoolToPtr(readOnly),
				Bootloader: "none",
			},
		},
	}
}

func efiMkdirStageOptions() *osbuild.MkdirStageOptions {
	return &osbuild.MkdirStageOptions{
		Paths: []osbuild.Path{
			{
				Path: "/boot/efi",
				Mode: os.FileMode(0700),
			},
		},
	}
}
