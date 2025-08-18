package distro

import (
	"fmt"
	"reflect"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/customizations/shell"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/customizations/users"
	"github.com/osbuild/images/pkg/customizations/wsl"
	"github.com/osbuild/images/pkg/osbuild"
)

// ImageConfig represents a (default) configuration applied to the image payload.
type ImageConfig struct {
	Hostname            *string                     `yaml:"hostname,omitempty"`
	Timezone            *string                     `yaml:"timezone,omitempty"`
	TimeSynchronization *osbuild.ChronyStageOptions `yaml:"time_synchronization,omitempty"`
	Locale              *string                     `yaml:"locale,omitempty"`
	Keyboard            *osbuild.KeymapStageOptions
	EnabledServices     []string `yaml:"enabled_services,omitempty"`
	DisabledServices    []string `yaml:"disabled_services,omitempty"`
	MaskedServices      []string
	DefaultTarget       *string `yaml:"default_target,omitempty"`

	Sysconfig           *Sysconfig `yaml:"sysconfig,omitempty"`
	DefaultKernel       *string    `yaml:"default_kernel,omitempty"`
	UpdateDefaultKernel *bool      `yaml:"update_default_kernel,omitempty"`
	KernelOptions       []string   `yaml:"kernel_options,omitempty"`

	// The name of the default kernel to use for the image type.
	// NOTE: Currently this overrides the kernel named in the blueprint. The
	// only image type that uses it is the azure-cvm, which doesn't allow
	// kernel selection. The option should generally be a fallback for when the
	// blueprint doesn't specify a kernel.
	//
	// This option has no effect on the DefaultKernel option under Sysconfig.
	// If both options are set, they should have the same value.
	// These two options should be unified.
	DefaultKernelName *string `yaml:"default_kernel_name"`

	// List of files from which to import GPG keys into the RPM database
	GPGKeyFiles []string `yaml:"gpgkey_files,omitempty"`

	// Disable SELinux labelling
	NoSELinux *bool `yaml:"no_selinux,omitempty"`

	// Do not use. Forces auto-relabelling on first boot.
	// See https://github.com/osbuild/osbuild/commit/52cb27631b587c1df177cd17625c5b473e1e85d2
	SELinuxForceRelabel *bool `yaml:"selinux_force_relabel,omitempty"`

	// Disable documentation
	ExcludeDocs *bool `yaml:"exclude_docs,omitempty"`

	ShellInit []shell.InitFile `yaml:"shell_init,omitempty"`

	// for RHSM configuration, we need to potentially distinguish the case
	// when the user want the image to be subscribed on first boot and when not
	RHSMConfig    map[subscription.RHSMStatus]*subscription.RHSMConfig `yaml:"rhsm_config,omitempty"`
	SystemdLogind []*osbuild.SystemdLogindStageOptions                 `yaml:"systemd_logind,omitempty"`
	CloudInit     []*osbuild.CloudInitStageOptions                     `yaml:"cloud_init"`
	Modprobe      []*osbuild.ModprobeStageOptions
	DracutConf    []*osbuild.DracutConfStageOptions        `yaml:"dracut_conf"`
	SystemdDropin []*osbuild.SystemdUnitStageOptions       `yaml:"systemd_dropin,omitempty"`
	SystemdUnit   []*osbuild.SystemdUnitCreateStageOptions `yaml:"systemd_unit,omitempty"`
	Authselect    *osbuild.AuthselectStageOptions          `yaml:"authselect"`
	SELinuxConfig *osbuild.SELinuxConfigStageOptions       `yaml:"selinux_config,omitempty"`
	Tuned         *osbuild.TunedStageOptions
	Tmpfilesd     []*osbuild.TmpfilesdStageOptions
	PamLimitsConf []*osbuild.PamLimitsConfStageOptions `yaml:"pam_limits_conf,omitempty"`
	Sysctld       []*osbuild.SysctldStageOptions
	// Do not use DNFConfig directly, call "DNFConfigOptions()"
	DNFConfig           *DNFConfig                      `yaml:"dnf_config"`
	SshdConfig          *osbuild.SshdConfigStageOptions `yaml:"sshd_config"`
	Authconfig          *osbuild.AuthconfigStageOptions
	PwQuality           *osbuild.PwqualityConfStageOptions
	WAAgentConfig       *osbuild.WAAgentConfStageOptions        `yaml:"waagent_config,omitempty"`
	Grub2Config         *osbuild.GRUB2Config                    `yaml:"grub2_config,omitempty"`
	DNFAutomaticConfig  *osbuild.DNFAutomaticConfigStageOptions `yaml:"dnf_automatic_config"`
	YumConfig           *osbuild.YumConfigStageOptions          `yaml:"yum_config,omitempty"`
	YUMRepos            []*osbuild.YumReposStageOptions         `yaml:"yum_repos,omitempty"`
	Firewall            *osbuild.FirewallStageOptions
	UdevRules           *osbuild.UdevRulesStageOptions      `yaml:"udev_rules,omitempty"`
	GCPGuestAgentConfig *osbuild.GcpGuestAgentConfigOptions `yaml:"gcp_guest_agent_config,omitempty"`
	NetworkManager      *osbuild.NMConfStageOptions         `yaml:"network_manager,omitempty"`
	Presets             []osbuild.Preset                    `yaml:"presets,omitempty"`

	WSL *wsl.WSL `yaml:"wsl,omitempty"`

	Users []users.User

	Files       []*fsnode.File
	Directories []*fsnode.Directory

	// KernelOptionsBootloader controls whether kernel command line options
	// should be specified in the bootloader grubenv configuration. Otherwise
	// they are specified in /etc/kernel/cmdline (default).
	//
	// This should only be used for old distros that use grub and it is
	// applied on all architectures, except for s390x.
	KernelOptionsBootloader *bool `yaml:"kernel_options_bootloader,omitempty"`

	// The default OSCAP datastream to use for the image as a fallback,
	// if no datastream value is provided by the user.
	DefaultOSCAPDatastream *string `yaml:"default_oscap_datastream,omitempty"`

	// NoBLS configures the image bootloader with traditional menu entries
	// instead of BLS. Required for legacy systems like RHEL 7.
	NoBLS *bool `yaml:"no_bls,omitempty"`

	// OSTree specific configuration

	// Read only sysroot and boot
	OSTreeConfSysrootReadOnly *bool `yaml:"ostree_conf_sysroot_readonly,omitempty"`

	// Lock the root account in the deployment unless the user defined root
	// user options in the build configuration.
	LockRootUser *bool `yaml:"lock_root_user,omitempty"`

	IgnitionPlatform *string `yaml:"ignition_platform,omitempty"`

	// InstallWeakDeps enables installation of weak dependencies for packages
	// that are statically defined for the pipeline.
	InstallWeakDeps *bool `yaml:"install_weak_deps,omitempty"`

	// How to handle the /etc/machine-id file, when set to true it causes the
	// machine id to be set to 'uninitialized' which causes ConditionFirstboot
	// to be triggered in systemd
	MachineIdUninitialized *bool `yaml:"machine_id_uninitialized,omitempty"`

	// MountUnits creates systemd .mount units to describe the filesystem
	// instead of writing to /etc/fstab
	MountUnits *bool `yaml:"mount_units,omitempty"`

	// Indicates if rhc should be set to permissive when creating the registration script
	PermissiveRHC *bool `yaml:"permissive_rhc,omitempty"`

	// VersionlockPackges uses dnf versionlock to lock a package to the version
	// that is installed during image build, preventing it from being updated.
	// This is only supported for distributions that use dnf4, because osbuild
	// only has a stage for dnf4 version locking.
	VersionlockPackages []string `yaml:"versionlock_packages,omitempty"`
}

