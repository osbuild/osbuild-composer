package platform

import (
	"encoding/json"
	"fmt"

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
)

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

func (p BasePlatform) GetBootFiles() [][2]string {
	return [][2]string{}
}
