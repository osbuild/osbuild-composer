package platform

type BootMode uint64

const (
	BOOT_NONE BootMode = iota
	BOOT_LEGACY
	BOOT_UEFI
	BOOT_HYBRID
)

func (m BootMode) String() string {
	switch m {
	case BOOT_NONE:
		return "none"
	case BOOT_LEGACY:
		return "legacy"
	case BOOT_UEFI:
		return "uefi"
	case BOOT_HYBRID:
		return "hybrid"
	default:
		panic("invalid boot mode")
	}
}
