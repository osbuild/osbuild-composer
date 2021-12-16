package distro

import "github.com/osbuild/osbuild-composer/internal/osbuild2"

type RHSMSubscriptionStatus string

const (
	RHSMConfigWithSubscription RHSMSubscriptionStatus = "with-subscription"
	RHSMConfigNoSubscription   RHSMSubscriptionStatus = "no-subscription"
)

// ImageConfig represents a (default) configuration applied to the image
type ImageConfig struct {
	Timezone            string
	TimeSynchronization *osbuild2.ChronyStageOptions
	Locale              string
	Keyboard            *osbuild2.KeymapStageOptions
	EnabledServices     []string
	DisabledServices    []string
	DefaultTarget       string
	Sysconfig           []*osbuild2.SysconfigStageOptions

	// for RHSM configuration, we need to potentially distinguish the case
	// when the user want the image to be subscribed on first boot and when not
	RHSMConfig map[RHSMSubscriptionStatus]*osbuild2.RHSMStageOptions
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
		if finalConfig.RHSMConfig == nil {
			finalConfig.RHSMConfig = parentConfig.RHSMConfig
		}
	}
	return &finalConfig
}
