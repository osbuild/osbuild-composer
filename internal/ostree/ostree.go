package ostree

import (
	"encoding/hex"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strings"
)

var ostreeRefRE = regexp.MustCompile(`^(?:[\w\d][-._\w\d]*\/)*[\w\d][-._\w\d]*$`)

type RequestParams struct {
	URL    string `json:"url"`
	Ref    string `json:"ref"`
	Parent string `json:"parent"`
}

// CommitSource defines the source URL from which to fetch a specific commit.
type CommitSource struct {
	Checksum string
	URL      string
}

func VerifyRef(ref string) bool {
	return len(ref) > 0 && ostreeRefRE.MatchString(ref)
}

// ResolveRef resolves the URL path specified by the location and ref
// (location+"refs/heads/"+ref) and returns the commit ID for the named ref. If
// there is an error, it will be of type ResolveRefError.
func ResolveRef(location, ref string) (string, error) {
	u, err := url.Parse(location)
	if err != nil {
		return "", NewResolveRefError(err.Error())
	}
	u.Path = path.Join(u.Path, "refs/heads/", ref)
	resp, err := http.Get(u.String())
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

// ResolveParams resolves all necessary missing parameters in the given struct:
// it sets the defaultRef if none is provided and resolves the parent commit if
// a URL and Ref are provided. If there is an error, it will be of type
// InvalidParameterError or ResolveRefError (from the ResolveRef function)
func ResolveParams(params RequestParams, defaultRef string) (RequestParams, error) {
	resolved := RequestParams{}
	resolved.Ref = params.Ref
	// if ref is not provided, use distro default
	if resolved.Ref == "" {
		resolved.Ref = defaultRef
	} else if !VerifyRef(params.Ref) { // only verify if specified in params
		return resolved, NewRefError("Invalid ostree ref %q", params.Ref)
	}

	if params.Parent != "" {
		// parent must also be a valid ref
		if !VerifyRef(params.Parent) {
			return resolved, NewRefError("Invalid ostree parent ref %q", params.Parent)
		}
		if params.URL == "" {
			// specifying parent ref also requires URL
			return resolved, NewParameterComboError("ostree parent ref specified, but no URL to retrieve it")
		}
	}

	resolved.URL = params.URL
	if resolved.URL != "" {
		// if a URL is specified, we need to fetch the commit at the URL
		// the reference to resolve is the parent commit which is defined by
		// the 'parent' argument
		// if the parent argument is not specified, we use the specified ref
		// if neither is specified, we use the default ref
		parentRef := params.Parent
		if parentRef == "" {
			parentRef = resolved.Ref
		}
		parent, err := ResolveRef(resolved.URL, parentRef)
		if err != nil {
			return resolved, err // ResolveRefError
		}
		resolved.Parent = parent
	}
	return resolved, nil
}
