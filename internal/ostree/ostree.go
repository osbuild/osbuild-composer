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
		return resolved, NewInvalidParameterError("Invalid ostree ref %q", params.Ref)
	}

	resolved.URL = params.URL
	// Fetch parent ostree commit from ref + url if commit is not
	// provided. The parameter name "parent" is perhaps slightly misleading
	// as it represent whatever commit sha the image type requires, not
	// strictly speaking just the parent commit.
	if resolved.Ref != "" && resolved.URL != "" {
		if params.Parent != "" {
			return resolved, NewInvalidParameterError("Supply at most one of Parent and URL")
		}

		parent, err := ResolveRef(resolved.URL, resolved.Ref)
		if err != nil {
			return resolved, err // ResolveRefError
		}
		resolved.Parent = parent
	}
	return resolved, nil
}
