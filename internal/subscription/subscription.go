package subscription

// The SubscriptionImageOptions specify subscription-specific image options
// ServerUrl denotes the host to register the system with
// BaseUrl specifies the repository URL for DNF
type SubscriptionImageOptions struct {
	Organization  string
	ActivationKey string
	ServerUrl     string
	BaseUrl       string
	Insights      bool
	Rhc           bool
}

type RHSMSubscriptionStatus string

const (
	RHSMConfigWithSubscription RHSMSubscriptionStatus = "with-subscription"
	RHSMConfigNoSubscription   RHSMSubscriptionStatus = "no-subscription"
)
