package manifest

import (
	"encoding/json"
	"fmt"

	"github.com/osbuild/images/internal/common"
)

type PayloadLocation uint

const (
	// on the iso is the default, for compatibility
	PAYLOAD_LOCATION_ISO PayloadLocation = iota
	PAYLOAD_LOCATION_ROOTFS
)

func (v PayloadLocation) String() string {
	switch v {
	case PAYLOAD_LOCATION_ISO:
		return "iso"
	case PAYLOAD_LOCATION_ROOTFS:
		return "rootfs"
	default:
		panic(fmt.Sprintf("unknown or unsupported payload location enum value %d", v))
	}
}

func (v *PayloadLocation) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	new, err := NewPayloadLocation(s)
	if err != nil {
		return err
	}
	*v = new
	return nil
}

func (v *PayloadLocation) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(v, unmarshal)
}

func NewPayloadLocation(s string) (PayloadLocation, error) {
	switch s {
	case "iso":
		return PAYLOAD_LOCATION_ISO, nil
	case "rootfs":
		return PAYLOAD_LOCATION_ROOTFS, nil
	default:
		return 0, fmt.Errorf("unknown or unsupported payload location name: %s", s)
	}
}

type PayloadKickstart uint

const (
	// on the iso rootfs is the default, for compatibility
	PAYLOAD_KICKSTART_ROOT PayloadKickstart = iota
	PAYLOAD_KICKSTART_INTERACTIVE_DEFAULTS
)

func (v PayloadKickstart) String() string {
	switch v {
	case PAYLOAD_KICKSTART_ROOT:
		return "root"
	case PAYLOAD_KICKSTART_INTERACTIVE_DEFAULTS:
		return "interactive-defaults"
	default:
		panic(fmt.Sprintf("unknown or unsupported payload kickstart enum value %d", v))
	}
}

func (v *PayloadKickstart) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}

	new, err := NewPayloadKickstart(s)
	if err != nil {
		return err
	}
	*v = new
	return nil
}

func (v *PayloadKickstart) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(v, unmarshal)
}

func NewPayloadKickstart(s string) (PayloadKickstart, error) {
	switch s {
	case "root":
		return PAYLOAD_KICKSTART_ROOT, nil
	case "interactive-defaults":
		return PAYLOAD_KICKSTART_INTERACTIVE_DEFAULTS, nil
	default:
		return 0, fmt.Errorf("unknown or unsupported payload kickstart name: %s", s)
	}
}

// Contains all configuration applied to installer type images such as
// Anaconda or CoreOS installer ones.
type InstallerCustomizations struct {
	FIPS bool

	KernelOptionsAppend []string

	EnabledAnacondaModules  []string
	DisabledAnacondaModules []string

	AdditionalDracutModules []string
	AdditionalDrivers       []string

	// Uses the old, deprecated, Anaconda config option "kickstart-modules".
	// Only for RHEL 8.
	UseLegacyAnacondaConfig bool

	LoraxTemplates       []InstallerLoraxTemplate // Templates to run with org.osbuild.lorax
	LoraxTemplatePackage string                   // Package containing lorax templates, added to build pipeline
	LoraxLogosPackage    string                   // eg. fedora-logos, fedora-eln-logos, redhat-logos
	LoraxReleasePackage  string                   // eg. fedora-release, fedora-release-eln, redhat-release

	// ISOFiles contains files to copy from the `anaconda-tree` to the ISO root, this is
	// used to copy (for example) license and legal information into the root of the ISO. An
	// array of source (in anaconda-tree) and destination (in iso-tree).
	ISOFiles [][2]string

	// Install weak dependencies in the installer environment
	InstallWeakDeps bool

	DefaultMenu int

	Product   string
	Variant   string
	OSVersion string
	Release   string
	Preview   bool

	RPMKeysBinary string

	Payload struct {
		// The path where the payload (tarball, ostree repo, or container) will be stored.
		Path string

		// If set the skopeo stage will remove signatures during copy (relevant for container
		// payloads)
		ContainerRemoveSignatures bool

		Location  PayloadLocation
		Kickstart PayloadKickstart
	}
}

type InstallerLoraxTemplate struct {
	Path string `yaml:"path"`
	// Should this template be executed after dracut? Defaults to not.
	AfterDracut bool `yaml:"after_dracut,omitempty"`
}
