package osbuild2

import (
	"encoding/json"
	"fmt"
)

// Install the grub2 boot loader for non-UEFI systems or hybrid boot

type Grub2InstStageOptions struct {
	// Filename of the disk image
	Filename string `json:"filename"`

	// Platform of the target system
	Platform string `json:"platform"`

	Location uint64 `json:"location,omitempty"`

	// How to obtain the GRUB core image
	Core CoreMkImage `json:"core"`

	// Location of grub config
	Prefix PrefixPartition `json:"prefix"`

	// Sector size (in bytes)
	SectorSize *uint64 `json:"sector-size,omitempty"`
}

func (Grub2InstStageOptions) isStageOptions() {}

// Generate the core image via grub-mkimage
type CoreMkImage struct {
	Type string `json:"type"`

	PartLabel string `json:"partlabel"`

	Filesystem string `json:"filesystem"`
}

// Grub2 config on a specific partition, e.g. (,gpt3)/boot
type PrefixPartition struct {
	Type string `json:"type"`

	PartLabel string `json:"partlabel"`

	// The partition number, starting at zero
	Number uint `json:"number"`

	// Location of the grub config inside the partition
	Path string `json:"path"`
}

func NewGrub2InstStage(options *Grub2InstStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.grub2.inst",
		Options: options,
	}
}

// alias for custom marshaller
type grub2instStageOptions Grub2InstStageOptions

func (options Grub2InstStageOptions) MarshalJSON() ([]byte, error) {
	g2options := grub2instStageOptions(options)

	valueIn := func(v string, options []string) bool {
		for _, o := range options {
			if v == o {
				return true
			}
		}
		return false
	}

	// verify enum values
	if g2options.Core.Type != "mkimage" {
		return nil, fmt.Errorf("org.osbuild.grub2.inst: invalid value %q for core.type", g2options.Core.Type)
	}
	if !valueIn(g2options.Core.PartLabel, []string{"gpt", "dos"}) {
		return nil, fmt.Errorf("org.osbuild.grub2.inst: invalid value %q for core.partlabel", g2options.Core.PartLabel)
	}
	if !valueIn(g2options.Core.Filesystem, []string{"ext4", "xfs", "btrfs"}) {
		return nil, fmt.Errorf("org.osbuild.grub2.inst: invalid value %q for core.filesystem", g2options.Core.Filesystem)
	}

	if g2options.Prefix.Type != "partition" {
		return nil, fmt.Errorf("org.osbuild.grub2.inst: invalid value %q for prefix.type", g2options.Prefix.Type)
	}
	if !valueIn(g2options.Prefix.PartLabel, []string{"gpt", "dos"}) {
		return nil, fmt.Errorf("org.osbuild.grub2.inst: invalid value %q for core.partlabel", g2options.Core.PartLabel)
	}

	return json.Marshal(g2options)
}
