// Package blueprint contains primitives for representing weldr blueprints
package blueprint

import (
	"encoding/json"
	"fmt"

	"github.com/coreos/go-semver/semver"
)

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

// DeepCopy returns a deep copy of the blueprint
// This uses json.Marshal and Unmarshal which are not very efficient
func (b *Blueprint) DeepCopy() (Blueprint, error) {
	bpJSON, err := json.Marshal(b)
	if err != nil {
		panic(err)
	}

	var bp Blueprint
	err = json.Unmarshal(bpJSON, &bp)
	if err != nil {
		panic(err)
	}
	return bp, nil
}

// Initialize ensures that the blueprint has sane defaults for any missing fields
func (b *Blueprint) Initialize() error {
	if b.Packages == nil {
		b.Packages = []Package{}
	}
	if b.Modules == nil {
		b.Modules = []Package{}
	}
	if b.Groups == nil {
		b.Groups = []Group{}
	}
	if b.Version == "" {
		b.Version = "0.0.0"
	}
	// Return an error if the version is not valid
	_, err := semver.NewVersion(b.Version)
	if err != nil {
		return fmt.Errorf("Invalid 'version', must use Semantic Versioning: %s", err.Error())
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

func (p Package) ToNameVersion() string {
	// Omit version to prevent all packages with prefix of name to be installed
	if p.Version == "*" {
		return p.Name
	}

	return p.Name + "-" + p.Version
}