// shallowMerge creates a new struct by merging a child and a parent.
// Only values unset in the child will be copied from the parent.
// It is not recursive.
//
// Returns a pointer to a new struct instance of type T.
func shallowMerge[T any](child *T, parent *T) *T {
	finalConfig := *child

	if parent != nil {
		// iterate over all struct fields and copy unset values from the parent
		for i := 0; i < reflect.TypeOf(*child).NumField(); i++ {
			fieldName := reflect.TypeOf(*child).Field(i).Name
			field := reflect.ValueOf(&finalConfig).Elem().FieldByName(fieldName)

			// Only container types or pointer are supported.
			// The reason is that with basic types, we can't distinguish between unset value and zero value.
			if kind := field.Kind(); kind != reflect.Ptr && kind != reflect.Slice && kind != reflect.Map {
				panic(fmt.Sprintf("unsupported field type for %s: %s (only container types or pointer are supported)",
					fieldName, field.Kind()))
			}

			if field.IsNil() {
				field.Set(reflect.ValueOf(parent).Elem().FieldByName(fieldName))
			}
		}
	}
	return &finalConfig
}

type DNFConfig struct {
	Options          []*osbuild.DNFConfigStageOptions
	SetReleaseVerVar *bool `yaml:"set_release_ver_var"`
}

