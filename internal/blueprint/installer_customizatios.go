package blueprint

type InstallerCustomization struct {
	Unattended   bool     `json:"unattended,omitempty" toml:"unattended,omitempty"`
	SudoNopasswd []string `json:"sudo-nopasswd,omitempty" toml:"sudo-nopasswd,omitempty"`
}
