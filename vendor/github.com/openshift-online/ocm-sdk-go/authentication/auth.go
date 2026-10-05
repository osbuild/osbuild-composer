package authentication

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
)

var (
	conf        *oauth2.Config
	exchangeCtx context.Context
	verifier    string
	authToken   string
)

const (
	RedirectURL     = "http://127.0.0.1"
	RedirectPort    = "9998"
	DefaultAuthURL  = "https://sso.redhat.com/auth/realms/redhat-external/protocol/openid-connect/auth"
	CallbackHandler = "/oauth/callback"
)

func callbackHandler(w http.ResponseWriter, r *http.Request) {
	queryParts, _ := url.ParseQuery(r.URL.RawQuery)

	// Use the authorization code that is pushed to the redirect URL
	code := queryParts["code"][0]

	// Exchange will do the handshake to retrieve the initial token.
	tok, err := conf.Exchange(exchangeCtx, code, oauth2.VerifierOption(verifier))
	if err != nil {
		log.Fatal(err)
	}

	// Get the refresh token and ask user to go back to CLI
	authToken = tok.RefreshToken
	_, err = io.WriteString(w, "Login successful! Please close this window and return back to CLI")
	if err != nil {
		log.Fatal(err)
	}
}

func serve(wg *sync.WaitGroup) *http.Server {
	server := &http.Server{Addr: fmt.Sprintf(":%s", RedirectPort)}
	http.HandleFunc(CallbackHandler, callbackHandler)
	go func() {
		defer wg.Done() // let main know we are done cleaning up

		// always returns error. ErrServerClosed on graceful close
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			// unexpected error. port in use?
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// returning reference so caller can call Shutdown()
	return server
}

func shutdown(server *http.Server) {
	if err := server.Shutdown(context.TODO()); err != nil {
		log.Fatalf("HTTP shutdown error: %v", err)
	}
}

// InitiateAuthCode runs the browser-based OAuth2 authorization code (PKCE) flow using the given
// client identifier. TLS certificate verification is enabled by default. Use AuthCodeConfig to
// configure additional trusted CAs or to opt in to insecure TLS for development only.
func InitiateAuthCode(clientID string) (string, error) {
	return NewAuthCodeConfig().
		Client(clientID).
		InitiateAuthCode(context.Background())
}

// InitiateAuthCode runs the browser-based OAuth2 authorization code (PKCE) flow.
func (c *AuthCodeConfig) InitiateAuthCode(ctx context.Context) (string, error) {
	if c.ClientID == "" {
		return "", fmt.Errorf("client identifier is mandatory")
	}

	authToken = ""
	// Create config for OAuth2, redirect to localhost for callback verification and retrieving tokens
	conf = &oauth2.Config{
		ClientID:     c.ClientID,
		ClientSecret: "",
		Scopes:       []string{"openid"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  DefaultAuthURL,
			TokenURL: c.tokenEndpoint(),
		},
		RedirectURL: fmt.Sprintf("%s:%s%s", RedirectURL, RedirectPort, CallbackHandler),
	}
	verifier = oauth2.GenerateVerifier()

	oauthClient, err := c.oauth2HTTPClient(ctx)
	if err != nil {
		return "", err
	}
	exchangeCtx = context.WithValue(ctx, oauth2.HTTPClient, oauthClient)

	// Create URL with PKCE
	url := conf.AuthCodeURL("state", oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier))

	httpServerExitDone := &sync.WaitGroup{}

	httpServerExitDone.Add(1)
	server := serve(httpServerExitDone)

	err = open.Run(url)
	if err != nil {
		return authToken, err
	}
	fiveMinTimer := time.Now().Local().Add(time.Minute * 5)

	// Wait for the user to finish auth process, and return back with authToken. Otherwise, return an error after 5 mins
	for {
		if authToken != "" {
			shutdown(server)
			return authToken, nil
		}
		if time.Now().After(fiveMinTimer) {
			shutdown(server)
			return authToken, fmt.Errorf("time expired")
		}
	}
}
