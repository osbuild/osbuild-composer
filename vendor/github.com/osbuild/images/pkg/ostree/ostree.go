package ostree

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/osbuild/images/pkg/rhsm"
)

var (
	ostreeRefRE    = regexp.MustCompile(`^(?:[\w\d][-._\w\d]*\/)*[\w\d][-._\w\d]*$`)
	ostreeCommitRE = regexp.MustCompile("^[0-9a-f]{64}$")
)

// SourceSpec serves as input for ResolveParams, and contains all necessary
// variables to resolve a ref, which can then be turned into a CommitSpec.
type SourceSpec struct {
	URL string
	Ref string
	// RHSM indicates to use RHSM secrets when pulling from the remote. Alternatively, you can use MTLS with plain certs.
	RHSM bool
	// MTLS information. Will be ignored if RHSM is set.
	MTLS *MTLS
	// Proxy as HTTP proxy to use when fetching the ref.
	Proxy string
}

// MTLS contains the options for resolving an ostree source.
type MTLS struct {
	CA         string
	ClientCert string
	ClientKey  string
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

// ImageOptions specify an ostree ref, checksum, URL, ContentURL, and RHSM. The
// meaning of each parameter depends on the image type being built. The type
// is used to specify ostree-related image options when initializing a
// Manifest.
type ImageOptions struct {
	// For ostree commit and container types: The ref of the new commit to be
	// built.
	// For ostree installers and raw images: The ref of the commit being
	// embedded in the installer or deployed in the image.
	ImageRef string `json:"ref"`

	// For ostree commit and container types: The ParentRef specifies the parent
	// ostree commit that the new commit will be based on.
	// For ostree installers and raw images: The ParentRef does not apply.
	ParentRef string `json:"parent"`

	// The URL from which to fetch the commit specified by the checksum.
	URL string `json:"url"`

	// If specified, the URL will be used only for metadata.
	ContentURL string `json:"contenturl"`

	// Indicate if the 'org.osbuild.rhsm.consumer' secret should be added when pulling from the
	// remote.
	RHSM bool `json:"rhsm"`
}

// Validate the image options. This doesn't verify the existence of any remote
// objects and does not guarantee that refs will be successfully resolved. It
// only checks that the values and value combinations are valid.
//
// The function checks the following:
// - The ImageRef, if specified, is a valid ref and does not look like a
// checksum.
// - The ParentRef, if specified, must be a valid ref or a checksum.
// - If the ParentRef is specified, the URL must also be specified.
// - URLs must be valid.
func (options ImageOptions) Validate() error {
	if ref := options.ImageRef; ref != "" {
		// image ref must not look like a checksum
		if verifyChecksum(ref) {
			return NewRefError("ostree image ref looks like a checksum %q", ref)
		}
		if !verifyRef(ref) {
			return NewRefError("invalid ostree image ref %q", ref)
		}
	}

	if parent := options.ParentRef; parent != "" {
		if !verifyChecksum(parent) && !verifyRef(parent) {
			return NewRefError("invalid ostree parent ref or commit %q", parent)
		}

		// valid URL required
		if purl := options.URL; purl == "" {
			return NewParameterComboError("ostree parent ref specified, but no URL to retrieve it")
		}
	}

	// whether required or not, any URL specified must be valid
	if purl := options.URL; purl != "" {
		if _, err := url.ParseRequestURI(purl); err != nil {
			return fmt.Errorf("ostree URL %q is invalid", purl)
		}
	}

	if curl := options.ContentURL; curl != "" {
		if _, err := url.ParseRequestURI(curl); err != nil {
			return fmt.Errorf("ostree content URL %q is invalid", curl)
		}
	}

	return nil
}

// Remote defines the options that can be set for an OSTree Remote configuration.
type Remote struct {
	Name        string
	URL         string
	ContentURL  string
	GPGKeyPaths []string
}

func verifyRef(ref string) bool {
	return len(ref) > 0 && ostreeRefRE.MatchString(ref)
}

func verifyChecksum(commit string) bool {
	return len(commit) > 0 && ostreeCommitRE.MatchString(commit)
}

func httpClientForRef(scheme string, ss SourceSpec) (*http.Client, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if scheme == "https" {
		tlsConf := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		// If CA is set, load the CA certificate and add it to the TLS configuration. Otherwise, use the system CA.
		if ss.MTLS != nil && ss.MTLS.CA != "" {
			caCertPEM, err := os.ReadFile(ss.MTLS.CA)
			if err != nil {
				return nil, NewResolveRefError("error adding ca certificate when resolving ref: %s", err)
			}
			tlsConf.RootCAs = x509.NewCertPool()
			if ok := tlsConf.RootCAs.AppendCertsFromPEM(caCertPEM); !ok {
				return nil, NewResolveRefError("error adding ca certificate when resolving ref")
			}
		}

		if ss.MTLS != nil && ss.MTLS.ClientCert != "" && ss.MTLS.ClientKey != "" {
			cert, err := tls.LoadX509KeyPair(ss.MTLS.ClientCert, ss.MTLS.ClientKey)
			if err != nil {
				return nil, NewResolveRefError("error adding client certificate when resolving ref: %s", err)
			}
			tlsConf.Certificates = []tls.Certificate{cert}
		}

		transport.TLSClientConfig = tlsConf
	}

	if ss.Proxy != "" {
		host, port, err := net.SplitHostPort(ss.Proxy)
		if err != nil {
			return nil, NewResolveRefError("error parsing MTLS proxy URL '%s': %v", ss.URL, err)
		}

		proxyURL, err := url.Parse("http://" + host + ":" + port)
		if err != nil {
			return nil, NewResolveRefError("error parsing MTLS proxy URL '%s': %v", ss.URL, err)
		}

		transport.Proxy = func(request *http.Request) (*url.URL, error) {
			return proxyURL, nil
		}
	}

	return &http.Client{
		Transport: transport,
		Timeout:   300 * time.Second,
	}, nil
}

// resolveRef resolves the URL path specified by the location and ref
// (location+"refs/heads/"+ref) and returns the commit ID for the named ref. If
// there is an error, it will be of type ResolveRefError.
func resolveRef(ss SourceSpec) (string, error) {
	u, err := url.Parse(ss.URL)
	if err != nil {
		return "", NewResolveRefError("error parsing ostree repository location: %v", err)
	}
	u.Path = path.Join(u.Path, "refs", "heads", ss.Ref)

	client, err := httpClientForRef(u.Scheme, ss)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return "", NewResolveRefError("error preparing ostree resolve request: %s", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", NewResolveRefError("error sending request to ostree repository %q: %v", u.String(), err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", NewResolveRefError("ostree repository %q returned status: %s", u.String(), resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", NewResolveRefError("error reading response from ostree repository %q: %v", u.String(), err)
	}
	checksum := strings.TrimSpace(string(body))
	// Check that this is at least a hex string.
	_, err = hex.DecodeString(checksum)
	if err != nil {
		return "", NewResolveRefError("ostree repository %q returned invalid reference", u.String())
	}
	return checksum, nil
}

// Resolve the ostree source specification to a commit specification.
//
// If a URL is defined in the source specification, the checksum of the ref is
// resolved, otherwise the checksum is an empty string. Failure to resolve the
// checksum results in a ResolveRefError.
//
// If the ref is already a checksum (64 alphanumeric characters), it is not
// resolved or checked against the repository.
//
// If the ref is malformed, the function returns with a RefError.
func Resolve(source SourceSpec) (CommitSpec, error) {
	commit := CommitSpec{
		Ref: source.Ref,
		URL: source.URL,
	}

	if source.RHSM && source.MTLS != nil {
		return commit, NewResolveRefError("cannot use both RHSM and MTLS when resolving ref")
	}

	if source.RHSM {
		var subs *rhsm.Subscriptions
		var err error

		commit.Secrets = "org.osbuild.rhsm.consumer"
		subs, err = rhsm.LoadSystemSubscriptions()

		if err != nil {
			return commit, NewResolveRefError("error adding rhsm certificates when resolving ref: %s", err)
		}

		if subs.Consumer == nil {
			return commit, NewResolveRefError("error adding rhsm certificates when resolving ref")
		}

		source.MTLS = &MTLS{
			ClientCert: subs.Consumer.ConsumerCert,
			ClientKey:  subs.Consumer.ConsumerKey,
		}
	} else if source.MTLS != nil {
		commit.Secrets = "org.osbuild.mtls"
	}

	if verifyChecksum(source.Ref) {
		// the ref is a commit: return as is
		commit.Checksum = source.Ref
		return commit, nil
	}

	if !verifyRef(source.Ref) {
		// the ref is not a commit and it's also an invalid ref
		return CommitSpec{}, NewRefError("Invalid ostree ref or commit %q", source.Ref)
	}

	// URL set: Resolve checksum
	if source.URL != "" {
		// If a URL is specified, we need to fetch the commit at the URL.
		checksum, err := resolveRef(source)
		if err != nil {
			return CommitSpec{}, err // ResolveRefError
		}
		commit.Checksum = checksum
	}
	return commit, nil
}
