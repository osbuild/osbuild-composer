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

type OSTreeRequest struct {
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