// InheritFrom inherits unset values from the provided parent configuration and
// returns a new structure instance, which is a result of the inheritance.
func (c *ImageConfig) InheritFrom(parentConfig *ImageConfig) *ImageConfig {
	if c == nil {
		c = &ImageConfig{}
	}
	return shallowMerge(c, parentConfig)
}

func (c *ImageConfig) DNFConfigOptions(osVersion string) []*osbuild.DNFConfigStageOptions {
	if c.DNFConfig == nil {
		return nil
	}
	if c.DNFConfig.SetReleaseVerVar == nil || !*c.DNFConfig.SetReleaseVerVar {
		return c.DNFConfig.Options
	}

	// We currently have no use-case where we set both a custom
	// DNFConfig and DNFSetReleaseVerVar. If we have one this needs
	// to change and we need to decide if we want two dnf
	// configurations or if we want to merge the variable into all
	// existing once (exactly once) and we need to consider what to
	// do about potentially conflicting (manually set) "releasever"
	// values by the user.
	if c.DNFConfig.SetReleaseVerVar != nil && c.DNFConfig.Options != nil {
		err := fmt.Errorf("internal error: currently DNFConfig and DNFSetReleaseVerVar cannot be used together, please reporting this as a feature request")
		panic(err)
	}
	return []*osbuild.DNFConfigStageOptions{
		osbuild.NewDNFConfigStageOptions(
			[]osbuild.DNFVariable{
				{
					Name:  "releasever",
					Value: osVersion,
				},
			},
			nil,
		),
	}
}

type Sysconfig struct {
	Networking bool `yaml:"networking,omitempty"`
	NoZeroConf bool `yaml:"no_zero_conf,omitempty"`

	CreateDefaultNetworkScripts bool `yaml:"create_default_network_scripts,omitempty"`
}

func (c *ImageConfig) SysconfigStageOptions() []*osbuild.SysconfigStageOptions {
	var opts *osbuild.SysconfigStageOptions

	if c.DefaultKernel != nil {
		if opts == nil {
			opts = &osbuild.SysconfigStageOptions{}
		}
		if opts.Kernel == nil {
			opts.Kernel = &osbuild.SysconfigKernelOptions{}
		}
		opts.Kernel.DefaultKernel = *c.DefaultKernel
	}
	if c.UpdateDefaultKernel != nil {
		if opts == nil {
			opts = &osbuild.SysconfigStageOptions{}
		}
		if opts.Kernel == nil {
			opts.Kernel = &osbuild.SysconfigKernelOptions{}
		}
		opts.Kernel.UpdateDefault = *c.UpdateDefaultKernel
	}
	if c.Sysconfig != nil {
		if c.Sysconfig.Networking {
			if opts == nil {
				opts = &osbuild.SysconfigStageOptions{}
			}
			if opts.Network == nil {
				opts.Network = &osbuild.SysconfigNetworkOptions{}
			}
			opts.Network.Networking = c.Sysconfig.Networking
			opts.Network.NoZeroConf = c.Sysconfig.NoZeroConf
			if c.Sysconfig.CreateDefaultNetworkScripts {
				opts.NetworkScripts = &osbuild.NetworkScriptsOptions{
					IfcfgFiles: map[string]osbuild.IfcfgFile{
						"eth0": {
							Device:    "eth0",
							Bootproto: osbuild.IfcfgBootprotoDHCP,
							OnBoot:    common.ToPtr(true),
							Type:      osbuild.IfcfgTypeEthernet,
							UserCtl:   common.ToPtr(true),
							PeerDNS:   common.ToPtr(true),
							IPv6Init:  common.ToPtr(false),
						},
					},
				}
			}
		}
	}

	if opts == nil {
		return nil
	}
	return []*osbuild.SysconfigStageOptions{opts}
}
