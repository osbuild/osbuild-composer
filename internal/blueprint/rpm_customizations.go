package blueprint

type RPMImportKeys struct {
	// File paths in the image to import keys from
	Files []string `json:"files,omitempty" toml:"files,omitempty"`
}

type RPMCustomization struct {
	ImportKeys *RPMImportKeys `json:"import_keys,omitempty" toml:"import_keys,omitempty"`
}
