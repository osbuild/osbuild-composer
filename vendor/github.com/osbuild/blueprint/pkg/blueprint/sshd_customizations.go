package blueprint

type SshdCustomization struct {
	PasswordAuthentication *bool  `json:"password_authentication,omitempty" toml:"password_authentication,omitempty"`
	ClientAliveInterval    *int   `json:"client_alive_interval,omitempty" toml:"client_alive_interval,omitempty"`
	PermitRootLogin        string `json:"permit_root_login,omitempty" toml:"permit_root_login,omitempty"`

	// Note that this is named differently since ChallengeResponseAuthentication (the
	// name in `images`) is a deprecated alias.
	KbdInteractiveAuthentication *bool `json:"kbd_interactive_authentication,omitempty" toml:"kbd_interactive_authentication,omitempty"`
}
