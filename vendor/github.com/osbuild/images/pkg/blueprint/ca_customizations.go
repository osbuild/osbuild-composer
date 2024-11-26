package blueprint

type CACustomization struct {
	PEMCerts []string `json:"pem_certs,omitempty" toml:"pem_certs,omitempty"`
}
