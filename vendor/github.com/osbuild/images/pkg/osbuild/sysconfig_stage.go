package osbuild

type SysconfigStageOptions struct {
	Kernel         *SysconfigKernelOptions  `json:"kernel,omitempty" yaml:"kernel,omitempty"`
	Network        *SysconfigNetworkOptions `json:"network,omitempty" yaml:"network,omitempty"`
	NetworkScripts *NetworkScriptsOptions   `json:"network-scripts,omitempty" yaml:"network-scripts,omitempty"`
	Desktop        *SysconfigDesktopOptions `json:"desktop,omitempty" yaml:"desktop,omitempty"`
	LiveSys        *SysconfigLivesysOptions `json:"livesys,omitempty" yaml:"libesys,omitempty"`
}

func (SysconfigStageOptions) isStageOptions() {}

func NewSysconfigStage(options *SysconfigStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.sysconfig",
		Options: options,
	}
}

type SysconfigNetworkOptions struct {
	Networking bool `json:"networking,omitempty"`
	// XXX: ideally this would be no_zeroconf" (because zeroconf
	// is the program name) but we need to keep for compatibility
	NoZeroConf bool `json:"no_zero_conf,omitempty" yaml:"no_zero_conf,omitempty"`
}

type SysconfigKernelOptions struct {
	UpdateDefault bool   `json:"update_default,omitempty" yaml:"update_default,omitempty"`
	DefaultKernel string `json:"default_kernel,omitempty" yaml:"default_kernel,omitempty"`
}

type SysconfigDesktopOptions struct {
	Preferred      string `json:"preferred,omitempty"`
	DisplayManager string `json:"displaymanager,omitempty"`
}

type SysconfigLivesysOptions struct {
	Session string `json:"session"`
}

type NetworkScriptsOptions struct {
	// Keys are interface names, values are objects containing interface configuration
	IfcfgFiles map[string]IfcfgFile `json:"ifcfg,omitempty"`
}

type IfcfgBootprotoValue string

// Valid values for the 'Bootproto' item of 'IfcfgFile' struct
const (
	IfcfgBootprotoNone   IfcfgBootprotoValue = "none"
	IfcfgBootprotoBootp  IfcfgBootprotoValue = "bootp"
	IfcfgBootprotoDHCP   IfcfgBootprotoValue = "dhcp"
	IfcfgBootprotoStatic IfcfgBootprotoValue = "static"
	IfcfgBootprotoIbft   IfcfgBootprotoValue = "ibft"
	IfcfgBootprotoAutoIP IfcfgBootprotoValue = "autoip"
	IfcfgBootprotoShared IfcfgBootprotoValue = "shared"
)

type IfcfgTypeValue string

// Valid values for the 'Type' item of 'IfcfgFile' struct
const (
	IfcfgTypeEthernet   IfcfgTypeValue = "Ethernet"
	IfcfgTypeWireless   IfcfgTypeValue = "Wireless"
	IfcfgTypeInfiniBand IfcfgTypeValue = "InfiniBand"
	IfcfgTypeBridge     IfcfgTypeValue = "Bridge"
	IfcfgTypeBond       IfcfgTypeValue = "Bond"
	IfcfgTypeVLAN       IfcfgTypeValue = "Vlan"
)

type IfcfgFile struct {
	// Method used for IPv4 protocol configuration
	Bootproto IfcfgBootprotoValue `json:"bootproto,omitempty"`

	// Interface name of the device
	Device string `json:"device,omitempty"`

	// Whether to initialize this device for IPv6 addressing
	IPv6Init *bool `json:"ipv6init,omitempty"`

	// Whether the connection should be autoconnected
	OnBoot *bool `json:"onboot,omitempty"`

	// Whether to modify /etc/resolv.conf
	PeerDNS *bool `json:"peerdns,omitempty"`

	// Base type of the connection
	Type IfcfgTypeValue `json:"type,omitempty"`

	// Whether non-root users are allowed to control the device
	UserCtl *bool `json:"userctl,omitempty"`
}
