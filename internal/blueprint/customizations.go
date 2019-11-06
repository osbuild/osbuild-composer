package blueprint

type Customizations struct {
	Hostname *string                `json:"hostname,omitempty"`
	Kernel   *KernelCustomization   `json:"kernel,omitempty"`
	SSHKey   []SSHKeyCustomization  `json:"sshkey,omitempty"`
	User     []UserCustomization    `json:"user,omitempty"`
	Group    []GroupCustomization   `json:"group,omitempty"`
	Timezone *TimezoneCustomization `json:"timezone,omitempty"`
	Locale   *LocaleCustomization   `json:"locale,omitempty"`
	Firewall *FirewallCustomization `json:"firewall,omitempty"`
	Services *ServicesCustomization `json:"services,omitempty"`
}

type KernelCustomization struct {
	Append string `json:"append"`
}

type SSHKeyCustomization struct {
	User string `json:"user"`
	Key  string `json:"key"`
}

type UserCustomization struct {
	Name        string   `json:"name"`
	Description *string  `json:"description,omitempty"`
	Password    *string  `json:"password,omitempty"`
	Key         *string  `json:"key,omitempty"`
	Home        *string  `json:"home,omitempty"`
	Shell       *string  `json:"shell,omitempty"`
	Groups      []string `json:"groups,omitempty"`
	UID         *int     `json:"uid,omitempty"`
	GID         *int     `json:"gid,omitempty"`
}

type GroupCustomization struct {
	Name string `json:"name"`
	GID  *int   `json:"gid,omitempty"`
}

type TimezoneCustomization struct {
	Timezone   *string  `json:"timezone,omitempty"`
	NTPServers []string `json:"ntpservers,omitempty"`
}

type LocaleCustomization struct {
	Languages []string `json:"languages,omitempty"`
	Keyboard  *string  `json:"keyboard,omitempty"`
}

type FirewallCustomization struct {
	Ports    []string                       `json:"ports,omitempty"`
	Services *FirewallServicesCustomization `json:"services,omitempty"`
}

type FirewallServicesCustomization struct {
	Enabled  []string `json:"enabled,omitempty"`
	Disabled []string `json:"disabled,omitempty"`
}

type ServicesCustomization struct {
	Enabled  []string `json:"enabled,omitempty"`
	Disabled []string `json:"disabled,omitempty"`
}

type CustomizationError struct {
	Message string
}

func (e *CustomizationError) Error() string {
	return e.Message
}
