package platform

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/arch"
)

type ImageFormat uint64

const ( // image format enum
	FORMAT_UNSET ImageFormat = iota
	FORMAT_RAW
	FORMAT_ISO
	FORMAT_QCOW2
	FORMAT_VMDK
	FORMAT_VHD
	FORMAT_GCE
	FORMAT_OVA
	FORMAT_VAGRANT_LIBVIRT
	FORMAT_VAGRANT_VIRTUALBOX
)

type Bootloader int

const ( // bootloader enum
	BOOTLOADER_NONE Bootloader = iota
	BOOTLOADER_GRUB2
	BOOTLOADER_ZIPL
	BOOTLOADER_UKI
)

func (b *Bootloader) UnmarshalJSON(data []byte) (err error) {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	*b, err = FromString(s)
	return err
}

func (b *Bootloader) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(b, unmarshal)
}

func FromString(b string) (Bootloader, error) {
	// ignore case
	switch strings.ToLower(b) {
	case "grub2":
		return BOOTLOADER_GRUB2, nil
	case "zipl":
		return BOOTLOADER_ZIPL, nil
	case "uki":
		return BOOTLOADER_UKI, nil
	case "", "none":
		return BOOTLOADER_NONE, nil
	default:
		return BOOTLOADER_NONE, fmt.Errorf("unsupported bootloader %q", b)
	}
}

func (f ImageFormat) String() string {
	switch f {
	case FORMAT_UNSET:
		return "unset"
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
	case FORMAT_GCE:
		return "gce"
	case FORMAT_OVA:
		return "ova"
	case FORMAT_VAGRANT_LIBVIRT:
		return "vagrant_libvirt"
	case FORMAT_VAGRANT_VIRTUALBOX:
		return "vagrant_virtualbox"
	default:
		panic(fmt.Errorf("unknown image format %d", f))
	}
}

func (f *ImageFormat) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "unset":
		*f = FORMAT_UNSET
	case "raw":
		*f = FORMAT_RAW
	case "iso":
		*f = FORMAT_ISO
	case "qcow2":
		*f = FORMAT_QCOW2
	case "vmdk":
		*f = FORMAT_VMDK
	case "vhd":
		*f = FORMAT_VHD
	case "gce":
		*f = FORMAT_GCE
	case "ova":
		*f = FORMAT_OVA
	case "vagrant_libvirt":
		*f = FORMAT_VAGRANT_LIBVIRT
	case "vagrant_virtualbox":
		*f = FORMAT_VAGRANT_VIRTUALBOX
	default:
		panic(fmt.Errorf("unknown image format %q", s))
	}
	return nil
}

func (f *ImageFormat) UnmarshalYAML(unmarshal func(any) error) error {
	return common.UnmarshalYAMLviaJSON(f, unmarshal)
}

type Platform interface {
	GetArch() arch.Arch
	GetImageFormat() ImageFormat
	GetQCOW2Compat() string
	GetBIOSPlatform() string
	GetUEFIVendor() string
	GetZiplSupport() bool
	GetPackages() []string
	GetBuildPackages() []string
	GetBootFiles() [][2]string
	GetBootloader() Bootloader
	GetFIPSMenu() bool
}

type BasePlatform struct {
	ImageFormat      ImageFormat
	QCOW2Compat      string
	FirmwarePackages []string
	FIPSMenu         bool // Add FIPS entry to iso bootloader menu
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

func (p BasePlatform) GetBootFiles() [][2]string {
	return [][2]string{}
}

func (p BasePlatform) GetBootloader() Bootloader {
	return BOOTLOADER_NONE
}

// GetFIPSMenu is used to add the FIPS entry to the iso bootloader menu
func (p BasePlatform) GetFIPSMenu() bool {
	return p.FIPSMenu
}
