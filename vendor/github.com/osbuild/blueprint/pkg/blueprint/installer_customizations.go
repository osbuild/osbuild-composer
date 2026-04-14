package blueprint

type InstallerCustomization struct {
	Unattended   bool                 `json:"unattended,omitempty" toml:"unattended,omitempty"`
	SudoNopasswd []string             `json:"sudo-nopasswd,omitempty" toml:"sudo-nopasswd,omitempty"`
	Kickstart    *Kickstart           `json:"kickstart,omitempty" toml:"kickstart,omitempty"`
	Modules      *AnacondaModules     `json:"modules,omitempty" toml:"modules,omitempty"`
	Bootloader   *InstallerBootloader `json:"bootloader,omitempty" toml:"bootloader,omitempty"`
	Payload      *InstallerPayload    `json:"payload,omitempty" toml:"payload,omitempty"`
}

type Kickstart struct {
	Contents string `json:"contents" toml:"contents"`
}

type AnacondaModules struct {
	Enable  []string `json:"enable,omitempty" toml:"enable,omitempty"`
	Disable []string `json:"disable,omitempty" toml:"disable,omitempty"`
}

type InstallerBootloader struct {
	Grub2 *InstallerGrub2 `json:"grub2,omitempty" toml:"grub2,omitempty"`
}

type InstallerGrub2 struct {
	MenuTimeout *int `json:"menu-timeout,omitempty" toml:"menu-timeout,omitempty"`
}

type InstallerPayload struct {
	Flatpaks *FlatpakMeta `json:"flatpaks,omitempty" toml:"flatpaks,omitempty"`
}

type FlatpakMeta struct {
	Force []Flatpak `json:"force,omitempty" toml:"force,omitempty"`
}

type Flatpak struct {
	Registry   *FlatpakRegistry `json:"registry,omitempty" toml:"registry,omitempty"`
	References []string         `json:"references,omitempty" toml:"references,omitempty"`
}

type FlatpakRegistry struct {
	RemoteName string `json:"remote_name,omitempty" toml:"remote_name,omitempty"`
	URL        string `json:"url,omitempty" toml:"url,omitempty"`
}
