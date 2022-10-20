package ostree

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/osbuild/osbuild-composer/internal/rhsm"
)

var ostreeRefRE = regexp.MustCompile(`^(?:[\w\d][-._\w\d]*\/)*[\w\d][-._\w\d]*$`)

// RequestParams serves as input for ResolveParams, and contains all necessary variables to resolve
// a ref, which can then be turned into a CommitSpec.
type RequestParams struct {
	URL    string `json:"url"`
	Ref    string `json:"ref"`
	Parent string `json:"parent"`
	RHSM   bool   `json:"rhsm"`
}

// CommitSpec specifies an ostree commit using any combination of Ref (branch), URL (source), and Checksum (commit ID).
type CommitSpec struct {

	// Ref for the commit. Can be empty.
	Ref string

	// URL of the repo where the commit can be fetched, if available.
	URL string

	ContentURL string

	Secrets string

	// Checksum of the commit.
	Checksum string
}

// Remote defines the options that can be set for an OSTree Remote configuration.
type Remote struct {
	Name        string
	URL         string
	ContentURL  string
	GPGKeyPaths []string
}

func VerifyRef(ref string) bool {
	return len(ref) > 0 && ostreeRefRE.MatchString(ref)
}

// ResolveRef resolves the URL path specified by the location and ref
// (location+"refs/heads/"+ref) and returns the commit ID for the named ref. If
// there is an error, it will be of type ResolveRefError.
func ResolveRef(location, ref string, consumerCerts bool, subs *rhsm.Subscriptions) (string, error) {
	u, err := url.Parse(location)
	if err != nil {
		return "", NewResolveRefError(err.Error())
	}
	u.Path = path.Join(u.Path, "refs/heads/", ref)

	var client *http.Client
	if consumerCerts {
		if subs == nil {
			subs, err = rhsm.LoadSystemSubscriptions()
			if subs.Consumer == nil || err != nil {
				return "", NewResolveRefError("error adding rhsm certificates when resolving ref")
			}
		}
		caCertPEM, err := ioutil.ReadFile(subs.Consumer.CACert)
		if err != nil {
			return "", NewResolveRefError("error adding rhsm certificates when resolving ref")
		}

		roots := x509.NewCertPool()
		ok := roots.AppendCertsFromPEM(caCertPEM)
		if !ok {
			return "", NewResolveRefError("error adding rhsm certificates when resolving ref")
		}

		cert, err := tls.LoadX509KeyPair(subs.Consumer.ConsumerCert, subs.Consumer.ConsumerKey)
		if err != nil {
			return "", NewResolveRefError("error adding rhsm certificates when resolving ref")
		}
		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					Certificates: []tls.Certificate{cert},
					RootCAs:      roots,
					MinVersion:   tls.VersionTLS12,
				},
			},
			Timeout: 300 * time.Second,
		}
	} else {
		client = &http.Client{}
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", NewResolveRefError("error adding rhsm certificates when resolving ref")
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", NewResolveRefError(err.Error())
	}
	if resp.StatusCode != http.StatusOK {
		return "", NewResolveRefError("ostree repository %q returned status: %s", u.String(), resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", NewResolveRefError(err.Error())
	}
	parent := strings.TrimSpace(string(body))
	// Check that this is at least a hex string.
	_, err = hex.DecodeString(parent)
	if err != nil {
		return "", NewResolveRefError("ostree repository %q returned invalid reference", u.String())
	}
	return parent, nil
}

// ResolveParams resolves the ostree request parameters into the necessary ref
// for the image build pipeline and a commit (checksum) to be fetched.
//
// If a URL is defined in the RequestParams, the checksum of the Parent ref is
// resolved, otherwise the checksum is an empty string. Specifying Parent
// without URL results in a ParameterComboError. Failure to resolve the
// checksum results in a ResolveRefError.
//
// If Parent is not specified in the RequestParams, the value of Ref is used.
//
// If any ref (Ref or Parent) is malformed, the function returns with a RefError.
func ResolveParams(params RequestParams) (ref, checksum string, err error) {
	ref = params.Ref

	// Determine value of ref
	if !VerifyRef(params.Ref) {
		return "", "", NewRefError("Invalid ostree ref %q", params.Ref)
	}

	// Determine value of parentRef
	parentRef := params.Parent
	if parentRef != "" {
		// verify format of parent ref
		if !VerifyRef(params.Parent) {
			return "", "", NewRefError("Invalid ostree parent ref %q", params.Parent)
		}
		if params.URL == "" {
			// specifying parent ref also requires URL
			return "", "", NewParameterComboError("ostree parent ref specified, but no URL to retrieve it")
		}
	} else {
		// if parent is not provided, use ref
		parentRef = params.Ref
	}

	// Resolve parent checksum
	if params.URL != "" {
		// If a URL is specified, we need to fetch the commit at the URL.
		parent, err := ResolveRef(params.URL, parentRef, params.RHSM, nil)
		if err != nil {
			return "", "", err // ResolveRefError
		}
		checksum = parent
	}
	return
}
