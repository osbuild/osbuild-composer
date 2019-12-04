package blueprint

type Customizations struct {
	Hostname *string                `json:"hostname,omitempty" toml:"hostname,omitempty"`
	Kernel   *KernelCustomization   `json:"kernel,omitempty" toml:"kernel,omitempty"`
	SSHKey   []SSHKeyCustomization  `json:"sshkey,omitempty" toml:"sshkey,omitempty"`
	User     []UserCustomization    `json:"user,omitempty" toml:"user,omitempty"`
	Group    []GroupCustomization   `json:"group,omitempty" toml:"group,omitempty"`
	Timezone *TimezoneCustomization `json:"timezone,omitempty" toml:"timezone,omitempty"`
	Locale   *LocaleCustomization   `json:"locale,omitempty" toml:"locale,omitempty"`
	Firewall *FirewallCustomization `json:"firewall,omitempty" toml:"firewall,omitempty"`
	Services *ServicesCustomization `json:"services,omitempty" toml:"services,omitempty"`
}

type KernelCustomization struct {
	Append string `json:"append" toml:"append"`
}

type SSHKeyCustomization struct {
	User string `json:"user" toml:"user"`
	Key  string `json:"key" toml:"key"`
}

type UserCustomization struct {
	Name        string   `json:"name" toml:"name"`
	Description *string  `json:"description,omitempty" toml:"description,omitempty"`
	Password    *string  `json:"password,omitempty" toml:"password,omitempty"`
	Key         *string  `json:"key,omitempty" toml:"key,omitempty"`
	Home        *string  `json:"home,omitempty" toml:"home,omitempty"`
	Shell       *string  `json:"shell,omitempty" toml:"shell,omitempty"`
	Groups      []string `json:"groups,omitempty" toml:"groups,omitempty"`
	UID         *int     `json:"uid,omitempty" toml:"uid,omitempty"`
	GID         *int     `json:"gid,omitempty" toml:"gid,omitempty"`
}

type GroupCustomization struct {
	Name string `json:"name" toml:"name"`
	GID  *int   `json:"gid,omitempty" toml:"gid,omitempty"`
}

type TimezoneCustomization struct {
	Timezone   *string  `json:"timezone,omitempty" toml:"timezone,omitempty"`
	NTPServers []string `json:"ntpservers,omitempty" toml:"ntpservers,omitempty"`
}

type LocaleCustomization struct {
	Languages []string `json:"languages,omitempty" toml:"languages,omitempty"`
	Keyboard  *string  `json:"keyboard,omitempty" toml:"keyboard,omitempty"`
}

type FirewallCustomization struct {
	Ports    []string                       `json:"ports,omitempty" toml:"ports,omitempty"`
	Services *FirewallServicesCustomization `json:"services,omitempty" toml:"services,omitempty"`
}

type FirewallServicesCustomization struct {
	Enabled  []string `json:"enabled,omitempty" toml:"enabled,omitempty"`
	Disabled []string `json:"disabled,omitempty" toml:"disabled,omitempty"`
}

type ServicesCustomization struct {
	Enabled  []string `json:"enabled,omitempty" toml:"enabled,omitempty"`
	Disabled []string `json:"disabled,omitempty" toml:"disabled,omitempty"`
}

type CustomizationError struct {
	Message string
}

func (e *CustomizationError) Error() string {
	return e.Message
}
