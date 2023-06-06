package ostree

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/osbuild/osbuild-composer/internal/rhsm"
)

var ostreeRefRE = regexp.MustCompile(`^(?:[\w\d][-._\w\d]*\/)*[\w\d][-._\w\d]*$`)

// SourceSpec serves as input for ResolveParams, and contains all necessary
// variables to resolve a ref, which can then be turned into a CommitSpec.
type SourceSpec struct {
	URL  string
	Ref  string
	RHSM bool
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
func ResolveRef(location, ref string, consumerCerts bool, subs *rhsm.Subscriptions, ca *string) (string, error) {
	u, err := url.Parse(location)
	if err != nil {
		return "", NewResolveRefError(fmt.Sprintf("error parsing ostree repository location: %v", err))
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

		tlsConf := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}

		if ca != nil {
			caCertPEM, err := os.ReadFile(*ca)
			if err != nil {
				return "", NewResolveRefError("error adding rhsm certificates when resolving ref")
			}
			roots := x509.NewCertPool()
			ok := roots.AppendCertsFromPEM(caCertPEM)
			if !ok {
				return "", NewResolveRefError("error adding rhsm certificates when resolving ref")
			}
			tlsConf.RootCAs = roots
		}

		cert, err := tls.LoadX509KeyPair(subs.Consumer.ConsumerCert, subs.Consumer.ConsumerKey)
		if err != nil {
			return "", NewResolveRefError("error adding rhsm certificates when resolving ref")
		}
		tlsConf.Certificates = []tls.Certificate{cert}

		client = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: tlsConf,
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
		return "", NewResolveRefError(fmt.Sprintf("error sending request to ostree repository %q: %v", u.String(), err))
	}
	if resp.StatusCode != http.StatusOK {
		return "", NewResolveRefError("ostree repository %q returned status: %s", u.String(), resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", NewResolveRefError(fmt.Sprintf("error reading response from ostree repository %q: %v", u.String(), err))
	}
	checksum := strings.TrimSpace(string(body))
	// Check that this is at least a hex string.
	_, err = hex.DecodeString(checksum)
	if err != nil {
		return "", NewResolveRefError("ostree repository %q returned invalid reference", u.String())
	}
	return checksum, nil
}

// Resolve the ostree source specification into a  commit specification.
//
// If a URL is defined in the source specification, the checksum of the ref is
// resolved, otherwise the checksum is an empty string. Failure to resolve the
// checksum results in a ResolveRefError.
//
// If the ref is malformed, the function returns with a RefError.
func Resolve(source SourceSpec) (CommitSpec, error) {
	if !VerifyRef(source.Ref) {
		return CommitSpec{}, NewRefError("Invalid ostree ref %q", source.Ref)
	}

	commit := CommitSpec{
		Ref: source.Ref,
		URL: source.URL,
	}
	if source.RHSM {
		commit.Secrets = "org.osbuild.rhsm.consumer"
	}

	// URL set: Resolve checksum
	if source.URL != "" {
		// If a URL is specified, we need to fetch the commit at the URL.
		checksum, err := ResolveRef(source.URL, source.Ref, source.RHSM, nil, nil)
		if err != nil {
			return CommitSpec{}, err // ResolveRefError
		}
		commit.Checksum = checksum
	}
	return commit, nil
}
