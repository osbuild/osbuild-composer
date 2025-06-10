package platform

import (
	"github.com/osbuild/images/pkg/arch"
)

// PlatformConf is a platform configured from YAML inputs
// that implements the "Platform" interface
type PlatformConf struct {
	Arch         arch.Arch   `yaml:"arch"`
	ImageFormat  ImageFormat `yaml:"image_format"`
	QCOW2Compat  string      `yaml:"qcow2_compat"`
	BIOSPlatform string      `yaml:"bios_platform"`
	UEFIVendor   string      `yaml:"uefi_vendor"`
	ZiplSupport  bool        `yaml:"zipl_support"`
	// packages are index by an arbitrary string key to
	// make them YAML mergable, a good key is e.g. "bios"
	// to indicate that these packages are needed for
	// bios support
	Packages      map[string][]string `yaml:"packages"`
	BuildPackages map[string][]string `yaml:"build_packages"`
	BootFiles     [][2]string         `yaml:"boot_files"`

	Bootloader Bootloader `yaml:"bootloader"`
}

// ensure PlatformConf implements the Platform interface
var _ = Platform(&PlatformConf{})

func (pc *PlatformConf) GetArch() arch.Arch {
	return pc.Arch
}
func (pc *PlatformConf) GetImageFormat() ImageFormat {
	return pc.ImageFormat
}
func (pc *PlatformConf) GetQCOW2Compat() string {
	return pc.QCOW2Compat
}
func (pc *PlatformConf) GetBIOSPlatform() string {
	return pc.BIOSPlatform
}
func (pc *PlatformConf) GetUEFIVendor() string {
	return pc.UEFIVendor
}
func (pc *PlatformConf) GetZiplSupport() bool {
	return pc.ZiplSupport
}
func (pc *PlatformConf) GetPackages() []string {
	var merged []string
	for _, pkgList := range pc.Packages {
		merged = append(merged, pkgList...)
	}
	return merged
}
func (pc *PlatformConf) GetBuildPackages() []string {
	var merged []string
	for _, pkgList := range pc.BuildPackages {
		merged = append(merged, pkgList...)
	}
	return merged
}
func (pc *PlatformConf) GetBootFiles() [][2]string {
	return pc.BootFiles
}

func (pc *PlatformConf) GetBootloader() Bootloader {
	return pc.Bootloader
}
