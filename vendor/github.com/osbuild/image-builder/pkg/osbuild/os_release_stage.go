package osbuild

type OSReleaseStageOptions struct {
	Path                   string             `json:"path,omitempty"`
	ExtensionReleaseStrict *bool              `json:"extension-release-strict,omitempty"`
	Vars                   OSReleaseStageVars `json:"vars"`
}

func (OSReleaseStageOptions) isStageOptions() {}

type OSReleaseStageVars struct {
	Name                   string `json:"NAME,omitempty"`
	ID                     string `json:"ID,omitempty"`
	IDLike                 string `json:"ID_LIKE,omitempty"`
	Version                string `json:"VERSION,omitempty"`
	VersionID              string `json:"VERSION_ID,omitempty"`
	VersionCodename        string `json:"VERSION_CODENAME,omitempty"`
	PrettyName             string `json:"PRETTY_NAME,omitempty"`
	ANSIColor              string `json:"ANSI_COLOR,omitempty"`
	CPEName                string `json:"CPE_NAME,omitempty"`
	HomeURL                string `json:"HOME_URL,omitempty"`
	DocumentationURL       string `json:"DOCUMENTATION_URL,omitempty"`
	SupportURL             string `json:"SUPPORT_URL,omitempty"`
	BugReportURL           string `json:"BUG_REPORT_URL,omitempty"`
	PrivacyPolicyURL       string `json:"PRIVACY_POLICY_URL,omitempty"`
	BuildID                string `json:"BUILD_ID,omitempty"`
	Variant                string `json:"VARIANT,omitempty"`
	VariantID              string `json:"VARIANT_ID,omitempty"`
	Logo                   string `json:"LOGO,omitempty"`
	ImageID                string `json:"IMAGE_ID,omitempty"`
	ImageVersion           string `json:"IMAGE_VERSION,omitempty"`
	DefaultHostname        string `json:"DEFAULT_HOSTNAME,omitempty"`
	Architecture           string `json:"ARCHITECTURE,omitempty"`
	VendorName             string `json:"VENDOR_NAME,omitempty"`
	VendorURL              string `json:"VENDOR_URL,omitempty"`
	SysextLevel            string `json:"SYSEXT_LEVEL,omitempty"`
	ConfextLevel           string `json:"CONFEXT_LEVEL,omitempty"`
	SysextScope            string `json:"SYSEXT_SCOPE,omitempty"`
	ConfextScope           string `json:"CONFEXT_SCOPE,omitempty"`
	PortablePrefixes       string `json:"PORTABLE_PREFIXES,omitempty"`
	PlatformID             string `json:"PLATFORM_ID,omitempty"`
	SupportEnd             string `json:"SUPPORT_END,omitempty"`
	OstreeVersion          string `json:"OSTREE_VERSION,omitempty"`
	ExtensionReloadManager string `json:"EXTENSION_RELOAD_MANAGER,omitempty"`
	PortableScope          string `json:"PORTABLE_SCOPE,omitempty"`
	ReleaseType            string `json:"RELEASE_TYPE,omitempty"`
}

func NewOSReleaseStage(options *OSReleaseStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.os-release",
		Options: options,
	}
}
