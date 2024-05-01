package subscription

// The ImageOptions specify subscription-specific image options
// ServerUrl denotes the host to register the system with
// BaseUrl specifies the repository URL for DNF
type ImageOptions struct {
	Organization  string `json:"organization"`
	ActivationKey string `json:"activation_key"`
	ServerUrl     string `json:"server_url"`
	BaseUrl       string `json:"base_url"`
	Insights      bool   `json:"insights"`
	Rhc           bool   `json:"rhc"`
}

type RHSMStatus string

const (
	RHSMConfigWithSubscription RHSMStatus = "with-subscription"
	RHSMConfigNoSubscription   RHSMStatus = "no-subscription"
)
