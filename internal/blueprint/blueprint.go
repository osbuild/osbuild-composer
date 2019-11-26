// Package blueprint contains primitives for representing weldr blueprints
package blueprint

// An InvalidOutputFormatError is returned when a requested output format is
// not supported. The requested format is included as the error message.
type InvalidOutputFormatError struct {
	message string
}

func (e *InvalidOutputFormatError) Error() string {
	return e.message
}

// A Blueprint is a high-level description of an image.
type Blueprint struct {
	Name           string          `json:"name"`
	Description    string          `json:"description"`
	Version        string          `json:"version,omitempty"`
	Packages       []Package       `json:"packages"`
	Modules        []Package       `json:"modules"`
	Groups         []Group         `json:"groups"`
	Customizations *Customizations `json:"customizations,omitempty"`
}

type Change struct {
	Commit    string    `json:"commit"`
	Message   string    `json:"message"`
	Revision  *string   `json:"revision"`
	Timestamp string    `json:"timestamp"`
	Blueprint Blueprint `json:"-"`
}

// A Package specifies an RPM package.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// A group specifies an package group.
type Group struct {
	Name string `json:"name"`
}

func (b *Blueprint) GetKernelCustomization() *KernelCustomization {
	if b.Customizations == nil {
		return nil
	}

	return b.Customizations.Kernel
}

func (b *Blueprint) GetFirewallCustomization() *FirewallCustomization {
	if b.Customizations == nil {
		return nil
	}

	return b.Customizations.Firewall
}

func (b *Blueprint) GetServicesCustomization() *ServicesCustomization {
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
