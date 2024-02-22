// Package blueprint contains primitives for representing weldr blueprints
package blueprint

import (
	"encoding/json"
	"fmt"

	"github.com/osbuild/images/pkg/crypt"

	"github.com/coreos/go-semver/semver"
	iblueprint "github.com/osbuild/images/pkg/blueprint"
)

// A Blueprint is a high-level description of an image.
type Blueprint struct {
	Name           string          `json:"name" toml:"name"`
	Description    string          `json:"description" toml:"description"`
	Version        string          `json:"version,omitempty" toml:"version,omitempty"`
	Packages       []Package       `json:"packages" toml:"packages"`
	Modules        []Package       `json:"modules" toml:"modules"`
	Groups         []Group         `json:"groups" toml:"groups"`
	Containers     []Container     `json:"containers,omitempty" toml:"containers,omitempty"`
	Customizations *Customizations `json:"customizations,omitempty" toml:"customizations"`
	Distro         string          `json:"distro" toml:"distro"`
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

// A group specifies an package group.
type Group struct {
	Name string `json:"name" toml:"name"`
}

type Container struct {
	Source string `json:"source,omitempty" toml:"source"`
	Name   string `json:"name,omitempty" toml:"name,omitempty"`

	TLSVerify           *bool   `json:"tls-verify,omitempty" toml:"tls-verify,omitempty"`
	ContainersTransport *string `json:"containers-transport,omitempty" toml:"containers-transport,omitempty"`
	StoragePath         *string `json:"source-path,omitempty" toml:"source-path,omitempty"`
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

func Convert(bp Blueprint) iblueprint.Blueprint {
	var pkgs []iblueprint.Package
	if len(bp.Packages) > 0 {
		pkgs = make([]iblueprint.Package, len(bp.Packages))
		for idx := range bp.Packages {
			pkgs[idx] = iblueprint.Package(bp.Packages[idx])
		}
	}

	var modules []iblueprint.Package
	if len(bp.Modules) > 0 {
		modules = make([]iblueprint.Package, len(bp.Modules))
		for idx := range bp.Modules {
			modules[idx] = iblueprint.Package(bp.Modules[idx])
		}
	}

	var groups []iblueprint.Group
	if len(bp.Groups) > 0 {
		groups = make([]iblueprint.Group, len(bp.Groups))
		for idx := range bp.Groups {
			groups[idx] = iblueprint.Group(bp.Groups[idx])
		}
	}

	var containers []iblueprint.Container

	if len(bp.Containers) > 0 {
		containers = make([]iblueprint.Container, len(bp.Containers))
		for idx := range bp.Containers {
			containers[idx] = iblueprint.Container(bp.Containers[idx])
		}
	}

	var customizations *iblueprint.Customizations
	if c := bp.Customizations; c != nil {
		customizations = &iblueprint.Customizations{
			Hostname:           c.Hostname,
			InstallationDevice: c.InstallationDevice,
		}

		if fdo := c.FDO; fdo != nil {
			ifdo := iblueprint.FDOCustomization(*fdo)
			customizations.FDO = &ifdo
		}
		if oscap := c.OpenSCAP; oscap != nil {
			ioscap := iblueprint.OpenSCAPCustomization{
				DataStream: oscap.DataStream,
				ProfileID:  oscap.ProfileID,
			}
			if tailoring := oscap.Tailoring; tailoring != nil {
				itailoring := iblueprint.OpenSCAPTailoringCustomizations(*tailoring)
				ioscap.Tailoring = &itailoring
			}
			customizations.OpenSCAP = &ioscap
		}
		if ign := c.Ignition; ign != nil {
			iign := iblueprint.IgnitionCustomization{}
			if embed := ign.Embedded; embed != nil {
				iembed := iblueprint.EmbeddedIgnitionCustomization(*embed)
				iign.Embedded = &iembed
			}
			if fb := ign.FirstBoot; fb != nil {
				ifb := iblueprint.FirstBootIgnitionCustomization(*fb)
				iign.FirstBoot = &ifb
			}
			customizations.Ignition = &iign
		}
		if dirs := c.Directories; dirs != nil {
			idirs := make([]iblueprint.DirectoryCustomization, len(dirs))
			for idx := range dirs {
				idirs[idx] = iblueprint.DirectoryCustomization(dirs[idx])
			}
			customizations.Directories = idirs
		}
		if files := c.Files; files != nil {
			ifiles := make([]iblueprint.FileCustomization, len(files))
			for idx := range files {
				ifiles[idx] = iblueprint.FileCustomization(files[idx])
			}
			customizations.Files = ifiles
		}
		if repos := c.Repositories; repos != nil {
			irepos := make([]iblueprint.RepositoryCustomization, len(repos))
			for idx := range repos {
				irepos[idx] = iblueprint.RepositoryCustomization(repos[idx])
			}
			customizations.Repositories = irepos
		}
		if kernel := c.Kernel; kernel != nil {
			ikernel := iblueprint.KernelCustomization(*kernel)
			customizations.Kernel = &ikernel
		}
		if sshkeys := c.SSHKey; sshkeys != nil {
			isshkeys := make([]iblueprint.SSHKeyCustomization, len(sshkeys))
			for idx := range sshkeys {
				isshkeys[idx] = iblueprint.SSHKeyCustomization(sshkeys[idx])
			}
			customizations.SSHKey = isshkeys
		}
		if users := c.User; users != nil {
			iusers := make([]iblueprint.UserCustomization, len(users))
			for idx := range users {
				iusers[idx] = iblueprint.UserCustomization(users[idx])
			}
			customizations.User = iusers
		}
		if groups := c.Group; groups != nil {
			igroups := make([]iblueprint.GroupCustomization, len(groups))
			for idx := range groups {
				igroups[idx] = iblueprint.GroupCustomization(groups[idx])
			}
			customizations.Group = igroups
		}
		if fs := c.Filesystem; fs != nil {
			ifs := make([]iblueprint.FilesystemCustomization, len(fs))
			for idx := range fs {
				ifs[idx] = iblueprint.FilesystemCustomization(fs[idx])
			}
			customizations.Filesystem = ifs
		}
		if tz := c.Timezone; tz != nil {
			itz := iblueprint.TimezoneCustomization(*tz)
			customizations.Timezone = &itz
		}
		if locale := c.Locale; locale != nil {
			ilocale := iblueprint.LocaleCustomization(*locale)
			customizations.Locale = &ilocale
		}
		if fw := c.Firewall; fw != nil {
			ifw := iblueprint.FirewallCustomization{
				Ports: fw.Ports,
			}
			if services := fw.Services; services != nil {
				iservices := iblueprint.FirewallServicesCustomization(*services)
				ifw.Services = &iservices
			}
			if zones := fw.Zones; zones != nil {
				izones := make([]iblueprint.FirewallZoneCustomization, len(zones))
				for idx := range zones {
					izones[idx] = iblueprint.FirewallZoneCustomization(zones[idx])
				}
				ifw.Zones = izones
			}
			customizations.Firewall = &ifw
		}
		if services := c.Services; services != nil {
			iservices := iblueprint.ServicesCustomization(*services)
			customizations.Services = &iservices
		}
		if fips := c.FIPS; fips != nil {
			customizations.FIPS = fips
		}
	}

	ibp := iblueprint.Blueprint{
		Name:           bp.Name,
		Description:    bp.Description,
		Version:        bp.Version,
		Packages:       pkgs,
		Modules:        modules,
		Groups:         groups,
		Containers:     containers,
		Customizations: customizations,
		Distro:         bp.Distro,
	}

	return ibp
}
