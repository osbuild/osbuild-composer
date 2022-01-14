package ostree

import (
	"encoding/hex"
	"fmt"
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
	if len(ref) > 0 && ostreeRefRE.MatchString(ref) {
		return true
	}
	return false
}

func ResolveRef(location, ref string) (string, error) {
	u, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, "refs/heads/", ref)
	resp, err := http.Get(u.String())
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ostree repository %q returned status: %s", u.String(), resp.Status)
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	parent := strings.TrimSpace(string(body))
	// Check that this is at least a hex string.
	_, err = hex.DecodeString(parent)
	if err != nil {
		return "", fmt.Errorf("ostree repository %q returned invalid reference", u.String())
	}
	return parent, nil
}

func ResolveParams(params RequestParams, defaultRef string) (RequestParams, error) {
	resolved := RequestParams{}
	resolved.Ref = params.Ref
	// if ref is not provided, use distro default
	if resolved.Ref == "" {
		resolved.Ref = defaultRef
	} else if !VerifyRef(params.Ref) { // only verify if specified in params
		return resolved, fmt.Errorf("Invalid ostree ref %q", params.Ref)
	}

	resolved.URL = params.URL
	// Fetch parent ostree commit from ref + url if commit is not
	// provided. The parameter name "parent" is perhaps slightly misleading
	// as it represent whatever commit sha the image type requires, not
	// strictly speaking just the parent commit.
	if resolved.Ref != "" && resolved.URL != "" {
		if params.Parent != "" {
			return resolved, fmt.Errorf("Supply at most one of Parent and URL")
		}

		parent, err := ResolveRef(resolved.URL, resolved.Ref)
		if err != nil {
			return resolved, err
		}
		resolved.Parent = parent
	}
	return resolved, nil
}
