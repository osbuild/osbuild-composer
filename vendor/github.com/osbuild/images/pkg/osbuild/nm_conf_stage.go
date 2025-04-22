package osbuild

import "fmt"

const nmConfStageType = "org.osbuild.nm.conf"

type NMConfStageOptions struct {
	Path     string              `json:"path"`
	Settings NMConfStageSettings `json:"settings"`
}

func (*NMConfStageOptions) isStageOptions() {}

type NMConfStageSettings struct {
	Main            *NMConfSettingsMain             `json:"main,omitempty"`
	Device          []NMConfSettingsDevice          `json:"device,omitempty"`
	GlobalDNSDomain []NMConfSettingsGlobalDNSDomain `json:"global-dns-domain,omitempty"`
	Keyfile         *NMConfSettingsKeyfile          `json:"keyfile,omitempty"`
}

type NMConfSettingsMain struct {
	NoAutoDefault []string `json:"no-auto-default,omitempty"`
	Plugins       []string `json:"plugins,omitempty"`
}

type NMConfSettingsDevice struct {
	Name   string             `json:"name,omitempty"`
	Config NMConfDeviceConfig `json:"config"`
}

type NMConfSettingsGlobalDNSDomain struct {
	Name   string                              `json:"name"`
	Config NMConfSettingsGlobalDNSDomainConfig `json:"config"`
}

type NMConfSettingsGlobalDNSDomainConfig struct {
	Servers []string `json:"servers"`
}

type NMConfSettingsKeyfile struct {
	UnmanagedDevices []string `json:"unmanaged-devices,omitempty"`
}

type NMConfDeviceConfig struct {
	Managed                bool `json:"managed"`
	WifiScanRandMacAddress bool `json:"wifi.scan-rand-mac-address"`
}

func (o NMConfStageOptions) validate() error {
	if o.Path == "" {
		return fmt.Errorf("%s: path is a required property", nmConfStageType)
	}

	if err := validatePath(o.Path); err != nil {
		return fmt.Errorf("%s: %s", nmConfStageType, err)
	}

	settings := o.Settings
	// at least one of the settings must be defined
	if settings.Main == nil &&
		settings.Device == nil &&
		settings.GlobalDNSDomain == nil &&
		settings.Keyfile == nil {
		return fmt.Errorf("%s: at least one setting must be set", nmConfStageType)
	}

	if main := settings.Main; main != nil {
		if err := validateArrayHasItems(main.NoAutoDefault); err != nil {
			return fmt.Errorf("%s: main.no-auto-default %s", nmConfStageType, err)
		}
		if err := validateArrayHasItems(main.Plugins); err != nil {
			return fmt.Errorf("%s: main.plugins %s", nmConfStageType, err)
		}
	}

	if err := validateArrayHasItems(settings.Device); err != nil {
		return fmt.Errorf("%s: device %s", nmConfStageType, err)
	}

	if err := validateArrayHasItems(settings.GlobalDNSDomain); err != nil {
		return fmt.Errorf("%s: global-dns-domain %s", nmConfStageType, err)
	}

	for _, gdd := range settings.GlobalDNSDomain {
		if gdd.Name == "" {
			return fmt.Errorf("%s: global-dns-domain name is a required property", nmConfStageType)
		}

		if err := validateArrayHasItems(gdd.Config.Servers); err != nil {
			return fmt.Errorf("%s: global-dns-domain.config.servers %s", nmConfStageType, err)
		}
	}

	if keyfile := settings.Keyfile; keyfile != nil {
		if err := validateArrayHasItems(keyfile.UnmanagedDevices); err != nil {
			return fmt.Errorf("%s: keyfile.unmanaged-devices %s", nmConfStageType, err)
		}
	}

	return nil
}

// validateArrayHasItems returns an error if an array is non-nil but also
// contains no items.
func validateArrayHasItems[T any](arr []T) error {
	if arr != nil && len(arr) == 0 {
		return fmt.Errorf("requires at least one element when defined")
	}
	return nil
}

func NewNMConfStage(options *NMConfStageOptions) *Stage {
	return &Stage{
		Type:    nmConfStageType,
		Options: options,
	}
}
