package blueprint

type ISOCustomization struct {
	ApplicationID string `json:"application_id,omitempty" toml:"application_id,omitempty"`
	Publisher     string `json:"publisher,omitempty" toml:"publisher,omitempty"`
	VolumeID      string `json:"volume_id,omitempty" toml:"volume_id,omitempty"`
}
