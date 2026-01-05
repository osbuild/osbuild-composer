// Package blueprint contains primitives for representing weldr blueprints
package blueprint

import (
	"encoding/json"
	"fmt"

	"github.com/osbuild/images/pkg/crypt"

	"github.com/coreos/go-semver/semver"
)

// A Blueprint is a high-level description of an image.
type Blueprint struct {
	Name        string    `json:"name,omitempty" toml:"name,omitempty"`
	Description string    `json:"description,omitempty" toml:"description,omitempty"`
	Version     string    `json:"version,omitempty" toml:"version,omitempty"`
	Packages    []Package `json:"packages" toml:"packages"`
	Modules     []Package `json:"modules" toml:"modules"`

	// Note, this is called "enabled modules" because we already have "modules" except
	// the "modules" refers to packages and "enabled modules" refers to modularity modules.
	EnabledModules []EnabledModule `json:"enabled_modules" toml:"enabled_modules"`

	Groups         []Group         `json:"groups" toml:"groups"`
	Containers     []Container     `json:"containers,omitempty" toml:"containers,omitempty"`
	Customizations *Customizations `json:"customizations,omitempty" toml:"customizations,omitempty"`
	Distro         string          `json:"distro,omitempty" toml:"distro,omitempty"`
	Arch           string          `json:"architecture,omitempty" toml:"architecture,omitempty"`
}

type Change struct {
	Commit    string    `json:"commit" toml:"commit"`
	Message   string    `json:"message" toml:"message"`
	Revision  *int      `json:"revision" toml:"revision"`
	Timestamp string    `json:"timestamp" toml:"timestamp"`
	Blueprint Blueprint `json:"-" toml:"-"`
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

// DeepCopy returns a deep copy of the blueprint
// This uses json.Marshal and Unmarshal which are not very efficient
func (b *Blueprint) DeepCopy() Blueprint {
	bpJSON, err := json.Marshal(b)
	if err != nil {
		panic(err)
	}

	var bp Blueprint
	err = json.Unmarshal(bpJSON, &bp)
	if err != nil {
		panic(err)
	}
	return bp
}

// Initialize ensures that the blueprint has sane defaults for any missing fields
func (b *Blueprint) Initialize() error {
	if len(b.Name) == 0 {
		return fmt.Errorf("empty blueprint name not allowed")
	}

	if b.Packages == nil {
		b.Packages = []Package{}
	}
	if b.Modules == nil {
		b.Modules = []Package{}
	}
	if b.EnabledModules == nil {
		b.EnabledModules = []EnabledModule{}
	}
	if b.Groups == nil {
		b.Groups = []Group{}
	}
	if b.Containers == nil {
		b.Containers = []Container{}
	}
	if b.Version == "" {
		b.Version = "0.0.0"
	}
	// Return an error if the version is not valid
	_, err := semver.NewVersion(b.Version)
	if err != nil {
		return fmt.Errorf("Invalid 'version', must use Semantic Versioning: %s", err.Error())
	}

	err = b.CryptPasswords()
	if err != nil {
		return fmt.Errorf("Error hashing passwords: %s", err.Error())
	}

	for i, pkg := range b.Packages {
		if pkg.Name == "" {
			var errMsg string
			if pkg.Version == "" {
				errMsg = fmt.Sprintf("Entry #%d has no name.", i+1)
			} else {
				errMsg = fmt.Sprintf("Entry #%d has version '%v' but no name.", i+1, pkg.Version)
			}
			return fmt.Errorf("All package entries need to contain the name of the package. %s", errMsg)
		}
	}

	return nil
}

// BumpVersion increments the previous blueprint's version
// If the old version string is not vaild semver it will use the new version as-is
// This assumes that the new blueprint's version has already been validated via Initialize
func (b *Blueprint) BumpVersion(old string) {
	var ver *semver.Version
	ver, err := semver.NewVersion(old)
	if err != nil {
		return
	}

	ver.BumpPatch()
	b.Version = ver.String()
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

func (p Package) ToNameVersion() string {
	// Omit version to prevent all packages with prefix of name to be installed
	if p.Version == "*" || p.Version == "" {
		return p.Name
	}

	return p.Name + "-" + p.Version
}

func (b *Blueprint) GetEnabledModules() []string {
	modules := []string{}

	for _, mod := range b.EnabledModules {
		modules = append(modules, mod.ToNameStream())
	}

	return modules
}

func (p EnabledModule) ToNameStream() string {
	return p.Name + ":" + p.Stream
}

// CryptPasswords ensures that all blueprint passwords are hashed
func (b *Blueprint) CryptPasswords() error {
	if b.Customizations == nil {
		return nil
	}

	// Any passwords for users?
	for i := range b.Customizations.User {
		// Missing or empty password
		if b.Customizations.User[i].Password == nil {
			continue
		}

		// Prevent empty password from being hashed
		if len(*b.Customizations.User[i].Password) == 0 {
			b.Customizations.User[i].Password = nil
			continue
		}

		if !crypt.PasswordIsCrypted(*b.Customizations.User[i].Password) {
			pw, err := crypt.CryptSHA512(*b.Customizations.User[i].Password)
			if err != nil {
				return err
			}

			// Replace the password with the
			b.Customizations.User[i].Password = &pw
		}
	}

	return nil
}
