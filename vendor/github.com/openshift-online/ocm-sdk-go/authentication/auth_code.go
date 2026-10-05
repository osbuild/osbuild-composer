package authentication

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	"github.com/openshift-online/ocm-sdk-go/internal"
	"github.com/openshift-online/ocm-sdk-go/logging"
)

// AuthCodeConfig configures the OAuth2 authorization code (PKCE) login flow.
type AuthCodeConfig struct {
	ClientID   string
	logger     logging.Logger
	trustedCAs []interface{}
	insecure   bool
	tokenURL   string
}

// NewAuthCodeConfig creates a new auth code flow configuration with secure TLS defaults.
func NewAuthCodeConfig() *AuthCodeConfig {
	return &AuthCodeConfig{}
}

// Client sets the OAuth2 client identifier.
func (c *AuthCodeConfig) Client(value string) *AuthCodeConfig {
	c.ClientID = value
	return c
}

// Logger sets the logger used when loading trusted CA certificates.
func (c *AuthCodeConfig) Logger(value logging.Logger) *AuthCodeConfig {
	c.logger = value
	return c
}

// TrustedCA sets a source that contains the certificate authorities that will be trusted by the
// HTTP client used for the token exchange. If this isn't explicitly specified then the client will
// trust the certificate authorities trusted by default by the system. The value can be a
// *x509.CertPool or a string; anything else will cause an error when the flow is started. If it is
// a *x509.CertPool then the value will replace any other source given before. If it is a string
// then it should be the name of a PEM file.
func (c *AuthCodeConfig) TrustedCA(value interface{}) *AuthCodeConfig {
	if value != nil {
		c.trustedCAs = append(c.trustedCAs, value)
	}
	return c
}

// TrustedCAs sets certificate authority sources for the token exchange HTTP client. See the
// documentation of the TrustedCA method for more information about the accepted values.
func (c *AuthCodeConfig) TrustedCAs(values ...interface{}) *AuthCodeConfig {
	for _, value := range values {
		c.TrustedCA(value)
	}
	return c
}

// Insecure enables insecure communication with the OpenID server during the token exchange. This
// disables verification of TLS certificates and host names and it isn't recommended for a
// production environment.
func (c *AuthCodeConfig) Insecure(flag bool) *AuthCodeConfig {
	c.insecure = flag
	return c
}

func (c *AuthCodeConfig) resolveLogger() (logging.Logger, error) {
	if c.logger != nil {
		return c.logger, nil
	}
	return logging.NewGoLoggerBuilder().Build()
}

func (c *AuthCodeConfig) oauth2HTTPClient(ctx context.Context) (*http.Client, error) {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: c.insecure, // #nosec G402 -- off by default; opt-in via Insecure
	}
	if len(c.trustedCAs) > 0 {
		logger, err := c.resolveLogger()
		if err != nil {
			return nil, fmt.Errorf("can't build logger: %w", err)
		}
		selector, err := internal.NewClientSelector().
			Logger(logger).
			TrustedCAs(c.trustedCAs...).
			Insecure(c.insecure).
			Build(ctx)
		if err != nil {
			return nil, err
		}
		tlsConfig.RootCAs = selector.TrustedCAs()
		tlsConfig.InsecureSkipVerify = selector.Insecure()
	}
	transport := &http.Transport{
		TLSClientConfig: tlsConfig,
		Proxy:           http.ProxyFromEnvironment,
	}
	return &http.Client{Transport: transport}, nil
}

func (c *AuthCodeConfig) tokenEndpoint() string {
	if c.tokenURL != "" {
		return c.tokenURL
	}
	return DefaultTokenURL
}
