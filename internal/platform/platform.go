package platform

type Arch uint64
type ImageFormat uint64

const (
	ARCH_AARCH64 Arch = iota
	ARCH_PPC64LE
	ARCH_S390X
	ARCH_X86_64
)

const (
	FORMAT_UNSET ImageFormat = iota
	FORMAT_RAW
	FORMAT_ISO
	FORMAT_QCOW2
	FORMAT_VMDK
	FORMAT_VHD
)

func (a Arch) String() string {
	switch a {
	case ARCH_AARCH64:
		return "aarch64"
	case ARCH_PPC64LE:
		return "ppc64le"
	case ARCH_S390X:
		return "s390x"
	case ARCH_X86_64:
		return "x86_64"
	default:
		panic("invalid architecture")
	}
}

func (f ImageFormat) String() string {
	switch f {
	case FORMAT_RAW:
		return "raw"
	case FORMAT_ISO:
		return "iso"
	case FORMAT_QCOW2:
		return "qcow2"
	case FORMAT_VMDK:
		return "vmdk"
	case FORMAT_VHD:
		return "vhd"
	default:
		panic("invalid architecture")
	}
}

type Platform interface {
	GetArch() Arch
	GetImageFormat() ImageFormat
	GetQCOW2Compat() string
	GetBIOSPlatform() string
	GetUEFIVendor() string
	GetZiplSupport() bool
	GetPackages() []string
	GetBuildPackages() []string
}

type BasePlatform struct {
	ImageFormat      ImageFormat
	QCOW2Compat      string
	FirmwarePackages []string
}

func (p BasePlatform) GetImageFormat() ImageFormat {
	return p.ImageFormat
}

func (p BasePlatform) GetQCOW2Compat() string {
	return p.QCOW2Compat
}

func (p BasePlatform) GetBIOSPlatform() string {
	return ""
}

func (p BasePlatform) GetUEFIVendor() string {
	return ""
}

func (p BasePlatform) GetZiplSupport() bool {
	return false
}

func (p BasePlatform) GetPackages() []string {
	return p.FirmwarePackages
}

func (p BasePlatform) GetBuildPackages() []string {
	return []string{}
}
