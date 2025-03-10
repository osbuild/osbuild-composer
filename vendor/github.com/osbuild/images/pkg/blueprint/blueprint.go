// Package blueprint contains primitives for representing weldr blueprints
package blueprint

// A Blueprint is a high-level description of an image.
type Blueprint struct {
	Name        string    `json:"name" toml:"name"`
	Description string    `json:"description" toml:"description"`
	Version     string    `json:"version,omitempty" toml:"version,omitempty"`
	Packages    []Package `json:"packages" toml:"packages"`
	Modules     []Package `json:"modules" toml:"modules"`

	// Note, this is called "enabled modules" because we already have "modules" except
	// the "modules" refers to packages and "enabled modules" refers to modularity modules.
	EnabledModules []EnabledModule `json:"enabled_modules" toml:"enabled_modules"`

	Groups         []Group         `json:"groups" toml:"groups"`
	Containers     []Container     `json:"containers,omitempty" toml:"containers,omitempty"`
	Customizations *Customizations `json:"customizations,omitempty" toml:"customizations"`
	Distro         string          `json:"distro" toml:"distro"`

	// EXPERIMENTAL
	Minimal bool `json:"minimal" toml:"minimal"`
}

// A Package specifies an RPM package.
type Package struct {
	Name    string `json:"name" toml:"name"`
	Version string `json:"version,omitempty" toml:"version,omitempty"`
}

// A module specifies a modularity stream.
type EnabledModule struct {
	Name   string `json:"name" toml:"name"`
	Stream string `json:"stream,omitempty" toml:"stream,omitempty"`
}

// A group specifies an package group.
type Group struct {
	Name string `json:"name" toml:"name"`
}

type Container struct {
	Source string `json:"source" toml:"source"`
	Name   string `json:"name,omitempty" toml:"name,omitempty"`

	TLSVerify    *bool `json:"tls-verify,omitempty" toml:"tls-verify,omitempty"`
	LocalStorage bool  `json:"local-storage,omitempty" toml:"local-storage,omitempty"`
}

// packages, modules, and groups all resolve to rpm packages right now. This
// function returns a combined list of "name-version" strings.
func (b *Blueprint) GetPackages() []string {
	return b.GetPackagesEx(true)
}

func (b *Blueprint) GetPackagesEx(bootable bool) []string {
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

	if bootable {
		kc := b.Customizations.GetKernel()
		kpkg := Package{Name: kc.Name}
		packages = append(packages, kpkg.ToNameVersion())
	}

	return packages
}

func (b *Blueprint) GetEnabledModules() []string {
	modules := []string{}

	for _, mod := range b.EnabledModules {
		modules = append(modules, mod.ToNameStream())
	}

	return modules
}

func (p Package) ToNameVersion() string {
	// Omit version to prevent all packages with prefix of name to be installed
	if p.Version == "*" || p.Version == "" {
		return p.Name
	}

	return p.Name + "-" + p.Version
}

func (p EnabledModule) ToNameStream() string {
	return p.Name + ":" + p.Stream
}
