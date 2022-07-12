package distro

import "github.com/osbuild/osbuild-composer/internal/osbuild"

type RHSMSubscriptionStatus string

const (
	RHSMConfigWithSubscription RHSMSubscriptionStatus = "with-subscription"
	RHSMConfigNoSubscription   RHSMSubscriptionStatus = "no-subscription"
)

// ImageConfig represents a (default) configuration applied to the image
type ImageConfig struct {
	Timezone            string
	TimeSynchronization *osbuild.ChronyStageOptions
	Locale              string
	Keyboard            *osbuild.KeymapStageOptions
	EnabledServices     []string
	DisabledServices    []string
	DefaultTarget       string
	Sysconfig           []*osbuild.SysconfigStageOptions

	// List of files from which to import GPG keys into the RPM database
	GPGKeyFiles []string

	// Disable SELinux labelling
	NoSElinux bool

	// Disable documentation
	ExcludeDocs bool

	// for RHSM configuration, we need to potentially distinguish the case
	// when the user want the image to be subscribed on first boot and when not
	RHSMConfig         map[RHSMSubscriptionStatus]*osbuild.RHSMStageOptions
	SystemdLogind      []*osbuild.SystemdLogindStageOptions
	CloudInit          []*osbuild.CloudInitStageOptions
	Modprobe           []*osbuild.ModprobeStageOptions
	DracutConf         []*osbuild.DracutConfStageOptions
	SystemdUnit        []*osbuild.SystemdUnitStageOptions
	Authselect         *osbuild.AuthselectStageOptions
	SELinuxConfig      *osbuild.SELinuxConfigStageOptions
	Tuned              *osbuild.TunedStageOptions
	Tmpfilesd          []*osbuild.TmpfilesdStageOptions
	PamLimitsConf      []*osbuild.PamLimitsConfStageOptions
	Sysctld            []*osbuild.SysctldStageOptions
	DNFConfig          []*osbuild.DNFConfigStageOptions
	SshdConfig         *osbuild.SshdConfigStageOptions
	Authconfig         *osbuild.AuthconfigStageOptions
	PwQuality          *osbuild.PwqualityConfStageOptions
	WAAgentConfig      *osbuild.WAAgentConfStageOptions
	Grub2Config        *osbuild.GRUB2Config
	DNFAutomaticConfig *osbuild.DNFAutomaticConfigStageOptions
	YumConfig          *osbuild.YumConfigStageOptions
	YUMRepos           []*osbuild.YumReposStageOptions
	Firewall           *osbuild.FirewallStageOptions
	UdevRules          *osbuild.UdevRulesStageOptions
}

// InheritFrom inherits unset values from the provided parent configuration and
// returns a new structure instance, which is a result of the inheritance.
func (c *ImageConfig) InheritFrom(parentConfig *ImageConfig) *ImageConfig {
	finalConfig := ImageConfig(*c)
	if parentConfig != nil {
		if finalConfig.Timezone == "" {
			finalConfig.Timezone = parentConfig.Timezone
		}
		if finalConfig.TimeSynchronization == nil {
			finalConfig.TimeSynchronization = parentConfig.TimeSynchronization
		}
		if finalConfig.Locale == "" {
			finalConfig.Locale = parentConfig.Locale
		}
		if finalConfig.Keyboard == nil {
			finalConfig.Keyboard = parentConfig.Keyboard
		}
		if finalConfig.EnabledServices == nil {
			finalConfig.EnabledServices = parentConfig.EnabledServices
		}
		if finalConfig.DisabledServices == nil {
			finalConfig.DisabledServices = parentConfig.DisabledServices
		}
		if finalConfig.DefaultTarget == "" {
			finalConfig.DefaultTarget = parentConfig.DefaultTarget
		}
		if finalConfig.Sysconfig == nil {
			finalConfig.Sysconfig = parentConfig.Sysconfig
		}
		if finalConfig.GPGKeyFiles == nil {
			finalConfig.GPGKeyFiles = parentConfig.GPGKeyFiles
		}
		if finalConfig.RHSMConfig == nil {
			finalConfig.RHSMConfig = parentConfig.RHSMConfig
		}
		if finalConfig.SystemdLogind == nil {
			finalConfig.SystemdLogind = parentConfig.SystemdLogind
		}
		if finalConfig.CloudInit == nil {
			finalConfig.CloudInit = parentConfig.CloudInit
		}
		if finalConfig.Modprobe == nil {
			finalConfig.Modprobe = parentConfig.Modprobe
		}
		if finalConfig.DracutConf == nil {
			finalConfig.DracutConf = parentConfig.DracutConf
		}
		if finalConfig.SystemdUnit == nil {
			finalConfig.SystemdUnit = parentConfig.SystemdUnit
		}
		if finalConfig.Authselect == nil {
			finalConfig.Authselect = parentConfig.Authselect
		}
		if finalConfig.SELinuxConfig == nil {
			finalConfig.SELinuxConfig = parentConfig.SELinuxConfig
		}
		if finalConfig.Tuned == nil {
			finalConfig.Tuned = parentConfig.Tuned
		}
		if finalConfig.Tmpfilesd == nil {
			finalConfig.Tmpfilesd = parentConfig.Tmpfilesd
		}
		if finalConfig.PamLimitsConf == nil {
			finalConfig.PamLimitsConf = parentConfig.PamLimitsConf
		}
		if finalConfig.Sysctld == nil {
			finalConfig.Sysctld = parentConfig.Sysctld
		}
		if finalConfig.DNFConfig == nil {
			finalConfig.DNFConfig = parentConfig.DNFConfig
		}
		if finalConfig.SshdConfig == nil {
			finalConfig.SshdConfig = parentConfig.SshdConfig
		}
		if finalConfig.Authconfig == nil {
			finalConfig.Authconfig = parentConfig.Authconfig
		}
		if finalConfig.PwQuality == nil {
			finalConfig.PwQuality = parentConfig.PwQuality
		}
		if finalConfig.DNFAutomaticConfig == nil {
			finalConfig.DNFAutomaticConfig = parentConfig.DNFAutomaticConfig
		}
		if finalConfig.YumConfig == nil {
			finalConfig.YumConfig = parentConfig.YumConfig
		}
		if finalConfig.YUMRepos == nil {
			finalConfig.YUMRepos = parentConfig.YUMRepos
		}
		if finalConfig.Firewall == nil {
			finalConfig.Firewall = parentConfig.Firewall
		}
		if finalConfig.UdevRules == nil {
			finalConfig.UdevRules = parentConfig.UdevRules
		}
	}
	return &finalConfig
}
