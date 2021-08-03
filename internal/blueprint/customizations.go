package blueprint

type Customizations struct {
	Hostname           *string                   `json:"hostname,omitempty" toml:"hostname,omitempty"`
	Kernel             *KernelCustomization      `json:"kernel,omitempty" toml:"kernel,omitempty"`
	SSHKey             []SSHKeyCustomization     `json:"sshkey,omitempty" toml:"sshkey,omitempty"`
	User               []UserCustomization       `json:"user,omitempty" toml:"user,omitempty"`
	Group              []GroupCustomization      `json:"group,omitempty" toml:"group,omitempty"`
	Timezone           *TimezoneCustomization    `json:"timezone,omitempty" toml:"timezone,omitempty"`
	Locale             *LocaleCustomization      `json:"locale,omitempty" toml:"locale,omitempty"`
	Firewall           *FirewallCustomization    `json:"firewall,omitempty" toml:"firewall,omitempty"`
	Services           *ServicesCustomization    `json:"services,omitempty" toml:"services,omitempty"`
	Filesystem         []FilesystemCustomization `json:"filesystem,omitempty" toml:"filesystem,omitempty"`
	InstallationDevice string                    `json:"installation_device,omitempty" toml:"installation_device,omitempty"`
}

type KernelCustomization struct {
	Name   string `json:"name,omitempty" toml:"name,omitempty"`
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

type FilesystemCustomization struct {
	Mountpoint string `json:"mountpoint,omitempty" toml:"mountpoint,omitempty"`
	MinSize    int    `json:"minsize,omitempty" toml:"size,omitempty"`
}

type CustomizationError struct {
	Message string
}

func (e *CustomizationError) Error() string {
	return e.Message
}

func (c *Customizations) GetHostname() *string {
	if c == nil {
		return nil
	}
	return c.Hostname
}

func (c *Customizations) GetPrimaryLocale() (*string, *string) {
	if c == nil {
		return nil, nil
	}
	if c.Locale == nil {
		return nil, nil
	}
	if len(c.Locale.Languages) == 0 {
		return nil, c.Locale.Keyboard
	}
	return &c.Locale.Languages[0], c.Locale.Keyboard
}

func (c *Customizations) GetTimezoneSettings() (*string, []string) {
	if c == nil {
		return nil, nil
	}
	if c.Timezone == nil {
		return nil, nil
	}
	return c.Timezone.Timezone, c.Timezone.NTPServers
}

func (c *Customizations) GetUsers() []UserCustomization {
	if c == nil {
		return nil
	}

	users := []UserCustomization{}

	// prepend sshkey for backwards compat (overridden by users)
	if len(c.SSHKey) > 0 {
		for _, c := range c.SSHKey {
			users = append(users, UserCustomization{
				Name: c.User,
				Key:  &c.Key,
			})
		}
	}

	return append(users, c.User...)
}

func (c *Customizations) GetGroups() []GroupCustomization {
	if c == nil {
		return nil
	}

	// This is for parity with lorax, which assumes that for each
	// user, a group with that name already exists. Thus, filter groups
	// named like an existing user.

	groups := []GroupCustomization{}
	for _, group := range c.Group {
		exists := false
		for _, user := range c.User {
			if user.Name == group.Name {
				exists = true
				break
			}
		}
		for _, key := range c.SSHKey {
			if key.User == group.Name {
				exists = true
				break
			}
		}
		if !exists {
			groups = append(groups, group)
		}
	}

	return groups
}

func (c *Customizations) GetKernel() *KernelCustomization {
	var name string
	var append string
	if c != nil && c.Kernel != nil {
		name = c.Kernel.Name
		append = c.Kernel.Append
	}

	if name == "" {
		name = "kernel"
	}

	return &KernelCustomization{
		Name:   name,
		Append: append,
	}
}

func (c *Customizations) GetFirewall() *FirewallCustomization {
	if c == nil {
		return nil
	}

	return c.Firewall
}

func (c *Customizations) GetServices() *ServicesCustomization {
	if c == nil {
		return nil
	}

	return c.Services
}

func (c *Customizations) GetFilesystems() []FilesystemCustomization {
	if c == nil {
		return nil
	}
	return c.Filesystem
}

func (c *Customizations) GetFilesystemsMinSize() uint64 {
	if c == nil {
		return 0
	}
	agg := 0
	for _, m := range c.Filesystem {
		agg += m.MinSize
	}
	// This ensures that file system customization `size` is a multiple of
	// sector size (512)
	if agg%512 != 0 {
		agg = (agg/512 + 1) * 512
	}
	return uint64(agg)
}
