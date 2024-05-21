package blueprint

type InstallerCustomization struct {
	Unattended   bool       `json:"unattended,omitempty" toml:"unattended,omitempty"`
	SudoNopasswd []string   `json:"sudo-nopasswd,omitempty" toml:"sudo-nopasswd,omitempty"`
	Kickstart    *Kickstart `json:"kickstart,omitempty" toml:"kickstart,omitempty"`
}

type Kickstart struct {
	Contents string `json:"contents" toml:"contents"`
}
