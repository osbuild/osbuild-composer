package platform

import (
	"github.com/osbuild/images/pkg/arch"
)

// Data is a platform configured from YAML inputs
// that implements the "Platform" interface
type Data struct {
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
	FIPSMenu   bool       `yaml:"fips_menu"` // Add FIPS entry to iso bootloader menu
}

// ensure platform.Data implements the Platform interface
var _ = Platform(&Data{})

func (d *Data) GetArch() arch.Arch {
	return d.Arch
}
func (d *Data) GetImageFormat() ImageFormat {
	return d.ImageFormat
}
func (d *Data) GetQCOW2Compat() string {
	return d.QCOW2Compat
}
func (d *Data) GetBIOSPlatform() string {
	return d.BIOSPlatform
}
func (d *Data) GetUEFIVendor() string {
	return d.UEFIVendor
}
func (d *Data) GetZiplSupport() bool {
	return d.ZiplSupport
}
func (d *Data) GetPackages() []string {
	var merged []string
	for _, pkgList := range d.Packages {
		merged = append(merged, pkgList...)
	}
	return merged
}
func (d *Data) GetBuildPackages() []string {
	var merged []string
	for _, pkgList := range d.BuildPackages {
		merged = append(merged, pkgList...)
	}
	return merged
}
func (d *Data) GetBootFiles() [][2]string {
	return d.BootFiles
}

func (d *Data) GetBootloader() Bootloader {
	return d.Bootloader
}

// GetFIPSMenu is used to add the FIPS entry to the iso bootloader menu
func (d *Data) GetFIPSMenu() bool {
	return d.FIPSMenu
}
