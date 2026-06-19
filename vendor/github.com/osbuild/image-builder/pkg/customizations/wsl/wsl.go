package wsl

type WSLConfig struct {
	BootSystemd bool `yaml:"boot_systemd,omitempty"`
}

type WSLDistributionOOBEConfig struct {
	DefaultUID  *int   `yaml:"default_uid,omitempty"`
	DefaultName string `yaml:"default_name,omitempty"`
}

type WSLDistributionShortcutConfig struct {
	Enabled bool   `yaml:"enabled,omitempty"`
	Icon    string `yaml:"icon,omitempty"`
}

type WSLDistributionConfig struct {
	OOBE     *WSLDistributionOOBEConfig     `yaml:"oobe,omitempty"`
	Shortcut *WSLDistributionShortcutConfig `yaml:"shortcut,omitempty"`
}

type WSL struct {
	Config             *WSLConfig             `yaml:"config,omitempty"`
	DistributionConfig *WSLDistributionConfig `yaml:"distribution_config,omitempty"`
}
