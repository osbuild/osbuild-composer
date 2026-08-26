package osbuild

import (
	"fmt"
	"regexp"
)

const vmdkRegex = "^[a-zA-Z0-9+_.-]*$"
const vhwvRegex = "^vmx-[0-9]+$"

type OVFStageOptions struct {
	Vmdk       string                     `json:"vmdk"`
	VMWare     *OVFVMWareStageOptions     `json:"vmware,omitempty"`
	VirtualBox *OVFVirtualBoxStageOptions `json:"virtualbox,omitempty"`
}

type OVFVMWareStageOptions struct {
	OSType                 string `json:"os_type,omitempty" yaml:"os_type,omitempty"`
	VirtualHardwareVersion string `json:"virtual_hardware_version,omitempty" yaml:"virtual_hardware_version,omitempty"`
}

type OVFVirtualBoxStageOptions struct {
	MacAddress string `json:"mac_address"`
}

func (OVFStageOptions) isStageOptions() {}

func (o OVFStageOptions) validate() error {
	if o.Vmdk == "" {
		return fmt.Errorf("'vmdk' option is empty")
	}

	exp := regexp.MustCompile(vmdkRegex)
	if !exp.MatchString(o.Vmdk) {
		return fmt.Errorf("'vmdk' name %q doesn't conform to schema (%s)", o.Vmdk, exp.String())
	}

	if o.VMWare != nil && o.VMWare.VirtualHardwareVersion != "" {
		exp = regexp.MustCompile(vhwvRegex)
		if !exp.MatchString(o.VMWare.VirtualHardwareVersion) {
			return fmt.Errorf("'vmdk' name %q doesn't conform to schema (%s)", o.Vmdk, exp.String())
		}
	}

	return nil
}

// Generates a file descriptor for an in-tree vmdk file
func NewOVFStage(options *OVFStageOptions) *Stage {
	if err := options.validate(); err != nil {
		panic(err)
	}

	return &Stage{
		Type:    "org.osbuild.ovf",
		Options: options,
	}
}
