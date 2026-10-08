package blueprint

import (
	"fmt"

	"regexp"
)

type ISOCustomization struct {
	ApplicationID string `json:"application_id,omitempty" toml:"application_id,omitempty"`
	Publisher     string `json:"publisher,omitempty" toml:"publisher,omitempty"`
	VolumeID      string `json:"volume_id,omitempty" toml:"volume_id,omitempty"`
}

func validateVolumeID(volumeID string) error {
	if regexp.MustCompile(`^[\w\d._$-]+$`).MatchString(volumeID) {
		return nil
	}
	return fmt.Errorf("invalid volume id %q, may contain letters, numbers, -, _, ., and $ only", volumeID)
}
