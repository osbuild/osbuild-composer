package osbuild

import (
	"encoding/json"
	"github.com/google/uuid"
)

// GRUB2Legacy is a union which contains either valid boolean or valid string.
//
// In this context "valid" means valid from the code logic point of view.
// There is no undefined value or undefined behavior involved in here.
type GRUB2Legacy struct {
	IsString  bool
	BoolValue bool
	StrValue  string
}

func GRUB2LegacyFromBool(b bool) GRUB2Legacy {
	return GRUB2Legacy{
		IsString:  false,
		BoolValue: b,
		StrValue:  "",
	}
}

func GRUB2LegacyFromString(s string) GRUB2Legacy {
	return GRUB2Legacy{
		IsString:  true,
		BoolValue: false,
		StrValue:  s,
	}
}

func (l GRUB2Legacy) MarshalJSON() ([]byte, error) {
	if l.IsString {
		return json.Marshal(l.StrValue)
	} else {
		return json.Marshal(l.BoolValue)
	}
}

func (l *GRUB2Legacy) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		var b bool
		err = json.Unmarshal(data, &b)
		if err != nil {
			return err
		}
		*l = GRUB2LegacyFromBool(b)
	} else {
		*l = GRUB2LegacyFromString(s)
	}
	return nil
}

// The GRUB2StageOptions describes the bootloader configuration.
//
// The stage is responsible for installing all bootloader files in
// /boot as well as config files in /etc necessary for regenerating
// the configuration in /boot.
//
// Note that it is the role of an assembler to install any necessary
// bootloaders that are stored in the image outside of any filesystem.
type GRUB2StageOptions struct {
	RootFilesystemUUID uuid.UUID   `json:"root_fs_uuid"`
	BootFilesystemUUID *uuid.UUID  `json:"boot_fs_uuid,omitempty"`
	KernelOptions      string      `json:"kernel_opts,omitempty"`
	Legacy             GRUB2Legacy `json:"legacy"`
	UEFI               *GRUB2UEFI  `json:"uefi,omitempty"`
}

type GRUB2UEFI struct {
	Vendor string `json:"vendor"`
}

func (GRUB2StageOptions) isStageOptions() {}

// NewGRUB2Stage creates a new GRUB2 stage object.
func NewGRUB2Stage(options *GRUB2StageOptions) *Stage {
	return &Stage{
		Name:    "org.osbuild.grub2",
		Options: options,
	}
}
