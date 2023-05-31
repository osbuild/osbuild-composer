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

// SourceSpec serves as input for ResolveParams, and contains all necessary variables to resolve
// a ref, which can then be turned into a CommitSpec.
type SourceSpec struct {
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

// ImageOptions specify an ostree ref, checksum, URL, ContentURL, and RHSM. The
// meaning of each parameter depends on the image type being built. The type
// is used to specify ostree-related image options when initializing a
// Manifest.
type ImageOptions struct {
	// For ostree commit and container types: The ref of the new commit to be
	// built.
	// For ostree installers and raw images: The ref of the commit being
	// embedded in the installer or deployed in the image.
	ImageRef string

	// For ostree commit and container types: The ParentRef specifies the parent
	// ostree commit that the new commit will be based on.
	// For ostree installers and raw images: The ParentRef does not apply.
	ParentRef string

	// The URL from which to fetch the commit specified by the checksum.
	URL string

	// If specified, the URL will be used only for metadata.
	ContentURL string

	// Indicate if the 'org.osbuild.rhsm.consumer' secret should be added when pulling from the
	// remote.
	RHSM bool
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
	parent := strings.TrimSpace(string(body))
	// Check that this is at least a hex string.
	_, err = hex.DecodeString(parent)
	if err != nil {
		return "", NewResolveRefError("ostree repository %q returned invalid reference", u.String())
	}
	return parent, nil
}

// Resolve the ostree source specification into the necessary ref for the image
// build pipeline and a commit (checksum) to be fetched.
//
// If a URL is defined in the source specification, the checksum of the Parent
// ref is resolved, otherwise the checksum is an empty string. Specifying
// Parent without URL results in a ParameterComboError. Failure to resolve the
// checksum results in a ResolveRefError.
//
// If Parent is not specified in the RequestParams, the value of Ref is used.
//
// If any ref (Ref or Parent) is malformed, the function returns with a RefError.
func Resolve(source SourceSpec) (ref, checksum string, err error) {
	// TODO: return CommitSpec instead and add RHSM option
	ref = source.Ref

	// Determine value of ref
	if !VerifyRef(source.Ref) {
		return "", "", NewRefError("Invalid ostree ref %q", source.Ref)
	}

	// Determine value of parentRef
	parentRef := source.Parent
	if parentRef != "" {
		// verify format of parent ref
		if !VerifyRef(source.Parent) {
			return "", "", NewRefError("Invalid ostree parent ref %q", source.Parent)
		}
		if source.URL == "" {
			// specifying parent ref also requires URL
			return "", "", NewParameterComboError("ostree parent ref specified, but no URL to retrieve it")
		}
	} else {
		// if parent is not provided, use ref
		parentRef = source.Ref
	}

	// Resolve parent checksum
	if source.URL != "" {
		// If a URL is specified, we need to fetch the commit at the URL.
		parent, err := ResolveRef(source.URL, parentRef, source.RHSM, nil, nil)
		if err != nil {
			return "", "", err // ResolveRefError
		}
		checksum = parent
	}
	return
}
