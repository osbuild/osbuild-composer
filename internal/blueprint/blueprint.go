// Package blueprint contains primitives for representing weldr blueprints
package blueprint

// A Blueprint is a high-level description of an image.
type Blueprint struct {
	Name           string          `json:"name" toml:"name"`
	Description    string          `json:"description" toml:"description"`
	Version        string          `json:"version,omitempty" toml:"version,omitempty"`
	Packages       []Package       `json:"packages" toml:"packages"`
	Modules        []Package       `json:"modules" toml:"modules"`
	Groups         []Group         `json:"groups" toml:"groups"`
	Customizations *Customizations `json:"customizations,omitempty" toml:"customizations,omitempty"`
}

type Change struct {
	Commit    string    `json:"commit" toml:"commit"`
	Message   string    `json:"message" toml:"message"`
	Revision  *string   `json:"revision" toml:"revision"`
	Timestamp string    `json:"timestamp" toml:"timestamp"`
	Blueprint Blueprint `json:"-" toml:"-"`
}

// A Package specifies an RPM package.
type Package struct {
	Name    string `json:"name" toml:"name"`
	Version string `json:"version,omitempty" toml:"version,omitempty"`
}

// A group specifies an package group.
type Group struct {
	Name string `json:"name" toml:"name"`
}

// packages, modules, and groups all resolve to rpm packages right now. This
// function returns a combined list of "name-version" strings.
func (b *Blueprint) GetPackages() []string {
	packages := []string{}
	for _, pkg := range b.Packages {
		packages = append(packages, pkg.ToNameVersion())
	}
	for _, pkg := range b.Modules {
		packages = append(packages, pkg.ToNameVersion())
	}
	for _, group := range b.Groups {
		packages = append(packages, "@"+group.Name)
	}
	return packages
}

func (b *Blueprint) GetHostname() *string {
	if b.Customizations == nil {
		return nil
	}
	return b.Customizations.Hostname
}

func (b *Blueprint) GetPrimaryLocale() (*string, *string) {
	if b.Customizations == nil {
		return nil, nil
	}
	if b.Customizations.Locale == nil {
		return nil, nil
	}
	if len(b.Customizations.Locale.Languages) == 0 {
		return nil, b.Customizations.Locale.Keyboard
	}
	return &b.Customizations.Locale.Languages[0], b.Customizations.Locale.Keyboard
}

func (b *Blueprint) GetTimezoneSettings() (*string, []string) {
	if b.Customizations == nil {
		return nil, nil
	}
	if b.Customizations.Timezone == nil {
		return nil, nil
	}
	return b.Customizations.Timezone.Timezone, b.Customizations.Timezone.NTPServers
}

func (b *Blueprint) GetUsers() []UserCustomization {
	if b.Customizations == nil {
		return nil
	}

	users := []UserCustomization{}

	// prepend sshkey for backwards compat (overridden by users)
	if len(b.Customizations.SSHKey) > 0 {
		for _, c := range b.Customizations.SSHKey {
			users = append(users, UserCustomization{
				Name: c.User,
				Key:  &c.Key,
			})
		}
	}

	return append(users, b.Customizations.User...)
}

func (b *Blueprint) GetGroups() []GroupCustomization {
	if b.Customizations == nil {
		return nil
	}

	// This is for parity with lorax, which assumes that for each
	// user, a group with that name already exists. Thus, filter groups
	// named like an existing user.

	groups := []GroupCustomization{}
	for _, group := range b.Customizations.Group {
		exists := false
		for _, user := range b.Customizations.User {
			if user.Name == group.Name {
				exists = true
				break
			}
		}
		for _, key := range b.Customizations.SSHKey {
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

func (b *Blueprint) GetKernel() *KernelCustomization {
	if b.Customizations == nil {
		return nil
	}

	return b.Customizations.Kernel
}

func (b *Blueprint) GetFirewall() *FirewallCustomization {
	if b.Customizations == nil {
		return nil
	}

	return b.Customizations.Firewall
}

func (b *Blueprint) GetServices() *ServicesCustomization {
	if b.Customizations == nil {
		return nil
	}

	return b.Customizations.Services
}

func (p Package) ToNameVersion() string {
	// Omit version to prevent all packages with prefix of name to be installed
	if p.Version == "*" {
		return p.Name
	}

	return p.Name + "-" + p.Version
}
