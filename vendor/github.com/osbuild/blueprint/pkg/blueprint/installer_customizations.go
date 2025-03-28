package blueprint

type InstallerCustomization struct {
	Unattended   bool             `json:"unattended,omitempty" toml:"unattended,omitempty"`
	SudoNopasswd []string         `json:"sudo-nopasswd,omitempty" toml:"sudo-nopasswd,omitempty"`
	Kickstart    *Kickstart       `json:"kickstart,omitempty" toml:"kickstart,omitempty"`
	Modules      *AnacondaModules `json:"modules,omitempty" toml:"modules,omitempty"`
}

type Kickstart struct {
	Contents string `json:"contents" toml:"contents"`
}

type AnacondaModules struct {
	Enable  []string `json:"enable,omitempty" toml:"enable,omitempty"`
	Disable []string `json:"disable,omitempty" toml:"disable,omitempty"`
}
